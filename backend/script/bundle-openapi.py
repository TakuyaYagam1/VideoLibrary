#!/usr/bin/env python3
"""Bundle the local modular OpenAPI layout when Redocly is unavailable."""

from __future__ import annotations

import argparse
import copy
import json
import subprocess
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError:  # pragma: no cover - depends on local codegen environment.
    yaml = None


def load_yaml(path: Path) -> dict[str, Any]:
    if yaml is not None:
        with path.open("r", encoding="utf-8") as source:
            loaded = yaml.safe_load(source) or {}
    else:
        loaded = load_yaml_with_yq(path)

    if not isinstance(loaded, dict):
        raise ValueError(f"{path} must contain a YAML mapping")

    return loaded


def load_yaml_with_yq(path: Path) -> Any:
    try:
        completed = subprocess.run(
            ["yq", "-o=json", ".", str(path)],
            check=True,
            capture_output=True,
            text=True,
        )
    except (FileNotFoundError, subprocess.CalledProcessError) as exc:
        raise SystemExit(
            "PyYAML is not installed and yq fallback failed; run make install-codegen-python-deps",
        ) from exc

    return json.loads(completed.stdout or "{}")


def write_yaml(path: Path, document: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    if yaml is not None:
        with path.open("w", encoding="utf-8") as target:
            yaml.safe_dump(document, target, sort_keys=False, allow_unicode=False)
        return

    try:
        completed = subprocess.run(
            ["yq", "-P", ".", "-"],
            input=json.dumps(document),
            check=True,
            capture_output=True,
            text=True,
        )
    except (FileNotFoundError, subprocess.CalledProcessError) as exc:
        raise SystemExit(
            "PyYAML is not installed and yq fallback failed; run make install-codegen-python-deps",
        ) from exc

    path.write_text(completed.stdout, encoding="utf-8")


def resolve_pointer(document: Any, pointer: str) -> Any:
    if pointer in ("", "#"):
        return copy.deepcopy(document)

    if pointer.startswith("#"):
        pointer = pointer[1:]

    if not pointer.startswith("/"):
        raise ValueError(f"unsupported JSON pointer {pointer!r}")

    current = document
    for raw_part in pointer[1:].split("/"):
        part = raw_part.replace("~1", "/").replace("~0", "~")
        current = current[part]

    return copy.deepcopy(current)


def resolve_external_ref(ref: str, base_dir: Path) -> Any:
    path_part, _, fragment = ref.partition("#")
    target_path = (base_dir / path_part).resolve()
    return resolve_pointer(load_yaml(target_path), f"#{fragment}" if fragment else "#")


def normalize_refs(node: Any) -> Any:
    if isinstance(node, dict):
        ref = node.get("$ref")
        if isinstance(ref, str):
            normalized = normalize_schema_ref(ref)
            if normalized is not None:
                node["$ref"] = normalized

        for value in node.values():
            normalize_refs(value)
    elif isinstance(node, list):
        for item in node:
            normalize_refs(item)

    return node


def normalize_schema_ref(ref: str) -> str | None:
    prefixes = (
        "../components/schemas.yml#/",
        "./components/schemas.yml#/",
        "components/schemas.yml#/",
    )
    for prefix in prefixes:
        if ref.startswith(prefix):
            schema_name = ref.removeprefix(prefix)
            return f"#/components/schemas/{schema_name}"

    return None


def bundle_openapi(entry: Path, output: Path) -> None:
    root = load_yaml(entry)
    base_dir = entry.parent
    bundled = copy.deepcopy(root)

    bundled["paths"] = {}
    for path_name, path_item in root.get("paths", {}).items():
        if isinstance(path_item, dict) and set(path_item) == {"$ref"}:
            path_item = resolve_external_ref(path_item["$ref"], base_dir)
        bundled["paths"][path_name] = normalize_refs(path_item)

    components = copy.deepcopy(root.get("components", {}))
    schemas = components.get("schemas")
    if isinstance(schemas, dict) and set(schemas) == {"$ref"}:
        components["schemas"] = resolve_external_ref(schemas["$ref"], base_dir)

    security_schemes = components.get("securitySchemes")
    if isinstance(security_schemes, dict) and set(security_schemes) == {"$ref"}:
        components["securitySchemes"] = resolve_external_ref(security_schemes["$ref"], base_dir)

    bundled["components"] = normalize_refs(components)
    write_yaml(output, bundled)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--entry", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    bundle_openapi(args.entry, args.output)


if __name__ == "__main__":
    main()
