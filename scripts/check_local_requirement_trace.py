#!/usr/bin/env python3
"""Check that LOCAL-PLAN's ES1 identifier crosswalk covers each source ID once."""

from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[1]
ES1 = ROOT / "planning/referencias/ES1"
TRACE = ROOT / "planning/trazabilidad_requisitos_backend_local.md"


def require(label: str, expected: set[str], actual: list[str]) -> list[str]:
    errors: list[str] = []
    counts = {item: actual.count(item) for item in expected}
    missing = sorted(item for item in expected if counts[item] == 0)
    duplicate = sorted(item for item in expected if counts[item] > 1)
    unexpected = sorted(set(actual) - expected)
    if missing:
        errors.append(f"{label}: faltan {', '.join(missing)}")
    if duplicate:
        errors.append(f"{label}: aparecen más de una vez {', '.join(duplicate)}")
    if unexpected:
        errors.append(f"{label}: IDs fuera del catálogo {', '.join(unexpected)}")
    if not errors:
        print(f"{label}: {len(expected)} IDs, cobertura exacta")
    return errors


def table_ids(text: str, pattern: str) -> list[str]:
    return re.findall(pattern, text, flags=re.MULTILINE)


def main() -> int:
    trace = TRACE.read_text(encoding="utf-8")
    b = (ES1 / "B_requerimientos_funcionales.md").read_text(encoding="utf-8")
    c = (ES1 / "C_requerimientos_no_funcionales.md").read_text(encoding="utf-8")
    d = (ES1 / "D_casos_de_uso.md").read_text(encoding="utf-8")
    e = (ES1 / "E_historias_de_usuario.md").read_text(encoding="utf-8")

    catalogs = {
        "RQF": (
            set(re.findall(r"\|\s*(RQF-\d+)\s*\|", b)),
            trace.split("## RQF (236)", 1)[-1].split("## RNF", 1)[0],
            r"^\|\s*(RQF-\d+)\s*\|",
        ),
        "RNF": (
            set(re.findall(r"\|\s*(RNF-\d+)\s*\|", c)),
            trace.split("## RNF (43)", 1)[-1].split("## CU", 1)[0],
            r"^\|\s*(RNF-\d+)\s*\|",
        ),
        "CU": (
            set(re.findall(r"(?m)^### (CU-\d+):", d)),
            trace.split("## CU (52)", 1)[-1],
            r"^\|\s*(CU-\d+)\s+—",
        ),
        "HU": (
            set(re.findall(r"(?m)^## (HU\d+)\s*-", e)),
            trace.split("## CU (52)", 1)[-1],
            r"^\|\s*(HU\d+)\s+—",
        ),
    }

    errors: list[str] = []
    for label, (expected, section, pattern) in catalogs.items():
        errors.extend(require(label, expected, table_ids(section, pattern)))
    if errors:
        print("\n".join(errors), file=sys.stderr)
        return 1
    print("Trazabilidad LOCAL-PLAN-01: válida")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
