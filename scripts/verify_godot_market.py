#!/usr/bin/env python3
"""Recompile the real market CUE and compare ANG with the read-only Godot oracle.

Uses only the Python standard library. Output must be a separate staging directory;
the oracle, its GLBs and the owner's Godot project are never modified.
"""
import argparse
import hashlib
import json
import math
from pathlib import Path
import shutil
import subprocess


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def run(command, log, cwd=None):
    with log.open("w") as stream:
        result = subprocess.run(command, cwd=cwd, stdout=stream, stderr=subprocess.STDOUT)
    if result.returncode:
        raise RuntimeError(f"exit {result.returncode}; see {log}")
    if "ERROR:" in log.read_text():
        raise RuntimeError(f"engine reported errors; see {log}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--game-root", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--godot", help="Godot executable; omit for placement comparison only")
    parser.add_argument("--report", type=Path, help="also save compact verification evidence here")
    args = parser.parse_args()
    repo = Path(__file__).resolve().parents[1]
    game = args.game_root.resolve()
    out = args.out.resolve()
    if out == game or game in out.parents or out == repo:
        raise ValueError("use a separate staging directory")
    out.mkdir(parents=True, exist_ok=True)
    oracle = game / "art-source/transport-hub/godot-spike/real-market"
    cue = game / "content/world/godot/market"
    protected = [oracle / name for name in (
        "market-placements.json", "market.compiled.json", "gen_market.py", "orientation.json"
    )]
    before = {str(p.relative_to(game)): digest(p) for p in protected}
    run(["go", "run", "./cmd/ang", "godot", "-layout", "example.market_hub",
         "-placement", str(cue), "-models", str(cue / "models.json"), "-out", str(out),
         str(game / "content/world/location_sdk")], out / "generation.log", repo)
    manifest = json.loads((out / "placements.json").read_text())
    expected = json.loads((oracle / "market-placements.json").read_text())
    if manifest["structural_hash"] != expected["structural_hash"]:
        raise AssertionError("compiled structure differs from the oracle")
    models = json.loads((cue / "models.json").read_text())
    rows = {i["instance_id"]: i for i in expected["instances"]}
    if rows.keys() != manifest["instances"].keys():
        raise AssertionError("oracle/ANG placement identities differ")
    position_error = basis_error = 0.0
    mismatches = []
    native_expected = {"instances": {}}
    for instance_id, row in rows.items():
        actual = manifest["instances"][instance_id]
        if "source_transform" in actual or "override_reason" in actual:
            raise AssertionError(f"visual-only override: {instance_id}")
        pose = actual["world_transform"]
        position_error = max(position_error, *(abs(a-b) for a,b in zip(pose["position"], row["godot_world_m"])))
        a = math.radians(pose["rotation_degrees"][1])
        c, s = math.cos(a), math.sin(a)
        sx, sy, sz = pose["scale"]
        basis = [[c*sx, 0, -s*sx], [0, sy, 0], [s*sz, 0, c*sz]]
        basis_error = max(basis_error, *(abs(a-b) for av,bv in zip(basis,row["godot_world_basis_columns"]) for a,b in zip(av,bv)))
        expected_model = "res://" + row["glb"] if row.get("glb") else None
        if models.get(actual["model_ref"]) != expected_model:
            mismatches.append(instance_id)
        native_expected["instances"][instance_id] = {
            "position": row["godot_world_m"], "basis_columns": row["godot_world_basis_columns"]
        }
    if position_error > 1e-9 or basis_error > 1e-9 or mismatches:
        raise AssertionError(f"oracle mismatch: position {position_error}, basis {basis_error}, models {mismatches}")
    report = {
        "passed": True, "layout_id": manifest["layout_id"], "placements": len(rows),
        "max_position_error_m": position_error, "max_basis_error": basis_error,
        "model_mismatches": mismatches, "visual_position_overrides": 0, "structural_hash": manifest["structural_hash"],
        "oracle_sha256": before,
        "generated_sha256": {name: digest(out / name) for name in ("compiled.json", "placements.json", "location.tscn")},
        "scene_load": None,
    }
    if args.godot:
        for resource in set(models.values()):
            relative = resource.removeprefix("res://")
            target = out / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(oracle / relative, target)
        shutil.copy2(cue / "models.json", out / "models.json")
        shutil.copy2(repo / "scripts/godot/verify_scene.gd", out / "verify_scene.gd")
        (out / "oracle.json").write_text(json.dumps(native_expected))
        (out / "project.godot").write_text(
            'config_version=5\n[application]\nconfig/name="ANG generated market verification"\n'
            'run/main_scene="res://location.tscn"\n[rendering]\n'
            'renderer/rendering_method="gl_compatibility"\n')
        run([args.godot, "--headless", "--path", str(out), "--log-file", str(out / "import-engine.log"), "--editor", "--import"], out / "import.log")
        run([args.godot, "--headless", "--path", str(out), "--log-file", str(out / "scene-engine.log"), "--script", "res://verify_scene.gd"], out / "scene-load.log")
        report["scene_load"] = json.loads((out / "scene-load-verification.json").read_text())
        if not report["scene_load"]["passed"]:
            raise AssertionError("Godot scene verification failed")
    if before != {str(p.relative_to(game)): digest(p) for p in protected}:
        raise AssertionError("oracle sources changed during verification")
    encoded = json.dumps(report, indent=2) + "\n"
    (out / "verification.json").write_text(encoded)
    if args.report:
        args.report.write_text(encoded)
    print(encoded, end="")


if __name__ == "__main__":
    main()
