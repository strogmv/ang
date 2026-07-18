package ppfacts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

// Canonicalize returns a deterministically ordered copy of env.
func Canonicalize(env Envelope) Envelope {
	out := env
	out.Facts = append([]Fact(nil), env.Facts...)
	for i := range out.Facts {
		out.Facts[i].Terms = append([]Term(nil), env.Facts[i].Terms...)
		out.Facts[i].EvidenceIDs = sortedUniqueStrings(env.Facts[i].EvidenceIDs)
	}
	sort.SliceStable(out.Facts, func(i, j int) bool {
		return out.Facts[i].ID < out.Facts[j].ID
	})

	out.Evidence = append([]Evidence(nil), env.Evidence...)
	sort.SliceStable(out.Evidence, func(i, j int) bool {
		return out.Evidence[i].ID < out.Evidence[j].ID
	})

	out.Diagnostics = append([]Diagnostic(nil), env.Diagnostics...)
	sort.SliceStable(out.Diagnostics, func(i, j int) bool {
		return out.Diagnostics[i].Code+"\x00"+out.Diagnostics[i].Severity <
			out.Diagnostics[j].Code+"\x00"+out.Diagnostics[j].Severity
	})
	return out
}

// CanonicalJSON returns stable JSON used for facts hashing.
func CanonicalJSON(env Envelope) ([]byte, error) {
	if err := Validate(env); err != nil {
		return nil, err
	}
	return json.Marshal(Canonicalize(env))
}

// Hash returns lowercase SHA-256 of CanonicalJSON(env).
func Hash(env Envelope) (string, error) {
	data, err := CanonicalJSON(env)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func factID(predicate string, terms ...Term) string {
	var b strings.Builder
	b.WriteString(predicate)
	for _, term := range terms {
		b.WriteByte(0)
		b.WriteString(term.Sort)
		b.WriteByte(0)
		b.WriteString(term.Value)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return "fact:" + hex.EncodeToString(sum[:])
}

func evidenceID(extractor, contentHash string) string {
	payload := extractor + "\x00" + contentHash
	sum := sha256.Sum256([]byte(payload))
	return "evidence:" + hex.EncodeToString(sum[:])
}

func contentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func sortedUniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := append([]string(nil), values...)
	sort.Strings(out)
	unique := out[:0]
	var prev string
	for _, value := range out {
		if value == prev {
			continue
		}
		unique = append(unique, value)
		prev = value
	}
	return unique
}
