package emitter

import (
	"fmt"
	"sort"
	"strings"
)

// frontendSDKModule is an optional part of the generated frontend SDK. A project
// lists the ones it does not use in target.frontend_sdk_skip; they stay
// available in ANG and come back when removed from that list.
type frontendSDKModule struct {
	Name        string
	Description string
	Outputs     []string // relative to the SDK directory
	Templates   []string // frontend templates that write the module
}

var frontendSDKModules = []frontendSDKModule{
	{Name: "mocks", Description: "MSW v2 request handlers and a test server for every endpoint", Outputs: []string{"mocks/handlers.ts", "mocks/server.ts"}, Templates: []string{"handlers", "msw-server"}},
	{Name: "routes", Description: "route definitions with loaders that prefetch GET queries", Outputs: []string{"routes.ts"}, Templates: []string{"routes"}},
	{Name: "app-router", Description: "TanStack Router tree built from entities and endpoints", Outputs: []string{"app-router.ts"}, Templates: []string{"app-router"}},
	{Name: "prefetch", Description: "queryClient.prefetchQuery helpers for list endpoints", Outputs: []string{"prefetch/index.ts"}, Templates: []string{"prefetch"}},
	{Name: "format", Description: "formatters for money, dates, quantities and enums", Outputs: []string{"format.ts"}, Templates: []string{"format"}},
	{Name: "pages", Description: "List/Create/Edit pages for CRUD entities", Outputs: []string{"pages"}},
	{Name: "gdpr-policy", Description: "GDPR field policy helpers (masking, retention)", Outputs: []string{"gdpr-policy.ts"}, Templates: []string{"gdpr-policy"}},
	{Name: "cookie-banner", Description: "cookie consent banner component", Outputs: []string{"cookie-banner.tsx"}, Templates: []string{"cookie-banner"}},
	{Name: "a11y", Description: "accessibility helpers", Outputs: []string{"a11y.ts"}, Templates: []string{"a11y"}},
	{Name: "error-boundaries", Description: "React error boundaries for query errors", Outputs: []string{"error-boundaries.tsx"}, Templates: []string{"error-boundaries"}},
	{Name: "suspense-boundaries", Description: "React Suspense boundaries for queries", Outputs: []string{"suspense-boundaries.tsx"}, Templates: []string{"suspense-boundaries"}},
	{Name: "ui-forms-alias", Description: "@ui/forms, a copy of components/ui/forms under the @ui alias", Outputs: []string{"@ui/forms"}},
}

func (e *Emitter) sdkModuleEnabled(name string) bool {
	for _, skipped := range e.FrontendSDKSkip {
		if strings.TrimSpace(skipped) == name {
			return false
		}
	}
	return true
}

// sdkModuleOfTemplate names the optional module a frontend template belongs to,
// or "" for templates every SDK has.
func sdkModuleOfTemplate(tmpl string) string {
	for _, m := range frontendSDKModules {
		for _, t := range m.Templates {
			if t == tmpl {
				return m.Name
			}
		}
	}
	return ""
}

// ValidateFrontendSDKSkip rejects names that are not SDK modules, so a typo
// cannot silently keep generating what it meant to skip.
func (e *Emitter) ValidateFrontendSDKSkip() error {
	known := map[string]bool{}
	names := make([]string, 0, len(frontendSDKModules))
	for _, m := range frontendSDKModules {
		known[m.Name] = true
		names = append(names, m.Name)
	}
	sort.Strings(names)
	for _, skipped := range e.FrontendSDKSkip {
		if !known[strings.TrimSpace(skipped)] {
			return fmt.Errorf("frontend_sdk_skip: unknown SDK module %q (modules: %s)", skipped, strings.Join(names, ", "))
		}
	}
	return nil
}

// removeSkippedSDKModules logs each skipped module and removes the files an
// earlier build generated for it (only files carrying an ANG banner).
func (e *Emitter) removeSkippedSDKModules() error {
	for _, m := range frontendSDKModules {
		if e.sdkModuleEnabled(m.Name) {
			continue
		}
		fmt.Printf("Skipping SDK module %s: listed in frontend_sdk_skip\n", m.Name)
		if err := removeGeneratedUnder(e.FrontendDir, m.Outputs...); err != nil {
			return err
		}
	}
	return nil
}
