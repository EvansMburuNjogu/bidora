#!/usr/bin/env python3
"""Build Bidora's canonical OpenAPI, schema catalogue, endpoint inventory and Postman collection.

Uses only the Python standard library. Run from the backend directory:
    python3 api/openapi/generate.py
    python3 api/openapi/generate.py --check
"""
from __future__ import annotations

import argparse
import copy
import json
import re
import sys
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[2]
REPO = ROOT.parent
SOURCE_DIR = ROOT / "api" / "openapi"
BUNDLE = SOURCE_DIR / "bidora.openapi.json"
SCHEMAS = SOURCE_DIR / "bidora.schemas.json"
INVENTORY = SOURCE_DIR / "endpoint-inventory.md"
POSTMAN = REPO / "postman.json"
METHODS = ("get", "post", "put", "patch", "delete")

SECURITY_SCHEMES: dict[str, Any] = {
    "browserSession": {
        "type": "apiKey", "in": "cookie", "name": "__Host-bidora_session",
        "description": "Secure, HttpOnly, SameSite=Lax, host-only opaque session. Presence alone grants no authority.",
    },
    "session": {
        "type": "apiKey", "in": "cookie", "name": "__Host-bidora_session",
        "description": "Alias retained for source-contract compatibility; equivalent to browserSession.",
    },
    "csrfCookie": {
        "type": "apiKey", "in": "cookie", "name": "__Host-bidora_csrf",
        "description": "Secure, SameSite=Lax double-submit value; unsafe requests also require the equal X-CSRF-Token header and exact Origin.",
    },
    "csrf": {
        "type": "apiKey", "in": "header", "name": "X-CSRF-Token",
        "description": "Session-bound CSRF value. It is sensitive in memory and must not be logged or persisted by clients.",
    },
}

PUBLIC_PATHS = {
    ("get", "/v1/shared/payments/pesapal/ipn"),
    ("post", "/v1/auth/register"), ("post", "/v1/auth/verify-email"),
    ("post", "/v1/auth/login"), ("post", "/v1/auth/password-recovery"),
    ("post", "/v1/auth/password-recovery/reset"),
    ("post", "/v1/auth/google/start"), ("get", "/v1/auth/google/callback"),
    ("get", "/v1/auth/google/status"),
    ("post", "/v1/auth/mfa/challenge"),
}


def load(path: Path) -> dict[str, Any]:
    with path.open(encoding="utf-8") as handle:
        value = json.load(handle)
    if not isinstance(value, dict):
        raise ValueError(f"{path}: root must be an object")
    return value


def slug(path: Path) -> str:
    return re.sub(r"[^A-Za-z0-9]+", "_", path.stem).strip("_")


def title_words(value: str) -> str:
    value = re.sub(r"([a-z0-9])([A-Z])", r"\1 \2", value)
    return re.sub(r"[_-]+", " ", value).strip().capitalize()


def operation_id(method: str, path: str) -> str:
    parts = [method] + [p for p in re.split(r"[/{}:_-]+", path) if p and p != "v1"]
    return parts[0].lower() + "".join(p[:1].upper() + p[1:] for p in parts[1:])


def portal_tag(path: str) -> str:
    if path.startswith("/health/"):
        return "Health"
    if path.startswith("/v1/auth/") or path.startswith("/v1/me"):
        return "Authentication"
    if path.startswith("/v1/platform/"):
        return "Platform Administration"
    if path.startswith("/v1/tenderpreneur/"):
        return "Tenderpreneur"
    if path.startswith("/v1/shared/"):
        return "Shared Portal"
    return "Operations"


def rewrite_refs(value: Any, namespace: str) -> Any:
    if isinstance(value, list):
        return [rewrite_refs(item, namespace) for item in value]
    if not isinstance(value, dict):
        return value
    out: dict[str, Any] = {}
    for key, item in value.items():
        if key == "$ref" and isinstance(item, str):
            match = re.fullmatch(r"#/components/(schemas|responses|parameters|headers|examples|requestBodies|callbacks|links)/(.*)", item)
            out[key] = f"#/components/{match.group(1)}/{namespace}__{match.group(2)}" if match else item
        else:
            out[key] = rewrite_refs(item, namespace)
    return out


def source_files() -> list[Path]:
    return [SOURCE_DIR / "health.json", *sorted((SOURCE_DIR / "drafts").glob("*.json"))]


