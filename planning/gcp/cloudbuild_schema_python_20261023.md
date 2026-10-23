# Cloud Build — Python del gate de esquema (2026-10-23)

El operador reportó que el release `dca208fe-d9b2-4de7-8186-b0176685ccc3` del merge `5d89800ca01ce60cdf498e2cb3a255fb63d3c43a` se detuvo en el paso 0 antes del lock/deploy: `python3: command not found`. La atestación ya había pasado generación y digest. SQL 74/74 y el binding metadata SMTP siguen validados por ese operador; no repetir esas mutaciones para corregir un runtime local del contenedor.

La imagen observada `gcr.io/google.com/cloudsdktool/google-cloud-cli:stable` tiene digest `sha256:1855ee6031f03bb7d4b251cfd0e5b88a18fd9a2552a742cf86d269810832c80b`. Ensayo local dentro de esa imagen: `command -v python3` no encuentra ejecutable; `gcloud info --format='value(basic.python_location)'` devuelve `/usr/lib/google-cloud-sdk/platform/bundledpythonunix/bin/python3`.

El paso de gate queda fijado a ese digest. Resuelve el runtime mediante `gcloud info`, exige ruta absoluta ejecutable e invoca el validador existente con esa ruta, sin instalar paquetes ni asumir una ubicación fija. Conserva el orden generación → digest → esquema → SMTP → lock y la misma SA/ruta de release. El [procedimiento incremental canónico](cloudsql_migraciones_incrementales.md) permanece vigente.

Pruebas ejecutadas:

- `scripts/test-cloudbuild-schema-sdk.sh`: PASS; extrae el comando de validación del YAML y lo ejecuta en la imagen fijada, red `none`, filesystem de solo lectura y config efímera sin credenciales. Reporte sintético válido aceptado; checksum de migración alterado rechazado. Usa el fixture del validador, no una atestación Cloud SQL real. Añadido a `application-images` en CI.
- `python3 scripts/test_img01_schema_release_evidence.py`: 7 tests PASS.
- `python3 scripts/validate_cloud_build_transition.py`, `bash -n scripts/test-cloudbuild-schema-sdk.sh` y `git diff --check`: PASS.

Falta el release real del nuevo SHA y E2E online; el ensayo del contenedor no los sustituye. El helper local histórico `scripts/verify-m01.py:100` aún espera 422 por falta de `use_preference` y `:136` espera rol arrendatario al registrar, contratos anteriores a #533. No se ejecutó ni se afirma cobertura integrada de ese helper; requiere actualización antes de reutilizarlo para el nuevo registro. Las pruebas vigentes de registro sin preferencia están descritas en `planning/imagenes/evidencia/onboard-dual-role/validacion.md`.
