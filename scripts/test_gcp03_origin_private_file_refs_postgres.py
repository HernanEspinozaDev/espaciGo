"""Opt-in PostgreSQL integration against the complete V1–V61 schema.

Run only from test-gcp03-private-file-refs-postgres.sh, which creates a fresh
network-isolated disposable PostgreSQL 18/PostGIS database and marks its Unix
socket directory. Fixtures are synthetic and all database writes are local.
"""
from __future__ import annotations

import json
import os
from pathlib import Path
import subprocess
import unittest

from gcp03_origin_private_file_refs import PRIVATE_FILE_REFS_SQL


ROOT = Path(__file__).resolve().parents[1]
SOCKET = os.environ.get("G03O_PRIVATE_FILE_REFS_TEST_SOCKET")
MARKER = ".gcp03-private-file-refs-isolated-test"

FIXTURE_SQL = r"""
SET session_replication_role = replica;
INSERT INTO public.foto_perfil_sintetica_local
  (id,usuario_id,archivo_id,mime_type,fixture_code,sha256,size_bytes,estado,creada_en,clave_idempotencia)
VALUES
  ('a0000000-0000-4000-8000-000000000001','a1000000-0000-4000-8000-000000000001','a2000000-0000-4000-8000-000000000001',
   'image/png','synthetic-png-v1',repeat('a',64),10,'activa',now(),'fixture-profile-01');
INSERT INTO public.espacio_galeria_sintetica_local
  (id,espacio_id,propietario_id,archivo_id,fixture_code,mime_type,sha256,size_bytes,estado,creada_en,clave_idempotencia)
VALUES
  ('b0000000-0000-4000-8000-000000000001','b1000000-0000-4000-8000-000000000001','b2000000-0000-4000-8000-000000000001','b3000000-0000-4000-8000-000000000001',
   'synthetic-png-v1','image/png',repeat('b',64),20,'activa',now(),'fixture-gallery-01');
INSERT INTO public.verificacion_evidencia_sintetica
  (id,verificacion_id,codigo_fixture,mime_type,tamano_bytes,sha256,archivo_id,retirar_archivo_en,archivo_retirado_en)
VALUES
  ('c0000000-0000-4000-8000-000000000001','c1000000-0000-4000-8000-000000000001','synthetic-png-v1','image/png',30,repeat('c',64),
   'c2000000-0000-4000-8000-000000000001',NULL,NULL),
  ('c0000000-0000-4000-8000-000000000002','c1000000-0000-4000-8000-000000000002','synthetic-png-v1','image/png',31,repeat('c',64),
   NULL,NULL,now());
INSERT INTO public.espacio_galeria_archivo_candidato_local
  (archivo_id,propietario_id,espacio_id,estado,creada_en,proximo_intento_en)
VALUES ('a2000000-0000-4000-8000-000000000001','d1000000-0000-4000-8000-000000000001','d2000000-0000-4000-8000-000000000001',
        'pendiente_limpieza',now(),now());
INSERT INTO public.operacion_arriendo_archivo_candidato_local
  (archivo_id,reserva_id,actor_id,tipo,estado,creada_en,proximo_intento_en)
VALUES ('d3000000-0000-4000-8000-000000000001','d4000000-0000-4000-8000-000000000001','d5000000-0000-4000-8000-000000000001',
        'checkin','pendiente_limpieza',now(),now());
INSERT INTO public.verificacion_archivo_retiro_pendiente_local
  (evidencia_id,archivo_id,disponible_en)
VALUES ('e1000000-0000-4000-8000-000000000001','c2000000-0000-4000-8000-000000000001',now());
INSERT INTO public.baja_archivo_pendiente_local
  (ejecucion_id,evidencia_id,disponible_en,completada_en)
VALUES
  ('f1000000-0000-4000-8000-000000000001','f2000000-0000-4000-8000-000000000001',now(),NULL),
  ('f1000000-0000-4000-8000-000000000001','f3000000-0000-4000-8000-000000000001',now(),now());
INSERT INTO public.operacion_arriendo_ensayo_local
  (id,reserva_id,tipo,actor_id,ocurrio_en,zona_horaria,ubicacion_sintetica,resultado,clave_idempotencia,huella_solicitud,creada_en)
VALUES
  ('10000000-0000-4000-8000-000000000001','11000000-0000-4000-8000-000000000001','checkin','12000000-0000-4000-8000-000000000001',now(),'America/Santiago',
   '{"source":"synthetic-fixture-v1","location_code":"santiago-demo-center-v1","latitude":-33.45,"longitude":-70.66}','registrada','fixture-op-checkin',decode(repeat('1',64),'hex'),now()),
  ('10000000-0000-4000-8000-000000000002','11000000-0000-4000-8000-000000000002','checkout','12000000-0000-4000-8000-000000000001',now(),'America/Santiago',
   '{"source":"synthetic-fixture-v1","location_code":"santiago-demo-center-v1","latitude":-33.45,"longitude":-70.66}','registrada','fixture-op-checkout',decode(repeat('2',64),'hex'),now()),
  ('10000000-0000-4000-8000-000000000003','11000000-0000-4000-8000-000000000003','recepcion','12000000-0000-4000-8000-000000000001',now(),'America/Santiago',
   '{"source":"synthetic-fixture-v1","location_code":"santiago-demo-center-v1","latitude":-33.45,"longitude":-70.66}','recepcion_conforme','fixture-op-recepcion',decode(repeat('3',64),'hex'),now()),
  ('10000000-0000-4000-8000-000000000004','11000000-0000-4000-8000-000000000004','descargo','12000000-0000-4000-8000-000000000001',now(),'America/Santiago',
   '{"source":"synthetic-fixture-v1","location_code":"santiago-demo-center-v1","latitude":-33.45,"longitude":-70.66}','registrada','fixture-op-descargo',decode(repeat('4',64),'hex'),now());
INSERT INTO public.operacion_arriendo_evidencia_ensayo_local
  (id,operacion_id,fixture_code,mime_type,sha256,size_bytes,creada_en,archivo_retirado_en)
VALUES
  ('20000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000001','synthetic-png-v1','image/png',repeat('2',64),40,now(),NULL),
  ('20000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000002','synthetic-png-v1','image/png',repeat('2',64),41,now(),NULL),
  ('20000000-0000-4000-8000-000000000003','10000000-0000-4000-8000-000000000003','synthetic-png-v1','image/png',repeat('2',64),42,now(),NULL),
  ('20000000-0000-4000-8000-000000000004','10000000-0000-4000-8000-000000000004','synthetic-png-v1','image/png',repeat('2',64),43,now(),now());
"""


