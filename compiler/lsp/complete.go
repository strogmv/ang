package lsp

import (
	"fmt"
	"regexp"
	"strings"

	sharedeffects "github.com/strogmv/ang/compiler/effects"
	"github.com/strogmv/ang/compiler/flowfn"
	"github.com/strogmv/ang/compiler/flowsem"
)

func CompletionItems(text string, pos Position) []CompletionItem {
	ctx := completionContext(text, pos)
	catalog := flowsem.ActionCatalog()
	items := make([]CompletionItem, 0, len(catalog))
	for _, entry := range catalog {
		detail := "ANG flow action"
		if entry.Effect != "" {
			detail = fmt.Sprintf("ANG flow action · effect=%s", entry.Effect)
		}
		unavailableReason := actionUnavailableReason(entry, ctx)
		deprecated := unavailableReason != ""
		if deprecated {
			detail = "Unavailable here"
			if entry.Effect != "" {
				detail += fmt.Sprintf(" · effect=%s", entry.Effect)
			}
			detail += " · " + unavailableReason
		}
		items = append(items, CompletionItem{
			Label:      entry.Name,
			Detail:     detail,
			InsertText: entry.Name,
			Deprecated: deprecated,
			SortText:   completionSortKey(entry, ctx, deprecated),
		})
	}
	sortCompletionItems(items)
	return items
}

func completionContext(text string, pos Position) CompletionContext {
	prefix := prefixBeforeCursor(text, pos)
	ctx := CompletionContext{
		Tags: map[string]bool{},
	}
	steps, _, err := flowfn.ParseValidateTranspile(trimToCompleteStatements(prefix))
	if err == nil {
		applyCompletionState(ctx.Tags, steps)
	}
	ctx.InTx = hasOpenTxBlock(prefix)
	if ctx.InTx {
		ctx.Tags[string(sharedeffects.ProduceTxOpen)] = true
	}
	ctx.Inside = currentBlockName(prefix)
	return ctx
}

func isActionAvailable(entry flowsem.ActionCatalogEntry, ctx CompletionContext) bool {
	return actionUnavailableReason(entry, ctx) == ""
}

func actionUnavailableReason(entry flowsem.ActionCatalogEntry, ctx CompletionContext) string {
	if entry.RequiresTx && !ctx.InTx {
		return "requires tx.Block"
	}
	if ctx.InTx && entry.Effect != "" && !entry.TxCompatible {
		return "not allowed inside tx.Block"
	}
	missing := make([]string, 0, len(entry.RequiresTags))
	for _, tag := range entry.RequiresTags {
		if !ctx.Tags[tag] {
			missing = append(missing, tag)
		}
	}
	if len(missing) > 0 {
		return "missing " + strings.Join(missing, ", ")
	}
	return ""
}

func completionSortKey(entry flowsem.ActionCatalogEntry, ctx CompletionContext, deprecated bool) string {
	score := 50
	if entry.Effect == "" {
		score = 10
	}
	if len(entry.RequiresTags) == 0 {
		score -= 2
	}
	if ctx.InTx && entry.RequiresTx {
		score -= 3
	}
	if deprecated {
		score += 900
	}
	return fmt.Sprintf("%03d_%s", score, entry.Name)
}

func applyCompletionState(tags map[string]bool, steps []flowfn.Step) {
	for _, step := range steps {
		logos, ok := flowsem.LookupLogos(step.Action)
		if !ok {
			continue
		}
		for _, tag := range logos.ProducesTags {
			tags[string(tag)] = true
		}
		for _, children := range step.Children {
			applyCompletionState(tags, children)
		}
	}
}

func prefixBeforeCursor(text string, pos Position) string {
	lines := strings.Split(text, "\n")
	if pos.Line < 0 {
		return ""
	}
	if pos.Line >= len(lines) {
		return text
	}
	var b strings.Builder
	for i := 0; i < pos.Line; i++ {
		b.WriteString(lines[i])
		b.WriteString("\n")
	}
	line := lines[pos.Line]
	char := pos.Character
	if char < 0 {
		char = 0
	}
	if char > len(line) {
		char = len(line)
	}
	b.WriteString(line[:char])
	return b.String()
}

func trimToCompleteStatements(prefix string) string {
	if prefix == "" {
		return ""
	}
	idx := strings.LastIndex(prefix, "\n")
	if idx < 0 {
		return ""
	}
	return prefix[:idx+1]
}

var blockOpenRe = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_.]*|if|for|try|catch|else)\b.*\{\s*$`)

func hasOpenTxBlock(prefix string) bool {
	stack := blockStack(prefix)
	for _, name := range stack {
		if name == "tx.Block" {
			return true
		}
	}
	return false
}

func currentBlockName(prefix string) string {
	stack := blockStack(prefix)
	if len(stack) == 0 {
		return ""
	}
	return stack[len(stack)-1]
}

func blockStack(prefix string) []string {
	lines := strings.Split(prefix, "\n")
	stack := make([]string, 0, 8)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "}") {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		if m := blockOpenRe.FindStringSubmatch(line); len(m) == 2 {
			stack = append(stack, m[1])
		}
	}
	return stack
}
