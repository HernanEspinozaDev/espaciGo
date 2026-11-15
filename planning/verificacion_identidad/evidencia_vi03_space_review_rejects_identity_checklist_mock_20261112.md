# Evidencia VI-03 — rechazar checklist identity en revisión space

- **Fecha:** 2026-11-12
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD:** `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Cambio de este subcorte:** solo una prueba de caracterización en `internal/documentverification/transport/http/reviewer_cases_test.go`; no se cambió código de producción.

## Comportamiento cubierto

`TestSpaceReviewRejectsIdentityChecklistBeforeMutation` envía por el handler real un POST de revisión de un caso `space`, con capability `review_space` y una carga completa sintética de campos exclusivos de revisión identity (`document_format_observed`, fecha civil y checklist). El handler responde 400 neutral sin ETag, conserva `Cache-Control: private, no-store` y solo realiza la lectura preflight; el fake no recibe llamada de revisión genérica ni identity, ni readback.

La brecha se eligió después de medir cobertura de `internal/documentverification/transport/http`: la rama de rechazo del checklist identity en el camino no tipado (línea 284 de `reviewer_cases.go`) tenía cero hits en el perfil inicial. El perfil focal posterior registra ambas instrucciones de esa rama. El test pasó en su primera ejecución, como corresponde a una caracterización del guard ya existente; no se afirma RED/GREEN ni cambio funcional.

## Verificación local

Con entorno mínimo sin variables DB/PG:

- Test focal `go test ./internal/documentverification/transport/http -run '^TestSpaceReviewRejectsIdentityChecklistBeforeMutation$' -count=1`: PASS.
- Cobertura focal confirma `reviewer_cases.go:284.3,286.1` con conteo `1`.
- `go test -race ./internal/documentverification/... -count=1`: PASS en dominio, inspección y transporte HTTP.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y `git diff --check`: PASS.

## Límites y estado del worktree

El authenticator y servicio son fakes; se caracteriza el guard HTTP y que el payload no alcance el puerto de mutación, no la autorización real, el servicio de dominio, la transición space, PostgreSQL ni persistencia. No se ejecutó SQL/DDL, migración, consulta ni conexión a base persistente o Cloud. No se usaron datos reales.

El checkout ya estaba ampliamente dirty, y tanto el test como el plan eran archivos untracked antes de este subcorte. Se preservó ese trabajo preexistente y se limitó el cambio a agregar esta prueba y la evidencia/actualización del plan. Sin commit, push, integración a `main` ni deploy.
