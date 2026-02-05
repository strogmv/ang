#!/usr/bin/env python3
import json
import re
from pathlib import Path
from typing import Any, Dict, List, Tuple

ROOT = Path("/home/strog/work/ang")
MAIN_SWAGGER = Path("/home/strog/work/dealingi-back/cmd/docs/swagger.json")
SEARCH_SWAGGER = Path("/home/strog/work/dealingi-back/serives/search-service/docs/swagger.json")

PUBLIC_HINTS = [
    "login",
    "register",
    "sign up",
    "signup",
    "refresh",
    "password",
    "reset",
    "google",
    "oauth",
]


def cue_ident(name: str) -> str:
    s = re.sub(r"[^0-9A-Za-z]+", "_", name)
    s = s.strip("_")
    if not s:
        s = "schema"
    if s[0].isdigit():
        s = "schema_" + s
    return s


def ref_to_ident(ref: str) -> str:
    if ref.startswith("#/components/schemas/"):
        name = ref.split("/", 3)[-1]
        return cue_ident(name)
    return cue_ident(ref)


def schema_type(schema: Dict[str, Any], indent: int = 0, domain_prefix: bool = False) -> str:
    if not schema:
        return "_"
    if "$ref" in schema:
        ident = ref_to_ident(schema["$ref"])
        return f"domain.{ident}" if domain_prefix else ident
    if "allOf" in schema:
        parts = [schema_type(s, indent, domain_prefix) for s in schema["allOf"]]
        return " & ".join(parts)
    if "oneOf" in schema:
        parts = [schema_type(s, indent, domain_prefix) for s in schema["oneOf"]]
        return " | ".join(parts)
    if "anyOf" in schema:
        parts = [schema_type(s, indent, domain_prefix) for s in schema["anyOf"]]
        return " | ".join(parts)

    typ = schema.get("type")
    if typ == "array":
        items = schema.get("items", {})
        return f"[...{schema_type(items, indent, domain_prefix)}]"
    if typ == "object" or "properties" in schema or "additionalProperties" in schema:
        props = schema.get("properties", {})
        required = set(schema.get("required", []))
        additional = schema.get("additionalProperties", None)
        if not props and additional:
            add_type = schema_type(additional, indent, domain_prefix)
            return "{\n" + "  " * (indent + 1) + f"[string]: {add_type}\n" + "  " * indent + "}"
        if not props:
            return "{}"
        lines = ["{"]
        for name, prop in props.items():
            field_name = cue_ident(name)
            opt = "" if name in required else "?"
            field_type = schema_type(prop, indent + 1, domain_prefix)
            lines.append("  " * (indent + 1) + f"{field_name}{opt}: {field_type}")
        lines.append("  " * indent + "}")
        return "\n".join(lines)
    if typ == "integer":
        return "int"
    if typ == "number":
        return "number"
    if typ == "boolean":
        return "bool"
    if typ == "string":
        return "string"
    return "_"


def render_schema_def(name: str, schema: Dict[str, Any]) -> str:
    ident = cue_ident(name)
    body = schema_type(schema, 0, False)
    return f"{ident}: {body}"


def op_name_from(path: str, method: str, summary: str, tag: str, used: Dict[str, int]) -> str:
    base = summary or f"{method} {path}"
    s = re.sub(r"[^0-9A-Za-z]+", " ", base).title().replace(" ", "")
    if not s:
        s = re.sub(r"[^0-9A-Za-z]+", " ", f"{method} {path}").title().replace(" ", "")
    if not s:
        s = "Operation"
    if s in used:
        used[s] += 1
        s = f"{s}{used[s]}"
    else:
        used[s] = 0
    return s


def is_public(summary: str, path: str, tag: str) -> bool:
    hay = " ".join([summary or "", path, tag]).lower()
    return any(h in hay for h in PUBLIC_HINTS)


def parse_parameters(params: List[Dict[str, Any]]) -> Dict[str, Dict[str, Any]]:
    fields = {}
    for p in params or []:
        name = p.get("name")
        if not name:
            continue
        schema = p.get("schema", {})
        fields[name] = schema
    return fields


def is_non_object_body(schema: Dict[str, Any]) -> bool:
    if not isinstance(schema, dict):
        return False
    if "$ref" in schema:
        return False
    t = schema.get("type")
    if t in ("array", "string", "integer", "number", "boolean"):
        return True
    if t == "object" or "properties" in schema or "additionalProperties" in schema:
        return False
    if "items" in schema:
        return True
    return False


def merge_body_and_params(body_schema: Any, params: Dict[str, Dict[str, Any]]) -> Tuple[str, bool]:
    if body_schema is None and not params:
        return "{}", False
    if isinstance(body_schema, dict) and "$ref" in body_schema:
        ref = ref_to_ident(body_schema["$ref"])
        if params:
            parts = [f"domain.{ref}", "{\n" + "\n".join(
                [f"  {cue_ident(k)}: {schema_type(v, 1, True)}" for k, v in params.items()]
            ) + "\n}"]
            return " & ".join(parts), True
        return f"domain.{ref}", True
    if body_schema:
        if is_non_object_body(body_schema):
            body = "{\n  body: " + schema_type(body_schema, 1, True) + "\n}"
        else:
            body = schema_type(body_schema, 0, True)
        if params:
            params_block = "{\n" + "\n".join(
                [f"  {cue_ident(k)}: {schema_type(v, 1, True)}" for k, v in params.items()]
            ) + "\n}"
            return body + " & " + params_block, True
        return body, True
    params_block = "{\n" + "\n".join(
        [f"  {cue_ident(k)}: {schema_type(v, 1, True)}" for k, v in params.items()]
    ) + "\n}"
    return params_block, True