def parameter_names(bundle: dict[str, Any], operation: dict[str, Any]) -> set[tuple[str, str]]:
    names: set[tuple[str, str]] = set()
    for parameter in operation.get("parameters", []):
        value = pointer_get(bundle, parameter["$ref"]) if "$ref" in parameter else parameter
        names.add((value.get("name", ""), value.get("in", "")))
    return names


def normalize_cross_cutting(bundle: dict[str, Any], path: str, method: str, operation: dict[str, Any]) -> None:
    """Apply only repository-approved universal browser and tenant binding rules."""
    parameters = operation.setdefault("parameters", [])
    names = parameter_names(bundle, operation)
    additions: list[dict[str, Any]] = []
    for name in re.findall(r"\{([^}]+)\}", path):
        if (name, "path") not in names:
            additions.append({"name": name, "in": "path", "required": True, "schema": {"type": "string", **({"format": "uuid"} if name.endswith("_id") else {})}})
    authenticated = bool(operation.get("security"))
    if authenticated and method in ("post", "put", "patch", "delete"):
        if ("Origin", "header") not in names:
            additions.append({"name": "Origin", "in": "header", "required": True, "schema": {"type": "string", "format": "uri"}})
        if ("X-CSRF-Token", "header") not in names:
            additions.append({"name": "X-CSRF-Token", "in": "header", "required": True, "schema": {"type": "string", "minLength": 43, "maxLength": 43}})
    if path.startswith("/v1/tenderpreneur/"):
        if ("X-Bidora-Tenant-ID", "header") not in names:
            additions.append({"name": "X-Bidora-Tenant-ID", "in": "header", "required": True, "description": "Tenant selector only; never authority or a database locator.", "schema": {"type": "string", "format": "uuid"}})
        if ("X-Bidora-Context-Generation", "header") not in names:
            additions.append({"name": "X-Bidora-Context-Generation", "in": "header", "required": True, "description": "Exact current membership context generation.", "schema": {"type": "integer", "format": "int64", "minimum": 1}})
    if additions:
        parameters.extend(additions)
        operation["x-normalized-cross-cutting"] = True


def build_bundle() -> dict[str, Any]:
    bundle: dict[str, Any] = {
        "openapi": "3.1.0",
        "info": {
            "title": "Bidora Backend API",
            "version": "1.0.0-contract-candidate",
            "description": (
                "Canonical contract bundle for implemented health, identity, shared, Platform Administration, "
                "and Tenderpreneur operations. Source status and approval gates are retained per operation. "
                "A documented operation is not proof that a route is enabled in a deployment."
            ),
            "license": {"name": "Private project — no redistribution license granted"},
        },
        "jsonSchemaDialect": "https://json-schema.org/draft/2020-12/schema",
        "servers": [{"url": "{scheme}://{host}", "variables": {
            "scheme": {"default": "https", "enum": ["https"]},
            "host": {"default": "api.example.test"},
        }}],
        "security": [{"browserSession": []}],
        "tags": [{"name": name} for name in (
            "Health", "Authentication", "Shared Portal", "Platform Administration", "Tenderpreneur", "Operations"
        )],
        "paths": {},
        "components": {"securitySchemes": copy.deepcopy(SECURITY_SCHEMES)},
        "x-contract-status": "IMPLEMENTED_NOT_VERIFIED",
        "x-generation": {"command": "cd backend && python3 api/openapi/generate.py", "source_count": 0},
    }
    seen_ids: set[str] = set()
    sources: list[str] = []
    for path in source_files():
        document = load(path)
        namespace = slug(path)
        relative = path.relative_to(ROOT).as_posix()
        sources.append(relative)
        for kind, entries in document.get("components", {}).items():
            if kind == "securitySchemes":
                continue
            target = bundle["components"].setdefault(kind, {})
            for name, value in entries.items():
                key = f"{namespace}__{name}"
                if key in target:
                    raise ValueError(f"duplicate component {kind}/{key}")
                target[key] = rewrite_refs(value, namespace)
        for api_path, path_item in document.get("paths", {}).items():
            target_item = bundle["paths"].setdefault(api_path, {})
            for method, raw_operation in path_item.items():
                if method not in METHODS:
                    continue
                if method in target_item:
                    raise ValueError(f"duplicate operation {method.upper()} {api_path}")
                operation = rewrite_refs(raw_operation, namespace)
                op_id = operation.get("operationId") or operation_id(method, api_path)
                if op_id in seen_ids:
                    op_id += namespace.title().replace("_", "")
                if op_id in seen_ids:
                    raise ValueError(f"duplicate operationId {op_id}")
                seen_ids.add(op_id)
                operation["operationId"] = op_id
                operation.setdefault("summary", title_words(op_id))
                operation["tags"] = list(dict.fromkeys([portal_tag(api_path), *operation.get("tags", [])]))
                operation["x-source-contract"] = relative
                operation["x-source-status"] = document.get("x-status", "source-status-unspecified")
                if "security" not in operation:
                    operation["security"] = [] if (method, api_path) in PUBLIC_PATHS or api_path.startswith("/health/") else [{"browserSession": []}]
                normalize_cross_cutting(bundle, api_path, method, operation)
                target_item[method] = operation
    bundle["x-generation"]["source_count"] = len(sources)
    bundle["x-generation"]["sources"] = sources
    return bundle


