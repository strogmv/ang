package facts

import (
	"fmt"
	"sort"
	"strings"
)

// ValidationError reports deterministic, path-addressable contract violations.
// It intentionally does not infer missing data: incomplete extraction is data
// that a later expert layer must represent as unknown, not silently repair.
type ValidationError struct {
	Problems []Problem
}

type Problem struct {
	Path    string
	Message string
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Problems))
	for _, problem := range e.Problems {
		parts = append(parts, problem.Path+": "+problem.Message)
	}
	return "invalid " + SchemaV1 + " document: " + strings.Join(parts, "; ")
}

// Validate checks the structural invariants shared by all ang/facts/v1
// producers. It does not check source-specific completeness or infer values.
func Validate(env Envelope) error {
	problems := make([]Problem, 0)
	if env.Schema != SchemaV1 {
		problems = append(problems, Problem{
			Path:    "schema",
			Message: fmt.Sprintf("must equal %q", SchemaV1),
		})
	}
	for i, entity := range env.Entities {
		path := fmt.Sprintf("entities[%d]", i)
		if strings.TrimSpace(entity.Name) == "" {
			problems = append(problems, Problem{Path: path + ".name", Message: "must not be empty"})
		}
		problems = append(problems, validateFields(path+".fields", entity.Fields)...)
	}
	for i, operation := range env.Operations {
		path := fmt.Sprintf("operations[%d]", i)
		if strings.TrimSpace(operation.Name) == "" {
			problems = append(problems, Problem{Path: path + ".name", Message: "must not be empty"})
		}
		problems = append(problems, validateFields(path+".input_fields", operation.InputFields)...)
		problems = append(problems, validateFields(path+".output_fields", operation.OutputFields)...)
	}
	for i, repository := range env.Repositories {
		path := fmt.Sprintf("repositories[%d]", i)
		if strings.TrimSpace(repository.Entity) == "" {
			problems = append(problems, Problem{Path: path + ".entity", Message: "must not be empty"})
		}
		for j, method := range repository.Methods {
			if strings.TrimSpace(method.Name) == "" {
				problems = append(problems, Problem{Path: fmt.Sprintf("%s.methods[%d].name", path, j), Message: "must not be empty"})
			}
		}
	}
	for i, event := range env.Events {
		path := fmt.Sprintf("events[%d]", i)
		if strings.TrimSpace(event.Name) == "" {
			problems = append(problems, Problem{Path: path + ".name", Message: "must not be empty"})
		}
		problems = append(problems, validateFields(path+".payload_fields", event.PayloadFields)...)
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Slice(problems, func(i, j int) bool {
		if problems[i].Path == problems[j].Path {
			return problems[i].Message < problems[j].Message
		}
		return problems[i].Path < problems[j].Path
	})
	return &ValidationError{Problems: problems}
}

func validateFields(path string, fields []Field) []Problem {
	problems := make([]Problem, 0)
	for i, field := range fields {
		if strings.TrimSpace(field.Name) == "" {
			problems = append(problems, Problem{Path: fmt.Sprintf("%s[%d].name", path, i), Message: "must not be empty"})
		}
	}
	return problems
}
