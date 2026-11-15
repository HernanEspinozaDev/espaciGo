# Evidencia — VI-05, root del Store sustituido

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron todos los cambios preexistentes.

## Cambio

- `internal/adapters/documentfs/store.go`: `Store` conserva `FileInfo` del directorio validado al construirlo. `openRoot` exige que el root abierto sea el mismo inode validado y coincida con el path no-symlink observado mediante `Lstat`; ante discrepancia cierra el root y devuelve `ErrStorage`.
- Tests en `internal/adapters/documentfs/store_test.go` cubren sustituciones por symlink y por otro directorio real privado. `store_root_replacement_test.go` además verifica en ambos escenarios que los objetos cifrados del árbol original renombrado y del destino siguen byte a byte intactos, y que las lecturas/borrados del Store antiguo fallan cerrado.

## TDD y verificación

- RED observado antes del primer cambio: delete a través del root symlink retornó `nil` en vez de `ErrStorage`. RED adicional antes del pinning de inode: read a través del path reemplazado por otro directorio devolvió sus bytes sintéticos en vez de `ErrStorage`.
- GREEN: pruebas focales de reemplazo, `go test -race ./internal/adapters/documentfs -count=1`, suite `go test -race ./internal/adapters/postgres/documentverification -run '^TestRetentionProcessor' -count=1`, `go vet` de ambos paquetes, gofmt y `git diff --check` pasan; `go build -o /dev/null ./cmd/api` pasa.
- Comandos Go ejecutados con variables DB/PG ausentes (`DATABASE_URL`, variables PG y URLs opt-in de test).

## Límites

La revisión read-only `deleg_fcca0f51` aprobó el caso inicial de symlink y advirtió que Lstat/Stat no ofrece una garantía atómica para toda carrera; operaciones posteriores permanecen ancladas al `os.Root` abierto y podrían continuar sobre el árbol original si se renombra luego de validar. La revisión ampliada `deleg_cf749a1a` no aprobó (`passed=false`) por el límite no atómico del chequeo de path durante reemplazos concurrentes y por la carrera no cubierta durante `New`; no encontró errores lógicos y recomendó evitar cualquier claim de resistencia general a TOCTOU. La evidencia y el plan limitan el resultado a sustituciones estáticas probadas, sin prometer protección contra toda carrera. Su sugerencia de verificar ambos árboles se cubrió después mediante `TestPrivateStoreRootReplacementRejectsAccessWithoutMutatingEitherTree`, que conserva byte a byte el árbol original y el destino en los escenarios symlink/directorio. La revisión final read-only `deleg_b1bf7e86` aprobó sin hallazgos; confirmó los dos escenarios estáticos y la preservación de ambos objetos, manteniendo explícito el límite de reemplazo concurrente. No se afirma resistencia a toda carrera del host, atacantes fuera del modelo local, PostgreSQL/job durable, Cloud/GCS/IAM, borrado forense, backup/recuperación, logging o worker de producción. Es cambio local del Store; datos temporales y sintéticos. No se ejecutaron SQL/DDL, migraciones, consultas/conexiones a bases, Cloud ni proveedores. Sin commit, push, integración a `main` o despliegue.
