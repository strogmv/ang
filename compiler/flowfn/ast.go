package flowfn

type Position struct {
	Offset int
	Line   int
	Column int
}

type Program struct {
	Nodes []Node
}

type Step struct {
	Action   string
	Args     map[string]any
	Children map[string][]Step
	Line     int
	Column   int
}

type Node interface {
	node()
	Position() Position
}

type CallNode struct {
	Pos    Position
	Action string
	Args   map[string]Value
	Blocks map[string][]Node
}

type IfNode struct {
	Pos       Position
	Condition string
	Then      []Node
	Else      []Node
}

type ForNode struct {
	Pos   Position
	Alias string
	Each  string
	Do    []Node
}

type TryNode struct {
	Pos   Position
	Do    []Node
	Catch []Node
}

type FragmentNode struct {
	Pos  Position
	Name string
	Body []Node
}

type UseNode struct {
	Pos  Position
	Name string
}

func (*CallNode) node()     {}
func (*IfNode) node()       {}
func (*ForNode) node()      {}
func (*TryNode) node()      {}
func (*FragmentNode) node() {}
func (*UseNode) node()      {}

func (n *CallNode) Position() Position     { return n.Pos }
func (n *IfNode) Position() Position       { return n.Pos }
func (n *ForNode) Position() Position      { return n.Pos }
func (n *TryNode) Position() Position      { return n.Pos }
func (n *FragmentNode) Position() Position { return n.Pos }
func (n *UseNode) Position() Position      { return n.Pos }

type ValueKind string

const (
	ValueString ValueKind = "string"
	ValueInt    ValueKind = "int"
	ValueBool   ValueKind = "bool"
	ValueExpr   ValueKind = "expr"
)

type Value struct {
	Kind ValueKind
	Raw  string
	Int  int
	Bool bool
}

func (v Value) Interface() any {
	switch v.Kind {
	case ValueString:
		return v.Raw
	case ValueInt:
		return v.Int
	case ValueBool:
		return v.Bool
	default:
		return v.Raw
	}
}