def response_schema(responses: Dict[str, Any]) -> Any:
    for code in ["200", "201", "202", "204"]:
        if code in responses:
            resp = responses[code]
            content = resp.get("content", {})
            app_json = content.get("application/json")
            if app_json and app_json.get("schema"):
                return app_json["schema"]
            if code == "204":
                return None
    return None


def generate_from_swagger(swagger_path: Path, service_override: str = "") -> Tuple[Dict[str, Dict[str, Any]], List[Dict[str, Any]], List[str]]:
    data = json.loads(swagger_path.read_text())
    schemas = data.get("components", {}).get("schemas", {})

    ops = []
    public_ops = []
    used_names: Dict[str, int] = {}

    paths = data.get("paths", {})
    for path, methods in paths.items():
        for method, op in methods.items():
            summary = op.get("summary", "")
            tags = op.get("tags") or ["api"]
            tag = tags[0]
            service = service_override or tag
            op_name = op_name_from(path, method, summary, tag, used_names)

            params = []
            if "parameters" in methods:
                params.extend(methods.get("parameters", []))
            params.extend(op.get("parameters", []) or [])
            param_fields = parse_parameters(params)

            req_body = op.get("requestBody", {})
            body_schema = None
            if req_body:
                content = req_body.get("content", {})
                app_json = content.get("application/json")
                if app_json and app_json.get("schema"):
                    body_schema = app_json["schema"]

            input_expr, has_input = merge_body_and_params(body_schema, param_fields)

            out_schema = response_schema(op.get("responses", {}))
            output_expr = "{}"
            if out_schema is None:
                output_expr = "{}"
            else:
                if isinstance(out_schema, dict) and "$ref" in out_schema:
                    output_expr = f"domain.{ref_to_ident(out_schema['$ref'])}"
                else:
                    output_expr = schema_type(out_schema, 0, True)

            ops.append({
                "name": op_name,
                "service": service,
                "input": input_expr,
                "output": output_expr,
                "method": method.upper(),
                "path": path,
                "summary": summary,
                "public": is_public(summary, path, tag),
            })
            if is_public(summary, path, tag):
                public_ops.append(op_name)

    return schemas, ops, public_ops


def write_domain(schemas_list: List[Dict[str, Dict[str, Any]]]) -> None:
    lines = ["package domain", "", "// Auto-generated from swagger.json. Do not edit manually.", ""]
    for schemas in schemas_list:
        for name, schema in schemas.items():
            lines.append(render_schema_def(name, schema))
            lines.append("")
    out = "\n".join(lines).rstrip() + "\n"
    (ROOT / "cue/domain/swagger_domain.cue").write_text(out)


def write_ops(ops: List[Dict[str, Any]]) -> None:
    lines = [
        "package api",
        "",
        "import (",
        "  \"github.com/strog/ang/cue/schema\"",
        "  \"github.com/strog/ang/cue/domain\"",
        ")",
        "",
        "// Auto-generated from swagger.json. Do not edit manually.",
        "",
    ]
    for op in ops:
        lines.append(f"{op['name']}: schema.#Operation & {{")
        lines.append(f"  service: \"{op['service']}\"")
        lines.append(f"  input: {op['input']}")
        lines.append(f"  output: {op['output']}")
        lines.append("}")
        lines.append("")
    out = "\n".join(lines).rstrip() + "\n"
    (ROOT / "cue/api/swagger_ops.cue").write_text(out)


def write_http(ops: List[Dict[str, Any]]) -> None:
    lines = [
        "package api",
        "",
        "import (",
        "  \"github.com/strog/ang/cue/schema\"",
        ")",
        "",
        "// Auto-generated from swagger.json. Do not edit manually.",
        "",
        "HTTP: schema.#HTTP & {",
    ]
    for op in ops:
        lines.append(f"  {op['name']}: {{")
        lines.append(f"    method: \"{op['method']}\"")
        lines.append(f"    path: \"{op['path']}\"")
        if op["summary"]:
            safe = op["summary"].replace('"', "'")
            lines.append(f"    description: \"{safe}\"")
        if not op["public"]:
            lines.append("    auth: {")
            lines.append("      type: \"jwt\"")
            lines.append("    }")
        lines.append("  }")
    lines.append("}")
    out = "\n".join(lines).rstrip() + "\n"
    (ROOT / "cue/api/http_swagger.cue").write_text(out)


def write_public_allowlist(ops: List[str]) -> None:
    lines = ["package policies", "", "// Auto-generated public endpoints from swagger.json.", "", "publicEndpointsSwagger: {"]
    for name in sorted(set(ops)):
        lines.append(f"  {name}: true")
    lines.append("}")
    out = "\n".join(lines).rstrip() + "\n"
    (ROOT / "cue/policies/auth_public_swagger.cue").write_text(out)


def main() -> None:
    schemas_main, ops_main, public_main = generate_from_swagger(MAIN_SWAGGER)
    schemas_search, ops_search, public_search = generate_from_swagger(SEARCH_SWAGGER, service_override="search")

    write_domain([schemas_main, schemas_search])
    write_ops(ops_main + ops_search)
    write_http(ops_main + ops_search)
    write_public_allowlist(public_main + public_search)


if __name__ == "__main__":
    main()