def pointer_get(document: dict[str, Any], pointer: str) -> Any:
    if not pointer.startswith("#/"):
        raise ValueError(f"external reference is not allowed: {pointer}")
    value: Any = document
    for token in pointer[2:].split("/"):
        token = token.replace("~1", "/").replace("~0", "~")
        if not isinstance(value, dict) or token not in value:
            raise ValueError(f"unresolved reference: {pointer}")
        value = value[token]
    return value


def validate_refs(document: dict[str, Any]) -> None:
    def visit(value: Any) -> None:
        if isinstance(value, list):
            for item in value:
                visit(item)
        elif isinstance(value, dict):
            for key, item in value.items():
                if key == "$ref":
                    pointer_get(document, item)
                else:
                    visit(item)
    visit(document)


def operations(bundle: dict[str, Any]):
    for path, item in bundle["paths"].items():
        for method in METHODS:
            if method in item:
                yield path, method, item[method]


def resolved_parameters(bundle: dict[str, Any], operation: dict[str, Any]) -> list[dict[str, Any]]:
    out = []
    for parameter in operation.get("parameters", []):
        value = pointer_get(bundle, parameter["$ref"]) if "$ref" in parameter else parameter
        out.append(value)
    return out


def security_text(operation: dict[str, Any]) -> str:
    security = operation.get("security", [])
    if not security:
        return "Public"
    return " OR ".join(" + ".join(requirement.keys()) for requirement in security)


def response_groups(operation: dict[str, Any]) -> tuple[str, str]:
    statuses = sorted(operation.get("responses", {}).keys())
    return ", ".join(s for s in statuses if str(s).startswith("2")) or "—", ", ".join(s for s in statuses if not str(s).startswith("2")) or "—"


def build_inventory(bundle: dict[str, Any]) -> str:
    rows = []
    for path, method, operation in operations(bundle):
        params = resolved_parameters(bundle, operation)
        headers = [p["name"] for p in params if p.get("in") == "header"]
        queries = [p["name"] + ("*" if p.get("required") else "") for p in params if p.get("in") == "query"]
        success, errors = response_groups(operation)
        idem = "Required" if any(name.lower() == "idempotency-key" for name in headers) else "No header contract"
        rows.append((portal_tag(path), method.upper(), path, operation["operationId"], operation["summary"], security_text(operation), ", ".join(headers) or "—", ", ".join(queries) or "—", success, errors, idem, operation["x-source-contract"]))
    rows.sort(key=lambda row: (row[0], row[2], row[1]))
    header = (
        "# Bidora canonical endpoint inventory\n\n"
        "> Generated by `cd backend && python3 api/openapi/generate.py`; do not hand-edit. "
        "Required query parameters are marked `*`. Authorization remains capability- and context-based; account type and selectors never grant authority.\n\n"
        f"**Operations:** {len(rows)}  \n**Canonical specification:** [`bidora.openapi.json`](bidora.openapi.json)\n\n"
        "| Portal | Method | Path | Operation ID | Purpose | Authentication | Headers | Query | Success | Errors | Idempotency | Source |\n"
        "|---|---:|---|---|---|---|---|---|---|---|---|---|\n"
    )
    body = "\n".join("| " + " | ".join(str(cell).replace("|", "\\|").replace("\n", " ") for cell in row) + " |" for row in rows)
    return header + body + "\n"


