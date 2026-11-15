# Evidencia — VI-05, límites de armado del archivo propio con fuentes sintéticas

- **Fecha:** 2026-11-13.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron cambios ajenos.

## Brecha y alcance

`TestExportOwnArchiveOmitsOperationalRolesAndKeepsFileBytesOutOfMetadata` ejecuta `privacy.Service.ExportOwnArchive` real con `memoryRepository` y un fake de fuentes de módulos. Comprueba que el módulo recibe el account ID indicado, que los roles operativos se excluyen del bloque identity, que `manifest.json` y su metadata de archivo tienen exactamente las claves JSON esperadas, y que el payload binario sintético del fixture aparece solo en su entrada ZIP y no en los JSON. `TestExportOwnArchiveReturnsNoPartialArchiveWhenModuleSourceFails` verifica que el error de una fuente no produce ZIP parcial y se propaga sin envoltorio. Ambos son tests-only de caracterización; pasaron al primer run válido, sin cambio productivo ni afirmación RED/GREEN.

## Verificación local

- Focal: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 -v ./internal/privacy -run '^(TestExportOwnArchiveOmitsOperationalRolesAndKeepsFileBytesOutOfMetadata|TestExportOwnArchiveReturnsNoPartialArchiveWhenModuleSourceFails)$'` — PASS (2/2).
- Paquete: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 ./internal/privacy` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/privacy` — PASS.
- `gofmt -d` y chequeo explícito de whitespace/newline del archivo Go — PASS.
- Revisión source-only independiente `deleg_b6c455c0`: PASS, sin concerns ni errores lógicos; confirmó las claves exactas de manifest/metadata, el payload fixture fuera de JSON y el alcance al owner ID. No ejecutó pruebas. La sugerencia de precisar que las fuentes son fakes quedó incorporada. No se amplió el corte a nombres `files/` vacíos o duplicados: el revisor no los consideró blocker ni halló vulnerabilidad demostrada bajo el productor revisado; este test no afirma cubrir esos casos.

## Límites

Las fuentes de módulos y el repositorio de identidad son fakes; account ID se pasa directamente al servicio. No demuestra autenticación HTTP, autorización del adaptador, aislamiento del repositorio, contenido exportado por un módulo productivo, SQL/PostgreSQL, persistencia, integridad frente a una cuenta ajena ni exportación en runtime. El test solo caracteriza el ensamblaje ZIP del servicio. No se ejecutó SQL/DDL ni se accedió a DB persistente, Cloud o datos reales. Sin commit, push, deploy o integración a `main`.
