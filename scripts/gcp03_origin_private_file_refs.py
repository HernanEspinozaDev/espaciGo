"""Canonical read-only inventory of live private-file references."""
import re

class PrivateFileReferenceQueryError(RuntimeError):
    """Sanitized query failure carrying only an optional PostgreSQL SQLSTATE."""
    def __init__(self, sqlstate: str | None = None):
        super().__init__("private file reference query failed")
        self.sqlstate = sqlstate if sqlstate and re.fullmatch(r"[0-9A-Z]{5}", sqlstate) else None

PRIVATE_FILE_REFS_SQL = """
SELECT COALESCE(json_agg(file_id::text ORDER BY file_id), '[]'::json)::text
FROM (
  SELECT DISTINCT file_id FROM (
    SELECT archivo_id AS file_id FROM public.foto_perfil_sintetica_local WHERE archivo_id IS NOT NULL
    UNION ALL SELECT archivo_id FROM public.espacio_galeria_sintetica_local WHERE archivo_id IS NOT NULL
    UNION ALL SELECT archivo_id FROM public.verificacion_evidencia_sintetica WHERE archivo_id IS NOT NULL
    UNION ALL SELECT archivo_id FROM public.espacio_galeria_archivo_candidato_local WHERE archivo_id IS NOT NULL
    UNION ALL SELECT archivo_id FROM public.operacion_arriendo_archivo_candidato_local WHERE archivo_id IS NOT NULL
    UNION ALL SELECT archivo_id FROM public.verificacion_archivo_retiro_pendiente_local WHERE archivo_id IS NOT NULL
    UNION ALL SELECT evidencia_id FROM public.baja_archivo_pendiente_local
      WHERE completada_en IS NULL
    UNION ALL SELECT id FROM public.operacion_arriendo_evidencia_ensayo_local
      WHERE archivo_retirado_en IS NULL
  ) AS all_file_refs
) AS unique_file_refs;
"""
