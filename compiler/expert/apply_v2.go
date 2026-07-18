package expert

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"
)

// ContentHash returns lowercase SHA-256 hex for file bytes.
func ContentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// ApplyChangeV2 applies one typed intent change to CUE source bytes.
func ApplyChangeV2(content []byte, change ChangeV2, force bool) ([]byte, error) {
	if err := validateIntentTarget(change.Target); err != nil {
		return nil, err
	}
	file, err := parser.ParseFile(change.Target.RelativePath, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse cue: %w", err)
	}
	root, err := providerInlineStruct(file)
	if err != nil {
		return nil, err
	}
	valueExpr, err := jsonValueToCueExpr(change.Value)
	if err != nil {
		return nil, err
	}
	switch change.Op {
	case ChangeMerge:
		if err := mergeAtCuePath(root, change.CUEPath, valueExpr, false); err != nil {
			return nil, err
		}
	case ChangeReplace:
		if err := mergeAtCuePath(root, change.CUEPath, valueExpr, true); err != nil {
			return nil, err
		}
	case ChangeInsert:
		if existsAtCuePath(root, change.CUEPath) {
			return nil, fmt.Errorf("cue_path %q already exists", change.CUEPath)
		}
		if err := mergeAtCuePath(root, change.CUEPath, valueExpr, force); err != nil {
			return nil, err
		}
	case ChangeDelete:
		if err := deleteAtCuePath(root, change.CUEPath); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported change op %q", change.Op)
	}
	formatted, err := format.Node(file)
	if err != nil {
		return nil, fmt.Errorf("format cue: %w", err)
	}
	return formatted, nil
}

// FindProposalV2 returns the proposal with the given ID.
func FindProposalV2(report ReportV2, proposalID string) (ProposalV2, error) {
	for _, proposal := range report.Proposals {
		if proposal.ID == proposalID {
			return proposal, nil
		}
	}
	return ProposalV2{}, fmt.Errorf("proposal %q not found", proposalID)
}

// VerifyChangeBeforeHash rejects stale proposals when before_hash is present.
func VerifyChangeBeforeHash(content []byte, change ChangeV2) error {
	want := strings.TrimSpace(change.BeforeHash)
	if want == "" {
		return nil
	}
	got := ContentHash(content)
	if got != want {
		return fmt.Errorf("before_hash mismatch: file changed since proposal was created")
	}
	return nil
}

// ApplyProposalV2 applies all changes from a proposal to a single CUE file.
func ApplyProposalV2(content []byte, proposal ProposalV2, force bool) ([]byte, error) {
	current := content
	for _, change := range proposal.Changes {
		if err := VerifyChangeBeforeHash(current, change); err != nil {
			return nil, err
		}
		next, err := ApplyChangeV2(current, change, force)
		if err != nil {
			return nil, fmt.Errorf("apply change to %s: %w", change.Target.RelativePath, err)
		}
		current = next
	}
	return current, nil
}

func providerInlineStruct(file *ast.File) (*ast.StructLit, error) {
	for _, decl := range file.Decls {
		field, ok := decl.(*ast.Field)
		if !ok || cueLabelString(field.Label) != "provider" {
			continue
		}
		inline := lastStructInIntersection(field.Value)
		if inline == nil {
			return nil, fmt.Errorf("provider field has no inline struct to merge into")
		}
		return inline, nil
	}
	return nil, fmt.Errorf("provider field not found")
}

func lastStructInIntersection(expr ast.Expr) *ast.StructLit {
	current := expr
	for {
		switch node := current.(type) {
		case *ast.StructLit:
			return node
		case *ast.BinaryExpr:
			if node.Op != token.AND {
				return nil
			}
			if s, ok := node.Y.(*ast.StructLit); ok {
				return s
			}
			if s, ok := node.X.(*ast.StructLit); ok {
				return s
			}
			current = node.Y
		default:
			return nil
		}
	}
}

func mergeAtCuePath(root *ast.StructLit, cuePath string, value ast.Expr, force bool) error {
	parts := splitCuePath(cuePath)
	if len(parts) == 0 {
		return fmt.Errorf("cue_path must not be empty")
	}
	current := root
	for i := 0; i < len(parts)-1; i++ {
		next, err := ensureStructField(current, parts[i])
		if err != nil {
			return err
		}
		current = next
	}
	last := parts[len(parts)-1]
	for _, decl := range current.Elts {
		field, ok := decl.(*ast.Field)
		if !ok || cueLabelString(field.Label) != last {
			continue
		}
		if force {
			field.Value = value
			return nil
		}
		return mergeFieldValue(field, value)
	}
	current.Elts = append(current.Elts, &ast.Field{
		Label: ast.NewIdent(last),
		Value: value,
	})
	return nil
}

func deleteAtCuePath(root *ast.StructLit, cuePath string) error {
	parts := splitCuePath(cuePath)
	if len(parts) == 0 {
		return fmt.Errorf("cue_path must not be empty")
	}
	current := root
	for i := 0; i < len(parts)-1; i++ {
		next, ok := structField(current, parts[i])
		if !ok {
			return fmt.Errorf("cue_path %q not found", cuePath)
		}
		current = next
	}
	last := parts[len(parts)-1]
	for i, decl := range current.Elts {
		field, ok := decl.(*ast.Field)
		if !ok || cueLabelString(field.Label) != last {
			continue
		}
		current.Elts = append(current.Elts[:i], current.Elts[i+1:]...)
		return nil
	}
	return fmt.Errorf("cue_path %q not found", cuePath)
}

