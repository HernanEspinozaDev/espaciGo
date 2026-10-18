"""Select the immutable V1-V61 migration set used by historical GCP-03 tools."""

from __future__ import annotations

from pathlib import Path
import re


class HistoricalMigrationInventoryError(ValueError):
    pass


def historical_v1_v61_migrations(directory: Path) -> list[Path]:
    """Return one regular source file for every fixed historical version 1..61.

    Later migrations belong to current tools and are intentionally outside this
    inventory. Missing, duplicate, symlink, or malformed historical files fail.
    """
    files: list[Path] = []
    for version in range(1, 62):
        matches = sorted(directory.glob(f"V{version:06d}__*.sql"))
        if len(matches) != 1:
            raise HistoricalMigrationInventoryError(
                f"expected exactly one historical V{version} migration, found {len(matches)}"
            )
        path = matches[0]
        if path.is_symlink() or not path.is_file() or not re.fullmatch(
            rf"V{version:06d}__[A-Za-z0-9_-]+\.sql", path.name
        ):
            raise HistoricalMigrationInventoryError(
                f"historical migration V{version} is not a regular versioned SQL file"
            )
        files.append(path)
    return files
