#!/usr/bin/env python3
"""Generate the deterministic, synthetic-only GCP-03 restore fixture."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re

from gcp03_migration_inventory import historical_v1_v61_migrations


ROOT = Path(__file__).resolve().parents[1]
MIGRATIONS = ROOT / "db" / "migrations"
FIXTURE_SQL = """-- Synthetic GCP-03 restore fixture v1. No source/dataset values.
BEGIN;
INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES
('aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaa01','restore-host@example.test','restore-host@example.test','fixture-not-a-credential','activo'),
('bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb','restore-renter@example.test','restore-renter@example.test','fixture-not-a-credential','activo');
INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria)
VALUES ('dddddddd-dddd-4ddd-8ddd-dddddddddddd','aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaa01','sala_multiproposito','Restore fixture','Espacio sintético generado solo para el ensayo local de dump y restore. No representa a una persona ni publicación real.',30,8,'Datos de prueba','hora',8000,'Dirección ficticia','America/Santiago');
INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores)
SELECT 'dddddddd-dddd-4ddd-8ddd-dddddddddddd','sala_multiproposito',max(version),'{}'::jsonb FROM public.categoria_perfil_atributos WHERE categoria_codigo='sala_multiproposito';
INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp)
VALUES ('dddddddd-dddd-4ddd-8ddd-dddddddddddd',1,'hora',8000);
INSERT INTO public.ocupacion(id,espacio_id,reserva_id,intervalo,tipo,activo,motivo)
VALUES
('eeeeeeee-eeee-4eee-8eee-eeeeeeeeee01','dddddddd-dddd-4ddd-8ddd-dddddddddddd',NULL,tstzrange('2026-09-06 04:00:00+00','2026-09-06 05:00:00+00','[)'),'bloqueo_manual',true,'fixture adyacente A'),
('eeeeeeee-eeee-4eee-8eee-eeeeeeeeee02','dddddddd-dddd-4ddd-8ddd-dddddddddddd',NULL,tstzrange('2026-09-06 05:00:00+00','2026-09-06 06:00:00+00','[)'),'bloqueo_manual',true,'fixture adyacente B');
COMMIT;
"""


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output-dir", default=".local/testing/gcp03-restore-rehearsal-20261016")
    args = parser.parse_args()
    output = (ROOT / args.output_dir).resolve()
    testing_root = (ROOT / ".local" / "testing").resolve()
    if not output.is_relative_to(testing_root):
        raise SystemExit("output directory must be under .local/testing")
    output.mkdir(parents=True, exist_ok=True, mode=0o700)
    os.chmod(output, 0o700)
    if (output / "fixture.sql").exists() or (output / "fixture-manifest.json").exists():
        raise SystemExit("refusing to overwrite fixture or manifest from an existing run")

    try:
        migration_files = historical_v1_v61_migrations(MIGRATIONS)
    except ValueError as exc:
        raise SystemExit(str(exc)) from exc
    migration_hashes = [{"file": f.name, "sha256": sha256(f.read_bytes())} for f in migration_files]
    inventory_bytes = "".join(f"{item['file']} {item['sha256']}\n" for item in migration_hashes).encode()
    if not re.fullmatch(r"[0-9a-f]{64}", sha256(inventory_bytes)):
        raise SystemExit("could not compute migration inventory")
    fixture = FIXTURE_SQL.encode()
    if b"@example.test" not in fixture or b"fixture-not-a-credential" not in fixture:
        raise SystemExit("fixture synthetic-data guard failed")
    if any(term in fixture.lower() for term in (b"password=", b"token=", b"@gmail.com", b"@outlook.com")):
        raise SystemExit("fixture contains a disallowed credential/real-email pattern")

    sql_path = output / "fixture.sql"
    manifest_path = output / "fixture-manifest.json"
    sql_path.write_bytes(fixture)
    manifest = {
        "format": "espacigo-gcp03-restore-fixture-v1",
        "source": "deterministic hand-authored synthetic fixture; no persistent source/dataset access",
        "fixture_sql": {"file": sql_path.name, "bytes": len(fixture), "sha256": sha256(fixture)},
        "migrations": {
            "count": len(migration_hashes),
            "first": migration_hashes[0]["file"],
            "last": migration_hashes[-1]["file"],
            "inventory_sha256": sha256(inventory_bytes),
            "files": migration_hashes,
        },
        "expected_records": {"usuario": 2, "espacio": 1, "ocupacion": 2},
    }
    manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    os.chmod(sql_path, 0o600)
    os.chmod(manifest_path, 0o600)
    print(json.dumps({"fixture_bytes": len(fixture), "fixture_sha256": sha256(fixture), "migrations": 61, "manifest": str(manifest_path)}))


if __name__ == "__main__":
    main()