func existsAtCuePath(root *ast.StructLit, cuePath string) bool {
	parts := splitCuePath(cuePath)
	current := root
	for i := 0; i < len(parts)-1; i++ {
		next, ok := structField(current, parts[i])
		if !ok {
			return false
		}
		current = next
	}
	last := parts[len(parts)-1]
	for _, decl := range current.Elts {
		field, ok := decl.(*ast.Field)
		if ok && cueLabelString(field.Label) == last {
			return true
		}
	}
	return false
}

func ensureStructField(root *ast.StructLit, name string) (*ast.StructLit, error) {
	if existing, ok := structField(root, name); ok {
		return existing, nil
	}
	created := &ast.StructLit{}
	root.Elts = append(root.Elts, &ast.Field{
		Label: ast.NewIdent(name),
		Value: created,
	})
	return created, nil
}

func structField(root *ast.StructLit, name string) (*ast.StructLit, bool) {
	for _, decl := range root.Elts {
		field, ok := decl.(*ast.Field)
		if !ok || cueLabelString(field.Label) != name {
			continue
		}
		switch value := field.Value.(type) {
		case *ast.StructLit:
			return value, true
		default:
			return nil, false
		}
	}
	return nil, false
}

func mergeFieldValue(field *ast.Field, patch ast.Expr) error {
	existing, ok := field.Value.(*ast.StructLit)
	patchStruct, patchOk := patch.(*ast.StructLit)
	if ok && patchOk {
		mergeStructFields(existing, patchStruct)
		return nil
	}
	field.Value = patch
	return nil
}

func mergeStructFields(dst, patch *ast.StructLit) {
	for _, decl := range patch.Elts {
		pField, ok := decl.(*ast.Field)
		if !ok {
			continue
		}
		label := cueLabelString(pField.Label)
		found := false
		for _, oDecl := range dst.Elts {
			oField, ok := oDecl.(*ast.Field)
			if !ok || cueLabelString(oField.Label) != label {
				continue
			}
			_ = mergeFieldValue(oField, pField.Value)
			found = true
			break
		}
		if !found {
			dst.Elts = append(dst.Elts, pField)
		}
	}
}

func splitCuePath(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	return strings.Split(path, ".")
}

func cueLabelString(label ast.Label) string {
	if label == nil {
		return ""
	}
	switch x := label.(type) {
	case *ast.Ident:
		return strings.TrimSpace(x.Name)
	case *ast.BasicLit:
		return strings.TrimSpace(x.Value)
	default:
		return strings.TrimSpace(fmt.Sprint(label))
	}
}

func jsonValueToCueExpr(raw json.RawMessage) (ast.Expr, error) {
	if len(raw) == 0 {
		return &ast.StructLit{}, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("decode change value: %w", err)
	}
	expr, err := anyToCueExpr(value)
	if err != nil {
		return nil, err
	}
	wrapped := fmt.Sprintf("patch: %s", exprString(expr))
	file, err := parser.ParseFile("patch.cue", wrapped, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse generated cue value: %w", err)
	}
	for _, decl := range file.Decls {
		field, ok := decl.(*ast.Field)
		if ok && cueLabelString(field.Label) == "patch" {
			return field.Value, nil
		}
	}
	return nil, fmt.Errorf("generated cue value missing patch field")
}

func anyToCueExpr(value any) (ast.Expr, error) {
	switch typed := value.(type) {
	case map[string]any:
		elts := make([]ast.Decl, 0, len(typed))
		for key, item := range typed {
			expr, err := anyToCueExpr(item)
			if err != nil {
				return nil, err
			}
			elts = append(elts, &ast.Field{Label: ast.NewIdent(key), Value: expr})
		}
		return &ast.StructLit{Elts: elts}, nil
	case []any:
		elts := make([]ast.Expr, 0, len(typed))
		for _, item := range typed {
			expr, err := anyToCueExpr(item)
			if err != nil {
				return nil, err
			}
			elts = append(elts, expr)
		}
		return &ast.ListLit{Elts: elts}, nil
	case string:
		return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(typed)}, nil
	case bool:
		if typed {
			return &ast.Ident{Name: "true"}, nil
		}
		return &ast.Ident{Name: "false"}, nil
	case float64:
		if typed == float64(int64(typed)) {
			return &ast.BasicLit{Kind: token.INT, Value: strconv.FormatInt(int64(typed), 10)}, nil
		}
		return &ast.BasicLit{Kind: token.FLOAT, Value: strconv.FormatFloat(typed, 'f', -1, 64)}, nil
	case nil:
		return &ast.Ident{Name: "null"}, nil
	default:
		return nil, fmt.Errorf("unsupported json value type %T", value)
	}
}

func exprString(expr ast.Expr) string {
	b, err := format.Node(expr)
	if err != nil {
		return fmt.Sprint(expr)
	}
	return strings.TrimSpace(string(bytes.TrimSpace(b)))
}
