# Evidencia VI-03 — scope del propietario en descargas reviewer space

**Fecha:** 2026-11-13
**Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
**HEAD:** `274f37e483a078890e92382a159fa49fd23c2fe8`

## Alcance

`TestReviewerSpaceDownloadRejectsUntrustedSpaceOwnerBeforeStoreRead` ejercita `ReviewerAccessService.Download` con caso space, archivo limpio y grant exacto sintéticos. Cubre las dos formas de fallo del guard de propietario: el actor reviewer también es el propietario del espacio y el `SpaceOwnerID` de la autorización no coincide con el titular del caso. Ambos devuelven `ErrNotFound`, documento vacío y cero lecturas de storage. El control positivo `TestReviewerAccessSpaceDownloadUsesAssignedSpaceScopeAndStore` confirma que un reviewer distinto del propietario, con dueño y grant exactos, conserva la descarga válida.

La inspección de cobertura sobre tests reviewer anteriores marcó el bloque de `reviewer_access.go:269-271` sin cubrir (`ReviewerAccessService.Download` 95% en esa selección); la corrida de cobertura focal posterior confirma que el guard ahora queda cubierto.

## Verificación ejecutada

- Casos negativo y positivo: `go test -race -count=20` — PASS.
- Perfil focal del guard de líneas 269–271: cubierto — PASS.
- `go vet ./internal/documentverification`: PASS.
- `gofmt`: PASS.
- Caracterización test-only; primer run válido PASS. No se observó RED ni se modificó código productivo.

## Límites

Repository, grants y storage son fakes. Esto prueba la rama de autorización en `ReviewerAccessService` en memoria, no grants efectivos, adapter PostgreSQL ni persistencia. No se ejecutaron SQL/DDL, acceso a DB persistente/Cloud, commit, push, deploy o integración a `main`.
