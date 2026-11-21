package emitter

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/bufbuild/protocompile"
	"github.com/strogmv/ang/angir/ir"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// RenderGDScriptContract emits one preloadable script containing typed message
// classes reachable from RPC inputs/outputs. It consumes the same compiled proto
// as protoc-gen-go, including synthetic map entries and proto3 presence.
func RenderGDScriptContract(def *ir.GRPC, module string) ([]byte, error) {
	p := GRPCProtoPath(def)
	c := protocompile.Compiler{Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{Accessor: protocompile.SourceAccessorFromMap(map[string]string{p: RenderGRPCProto(def, module)})})}
	files, err := c.Compile(context.Background(), p)
	if err != nil {
		return nil, fmt.Errorf("gdscript proto: %w", err)
	}
	return renderGDDescriptor(files[0], def), nil
}

func gdName(d protoreflect.Descriptor) string {
	name := strings.TrimPrefix(string(d.FullName()), string(d.ParentFile().Package())+".")
	// Escape underscores before joining nesting levels: Outer.Inner cannot
	// collide with a top-level Outer__Inner. Builtins cannot name inner classes.
	name = strings.Join(strings.Split(strings.ReplaceAll(name, "_", "_u"), "."), "__")
	builtins := " ViewCache ProtoMessage Object RefCounted Resource Node Node2D Node3D Control Vector2 Vector2i Vector3 Vector3i Vector4 Vector4i Transform2D Transform3D Projection Basis Color Quaternion Plane Rect2 Rect2i AABB Timer Image Texture Texture2D Mesh ArrayMesh Animation AnimationPlayer AnimationTree Skeleton3D Camera3D Light3D Sprite2D Sprite3D Window SceneTree PackedScene AStar2D AStar3D ResourceLoader ResourceSaver AABB RID Callable Signal String StringName NodePath Dictionary Array PackedByteArray JSON FileAccess Time OS Engine "
	if strings.Contains(builtins, " "+name+" ") || strings.Contains(gdKeywords, " "+name+" ") {
		name = "_PB_" + name
	}
	return name
}

// trait and namespace are reserved by GDScript 4 for later use: a field of that name does not parse (the hero file's
// choice has a trait).
const gdKeywords = " and or not when abstract trait namespace if elif else for while match break continue pass return class class_name extends is in as self tool signal func static const enum var onready export setget breakpoint preload yield assert void null true false await super set get int float bool PI TAU INF NAN "

// Keep proto names on the wire even when a field is a GDScript keyword or
// shadows RefCounted/Object members. Prefixing is injective (including pb_*).
func gdField(f protoreflect.FieldDescriptor) string {
	return GDScriptFieldName(string(f.Name()))
}

// GDScriptFieldName shares the contract's injective property mapping with typed UI emitters.
func GDScriptFieldName(n string) string {
	reserved := " class class_name extends is in as self tool signal func static const enum var onready export setget breakpoint preload yield assert void null true false if elif else for while match break continue pass return await super set get int float bool String Array Dictionary name new from_dict to_dict get_class get_script get_meta set_meta get_meta_list has_meta remove_meta get_instance_id is_class is_queued_for_deletion notification free call callv call_deferred emit_signal connect disconnect is_connected duplicate reference unreference init_ref get_reference_count set_script get_property_list get_method_list get_signal_list get_signal_connection_list get_incoming_connections has_method has_signal has_user_signal add_user_signal to_string set_deferred notify_property_list_changed property_can_revert property_get_revert tr tr_n set_block_signals is_blocking_signals cancel_free "
	if strings.Contains(reserved, " "+n+" ") || strings.Contains(gdKeywords, " "+n+" ") || strings.HasPrefix(n, "pb_") {
		return "pb_" + n
	}
	return n
}