@unittest.skipUnless(SOCKET, "run with the isolated private-file-reference PostgreSQL harness")
class PrivateFileReferencePostgresTests(unittest.TestCase):
    def psql(self, sql: str) -> str:
        marker = Path(SOCKET) / MARKER
        if not marker.is_file() or marker.is_symlink():
            self.fail("isolated PostgreSQL harness marker is absent")
        result = subprocess.run(
            ["psql", "-X", "-qAt", "--set=ON_ERROR_STOP=1", "--host=" + SOCKET,
             "--port=5432", "--username=postgres", "--dbname=espacigo_scratch"],
            input=sql, text=True, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
            check=True, timeout=60,
        )
        return result.stdout.strip()

    def test_complete_v1_to_v61_schema_live_reference_union(self):
        histories = self.psql("SELECT count(*) || ':' || min(version) || ':' || max(version) FROM public.schema_migrations")
        self.assertEqual(histories, "61:1:61")
        self.psql(FIXTURE_SQL)
        found = json.loads(self.psql(PRIVATE_FILE_REFS_SQL))
        expected = {
            "a2000000-0000-4000-8000-000000000001",
            "b3000000-0000-4000-8000-000000000001",
            "c2000000-0000-4000-8000-000000000001",
            "d3000000-0000-4000-8000-000000000001",
            "f2000000-0000-4000-8000-000000000001",
            "20000000-0000-4000-8000-000000000001",
            "20000000-0000-4000-8000-000000000002",
            "20000000-0000-4000-8000-000000000003",
        }
        self.assertEqual(set(found), expected)
        self.assertEqual(len(found), len(expected), "UNION must deduplicate overlapping direct references")
        self.assertNotIn("f3000000-0000-4000-8000-000000000001", found, "completed deletion queue entry is not live")
        self.assertNotIn("20000000-0000-4000-8000-000000000004", found, "retired descargo blob is not live")
        self.assertNotIn("c0000000-0000-4000-8000-000000000002", found, "retired metadata id is not a file id")


if __name__ == "__main__":
    unittest.main()
