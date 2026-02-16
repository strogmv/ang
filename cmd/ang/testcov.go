package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

// TestCoverageReport represents the test coverage analysis result.
type TestCoverageReport struct {
	TotalEndpoints  int                 `json:"totalEndpoints"`
	TestedEndpoints int                 `json:"testedEndpoints"`
	CoveragePercent float64             `json:"coveragePercent"`
	MissingTests    []EndpointCoverage  `json:"missingTests"`
	TestedBy        map[string][]string `json:"testedBy,omitempty"`
}

// EndpointCoverage represents a single endpoint's test coverage.
type EndpointCoverage struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	RPC     string `json:"rpc"`
	Service string `json:"service"`
}

// checkTestCoverage analyzes test files and reports endpoints without test coverage.
func checkTestCoverage(endpoints []normalizer.Endpoint, testDir string) (*TestCoverageReport, error) {
	// Build a map of all endpoints
	endpointMap := make(map[string]*EndpointCoverage)
	for _, ep := range endpoints {
		if ep.Method == "WS" {
			continue // Skip WebSocket endpoints for now
		}
		key := fmt.Sprintf("%s %s", ep.Method, ep.Path)
		endpointMap[key] = &EndpointCoverage{
			Method:  ep.Method,
			Path:    ep.Path,
			RPC:     ep.RPC,
			Service: ep.ServiceName,
		}
	}

	// Find all test files
	testFiles, err := findTestFiles(testDir)
	if err != nil {
		return nil, fmt.Errorf("finding test files: %w", err)
	}

	// Extract API calls from test files
	testedEndpoints := make(map[string][]string) // endpoint -> test files
	for _, testFile := range testFiles {
		calls, err := extractAPICalls(testFile)
		if err != nil {
			continue // Skip files that can't be parsed
		}
		relPath, _ := filepath.Rel(testDir, testFile)
		for _, call := range calls {
			testedEndpoints[call] = append(testedEndpoints[call], relPath)
		}
	}

	// Match tested endpoints with defined endpoints
	tested := make(map[string]bool)
	testedBy := make(map[string][]string)
	for key := range endpointMap {
		for testedKey, files := range testedEndpoints {
			if matchesEndpoint(key, testedKey) {
				tested[key] = true
				testedBy[key] = files
				break
			}
		}
	}

	// Build report
	report := &TestCoverageReport{
		TotalEndpoints:  len(endpointMap),
		TestedEndpoints: len(tested),
		TestedBy:        testedBy,
	}

	if report.TotalEndpoints > 0 {
		report.CoveragePercent = float64(report.TestedEndpoints) / float64(report.TotalEndpoints) * 100
	}

	// Collect missing tests
	for key, ep := range endpointMap {
		if !tested[key] {
			report.MissingTests = append(report.MissingTests, *ep)
		}
	}

	// Sort missing tests for consistent output
	sort.Slice(report.MissingTests, func(i, j int) bool {
		if report.MissingTests[i].Service != report.MissingTests[j].Service {
			return report.MissingTests[i].Service < report.MissingTests[j].Service
		}
		return report.MissingTests[i].Path < report.MissingTests[j].Path
	})

	return report, nil
}

// findTestFiles finds all test files in the given directory.
func findTestFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Match TypeScript/JavaScript test files
		if strings.HasSuffix(path, ".test.ts") || strings.HasSuffix(path, ".test.js") ||
			strings.HasSuffix(path, ".spec.ts") || strings.HasSuffix(path, ".spec.js") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// extractAPICalls extracts API calls from a test file.
func extractAPICalls(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var calls []string
	scanner := bufio.NewScanner(file)

	// Patterns to match API calls
	patterns := []*regexp.Regexp{
		// apiClient.get/post/patch/delete('/api/...')
		regexp.MustCompile(`apiClient\.(get|post|put|patch|delete)\s*[<(]\s*['"\x60]([^'"\x60]+)['"\x60]`),
		// axios.get/post/patch/delete('/api/...')
		regexp.MustCompile(`axios\.(get|post|put|patch|delete)\s*\(\s*['"\x60]([^'"\x60]+)['"\x60]`),
		// fetch('/api/...')
		regexp.MustCompile(`fetch\s*\(\s*['"\x60]([^'"\x60]+)['"\x60]`),
	}

	for scanner.Scan() {
		line := scanner.Text()
		for _, pattern := range patterns {
			matches := pattern.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				var method, path string
				if len(match) == 3 {
					method = strings.ToUpper(match[1])
					path = match[2]
				} else if len(match) == 2 {
					method = "GET" // fetch defaults to GET
					path = match[1]
				}
				if strings.HasPrefix(path, "/api/") {
					calls = append(calls, fmt.Sprintf("%s %s", method, path))
				}
			}
		}
	}

	return calls, scanner.Err()
}

// matchesEndpoint checks if a tested endpoint matches a defined endpoint.
func matchesEndpoint(defined, tested string) bool {
	// Split into method and path
	defParts := strings.SplitN(defined, " ", 2)
	testParts := strings.SplitN(tested, " ", 2)
	if len(defParts) != 2 || len(testParts) != 2 {
		return false
	}

	defMethod, defPath := defParts[0], defParts[1]
	testMethod, testPath := testParts[0], testParts[1]

	// Methods must match
	if defMethod != testMethod {
		return false
	}

	// Convert defined path to regex (replace {param} with pattern)
	regexPath := regexp.MustCompile(`\{[^}]+\}`).ReplaceAllString(defPath, `[^/]+`)
	// Remove query parameters from tested path
	if idx := strings.Index(testPath, "?"); idx != -1 {
		testPath = testPath[:idx]
	}
	// Handle template literals with ${...}
	testPath = regexp.MustCompile(`\$\{[^}]+\}`).ReplaceAllString(testPath, `[^/]+`)

	matched, _ := regexp.MatchString("^"+regexPath+"$", testPath)
	return matched
}

// printTestCoverageReport prints the test coverage report to stdout.
func printTestCoverageReport(report *TestCoverageReport, verbose bool) {
	fmt.Printf("\n📊 Test Coverage Report\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  Total Endpoints:  %d\n", report.TotalEndpoints)
	fmt.Printf("  Tested:           %d\n", report.TestedEndpoints)
	fmt.Printf("  Missing:          %d\n", len(report.MissingTests))
	fmt.Printf("  Coverage:         %.1f%%\n", report.CoveragePercent)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	if len(report.MissingTests) > 0 {
		fmt.Printf("\n⚠️  Endpoints without test coverage:\n\n")

		// Group by service
		byService := make(map[string][]EndpointCoverage)
		for _, ep := range report.MissingTests {
			byService[ep.Service] = append(byService[ep.Service], ep)
		}

		services := make([]string, 0, len(byService))
		for s := range byService {
			services = append(services, s)
		}
		sort.Strings(services)

		for _, svc := range services {
			eps := byService[svc]
			fmt.Printf("  [%s]\n", svc)
			for _, ep := range eps {
				fmt.Printf("    • %s %s (%s)\n", ep.Method, ep.Path, ep.RPC)
			}
			fmt.Println()
		}
	}

	if verbose && len(report.TestedBy) > 0 {
		fmt.Printf("\n✅ Tested endpoints:\n\n")
		keys := make([]string, 0, len(report.TestedBy))
		for k := range report.TestedBy {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			files := report.TestedBy[key]
			fmt.Printf("  %s\n", key)
			for _, f := range files {
				fmt.Printf("    ← %s\n", f)
			}
		}
	}
}