func gdType(f protoreflect.FieldDescriptor) string {
	if f.IsMap() {
		return "Dictionary"
	}
	if f.IsList() {
		return "Array[" + gdScalarType(f) + "]"
	}
	if f.HasPresence() && f.Kind() != protoreflect.MessageKind {
		return "Variant"
	}
	return gdScalarType(f)
}
func gdScalarType(f protoreflect.FieldDescriptor) string {
	switch f.Kind() {
	case protoreflect.MessageKind:
		return gdName(f.Message())
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "Variant" // lossless decimal string above Godot's signed range
	case protoreflect.EnumKind:
		return "Variant" // names, or numeric unknown enum values
	case protoreflect.StringKind:
		return "String"
	case protoreflect.BytesKind:
		return "PackedByteArray"
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return "float"
	default:
		return "int"
	}
}
func gdDefault(f protoreflect.FieldDescriptor) string {
	if f.IsMap() {
		return "{}"
	}
	if f.IsList() {
		return "[]"
	}
	if f.HasPresence() {
		return "null"
	}
	switch f.Kind() {
	case protoreflect.EnumKind:
		return fmt.Sprintf("%q", f.Enum().Values().Get(0).Name())
	case protoreflect.StringKind:
		return `""`
	case protoreflect.BytesKind:
		return "PackedByteArray()"
	case protoreflect.BoolKind:
		return "false"
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return "0.0"
	default:
		return "0"
	}
}
func gdDecode(f protoreflect.FieldDescriptor, v string) string {
	switch f.Kind() {
	case protoreflect.MessageKind:
		return gdName(f.Message()) + ".from_dict(" + v + ")"
	case protoreflect.EnumKind:
		return gdName(f.Enum()) + ".from_json(" + v + ")"
	case protoreflect.StringKind:
		return "str(" + v + ")"
	case protoreflect.BytesKind:
		return "Marshalls.base64_to_raw(" + v + ")"
	case protoreflect.BoolKind:
		return "bool(" + v + ")"
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return "_number(" + v + ")"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "_uint64(" + v + ")"
	default:
		return "int(" + v + ")"
	}
}
func gdEncode(f protoreflect.FieldDescriptor, v string) string {
	switch f.Kind() {
	case protoreflect.EnumKind:
		return gdName(f.Enum()) + ".from_json(" + v + ")"
	case protoreflect.MessageKind:
		return v + ".to_dict()"
	case protoreflect.BytesKind:
		return "Marshalls.raw_to_base64(" + v + ")"
	case protoreflect.Int64Kind, protoreflect.Uint64Kind, protoreflect.Sint64Kind, protoreflect.Fixed64Kind, protoreflect.Sfixed64Kind:
		return "str(" + v + ")"
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return "_json_number(" + v + ")"
	default:
		return v
	}
}

