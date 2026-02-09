#!/usr/bin/env python3
"""Contract subset checks for generated FastAPI apps."""

from __future__ import annotations

import argparse
import importlib
import re
import sys
from pathlib import Path
from typing import Dict, Set, Tuple

from fastapi.testclient import TestClient


def parse_openapi_paths(openapi_path: Path) -> Set[Tuple[str, str]]:
    text = openapi_path.read_text(encoding="utf-8")
    lines = text.splitlines()
    pairs: Set[Tuple[str, str]] = set()
    in_paths = False
    current_path = ""

    for line in lines:
        if re.match(r"^paths:\s*$", line):
            in_paths = True
            current_path = ""
            continue
        if in_paths and re.match(r"^[A-Za-z_]", line):
            break
        if not in_paths:
            continue

        m_path = re.match(r"^\s{2}(/[^:]+):\s*$", line)
        if m_path:
            current_path = m_path.group(1)
            continue

        m_method = re.match(r"^\s{4}(get|post|put|patch|delete|head|options):\s*$", line)
        if m_method and current_path:
            pairs.add((m_method.group(1).upper(), current_path))

    return pairs


def runtime_pairs(app) -> Set[Tuple[str, str]]:
    spec = app.openapi()
    out: Set[Tuple[str, str]] = set()
    paths: Dict[str, Dict[str, object]] = spec.get("paths", {})
    for path, methods in paths.items():
        for method in methods.keys():
            out.add((method.upper(), path))
    return out


def load_app(app_dir: Path):
    root = str(app_dir.parent)
    if root not in sys.path:
        sys.path.insert(0, root)
    mod = importlib.import_module("app.main")
    app = getattr(mod, "app", None)
    if app is None:
        raise RuntimeError(f"no 'app' object in {app_dir}/main.py")
    return app


def check_openapi_subset(app_dir: Path, openapi_path: Path) -> None:
    app = load_app(app_dir)
    expected = parse_openapi_paths(openapi_path)
    actual = runtime_pairs(app)
    missing = sorted(expected - actual)
    if missing:
        formatted = "\n".join(f"- {m} {p}" for m, p in missing[:30])
        raise RuntimeError(f"runtime app missing OpenAPI routes:\n{formatted}")
    print(f"openapi subset OK: {len(expected)} routes")


def check_pdf_endpoint(app_dir: Path, path: str) -> None:
    app = load_app(app_dir)
    client = TestClient(app)
    payload = {
        "title": "Contract PDF",
        "points": [{"x": 1, "y": 1.2}, {"x": 2, "y": 2.5}, {"x": 3, "y": 2.0}],
    }
    response = client.post(path, json=payload)
    if response.status_code != 200:
        raise RuntimeError(f"pdf endpoint returned {response.status_code}, expected 200")
    ctype = response.headers.get("content-type", "")
    if "application/pdf" not in ctype:
        raise RuntimeError(f"pdf endpoint content-type is '{ctype}', expected application/pdf")
    if not response.content:
        raise RuntimeError("pdf endpoint returned empty body")
    print(f"pdf endpoint OK: {path}")


def main() -> int:
    parser = argparse.ArgumentParser(description="Run contract subset checks for generated FastAPI app.")
    parser.add_argument("--app-dir", required=True, help="Path to generated app dir (contains main.py).")
    parser.add_argument("--openapi", required=True, help="Path to generated openapi.yaml.")
    parser.add_argument("--pdf-endpoint", default="", help="Optional PDF endpoint path to call with TestClient.")
    args = parser.parse_args()

    app_dir = Path(args.app_dir).resolve()
    openapi_path = Path(args.openapi).resolve()
    if not app_dir.exists():
        raise FileNotFoundError(app_dir)
    if not openapi_path.exists():
        raise FileNotFoundError(openapi_path)

    check_openapi_subset(app_dir, openapi_path)
    if args.pdf_endpoint:
        check_pdf_endpoint(app_dir, args.pdf_endpoint)

    print("python contract subset SUCCESS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

