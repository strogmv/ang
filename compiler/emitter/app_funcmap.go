package emitter

import (
	"sort"
	"strings"
	"text/template"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

func (e *Emitter) getAppFuncMap() template.FuncMap {
	appFuncs := e.getSharedFuncMap()

	// Add app-specific functions
	appFuncs["HasRepoEntitiesIR"] = func(services []ir.Service, entities []ir.Entity) bool {
		dtoEntities := make(map[string]bool, len(entities))
		mongoEntities := make(map[string]bool, len(entities))
		for _, ent := range entities {
			if dto, ok := ent.Metadata["dto"].(bool); ok && dto {
				dtoEntities[ent.Name] = true
			}
			if storage, ok := ent.Metadata["storage"].(string); ok && strings.EqualFold(storage, "mongo") {
				mongoEntities[ent.Name] = true
			}
		}

		seen := make(map[string]bool)

		var scanSteps func([]ir.FlowStep)
		scanSteps = func(steps []ir.FlowStep) {
			for _, step := range steps {
				if strings.HasPrefix(step.Action, "repo.") {
					if src, ok := step.Args["source"].(string); ok && src != "" && !dtoEntities[src] && !mongoEntities[src] {
						seen[src] = true
					}
				}
				if step.Action == "list.Enrich" {
					if src, ok := step.Args["lookupSource"].(string); ok && src != "" && !dtoEntities[src] && !mongoEntities[src] {
						seen[src] = true
					}
				}
				scanSteps(step.Steps)
				scanSteps(step.IfNew)
				scanSteps(step.IfExists)
				scanSteps(step.Then)
				scanSteps(step.Else)
				scanSteps(step.Default)
				for _, branch := range step.Cases {
					scanSteps(branch)
				}
			}
		}

		for _, svc := range services {
			for _, m := range svc.Methods {
				for _, src := range m.Sources {
					if src.Entity == "" || dtoEntities[src.Entity] || mongoEntities[src.Entity] {
						continue
					}
					seen[src.Entity] = true
				}
				scanSteps(m.Flow)
			}
		}

		return len(seen) > 0
	}
	appFuncs["EntityStorageByNameIR"] = func(entities []ir.Entity, name string) string {
		for _, ent := range entities {
			if ent.Name != name {
				continue
			}
			if ent.Metadata != nil {
				if v, ok := ent.Metadata["storage"].(string); ok && v != "" {
					return v
				}
			}
			return "sql"
		}
		return "sql"
	}
	appFuncs["HasMongoRepoEntitiesIR"] = func(entities []ir.Entity) bool {
		for _, ent := range entities {
			if v, ok := ent.Metadata["storage"].(string); ok && strings.EqualFold(v, "mongo") {
				return true
			}
		}
		return false
	}
	appFuncs["AllRepoEntitiesIR"] = func(entities []ir.Entity) []string {
		var res []string
		for _, ent := range entities {
			if isDTO, ok := ent.Metadata["dto"].(bool); ok && isDTO {
				continue
			}
			res = append(res, ent.Name)
		}
		sort.Strings(res)
		return res
	}
	appFuncs["HasEntityByNameIR"] = func(entities []ir.Entity, name string) bool {
		for _, ent := range entities {
			if strings.EqualFold(ent.Name, name) {
				return true
			}
		}
		return false
	}
	appFuncs["HasServiceByNameIR"] = func(services []ir.Service, name string) bool {
		for _, svc := range services {
			if strings.EqualFold(svc.Name, name) {
				return true
			}
		}
		return false
	}
	appFuncs["HasTxServicesIR"] = func(services []ir.Service) bool {
		var hasTx func([]ir.FlowStep) bool
		hasTx = func(steps []ir.FlowStep) bool {
			for _, step := range steps {
				if step.Action == "tx.Block" {
					return true
				}
				if hasTx(step.Steps) || hasTx(step.IfNew) || hasTx(step.IfExists) || hasTx(step.Then) || hasTx(step.Else) || hasTx(step.Default) {
					return true
				}
				for _, branch := range step.Cases {
					if hasTx(branch) {
						return true
					}
				}
			}
			return false
		}
		for _, svc := range services {
			for _, m := range svc.Methods {
				if (m.Impl != nil && m.Impl.RequiresTx) || hasTx(m.Flow) {
					return true
				}
			}
		}
		return false
	}
	appFuncs["AnyServiceHasIdempotencyOrOutboxIR"] = func(services []ir.Service) bool {
		var hasOutboxAction func([]ir.FlowStep) bool
		hasOutboxAction = func(steps []ir.FlowStep) bool {
			for _, step := range steps {
				if step.Action == "event.Outbox" {
					return true
				}
				if hasOutboxAction(step.Steps) || hasOutboxAction(step.IfNew) || hasOutboxAction(step.IfExists) || hasOutboxAction(step.Then) || hasOutboxAction(step.Else) || hasOutboxAction(step.Default) {
					return true
				}
				for _, branch := range step.Cases {
					if hasOutboxAction(branch) {
						return true
					}
				}
			}
			return false
		}
		for _, svc := range services {
			for _, m := range svc.Methods {
				if m.Idempotent || m.Outbox || hasOutboxAction(m.Flow) {
					return true
				}
			}
		}
		return false
	}
	appFuncs["getServiceDepsIR"] = func(s ir.Service) []string {
		if len(s.Uses) == 0 {
			return nil
		}
		deps := append([]string{}, s.Uses...)
		sort.Strings(deps)
		return deps
	}
	appFuncs["ServiceNeedsTxIR"] = func(s ir.Service) bool {
		var hasTx func([]ir.FlowStep) bool
		hasTx = func(steps []ir.FlowStep) bool {
			for _, step := range steps {
				if step.Action == "tx.Block" {
					return true
				}
				if hasTx(step.Steps) || hasTx(step.IfNew) || hasTx(step.IfExists) || hasTx(step.Then) || hasTx(step.Else) || hasTx(step.Default) {
					return true
				}
				for _, branch := range step.Cases {
					if hasTx(branch) {
						return true
					}
				}
			}
			return false
		}
		for _, m := range s.Methods {
			if (m.Impl != nil && m.Impl.RequiresTx) || hasTx(m.Flow) {
				return true
			}
		}
		return false
	}
	appFuncs["ServiceHasPublishesIR"] = func(s ir.Service) bool {
		var hasPublish func([]ir.FlowStep) bool
		hasPublish = func(steps []ir.FlowStep) bool {
			for _, step := range steps {
				if step.Action == "event.Publish" {
					return true
				}
				if hasPublish(step.Steps) || hasPublish(step.IfNew) || hasPublish(step.IfExists) || hasPublish(step.Then) || hasPublish(step.Else) || hasPublish(step.Default) {
					return true
				}
				for _, branch := range step.Cases {
					if hasPublish(branch) {
						return true
					}
				}
			}
			return false
		}
		for _, m := range s.Methods {
			if len(m.Publishes) > 0 || hasPublish(m.Flow) {
				return true
			}
		}
		return false
	}
	appFuncs["ServiceHasStorageIR"] = func(s ir.Service) bool {
		var hasStorage func([]ir.FlowStep) bool
		hasStorage = func(steps []ir.FlowStep) bool {
			for _, step := range steps {
				switch step.Action {
				case "storage.Upload", "storage.Download", "storage.GetURL", "storage.Delete", "storage.List":
					return true
				}
				if hasStorage(step.Steps) || hasStorage(step.IfNew) || hasStorage(step.IfExists) || hasStorage(step.Then) || hasStorage(step.Else) || hasStorage(step.Default) {
					return true
				}
				for _, branch := range step.Cases {
					if hasStorage(branch) {
						return true
					}
				}
			}
			return false
		}
		for _, m := range s.Methods {
			if hasStorage(m.Flow) {
				return true
			}
		}
		return false
	}
	appFuncs["AnyServiceHasStorageIR"] = func(services []ir.Service) bool {
		hasStorage := appFuncs["ServiceHasStorageIR"].(func(ir.Service) bool)
		for _, svc := range services {
			if hasStorage(svc) {
				return true
			}
		}
		return false
	}
	appFuncs["ServiceHasNotificationDispatchIR"] = func(s ir.Service) bool {
		var hasDispatch func([]ir.FlowStep) bool
		hasDispatch = func(steps []ir.FlowStep) bool {
			for _, step := range steps {
				if step.Action == "notification.Dispatch" || step.Action == "notify.Dispatch" || step.Action == "notify.Send" {
					return true
				}
				if hasDispatch(step.Steps) || hasDispatch(step.IfNew) || hasDispatch(step.IfExists) || hasDispatch(step.Then) || hasDispatch(step.Else) || hasDispatch(step.Default) {
					return true
				}
				for _, branch := range step.Cases {
					if hasDispatch(branch) {
						return true
					}
				}
			}
			return false
		}
		for _, m := range s.Methods {
			if hasDispatch(m.Flow) {
				return true
			}
		}
		return false
	}
	appFuncs["AnyServiceHasNotificationDispatchIR"] = func(services []ir.Service) bool {
		var hasDispatch func([]ir.FlowStep) bool
		hasDispatch = func(steps []ir.FlowStep) bool {
			for _, step := range steps {
				if step.Action == "notification.Dispatch" || step.Action == "notify.Dispatch" || step.Action == "notify.Send" {
					return true
				}
				if hasDispatch(step.Steps) || hasDispatch(step.IfNew) || hasDispatch(step.IfExists) || hasDispatch(step.Then) || hasDispatch(step.Else) || hasDispatch(step.Default) {
					return true
				}
				for _, branch := range step.Cases {
					if hasDispatch(branch) {
						return true
					}
				}
			}
			return false
		}
		for _, svc := range services {
			for _, m := range svc.Methods {
				if hasDispatch(m.Flow) {
					return true
				}
			}
		}
		return false
	}
	appFuncs["ServiceHasIdempotencyIR"] = func(s ir.Service) bool {
		for _, m := range s.Methods {
			if m.Idempotent {
				return true
			}
		}
		return false
	}
	appFuncs["ServiceHasOutboxIR"] = func(s ir.Service) bool {
		var hasOutboxAction func([]ir.FlowStep) bool
		hasOutboxAction = func(steps []ir.FlowStep) bool {
			for _, step := range steps {
				if step.Action == "event.Outbox" {
					return true
				}
				if hasOutboxAction(step.Steps) || hasOutboxAction(step.IfNew) || hasOutboxAction(step.IfExists) || hasOutboxAction(step.Then) || hasOutboxAction(step.Else) || hasOutboxAction(step.Default) {
					return true
				}
				for _, branch := range step.Cases {
					if hasOutboxAction(branch) {
						return true
					}
				}
			}
			return false
		}
		for _, m := range s.Methods {
			if m.Outbox || hasOutboxAction(m.Flow) {
				return true
			}
		}
		return false
	}
	appFuncs["ServiceHasStateActionsIR"] = func(s ir.Service) bool {
		var hasState func([]ir.FlowStep) bool
		hasState = func(steps []ir.FlowStep) bool {
			for _, step := range steps {
				switch step.Action {
				case "state.Get", "state.Set", "state.Delete":
					return true
				}
				for _, prefix := range []string{"idem.", "idempotency.", "dedupe.", "ratelimit.", "quota.", "budget.", "profile.", "concurrency.", "circuit.", "bulkhead.", "approval."} {
					if strings.HasPrefix(step.Action, prefix) {
						return true
					}
				}
				if hasState(step.Steps) || hasState(step.IfNew) || hasState(step.IfExists) || hasState(step.Then) || hasState(step.Else) || hasState(step.Default) {
					return true
				}
				for _, branch := range step.Cases {
					if hasState(branch) {
						return true
					}
				}
			}
			return false
		}
		for _, m := range s.Methods {
			if hasState(m.Flow) {
				return true
			}
		}
		return false
	}
	appFuncs["AnyServiceHasStateActionsIR"] = func(services []ir.Service) bool {
		hasState := appFuncs["ServiceHasStateActionsIR"].(func(ir.Service) bool)
		for _, svc := range services {
			if hasState(svc) {
				return true
			}
		}
		return false
	}
	appFuncs["ServiceHasPolicyActionsIR"] = func(s ir.Service) bool {
		var hasPolicy func([]ir.FlowStep) bool
		hasPolicy = func(steps []ir.FlowStep) bool {
			for _, step := range steps {
				if strings.HasPrefix(step.Action, "policy.") {
					return true
				}
				if hasPolicy(step.Steps) || hasPolicy(step.IfNew) || hasPolicy(step.IfExists) || hasPolicy(step.Then) || hasPolicy(step.Else) || hasPolicy(step.Default) {
					return true
				}
				for _, branch := range step.Cases {
					if hasPolicy(branch) {
						return true
					}
				}
			}
			return false
		}
		for _, m := range s.Methods {
			if hasPolicy(m.Flow) {
				return true
			}
		}
		return false
	}
	appFuncs["AnyServiceHasPolicyActionsIR"] = func(services []ir.Service) bool {
		hasPolicy := appFuncs["ServiceHasPolicyActionsIR"].(func(ir.Service) bool)
		for _, svc := range services {
			if hasPolicy(svc) {
				return true
			}
		}
		return false
	}
	appFuncs["getRepoEntitiesIR"] = func(s ir.Service, entities []ir.Entity) []string {
		dtoEntities := make(map[string]bool, len(entities))
		for _, ent := range entities {
			if dto, ok := ent.Metadata["dto"].(bool); ok && dto {
				dtoEntities[ent.Name] = true
			}
		}
		seen := make(map[string]bool)
		var out []string

		var scanSteps func([]ir.FlowStep)
		scanSteps = func(steps []ir.FlowStep) {
			for _, step := range steps {
				if strings.HasPrefix(step.Action, "repo.") {
					if src, ok := step.Args["source"].(string); ok && src != "" && !seen[src] && !dtoEntities[src] {
						seen[src] = true
						out = append(out, src)
					}
				}
				if step.Action == "list.Enrich" {
					if src, ok := step.Args["lookupSource"].(string); ok && src != "" && !seen[src] && !dtoEntities[src] {
						seen[src] = true
						out = append(out, src)
					}
				}
				if step.Action == "entity.PatchValidated" {
					hasUnique := false
					if fields, ok := step.Args["fields"].(map[string]map[string]string); ok {
						for _, cfg := range fields {
							if strings.TrimSpace(cfg["unique"]) != "" {
								hasUnique = true
								break
							}
						}
					}
					if hasUnique {
						repoEntity := ""
						if src, ok := step.Args["source"].(string); ok {
							repoEntity = strings.TrimSpace(src)
						}
						if repoEntity != "" && !seen[repoEntity] && !dtoEntities[repoEntity] {
							seen[repoEntity] = true
							out = append(out, repoEntity)
						}
					}
				}
				scanSteps(step.Steps)
				scanSteps(step.IfNew)
				scanSteps(step.IfExists)
				scanSteps(step.Then)
				scanSteps(step.Else)
				scanSteps(step.Default)
				for _, branch := range step.Cases {
					scanSteps(branch)
				}
			}
		}

		for _, m := range s.Methods {
			for _, src := range m.Sources {
				if src.Entity == "" || seen[src.Entity] || dtoEntities[src.Entity] {
					continue
				}
				seen[src.Entity] = true
				out = append(out, src.Entity)
			}
			scanSteps(m.Flow)
		}

		sort.Strings(out)
		return out
	}
	appFuncs["HasEventFieldIR"] = func(evtPayloads map[string]ir.Entity, evtName, fieldName string) bool {
		if fieldName == "" {
			return false
		}
		if entity, ok := evtPayloads[evtName]; ok {
			for _, f := range entity.Fields {
				if strings.EqualFold(f.Name, fieldName) {
					return true
				}
			}
		}
		return false
	}
	appFuncs["RoomFieldForEventIR"] = func(endpoints []ir.Endpoint, services []ir.Service, serviceName, eventName string) string {
		var methods map[string]ir.Method
		for _, svc := range services {
			if svc.Name != serviceName {
				continue
			}
			methods = make(map[string]ir.Method)
			for _, m := range svc.Methods {
				methods[m.Name] = m
			}
			break
		}
		firstPathParam := func(path string) string {
			start := strings.Index(path, "{")
			if start == -1 {
				return ""
			}
			end := strings.Index(path[start:], "}")
			if end == -1 {
				return ""
			}
			return path[start+1 : start+end]
		}
		for _, ep := range endpoints {
			if strings.ToUpper(ep.Method) != "WS" || ep.Service != serviceName {
				continue
			}
			found := false
			for _, msg := range ep.Messages {
				if msg == eventName {
					found = true
					break
				}
			}
			if !found {
				continue
			}
			roomParam := ep.RoomParam
			if roomParam == "" {
				roomParam = firstPathParam(ep.Path)
			}
			if roomParam != "" {
				return ExportName(roomParam)
			}
			if m, ok := methods[ep.RPC]; ok && m.Input != nil {
				for _, f := range m.Input.Fields {
					if strings.EqualFold(f.Name, "userId") {
						return "UserID"
					}
				}
			}
		}
		return ""
	}

	appFuncs["HasService"] = func(services []normalizer.Service, name string) bool {
		for _, svc := range services {
			if svc.Name == name {
				return true
			}
		}
		return false
	}
	appFuncs["HasTxServices"] = func(services []normalizer.Service) bool {
		for _, svc := range services {
			var hasTx func([]normalizer.FlowStep) bool
			hasTx = func(steps []normalizer.FlowStep) bool {
				for _, step := range steps {
					if step.Action == "tx.Block" {
						return true
					}
					if v, ok := step.Args["_do"].([]normalizer.FlowStep); ok && hasTx(v) {
						return true
					}
					if v, ok := step.Args["_then"].([]normalizer.FlowStep); ok && hasTx(v) {
						return true
					}
					if v, ok := step.Args["_else"].([]normalizer.FlowStep); ok && hasTx(v) {
						return true
					}
					if v, ok := step.Args["_default"].([]normalizer.FlowStep); ok && hasTx(v) {
						return true
					}
					if cases, ok := step.Args["_cases"].(map[string][]normalizer.FlowStep); ok {
						for _, branch := range cases {
							if hasTx(branch) {
								return true
							}
						}
					}
				}
				return false
			}
			for _, m := range svc.Methods {
				if (m.Impl != nil && m.Impl.RequiresTx) || hasTx(m.Flow) {
					return true
				}
			}
		}
		return false
	}
	appFuncs["HasMethod"] = func(s normalizer.Service, name string) bool {
		for _, m := range s.Methods {
			if m.Name == name {
				return true
			}
		}
		return false
	}
	appFuncs["HasRepoEntities"] = func(services []normalizer.Service, entities []normalizer.Entity) bool {
		dtoEntities := make(map[string]bool, len(entities))
		mongoEntities := make(map[string]bool, len(entities))
		for _, ent := range entities {
			if dto, ok := ent.Metadata["dto"].(bool); ok && dto {
				dtoEntities[ent.Name] = true
			}
			if storage, ok := ent.Metadata["storage"].(string); ok && strings.EqualFold(storage, "mongo") {
				mongoEntities[ent.Name] = true
			}
		}
		for _, svc := range services {
			unique := make(map[string]bool)
			var count int
			for _, m := range svc.Methods {
				for _, src := range m.Sources {
					if src.Entity != "" && !unique[src.Entity] && !dtoEntities[src.Entity] && !mongoEntities[src.Entity] {
						unique[src.Entity] = true
						count++
					}
				}
			}
			if count > 0 {
				return true
			}
		}
		return false
	}
	appFuncs["HasEntity"] = func(services []normalizer.Service, entity string) bool {
		for _, svc := range services {
			for _, m := range svc.Methods {
				for _, src := range m.Sources {
					if src.Entity == entity {
						return true
					}
				}
			}
		}
		return false
	}
	appFuncs["HasNonUserEntities"] = func(services []normalizer.Service) bool {
		for _, svc := range services {
			for _, m := range svc.Methods {
				for _, src := range m.Sources {
					if src.Entity != "" && src.Entity != "User" {
						return true
					}
				}
			}
		}
		return false
	}
	appFuncs["AllRepoEntities"] = func(entities []normalizer.Entity) []string {
		var res []string
		for _, ent := range entities {
			// Skip DTO-only entities
			if isDTO, ok := ent.Metadata["dto"].(bool); ok && isDTO {
				continue
			}
			res = append(res, ent.Name)
		}
		sort.Strings(res)
		return res
	}
	appFuncs["AllUsedRepoEntitiesIR"] = func(services []ir.Service, entities []ir.Entity) []string {
		dtoEntities := make(map[string]bool, len(entities))
		for _, ent := range entities {
			if dto, ok := ent.Metadata["dto"].(bool); ok && dto {
				dtoEntities[ent.Name] = true
			}
		}

		seen := make(map[string]bool)
		var out []string

		var scanSteps func([]ir.FlowStep)
		scanSteps = func(steps []ir.FlowStep) {
			for _, step := range steps {
				if strings.HasPrefix(step.Action, "repo.") {
					if src, ok := step.Args["source"].(string); ok && src != "" && !seen[src] && !dtoEntities[src] {
						seen[src] = true
						out = append(out, src)
					}
				}
				if step.Action == "list.Enrich" {
					if src, ok := step.Args["lookupSource"].(string); ok && src != "" && !seen[src] && !dtoEntities[src] {
						seen[src] = true
						out = append(out, src)
					}
				}
				scanSteps(step.Steps)
				scanSteps(step.IfNew)
				scanSteps(step.IfExists)
				scanSteps(step.Then)
				scanSteps(step.Else)
				scanSteps(step.Default)
				for _, branch := range step.Cases {
					scanSteps(branch)
				}
			}
		}

		for _, svc := range services {
			for _, m := range svc.Methods {
				for _, src := range m.Sources {
					if src.Entity == "" || seen[src.Entity] || dtoEntities[src.Entity] {
						continue
					}
					seen[src.Entity] = true
					out = append(out, src.Entity)
				}
				scanSteps(m.Flow)
			}
		}
		sort.Strings(out)
		return out
	}
	appFuncs["UniqueRepoEntities"] = func(services []normalizer.Service) []string {
		seen := make(map[string]bool)
		var res []string
		for _, svc := range services {
			for _, m := range svc.Methods {
				for _, src := range m.Sources {
					if src.Entity != "" && !seen[src.Entity] {
						seen[src.Entity] = true
						res = append(res, src.Entity)
					}
				}
			}
		}
		sort.Strings(res)
		return res
	}
	appFuncs["ToSnake"] = ToSnakeCase
	appFuncs["ToTitle"] = ToTitle
	appFuncs["HasEventField"] = func(evtPayloads map[string]normalizer.Entity, evtName, fieldName string) bool {
		if fieldName == "" {
			return false
		}
		if entity, ok := evtPayloads[evtName]; ok {
			for _, f := range entity.Fields {
				if strings.EqualFold(f.Name, fieldName) {
					return true
				}
			}
		}
		return false
	}
	appFuncs["RoomFieldForEvent"] = func(endpoints []normalizer.Endpoint, services []normalizer.Service, serviceName, eventName string) string {
		var methods map[string]normalizer.Method
		for _, svc := range services {
			if svc.Name != serviceName {
				continue
			}
			methods = make(map[string]normalizer.Method)
			for _, m := range svc.Methods {
				methods[m.Name] = m
			}
			break
		}
		firstPathParam := func(path string) string {
			start := strings.Index(path, "{")
			if start == -1 {
				return ""
			}
			end := strings.Index(path[start:], "}")
			if end == -1 {
				return ""
			}
			return path[start+1 : start+end]
		}
		for _, ep := range endpoints {
			if strings.ToUpper(ep.Method) != "WS" || ep.ServiceName != serviceName {
				continue
			}
			found := false
			for _, msg := range ep.Messages {
				if msg == eventName {
					found = true
					break
				}
			}
			if !found {
				continue
			}
			roomParam := ep.RoomParam
			if roomParam == "" {
				roomParam = firstPathParam(ep.Path)
			}
			if roomParam != "" {
				return ExportName(roomParam)
			}
			if m, ok := methods[ep.RPC]; ok {
				for _, f := range m.Input.Fields {
					if strings.EqualFold(f.Name, "userId") {
						return "UserID"
					}
				}
			}
		}
		return ""
	}

	return appFuncs
}

// EmitServiceMain generates main.go for a specific service.
