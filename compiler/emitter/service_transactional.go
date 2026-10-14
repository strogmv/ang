package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/angir/normalizer"
)

// wrapTransactionalOperations makes transactional: true run an operation in
// one database transaction. A flow becomes the body of one tx.Block, so it
// is rendered by the same code as a tx.Block written by hand (the closure
// gets the transaction's ctx, every repository call joins it, values the
// flow declares are hoisted, an error rolls back). An operation implemented
// in Go is wrapped by transactionalImplCode. Both need the service's
// txManager, which serviceImplNeedsTx adds for a transactional operation.
func wrapTransactionalOperations(services []normalizer.Service) error {
	for si := range services {
		for mi := range services[si].Methods {
			m := &services[si].Methods[mi]
			if !m.Transactional {
				continue
			}
			if m.IsStreaming {
				return fmt.Errorf("%s: operation %s.%s: transactional: true does not apply to a streaming operation", m.Source, services[si].Name, m.Name)
			}
			if m.Impl != nil && strings.TrimSpace(m.Impl.Code) != "" {
				continue // transactionalImplCode wraps it in the template
			}
			if len(m.Flow) == 0 {
				return fmt.Errorf("%s: operation %s.%s: transactional: true needs a flow or impls: go: code", m.Source, services[si].Name, m.Name)
			}
			if len(m.Flow) == 1 && m.Flow[0].Action == "tx.Block" {
				continue // already one transaction
			}
			first := m.Flow[0]
			m.Flow = []normalizer.FlowStep{{
				Action: "tx.Block",
				Args:   map[string]any{"_do": m.Flow},
				File:   first.File,
				Line:   first.Line,
			}}
		}
	}
	return nil
}
