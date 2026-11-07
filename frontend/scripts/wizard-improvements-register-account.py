#!/usr/bin/env python3
"""Disabled compatibility entrypoint; use the canonical script instead."""

from pathlib import Path
import sys

CANONICAL = Path(__file__).resolve().parents[2] / "scripts" / "wizard-improvements-register-account.py"


def main():
    print(f"Disabled legacy entrypoint. Use canonical entrypoint: {CANONICAL}", file=sys.stderr)
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
