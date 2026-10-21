package normalizer

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"cuelang.org/go/cue"
)

// GRPCDef is the normalized `GRPC:` declaration of cue/api (schema.#GRPC).
type GRPCDef struct {
	Package         string        `json:"package,omitempty"`
	GoPackage       string        `json:"go_package,omitempty"`
	CSharpNamespace string        `json:"csharp_namespace,omitempty"`
	CopyProtoTo     []string      `json:"copy_proto_to,omitempty"`
	Listen          string        `json:"listen,omitempty"`
	Handlers        string        `json:"handlers,omitempty"`
	CSharpOut       string        `json:"csharp_out,omitempty"`
	Reflection      bool          `json:"reflection,omitempty"`
	Enums           []GRPCEnum    `json:"enums,omitempty"`
	Messages        []GRPCMessage `json:"messages,omitempty"`
	Services        []GRPCService `json:"services,omitempty"`
	Source          string        `json:"source,omitempty"`
}

type GRPCEnum struct {
	Name          string          `json:"name,omitempty"`
	Description   string          `json:"description,omitempty"`
	Values        []GRPCEnumValue `json:"values,omitempty"` // sorted by number
	Reserved      []int           `json:"reserved,omitempty"`
	ReservedNames []string        `json:"reserved_names,omitempty"`
	Source        string          `json:"source,omitempty"`
}

type GRPCEnumValue struct {
	Name        string `json:"name,omitempty"`
	Number      int    `json:"number,omitempty"`
	Description string `json:"description,omitempty"`
}

type GRPCMessage struct {
	Name          string      `json:"name,omitempty"`
	Description   string      `json:"description,omitempty"`
	Fields        []GRPCField `json:"fields,omitempty"` // plain fields, sorted by number
	Oneofs        []GRPCOneof `json:"oneofs,omitempty"` // sorted by name
	Reserved      []int       `json:"reserved,omitempty"`
	ReservedNames []string    `json:"reserved_names,omitempty"`
	Source        string      `json:"source,omitempty"`
}

type GRPCOneof struct {
	Name    string      `json:"name,omitempty"`
	Members []GRPCField `json:"members,omitempty"` // sorted by number
}

type GRPCField struct {
	Name        string `json:"name,omitempty"`
	Number      int    `json:"number,omitempty"`
	Type        string `json:"type,omitempty"`
	Repeated    bool   `json:"repeated,omitempty"`
	MapKey      string `json:"map_key,omitempty"`
	Optional    bool   `json:"optional,omitempty"`
	Deprecated  bool   `json:"deprecated,omitempty"`
	Description string `json:"description,omitempty"`
}

type GRPCService struct {
	Name    string       `json:"name,omitempty"`
	Methods []GRPCMethod `json:"methods,omitempty"` // sorted by name
}

type GRPCMethod struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Stream      string `json:"stream,omitempty"` // none|server|client|bidi
	Request     string `json:"request,omitempty"`
	Response    string `json:"response,omitempty"`
	Source      string `json:"source,omitempty"`
}

var (
	grpcScalars   = map[string]bool{"double": true, "float": true, "int32": true, "int64": true, "uint32": true, "uint64": true, "bool": true, "string": true, "bytes": true}
	grpcFieldName = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)
	grpcTypeName  = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
)

