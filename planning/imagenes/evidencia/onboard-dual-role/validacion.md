# PR #533 — validación local de continuación (2026-10-23)

Fuente inicial `65ae5866b728631a7e93b46d416be6097cfc9fd2`, integrada con `origin/main` en `53cea9310d198d2a6f545b2b095ccf640c19c19c`; cambios y arnés incluidos en el mismo commit que este registro. Reproducir desde el HEAD actualizado del PR. No se modificó V1–V73; V74 permanece aditiva e intacta respecto al candidato inicial.

- `scripts/test-img01-release-migrations-postgres.sh`: PASS en PostgreSQL/PostGIS 18 desechable, red none/tmpfs, sin volumen retenido. V67→74 con repair V64 como owner NO SUPERUSER CREATEROLE, ACL TEMP separada, ledger V1–67 conservado, guards, roles mínimos, lector sin DDL y postcheck sin pendientes; instalación vacía V1–74 y variante de rutina ausente también pasan. Sin credenciales reales.
- `go test ./internal/migrator ./cmd/dbmigrate ./internal/identity/...`: PASS. Las integraciones que requieren DSN no se afirman por este comando; las tres integraciones de plan/ledger seleccionadas se ejecutan con DSN del arnés anterior.
- `python3 scripts/test_img01_schema_release_evidence.py`: 7 tests PASS, incluido rechazo de membresías adicionales/SET/INHERIT/otro grantor del worker.
- `python3 scripts/validate_cloud_build_transition.py`, `bash -n scripts/capture_img01_schema_release_evidence.sh` y `git diff --check`: PASS.
- Terraform 1.16.6 oficial con checksum verificado: `init -backend=false -lockfile=readonly`, `validate` y `test`: PASS, 13 planes mock, sin backend/credenciales ni cambios Cloud. `fmt -check` de archivos afectados: PASS. Nuevo binding metadata SMTP con opt-in, sin Accessor ni secreto/versión nuevos; plan remoto sigue pendiente del operador.
- Angular 22.2.2, Node 24.21.0, `npm ci`; `npm test -- --watch=false --include=src/app/core/identity/identity.guard.spec.ts --include=src/app/app.spec.ts --include=src/app/features/hosting/hosting-spaces-page.spec.ts`: 47 tests PASS. Guard y creación/refresco prueban cuenta sin roles y cuenta arrendataria; navegación cambia tras confirmación Go de ambos roles y sigue bloqueada en modo restringido. `npm run build` y `npm run lint` (incluida arquitectura): PASS.

La [captura del primer borrador para arrendatario](arrendatario-primer-borrador.png) usa navegador Chromium con servidor Angular local y API simulada: login sintético con rol arrendatario → enlace Ofrecer espacio → `/panel/espacios` → formulario de primer borrador. Sin registro de una cuenta real, SQL, SMTP, GCS ni creación persistente; es evidencia de navegación/UI, no E2E integrado. La confirmación tras crear se verifica en los tests de la fachada con respuesta simulada del servidor.

Cloud SQL, repairs remotos, V68–74 remotas, deploy y prueba online siguen pendientes del operador/revisión/autorización correspondientes. No se declara aceptación de producto.