def schemas_document(bundle: dict[str, Any]) -> dict[str, Any]:
    defs = copy.deepcopy(bundle["components"].get("schemas", {}))
    def rewrite(value: Any) -> None:
        if isinstance(value, list):
            for item in value: rewrite(item)
        elif isinstance(value, dict):
            for key, item in value.items():
                if key == "$ref" and isinstance(item, str) and item.startswith("#/components/schemas/"):
                    value[key] = "#/$defs/" + item.rsplit("/", 1)[1]
                else: rewrite(item)
    rewrite(defs)
    return {
        "$schema": "https://json-schema.org/draft/2020-12/schema",
        "$id": "https://contracts.bidora.invalid/v1/bidora.schemas.json",
        "title": "Bidora API DTO schema catalogue",
        "description": "Namespaced request, response, error and shared domain DTOs extracted from the canonical OpenAPI contract.",
        "$defs": defs,
    }


def schema_example(bundle: dict[str, Any], schema: Any, depth: int = 0) -> Any:
    if depth > 8 or not isinstance(schema, dict): return None
    if "$ref" in schema: return schema_example(bundle, pointer_get(bundle, schema["$ref"]), depth + 1)
    if "example" in schema: return copy.deepcopy(schema["example"])
    if "const" in schema: return schema["const"]
    if "enum" in schema and schema["enum"]: return schema["enum"][0]
    for combinator in ("oneOf", "anyOf", "allOf"):
        if combinator in schema and schema[combinator]: return schema_example(bundle, schema[combinator][0], depth + 1)
    typ = schema.get("type")
    if isinstance(typ, list): typ = next((x for x in typ if x != "null"), "null")
    if typ == "object" or "properties" in schema:
        required = schema.get("required", [])
        return {name: schema_example(bundle, schema["properties"][name], depth + 1) for name in required if name in schema.get("properties", {})}
    if typ == "array": return []
    if typ == "integer": return max(1, schema.get("minimum", 1))
    if typ == "number": return schema.get("minimum", 1)
    if typ == "boolean": return False
    if typ == "string":
        fmt = schema.get("format")
        if fmt == "uuid": return "00000000-0000-0000-0000-000000000000"
        if fmt in ("date-time", "datetime"): return "2026-01-01T00:00:00Z"
        if fmt == "date": return "2026-01-01"
        if fmt == "email": return "user@example.test"
        if fmt == "uri": return "https://example.invalid/resource"
        return "replace-with-synthetic-value"
    return None


def postman_variable(name: str) -> str:
    aliases = {
        "Origin": "origin", "X-Bidora-Tenant-ID": "tenant_id", "X-Bidora-Context-Generation": "context_generation",
        "X-Bidora-Platform-Context-Generation": "platform_context_generation", "X-CSRF-Token": "csrf_token",
        "Idempotency-Key": "idempotency_key", "If-Match": "expected_version", "state": "google_state", "code": "google_code",
    }
    return aliases.get(name, re.sub(r"[^a-z0-9]+", "_", name.lower()).strip("_"))


def postmanize_body(value: Any, path: str) -> Any:
    if isinstance(value, list):
        return [postmanize_body(item, path) for item in value]
    if not isinstance(value, dict):
        return value
    replacements = {
        "email": "email", "password": "password", "current_password": "password", "new_password": "new_password",
        "totp_code": "totp_code", "recovery_code": "recovery_code", "code": "totp_code", "state": "google_state",
    }
    out: dict[str, Any] = {}
    for key, item in value.items():
        variable = replacements.get(key)
        if key == "token":
            variable = "verification_token" if path.endswith("/verify-email") else "recovery_token"
        out[key] = "{{" + variable + "}}" if variable else postmanize_body(item, path)
    return out