// ExtractGRPC reads the top-level `GRPC` value of cue/api. It returns nil when
// the project declares none. Every check names the CUE position.
func (n *Normalizer) ExtractGRPC(val cue.Value) (*GRPCDef, error) {
	if !val.Exists() || val.IncompleteKind() == cue.BottomKind {
		return nil, nil
	}
	g := val.LookupPath(cue.ParsePath("GRPC"))
	if !g.Exists() {
		return nil, nil
	}
	def := &GRPCDef{
		Package:         getString(g, "package"),
		GoPackage:       getString(g, "go_package"),
		CSharpNamespace: getString(g, "csharp_namespace"),
		CopyProtoTo:     getStringList(g, "copy_proto_to"),
		Listen:          getStringWithDefault(g, "listen", ":50051"),
		Handlers:        strings.Trim(getString(g, "handlers"), "/ "),
		CSharpOut:       strings.TrimSpace(getString(g, "csharp_out")),
		Reflection:      true,
		Source:          formatPos(g),
	}
	if rv := g.LookupPath(cue.ParsePath("reflection")); rv.Exists() {
		if b, err := rv.Bool(); err == nil {
			def.Reflection = b
		}
	}
	if def.Package == "" {
		return nil, fmt.Errorf("%s: GRPC.package is required", def.Source)
	}

	if err := forEachField(g, "enums", func(name string, v cue.Value) error {
		e := GRPCEnum{Name: name, Description: getString(v, "description"), Source: formatPos(v)}
		e.Reserved = getIntList(v, "reserved")
		e.ReservedNames = getStringList(v, "reserved_names")
		vals := v.LookupPath(cue.ParsePath("values"))
		it, err := vals.List()
		if err != nil {
			return fmt.Errorf("%s: enum %s: values: %w", formatPos(v), name, err)
		}
		for it.Next() {
			ev := it.Value()
			num, _ := ev.LookupPath(cue.ParsePath("n")).Int64()
			e.Values = append(e.Values, GRPCEnumValue{Name: getString(ev, "name"), Number: int(num), Description: getString(ev, "description")})
		}
		sort.Slice(e.Values, func(i, j int) bool { return e.Values[i].Number < e.Values[j].Number })
		def.Enums = append(def.Enums, e)
		return nil
	}); err != nil {
		return nil, err
	}

	if err := forEachField(g, "messages", func(name string, v cue.Value) error {
		m := GRPCMessage{Name: name, Description: getString(v, "description"), Source: formatPos(v)}
		m.Reserved = getIntList(v, "reserved")
		m.ReservedNames = getStringList(v, "reserved_names")
		if err := forEachField(v, "fields", func(fname string, fv cue.Value) error {
			m.Fields = append(m.Fields, readGRPCField(fname, fv))
			return nil
		}); err != nil {
			return err
		}
		sort.Slice(m.Fields, func(i, j int) bool { return m.Fields[i].Number < m.Fields[j].Number })
		if err := forEachField(v, "oneofs", func(oname string, ov cue.Value) error {
			o := GRPCOneof{Name: oname}
			it, err := ov.Fields()
			if err != nil {
				return err
			}
			for it.Next() {
				o.Members = append(o.Members, readGRPCField(it.Selector().String(), it.Value()))
			}
			sort.Slice(o.Members, func(i, j int) bool { return o.Members[i].Number < o.Members[j].Number })
			m.Oneofs = append(m.Oneofs, o)
			return nil
		}); err != nil {
			return err
		}
		sort.Slice(m.Oneofs, func(i, j int) bool { return m.Oneofs[i].Name < m.Oneofs[j].Name })
		def.Messages = append(def.Messages, m)
		return nil
	}); err != nil {
		return nil, err
	}

	if err := forEachField(g, "services", func(sname string, sv cue.Value) error {
		svc := GRPCService{Name: sname}
		it, err := sv.Fields()
		if err != nil {
			return err
		}
		for it.Next() {
			mv := it.Value()
			svc.Methods = append(svc.Methods, GRPCMethod{
				Name:        it.Selector().String(),
				Description: getString(mv, "description"),
				Stream:      getStringWithDefault(mv, "stream", "none"),
				Request:     getString(mv, "request"),
				Response:    getString(mv, "response"),
				Source:      formatPos(mv),
			})
		}
		sort.Slice(svc.Methods, func(i, j int) bool { return svc.Methods[i].Name < svc.Methods[j].Name })
		def.Services = append(def.Services, svc)
		return nil
	}); err != nil {
		return nil, err
	}

	sort.Slice(def.Enums, func(i, j int) bool { return def.Enums[i].Name < def.Enums[j].Name })
	sort.Slice(def.Messages, func(i, j int) bool { return def.Messages[i].Name < def.Messages[j].Name })
	sort.Slice(def.Services, func(i, j int) bool { return def.Services[i].Name < def.Services[j].Name })

	if err := def.Validate(); err != nil {
		return nil, err
	}
	return def, nil
}

func readGRPCField(name string, v cue.Value) GRPCField {
	num, _ := v.LookupPath(cue.ParsePath("n")).Int64()
	f := GRPCField{Name: name, Number: int(num), Type: getString(v, "type"), MapKey: getString(v, "map_key"), Description: getString(v, "description")}
	f.Repeated, _ = v.LookupPath(cue.ParsePath("repeated")).Bool()
	f.Optional, _ = v.LookupPath(cue.ParsePath("optional")).Bool()
	f.Deprecated, _ = v.LookupPath(cue.ParsePath("deprecated")).Bool()
	return f
}

func forEachField(v cue.Value, path string, fn func(name string, v cue.Value) error) error {
	c := v.LookupPath(cue.ParsePath(path))
	if !c.Exists() {
		return nil
	}
	it, err := c.Fields()
	if err != nil {
		return fmt.Errorf("%s: %s: %w", formatPos(c), path, err)
	}
	for it.Next() {
		if err := fn(strings.Trim(it.Selector().String(), `"`), it.Value()); err != nil {
			return err
		}
	}
	return nil
}

func getIntList(v cue.Value, path string) []int {
	c := v.LookupPath(cue.ParsePath(path))
	if !c.Exists() {
		return nil
	}
	it, err := c.List()
	if err != nil {
		return nil
	}
	var out []int
	for it.Next() {
		if i, err := it.Value().Int64(); err == nil {
			out = append(out, int(i))
		}
	}
	return out
}