func renderGDDescriptor(file protoreflect.FileDescriptor, definitions ...*ir.GRPC) []byte {
	msgs := map[string]protoreflect.MessageDescriptor{}
	enums := map[string]protoreflect.EnumDescriptor{}
	var visit func(protoreflect.MessageDescriptor)
	visit = func(m protoreflect.MessageDescriptor) {
		if _, ok := msgs[gdName(m)]; ok {
			return
		}
		msgs[gdName(m)] = m
		for i := 0; i < m.Fields().Len(); i++ {
			f := m.Fields().Get(i)
			if f.IsMap() {
				f = f.MapValue()
			}
			if f.Message() != nil {
				visit(f.Message())
			}
			if f.Enum() != nil {
				enums[gdName(f.Enum())] = f.Enum()
			}
		}
	}
	for i := 0; i < file.Services().Len(); i++ {
		s := file.Services().Get(i)
		for j := 0; j < s.Methods().Len(); j++ {
			visit(s.Methods().Get(j).Input())
			visit(s.Methods().Get(j).Output())
		}
	}
	var def *ir.GRPC
	if len(definitions) > 0 {
		def = definitions[0]
	}
	delivery := def != nil && def.ViewSubscription != ""
	static := map[string]string{}
	if delivery {
		for _, m := range def.Messages {
			for _, f := range m.Fields {
				if f.Delivery == "static_on_change" {
					static[def.Package+"."+m.Name+"."+f.Name] = f.Revision
				}
			}
		}
	}
	needsCache := map[protoreflect.FullName]bool{}
	if delivery {
		for _, m := range msgs {
			for i := 0; i < m.Fields().Len(); i++ {
				if static[string(m.Fields().Get(i).FullName())] != "" {
					needsCache[m.FullName()] = true
				}
			}
		}
		for changed := true; changed; {
			changed = false
			for _, m := range msgs {
				for i := 0; i < m.Fields().Len(); i++ {
					f := m.Fields().Get(i)
					if f.IsMap() {
						f = f.MapValue()
					}
					if f.Message() != nil && needsCache[f.Message().FullName()] && !needsCache[m.FullName()] {
						needsCache[m.FullName()] = true
						changed = true
					}
				}
			}
		}
	}
	var b strings.Builder
	w := func(s string, a ...any) { fmt.Fprintf(&b, s, a...) }
	w("# Code generated by ANG from the GRPC contract. DO NOT EDIT.\n# Canonical protojson (UseProtoNames); preload this script for message classes.\nextends RefCounted\n\n")
	names := make([]string, 0, len(enums))
	for n := range enums {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		e := enums[n]
		w("class %s:\n", n)
		for i := 0; i < e.Values().Len(); i++ {
			v := e.Values().Get(i)
			w("\tconst %s = %q\n", v.Name(), v.Name())
		}

		w("\n\tstatic func from_json(v):\n\t\tmatch str(v):\n")
		for i := 0; i < e.Values().Len(); i++ {
			v := e.Values().Get(i)
			w("\t\t\t%q: return %q\n", fmt.Sprint(v.Number()), v.Name())
		}
		w("\t\treturn v\n\n")
	}
	if delivery {
		w("class ViewCache extends RefCounted:\n\tvar values: Dictionary = {}\n\n\tfunc reset() -> void:\n\t\tvalues.clear()\n")
		responses := map[string]bool{}
		for _, svc := range def.Services {
			for _, rpc := range svc.Methods {
				responses[rpc.Response] = true
			}
		}
		responseNames := make([]string, 0, len(responses))
		for name := range responses {
			responseNames = append(responseNames, name)
		}
		sort.Strings(responseNames)
		for _, name := range responseNames {
			m := file.Messages().ByName(protoreflect.Name(name))
			w("\n\tfunc decode_%s(d: Dictionary) -> %s:\n\t\treturn %s.from_dict(d, values)\n", gdName(m), gdName(m), gdName(m))
		}
		w("\n")
	}
	// Helpers live on a base class so every inner message can call them.
	w(`class ProtoMessage extends RefCounted:
	static func _number(v) -> float:
		if v is String:
			if v == "NaN": return NAN
			if v == "Infinity": return INF
			if v == "-Infinity": return -INF
		return float(v)

	static func _json_number(v: float):
		if is_nan(v): return "NaN"
		if is_inf(v): return "Infinity" if v > 0 else "-Infinity"
		return v

	static func _uint64(v):
		# Proto uint64 exceeds Godot's signed int range. Preserve those values
		# as decimal strings; all representable values become exact integers.
		var s := str(v)
		if s.length() > 19 or (s.length() == 19 and s > "9223372036854775807"):
			return s
		return int(v)

`)
	names = names[:0]
	for n := range msgs {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		m := msgs[n]
		w("class %s extends ProtoMessage:\n", n)
		for i := 0; i < m.Fields().Len(); i++ {
			f := m.Fields().Get(i)
			w("\tvar %s: %s = %s", gdField(f), gdType(f), gdDefault(f))
			o := f.ContainingOneof()
			if o != nil && !o.IsSynthetic() {
				w(":\n\t\tset(value):\n\t\t\t%s = value\n\t\t\tif value != null:\n", gdField(f))
				for j := 0; j < o.Fields().Len(); j++ {
					other := o.Fields().Get(j)
					if other != f {
						w("\t\t\t\t%s = null\n", gdField(other))
					}
				}
				if o.Fields().Len() == 1 {
					w("\t\t\t\tpass\n")
				}
			} else {
				w("\n")
			}
		}
		if delivery {
			w("\n\tstatic func from_dict(d: Dictionary, cache: Variant = null, path: String = %q) -> %s:\n\t\tvar result := %s.new()\n", string(m.FullName()), n, n)
		} else {
			w("\n\tstatic func from_dict(d: Dictionary) -> %s:\n\t\tvar result := %s.new()\n", n, n)
		}
		for i := 0; i < m.Fields().Len(); i++ {
			f := m.Fields().Get(i)
			fn := gdField(f)
			key := fmt.Sprintf("%q", f.Name())
			fieldPath := fmt.Sprintf("path + %q", fmt.Sprintf("/%d", f.Number()))
			decode := func(fd protoreflect.FieldDescriptor, value, suffix string) string {
				if delivery && fd.Kind() == protoreflect.MessageKind && needsCache[fd.Message().FullName()] {
					return fmt.Sprintf("%s.from_dict(%s, cache, %s%s)", gdName(fd.Message()), value, fieldPath, suffix)
				}
				return gdDecode(fd, value)
			}
			get := "d.get(" + key + ")"
			if string(f.Name()) != f.JSONName() {
				get = "d.get(" + key + ", d.get(" + fmt.Sprintf("%q", f.JSONName()) + "))"
			}
			w("\t\tvar v%d = %s\n\t\tif v%d != null:\n", i, get, i)
			if o := f.ContainingOneof(); o != nil && !o.IsSynthetic() {
				for j := 0; j < o.Fields().Len(); j++ {
					other := o.Fields().Get(j)
					if other != f {
						w("\t\t\tassert(result.%s == null, %q)\n", gdField(other), "multiple members of oneof "+string(o.Name()))
					}
				}
			}
			if f.IsMap() {
				w("\t\t\tfor key in v%d:\n", i)
				k := "key"
				switch f.MapKey().Kind() {
				case protoreflect.BoolKind:
					k = `(key == "true")`
				case protoreflect.StringKind:
				case protoreflect.Uint64Kind:
					k = "_uint64(key)"
				default:
					k = "int(key)"
				}
				w("\t\t\t\tresult.%s[%s] = %s\n", fn, k, decode(f.MapValue(), fmt.Sprintf("v%d[key]", i), ` + "/" + JSON.stringify(str(key))`))
			} else if f.IsList() {
				if delivery && f.Kind() == protoreflect.MessageKind && needsCache[f.Message().FullName()] {
					w("\t\t\tfor index in range(v%d.size()):\n\t\t\t\tresult.%s.append(%s)\n", i, fn, decode(f, fmt.Sprintf("v%d[index]", i), ` + "/" + str(index)`))
				} else {
					w("\t\t\tfor item in v%d:\n\t\t\t\tresult.%s.append(%s)\n", i, fn, gdDecode(f, "item"))
				}
			} else {
				w("\t\t\tresult.%s = %s\n", fn, decode(f, fmt.Sprintf("v%d", i), ""))
			}
		}
		for i := 0; i < m.Fields().Len(); i++ {
			f := m.Fields().Get(i)
			revision := static[string(f.FullName())]
			if revision == "" {
				continue
			}
			rev := m.Fields().ByName(protoreflect.Name(revision))
			w("\t\tif cache != null and result.%s != 0:\n", gdField(rev))
			w("\t\t\tvar key := path + %q\n\t\t\tvar previous: Dictionary = cache.get(key, {})\n", fmt.Sprintf("/%d", f.Number()))
			w("\t\t\tif not d.has(%q) and not d.has(%q) and previous.get(\"revision\") == result.%s:\n\t\t\t\tresult.%s = previous.value\n", f.Name(), f.JSONName(), gdField(rev), gdField(f))
			w("\t\t\tcache[key] = {\"revision\": result.%s, \"value\": result.%s}\n", gdField(rev), gdField(f))
		}
		w("\t\treturn result\n\n\tfunc to_dict() -> Dictionary:\n\t\tvar d: Dictionary = {}\n")
		for i := 0; i < m.Fields().Len(); i++ {
			f := m.Fields().Get(i)
			fn := gdField(f)
			condition := fn + " != " + gdDefault(f)
			if !f.HasPresence() && !f.IsList() && !f.IsMap() {
				if f.Kind() == protoreflect.EnumKind {
					condition = fmt.Sprintf("str(%s) not in [%q, \"0\"]", fn, f.Enum().Values().Get(0).Name())
				}
				if f.Kind() == protoreflect.Uint64Kind || f.Kind() == protoreflect.Fixed64Kind {
					condition = "str(" + fn + ") != \"0\""
				}
			}
			if f.IsMap() || f.IsList() || f.Kind() == protoreflect.BytesKind {
				condition = "not " + fn + ".is_empty()"
			}
			w("\t\tif %s:\n", condition)
			if f.IsMap() {
				w("\t\t\td[%q] = {}\n\t\t\tfor key in %s:\n", f.Name(), fn)
				k := "str(key)"
				if f.MapKey().Kind() == protoreflect.BoolKind {
					k = `("true" if key else "false")`
				}
				w("\t\t\t\td[%q][%s] = %s\n", f.Name(), k, gdEncode(f.MapValue(), fn+"[key]"))
			} else if f.IsList() {
				w("\t\t\td[%q] = []\n\t\t\tfor item in %s:\n\t\t\t\td[%q].append(%s)\n", f.Name(), fn, f.Name(), gdEncode(f, "item"))
			} else {
				w("\t\t\td[%q] = %s\n", f.Name(), gdEncode(f, fn))
			}
		}
		w("\t\treturn d\n\n")
	}
	return []byte(b.String())
}
