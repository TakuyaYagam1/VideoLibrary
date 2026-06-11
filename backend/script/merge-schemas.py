#!/usr/bin/env python3
"""Merge OpenAPI schema fragments into one schemas.yml document."""

from __future__ import annotations

import argparse
import json
import subprocess
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError:  # pragma: no cover - depends on local codegen environment.
    yaml = None


def load_schema_file(path: Path) -> dict[str, Any]:
    if yaml is not None:
        with path.open("r", encoding="utf-8") as source:
            loaded = yaml.safe_load(source) or {}
    else:
        loaded = load_schema_file_with_yq(path)

    if not isinstance(loaded, dict):
        raise ValueError(f"{path} must contain a YAML mapping")

    return loaded


def load_schema_file_with_yq(path: Path) -> Any:
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


def merge_schema_files(source_dir: Path) -> dict[str, Any]:
    merged: dict[str, Any] = {}
    for path in sorted(source_dir.glob("*.yml")):
        for name, schema in load_schema_file(path).items():
            if name in merged:
                raise ValueError(f"duplicate schema {name!r} in {path}")
            merged[name] = schema

    if not merged:
        raise ValueError(f"no schema fragments found in {source_dir}")

    return merged


def write_schema_file(output: Path, schemas: dict[str, Any]) -> None:
    output.parent.mkdir(parents=True, exist_ok=True)
    if yaml is not None:
        with output.open("w", encoding="utf-8") as target:
            yaml.safe_dump(schemas, target, sort_keys=False, allow_unicode=False)
        return

    try:
        completed = subprocess.run(
            ["yq", "-P", ".", "-"],
            input=json.dumps(schemas),
            check=True,
            capture_output=True,
            text=True,
        )
    except (FileNotFoundError, subprocess.CalledProcessError) as exc:
        raise SystemExit(
            "PyYAML is not installed and yq fallback failed; run make install-codegen-python-deps",
        ) from exc

    output.write_text(completed.stdout, encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    write_schema_file(args.output, merge_schema_files(args.source))


if __name__ == "__main__":
    main()
