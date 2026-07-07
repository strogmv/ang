package flowir

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/flowsem"
)

// ValidateSchemaContract keeps the typed registry aligned with the public
// validator/catalog contract. The typed registry is authoritative for argument
// shape; the legacy catalog must still recognize every typed action while its
// rows are progressively replaced by registry metadata.
func ValidateSchemaContract(catalog []flowsem.ActionCatalogEntry) error {
	external := make(map[string]flowsem.ActionCatalogEntry, len(catalog))
	for _, entry := range catalog {
		external[entry.Name] = entry
	}
	var problems []string
	for _, typed := range All() {
		entry, exists := external[typed.Name]
		if !exists {
			problems = append(problems, fmt.Sprintf("typed action %s is missing from flowsem schema", typed.Name))
			continue
		}
		_ = entry
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("typed Flow IR schema contract mismatch: %s", strings.Join(problems, "; "))
}
