package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// MemoryFact is a single durable fact the agent chose to remember across
// sessions. We store *conclusions*, never raw transcripts: facts that are not
// re-derivable from CUE intent or git (decisions, rationale, rejected
// alternatives, cross-session goals). Anything obtainable from `cue/` in one
// read should be fetched live instead of stored here.
type MemoryFact struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"` // fact | decision | learning
	Text       string   `json:"text"`
	Rationale  string   `json:"rationale,omitempty"` // required for decisions
	Tags       []string `json:"tags,omitempty"`
	CueRef     string   `json:"cue_ref,omitempty"` // e.g. cue/api/blog.cue#SubmitPost
	Stale      bool     `json:"stale,omitempty"`
	Supersedes string   `json:"supersedes,omitempty"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at,omitempty"`
}

// memoryMu guards the on-disk store. The MCP server is a single stdio process,
// but tool handlers may be invoked concurrently.
var memoryMu sync.Mutex

// memoryStorePath returns the JSON store under the writable cue/ root so memory
// is versioned and reviewed alongside the intent it explains.
func memoryStorePath() string {
	return filepath.Join(resolveMCPCueRoot("."), ".memory", "facts.json")
}

func loadMemory() ([]MemoryFact, error) {
	data, err := os.ReadFile(memoryStorePath())
	if os.IsNotExist(err) {
		return []MemoryFact{}, nil
	}
	if err != nil {
		return nil, err
	}
	var facts []MemoryFact
	if len(strings.TrimSpace(string(data))) == 0 {
		return []MemoryFact{}, nil
	}
	if err := json.Unmarshal(data, &facts); err != nil {
		return nil, fmt.Errorf("corrupt memory store: %w", err)
	}
	return facts, nil
}

func saveMemory(facts []MemoryFact) error {
	path := memoryStorePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(facts, "", "  ")
	if err != nil {
		return err
	}
	// Write atomically so a crash mid-write can't truncate the store.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func newFactID() string {
	var buf [4]byte
	_, _ = rand.Read(buf[:])
	return "f-" + time.Now().UTC().Format("20060102T150405") + "-" + hex.EncodeToString(buf[:])
}

func parseTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// normalizeForDedup lowercases and collapses whitespace so trivially-different
// phrasings of the same fact are caught before we store a duplicate.
func normalizeForDedup(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// findDuplicate returns a live fact whose text is effectively identical, so the
// agent updates it (supersede) instead of appending a contradicting twin —
// mirroring "check for an existing entry that already covers it".
func findDuplicate(facts []MemoryFact, text string) *MemoryFact {
	norm := normalizeForDedup(text)
	for i := range facts {
		if facts[i].Stale {
			continue
		}
		if normalizeForDedup(facts[i].Text) == norm {
			return &facts[i]
		}
	}
	return nil
}

func memoryError(msg string) *mcp.CallToolResult {
	b, _ := json.MarshalIndent(map[string]any{"status": "failed", "error": msg}, "", "  ")
	return mcp.NewToolResultText(string(b))
}

func memoryOK(payload map[string]any) *mcp.CallToolResult {
	payload["status"] = "ok"
	b, _ := json.MarshalIndent(payload, "", "  ")
	return mcp.NewToolResultText(string(b))
}

// memoryDigest renders a compact, newest-first index of live facts as Markdown.
// It is exposed as an MCP resource so the client can load it at session start
// the way an index file is always in context — passive recall, no tool call.
func memoryDigest() (string, error) {
	facts, err := loadMemory()
	if err != nil {
		return "", err
	}
	live := make([]MemoryFact, 0, len(facts))
	for _, f := range facts {
		if !f.Stale {
			live = append(live, f)
		}
	}
	sort.Slice(live, func(i, j int) bool { return live[i].CreatedAt > live[j].CreatedAt })

	var b strings.Builder
	b.WriteString("# ANG Memory\n\n")
	if len(live) == 0 {
		b.WriteString("_No stored facts yet. Use ang_memory_append / ang_memory_decision to remember durable conclusions (not transcripts, not anything re-derivable from CUE)._\n")
		return b.String(), nil
	}
	b.WriteString(fmt.Sprintf("_%d fact(s). Recall with ang_memory_search; correct with ang_memory_supersede; retire with ang_memory_forget._\n\n", len(live)))
	for _, f := range live {
		date := f.CreatedAt
		if len(date) >= 10 {
			date = date[:10]
		}
		marker := "•"
		if f.Kind == "decision" {
			marker = "◆"
		}
		b.WriteString(fmt.Sprintf("%s **%s** (`%s`, %s", marker, f.Text, f.ID, date))
		if f.CueRef != "" {
			b.WriteString(", " + f.CueRef)
		}
		if len(f.Tags) > 0 {
			b.WriteString(", #" + strings.Join(f.Tags, " #"))
		}
		b.WriteString(")\n")
		if f.Rationale != "" {
			b.WriteString("  - why: " + f.Rationale + "\n")
		}
	}
	return b.String(), nil
}

func registerMemoryTools(addTool toolAdder) {
	addTool("ang_memory_append", mcp.NewTool("ang_memory_append",
		mcp.WithDescription("Remember a durable fact across sessions. Store CONCLUSIONS, not transcripts. "+
			"Do NOT store anything re-derivable from CUE intent or git (current branch, file contents, "+
			"schema) — fetch those live. Good facts: constraints, cross-session goals, gotchas. "+
			"Link to the intent it explains via cue_ref."),
		mcp.WithString("text", mcp.Required(), mcp.Description("The fact, phrased so it is useful in a future session.")),
		mcp.WithString("tags", mcp.Description("Comma-separated tags for retrieval, e.g. \"auth,cookies\".")),
		mcp.WithString("cue_ref", mcp.Description("Intent this fact explains, e.g. cue/api/blog.cue#SubmitPost.")),
		mcp.WithBoolean("force", mcp.Description("Store even if a near-identical fact already exists (default false).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		text := strings.TrimSpace(mcp.ParseString(request, "text", ""))
		if text == "" {
			return memoryError("text is required"), nil
		}
		memoryMu.Lock()
		defer memoryMu.Unlock()
		facts, err := loadMemory()
		if err != nil {
			return memoryError(err.Error()), nil
		}
		if !mcp.ParseBoolean(request, "force", false) {
			if dup := findDuplicate(facts, text); dup != nil {
				return memoryOK(map[string]any{
					"deduped":  true,
					"existing": dup.ID,
					"hint":     "near-identical fact exists; use ang_memory_supersede to update it, or force=true to add anyway",
				}), nil
			}
		}
		f := MemoryFact{
			ID:        newFactID(),
			Kind:      "fact",
			Text:      text,
			Tags:      parseTags(mcp.ParseString(request, "tags", "")),
			CueRef:    strings.TrimSpace(mcp.ParseString(request, "cue_ref", "")),
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		facts = append(facts, f)
		if err := saveMemory(facts); err != nil {
			return memoryError(err.Error()), nil
		}
		return memoryOK(map[string]any{"id": f.ID, "stored": f}), nil
	})

	addTool("ang_memory_decision", mcp.NewTool("ang_memory_decision",
		mcp.WithDescription("Record a decision together with its rationale. A decision without a rationale is "+
			"useless next session — you can't tell whether it's safe to revisit. Also record rejected "+
			"alternatives in the rationale so the same option isn't re-litigated."),
		mcp.WithString("decision", mcp.Required(), mcp.Description("What was decided.")),
		mcp.WithString("rationale", mcp.Required(), mcp.Description("Why — including alternatives considered and why they were rejected.")),
		mcp.WithString("tags", mcp.Description("Comma-separated tags for retrieval.")),
		mcp.WithString("cue_ref", mcp.Description("Intent this decision affects, e.g. cue/api/blog.cue#SubmitPost.")),
		mcp.WithBoolean("force", mcp.Description("Store even if a near-identical decision already exists (default false).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		decision := strings.TrimSpace(mcp.ParseString(request, "decision", ""))
		rationale := strings.TrimSpace(mcp.ParseString(request, "rationale", ""))
		if decision == "" {
			return memoryError("decision is required"), nil
		}
		if rationale == "" {
			return memoryError("rationale is required for a decision"), nil
		}
		memoryMu.Lock()
		defer memoryMu.Unlock()
		facts, err := loadMemory()
		if err != nil {
			return memoryError(err.Error()), nil
		}
		if !mcp.ParseBoolean(request, "force", false) {
			if dup := findDuplicate(facts, decision); dup != nil {
				return memoryOK(map[string]any{
					"deduped":  true,
					"existing": dup.ID,
					"hint":     "near-identical decision exists; use ang_memory_supersede to update it, or force=true to add anyway",
				}), nil
			}
		}
		f := MemoryFact{
			ID:        newFactID(),
			Kind:      "decision",
			Text:      decision,
			Rationale: rationale,
			Tags:      parseTags(mcp.ParseString(request, "tags", "")),
			CueRef:    strings.TrimSpace(mcp.ParseString(request, "cue_ref", "")),
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		facts = append(facts, f)
		if err := saveMemory(facts); err != nil {
			return memoryError(err.Error()), nil
		}
		return memoryOK(map[string]any{"id": f.ID, "stored": f}), nil
	})

	addTool("ang_memory_supersede", mcp.NewTool("ang_memory_supersede",
		mcp.WithDescription("Replace an existing fact with a corrected version instead of appending a second, "+
			"contradicting one. Keeps the store consistent so search doesn't return stale truths."),
		mcp.WithString("id", mcp.Required(), mcp.Description("ID of the fact to replace.")),
		mcp.WithString("text", mcp.Required(), mcp.Description("The corrected fact/decision text.")),
		mcp.WithString("rationale", mcp.Description("Updated rationale (kept for decisions).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := strings.TrimSpace(mcp.ParseString(request, "id", ""))
		text := strings.TrimSpace(mcp.ParseString(request, "text", ""))
		if id == "" || text == "" {
			return memoryError("id and text are required"), nil
		}
		memoryMu.Lock()
		defer memoryMu.Unlock()
		facts, err := loadMemory()
		if err != nil {
			return memoryError(err.Error()), nil
		}
		var old *MemoryFact
		kept := facts[:0]
		for i := range facts {
			if facts[i].ID == id {
				cp := facts[i]
				old = &cp
				continue
			}
			kept = append(kept, facts[i])
		}
		if old == nil {
			return memoryError("no fact with id " + id), nil
		}
		now := time.Now().UTC().Format(time.RFC3339)
		rationale := strings.TrimSpace(mcp.ParseString(request, "rationale", ""))
		if rationale == "" {
			rationale = old.Rationale
		}
		f := MemoryFact{
			ID:         newFactID(),
			Kind:       old.Kind,
			Text:       text,
			Rationale:  rationale,
			Tags:       old.Tags,
			CueRef:     old.CueRef,
			Supersedes: old.ID,
			CreatedAt:  old.CreatedAt,
			UpdatedAt:  now,
		}
		kept = append(kept, f)
		if err := saveMemory(kept); err != nil {
			return memoryError(err.Error()), nil
		}
		return memoryOK(map[string]any{"id": f.ID, "superseded": old.ID, "stored": f}), nil
	})

	addTool("ang_memory_forget", mcp.NewTool("ang_memory_forget",
		mcp.WithDescription("Drop a fact (delete=true, when it was wrong) or soft-retire it (default: mark stale, "+
			"keeping the record but excluding it from search). Use this to keep the store from rotting."),
		mcp.WithString("id", mcp.Required(), mcp.Description("ID of the fact.")),
		mcp.WithBoolean("delete", mcp.Description("Hard-delete instead of marking stale (default false).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := strings.TrimSpace(mcp.ParseString(request, "id", ""))
		if id == "" {
			return memoryError("id is required"), nil
		}
		hardDelete := mcp.ParseBoolean(request, "delete", false)
		memoryMu.Lock()
		defer memoryMu.Unlock()
		facts, err := loadMemory()
		if err != nil {
			return memoryError(err.Error()), nil
		}
		found := false
		out := facts[:0]
		for i := range facts {
			if facts[i].ID == id {
				found = true
				if hardDelete {
					continue
				}
				facts[i].Stale = true
				facts[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			}
			out = append(out, facts[i])
		}
		if !found {
			return memoryError("no fact with id " + id), nil
		}
		if err := saveMemory(out); err != nil {
			return memoryError(err.Error()), nil
		}
		action := "marked_stale"
		if hardDelete {
			action = "deleted"
		}
		return memoryOK(map[string]any{"id": id, "action": action}), nil
	})

	addTool("ang_memory_search", mcp.NewTool("ang_memory_search",
		mcp.WithDescription("Recall facts at the start of a session or when context was replaced. Filters by "+
			"free-text query, tags and/or cue_ref. Excludes stale facts unless include_stale=true. "+
			"Results are sorted newest-first and include dates so you can judge freshness."),
		mcp.WithString("query", mcp.Description("Free-text substring matched against text and rationale.")),
		mcp.WithString("tags", mcp.Description("Comma-separated tags; a fact matches if it has any of them.")),
		mcp.WithString("cue_ref", mcp.Description("Return facts attached to this intent ref (prefix match).")),
		mcp.WithBoolean("include_stale", mcp.Description("Include retired facts (default false).")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := strings.ToLower(strings.TrimSpace(mcp.ParseString(request, "query", "")))
		wantTags := parseTags(mcp.ParseString(request, "tags", ""))
		cueRef := strings.TrimSpace(mcp.ParseString(request, "cue_ref", ""))
		includeStale := mcp.ParseBoolean(request, "include_stale", false)

		memoryMu.Lock()
		facts, err := loadMemory()
		memoryMu.Unlock()
		if err != nil {
			return memoryError(err.Error()), nil
		}

		matched := make([]MemoryFact, 0, len(facts))
		for _, f := range facts {
			if f.Stale && !includeStale {
				continue
			}
			if query != "" &&
				!strings.Contains(strings.ToLower(f.Text), query) &&
				!strings.Contains(strings.ToLower(f.Rationale), query) {
				continue
			}
			if cueRef != "" && !strings.HasPrefix(f.CueRef, cueRef) {
				continue
			}
			if len(wantTags) > 0 && !hasAnyTag(f.Tags, wantTags) {
				continue
			}
			matched = append(matched, f)
		}
		sort.Slice(matched, func(i, j int) bool {
			return matched[i].CreatedAt > matched[j].CreatedAt
		})
		return memoryOK(map[string]any{"count": len(matched), "facts": matched}), nil
	})
}

func hasAnyTag(have, want []string) bool {
	for _, w := range want {
		for _, h := range have {
			if strings.EqualFold(h, w) {
				return true
			}
		}
	}
	return false
}
