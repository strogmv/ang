package paymentprovider

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var schemaTopLevelNamePattern = regexp.MustCompile(`(?m)^([A-Z#][A-Za-z0-9_]*):\s*(?:\{|\()`)

func localSchemaExtensions(localPath string, bundled []byte) ([]string, error) {
	local, err := os.ReadFile(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	localNames := schemaTopLevelNames(local)
	bundledNames := schemaTopLevelNames(bundled)
	var extras []string
	for name := range localNames {
		if _, ok := bundledNames[name]; !ok {
			extras = append(extras, name)
		}
	}
	return extras, nil
}

func schemaTopLevelNames(content []byte) map[string]struct{} {
	names := map[string]struct{}{}
	for _, match := range schemaTopLevelNamePattern.FindAllStringSubmatch(string(content), -1) {
		if len(match) < 2 {
			continue
		}
		name := strings.TrimSpace(match[1])
		if name == "" {
			continue
		}
		names[name] = struct{}{}
	}
	return names
}

func guardSchemaOverwrite(dstPath string, bundled []byte, force bool) (extras []string, skip bool, err error) {
	if force {
		return nil, false, nil
	}
	extras, err = localSchemaExtensions(dstPath, bundled)
	if err != nil {
		return nil, false, err
	}
	if len(extras) > 0 {
		return extras, true, nil
	}
	return nil, false, nil
}

func formatGuardedSchemaFile(name string, extras []string) string {
	return fmt.Sprintf("%s: local extensions preserved (%s)", name, strings.Join(extras, ", "))
}