def postman_request(bundle: dict[str, Any], path: str, method: str, operation: dict[str, Any]) -> dict[str, Any]:
    url = "{{base_url}}" + re.sub(r"\{([^}]+)\}", lambda m: "{{" + m.group(1) + "}}", path)
    headers = []
    query = []
    for parameter in resolved_parameters(bundle, operation):
        name = parameter.get("name", "")
        if parameter.get("in") == "header":
            headers.append({"key": name, "value": "{{" + postman_variable(name) + "}}"})
        elif parameter.get("in") == "query":
            query.append({"key": name, "value": "{{" + postman_variable(name) + "}}", "disabled": not parameter.get("required", False)})
    request: dict[str, Any] = {"method": method.upper(), "header": headers, "url": {"raw": url, "host": ["{{base_url}}"], "path": [], "query": query}}
    content = operation.get("requestBody", {}).get("content", {})
    if "application/json" in content:
        media = content["application/json"]
        example = next(iter(media.get("examples", {}).values()), {}).get("value") if media.get("examples") else media.get("example")
        if example is None: example = schema_example(bundle, media.get("schema", {}))
        example = postmanize_body(example, path)
        headers.append({"key": "Content-Type", "value": "application/json"})
        request["body"] = {"mode": "raw", "raw": json.dumps(example, indent=2), "options": {"raw": {"language": "json"}}}
    item: dict[str, Any] = {"name": operation["summary"], "request": request, "description": f"`{operation['operationId']}` — source: `{operation['x-source-contract']}`"}
    if operation["operationId"] == "bootstrapSessionCSRF":
        item["event"] = [{"listen": "test", "script": {"type": "text/javascript", "exec": [
            "if (pm.response.code === 200) {",
            "  const payload = pm.response.json();",
            "  if (payload.data && payload.data.csrf_token) pm.collectionVariables.set('csrf_token', payload.data.csrf_token);",
            "}",
        ]}}]
    return item


def build_postman(bundle: dict[str, Any]) -> dict[str, Any]:
    folders: dict[str, list[Any]] = {name: [] for name in ("Authentication", "Shared Portal", "Platform Administration", "Tenderpreneur", "Health", "Operations")}
    variables = {
        "base_url": "https://localhost:8443", "origin": "https://app.example.test", "email": "user@example.test",
        "password": "replace-with-synthetic-password", "new_password": "replace-with-new-synthetic-password",
        "verification_token": "", "recovery_token": "", "totp_code": "", "recovery_code": "",
        "google_state": "", "google_code": "", "csrf_token": "", "idempotency_key": "{{$guid}}", "tenant_id": "00000000-0000-0000-0000-000000000000",
        "context_generation": "1", "platform_context_generation": "1", "expected_version": "1",
    }
    for path, method, operation in operations(bundle):
        folders[portal_tag(path)].append(postman_request(bundle, path, method, operation))
        for match in re.findall(r"\{([^}]+)\}", path): variables.setdefault(match, "00000000-0000-0000-0000-000000000000")
        for parameter in resolved_parameters(bundle, operation): variables.setdefault(postman_variable(parameter.get("name", "value")), "")
    return {
        "info": {"_postman_id": "8a112c9d-32d0-4a42-9c4d-102de298ac41", "name": "Bidora Backend API", "description": "Generated from backend/api/openapi/bidora.openapi.json. Uses synthetic placeholders only; selectors never grant authority.", "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},
        "auth": {"type": "noauth"},
        "event": [{"listen": "prerequest", "script": {"type": "text/javascript", "exec": ["pm.request.headers.upsert({ key: 'Origin', value: pm.collectionVariables.get('origin') });"]}}],
        "variable": [{"key": key, "value": value} for key, value in sorted(variables.items())],
        "item": [{"name": name, "item": items} for name, items in folders.items() if items],
    }


def encoded_json(value: Any) -> str:
    return json.dumps(value, indent=2, ensure_ascii=False, sort_keys=True) + "\n"


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="fail when generated artifacts differ")
    args = parser.parse_args()
    bundle = build_bundle()
    validate_refs(bundle)
    generated = {
        BUNDLE: encoded_json(bundle),
        SCHEMAS: encoded_json(schemas_document(bundle)),
        INVENTORY: build_inventory(bundle),
        POSTMAN: encoded_json(build_postman(bundle)),
    }
    stale = []
    for path, content in generated.items():
        if args.check:
            if not path.exists() or path.read_text(encoding="utf-8") != content: stale.append(path)
        else:
            path.write_text(content, encoding="utf-8")
    if stale:
        print("contract artifacts are stale:", file=sys.stderr)
        for path in stale: print(f"  {path.relative_to(REPO)}", file=sys.stderr)
        return 1
    print(f"validated {sum(1 for _ in operations(bundle))} operations, {len(bundle['components'].get('schemas', {}))} schemas and all local references")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