// Validate checks names, numbers and type references. The wire contract is the
// numbers, so a duplicate, a reserved or a protobuf-internal number is an error.
func (g *GRPCDef) Validate() error {
	enums := map[string]*GRPCEnum{}
	msgs := map[string]*GRPCMessage{}
	for i := range g.Enums {
		e := &g.Enums[i]
		if !grpcTypeName.MatchString(e.Name) {
			return fmt.Errorf("%s: enum name %q must be PascalCase", e.Source, e.Name)
		}
		enums[e.Name] = e
	}
	for i := range g.Messages {
		m := &g.Messages[i]
		if !grpcTypeName.MatchString(m.Name) {
			return fmt.Errorf("%s: message name %q must be PascalCase", m.Source, m.Name)
		}
		if _, dup := enums[m.Name]; dup {
			return fmt.Errorf("%s: %q is both an enum and a message", m.Source, m.Name)
		}
		msgs[m.Name] = m
	}

	for _, e := range g.Enums {
		names, nums := map[string]bool{}, map[int]bool{}
		res := intSet(e.Reserved)
		for _, v := range e.Values {
			if !regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`).MatchString(v.Name) {
				return fmt.Errorf("%s: enum %s value %q must be UPPER_SNAKE", e.Source, e.Name, v.Name)
			}
			if names[v.Name] || nums[v.Number] {
				return fmt.Errorf("%s: enum %s: duplicate value name %q or number %d", e.Source, e.Name, v.Name, v.Number)
			}
			if res[v.Number] {
				return fmt.Errorf("%s: enum %s: number %d is reserved", e.Source, e.Name, v.Number)
			}
			names[v.Name], nums[v.Number] = true, true
		}
	}

	checkType := func(m GRPCMessage, f GRPCField) error {
		if !grpcScalars[f.Type] && enums[f.Type] == nil && msgs[f.Type] == nil {
			return fmt.Errorf("%s: message %s field %s: unknown type %q", m.Source, m.Name, f.Name, f.Type)
		}
		if f.MapKey != "" {
			if f.Repeated {
				return fmt.Errorf("%s: message %s field %s: map_key excludes repeated", m.Source, m.Name, f.Name)
			}
			if f.Type == "bytes" || f.Type == "double" || f.Type == "float" {
				return fmt.Errorf("%s: message %s field %s: invalid map value type %q", m.Source, m.Name, f.Name, f.Type)
			}
		}
		if f.Optional && (f.Repeated || f.MapKey != "") {
			return fmt.Errorf("%s: message %s field %s: optional excludes repeated/map", m.Source, m.Name, f.Name)
		}
		if !grpcFieldName.MatchString(f.Name) {
			return fmt.Errorf("%s: message %s field %q must be lower_snake_case", m.Source, m.Name, f.Name)
		}
		return nil
	}
	for _, m := range g.Messages {
		nums := map[int]string{}
		names := map[string]bool{}
		res := intSet(m.Reserved)
		resN := map[string]bool{}
		for _, r := range m.ReservedNames {
			resN[r] = true
		}
		use := func(f GRPCField) error {
			if err := checkType(m, f); err != nil {
				return err
			}
			if f.Number >= 19000 && f.Number <= 19999 {
				return fmt.Errorf("%s: message %s field %s: number %d is reserved by protobuf", m.Source, m.Name, f.Name, f.Number)
			}
			if res[f.Number] || resN[f.Name] {
				return fmt.Errorf("%s: message %s field %s: number %d or name is in reserved", m.Source, m.Name, f.Name, f.Number)
			}
			if other, dup := nums[f.Number]; dup {
				return fmt.Errorf("%s: message %s: number %d used by both %s and %s", m.Source, m.Name, f.Number, other, f.Name)
			}
			if names[f.Name] {
				return fmt.Errorf("%s: message %s: duplicate field name %s", m.Source, m.Name, f.Name)
			}
			nums[f.Number], names[f.Name] = f.Name, true
			return nil
		}
		for _, f := range m.Fields {
			if err := use(f); err != nil {
				return err
			}
		}
		for _, o := range m.Oneofs {
			for _, f := range o.Members {
				if f.Repeated || f.MapKey != "" || f.Optional {
					return fmt.Errorf("%s: message %s oneof %s member %s: repeated/map/optional not allowed in oneof", m.Source, m.Name, o.Name, f.Name)
				}
				if err := use(f); err != nil {
					return err
				}
			}
		}
	}

	if len(g.Services) == 0 {
		return fmt.Errorf("%s: GRPC.services is empty", g.Source)
	}
	for _, s := range g.Services {
		if !grpcTypeName.MatchString(s.Name) {
			return fmt.Errorf("%s: service name %q must be PascalCase", g.Source, s.Name)
		}
		for _, m := range s.Methods {
			if !grpcTypeName.MatchString(m.Name) {
				return fmt.Errorf("%s: rpc name %q must be PascalCase", m.Source, m.Name)
			}
			if msgs[m.Request] == nil || msgs[m.Response] == nil {
				return fmt.Errorf("%s: rpc %s.%s: request %q and response %q must be declared messages", m.Source, s.Name, m.Name, m.Request, m.Response)
			}
		}
	}
	return nil
}

func intSet(xs []int) map[int]bool {
	m := map[int]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}
