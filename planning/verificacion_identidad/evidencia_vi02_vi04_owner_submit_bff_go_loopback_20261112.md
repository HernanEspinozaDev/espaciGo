# Evidencia VI-02/VI-04 — owner identity submit BFF→Go loopback (2026-11-12)

## Alcance

Caracterización local de solo pruebas del envío owner de una solicitud identity por el export `POST` de la ruta BFF Next y su allowlist generado, hasta el handler Go registrado y `documentverification.Service.Submit`. El backend usa `httptest.NewServer`, un repositorio/reader in-memory sintético y autenticador fake. El script ejecuta el export de ruta TypeScript transpilado; sustituye únicamente la configuración de upstream por la URL loopback del `httptest`.

Archivos nuevos:
- `internal/documentverification/transport/http/identity_owner_submit_bff_go_integration_test.go`
- `frontend/tooling/identity-owner-submit-bff-go-integration.mjs`

## Comportamiento afirmado por las aserciones

- Modo éxito: POST al recurso synthetic identity con `If-Match: "7"`, `Authorization: Bearer synthetic-session`, sin Cookie, body ni Content-Type. La respuesta BFF es 200 con ETag `"8"`, `private, no-store`, JSON y DTO exacto allowlisted: estado `enviada`, revisión 8, formato declarado `new` y tres roles identity únicos; no incluye ID del titular ni storage key.
- Modo `stale`: el fake inicia en `CaseDraft`/revisión 8 y el BFF envía `If-Match: "7"`. El handler Go devuelve 409 antes de `Submit`; el BFF responde con payload neutral exacto, `private, no-store`, sin ETag ni Set-Cookie y con `x-request-id` igual al `request_id`. No se filtran texto de revisión/conflicto ni detalle upstream.
- Modo `retry`: el fake inicia ya `CaseSubmitted`/revisión 8, con auditoría previa sintética de envío, y el BFF repite `If-Match: "7"`. La respuesta es el 200/ETag `"8"` y DTO vigente; se observa una lectura owner, cero transiciones y estado/revisión/auditoría sin cambios.
- El servicio y el handler Go son reales. El fake aplica la transición y audit sintéticos solo en éxito (revisión 7→8, dos lecturas owner y una transición); stale observa una lectura, cero transiciones y estado/revisión/auditoría sin cambios. El fake valida actor, titular, propósito, caso y revisión y ejecuta el callback de dominio si se alcanza.

El primer intento devolvió 404 porque el fixture sintético de archivos limpios omitía `ContentSHA256` no vacío requerido por `ValidateQuota`; se añadieron digests sintéticos distintos y el test focal pasó. También se vinculó explícitamente el `ActorID` sintético en la autorización fake. Es corrección del fixture de caracterización; no se afirma RED/GREEN ni defecto de producción.

## Verificación parent-owned

- Focal éxito: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" ESPACIGO_FRONTEND_ROOT=/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes/frontend ESPACIGO_OWNER_SUBMIT_SCENARIO=success go test -race -tags=bffintegration -count=1 ./internal/documentverification/transport/http -run '^TestNextBFFSubmitsOwnerIdentityCaseThroughActualGoService$' -v` — PASS, 1/1.
- Focal stale: mismo comando con `ESPACIGO_OWNER_SUBMIT_SCENARIO=stale` — PASS, 1/1.
- Focal retry inmediato: mismo comando con `ESPACIGO_OWNER_SUBMIT_SCENARIO=retry` — PASS, 1/1.
- Suite dominio+HTTP: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" ESPACIGO_FRONTEND_ROOT=/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes/frontend go test -race -tags=bffintegration -count=1 ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go vet -tags=bffintegration ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `node --check tooling/identity-owner-submit-bff-go-integration.mjs` y `node_modules/.bin/prettier --check tooling/identity-owner-submit-bff-go-integration.mjs` — PASS.
- `gofmt` y checks explícitos de whitespace para ambos archivos — PASS.
- Revisión independiente inicial del modo éxito `deleg_440c0288`: PASS sin hallazgos bloqueantes; sus sugerencias de aserción del seam Authenticator y unicidad de roles se incorporaron. La re-revisión `deleg_e143d7fc` aprobó esa versión sin hallazgos ni sugerencias.
- Revisión independiente del candidato éxito+stale `deleg_e5f177b6`: PASS, sin findings bloqueantes; sugirió aclarar la aserción de las dos respuestas según escenario y contar explícitamente requests al handler. El 200/DTO/headers y el 409 neutral/headers ya se verifican respectivamente en cada escenario; añadí `requestCount == 1` bajo mutex y repetí focales, suites y gates arriba. La re-revisión del delta `deleg_90554aba` aprobó sin concerns, errores lógicos ni sugerencias.
- Revisión independiente del candidato éxito/stale/retry `deleg_48beac7e`: PASS sin concerns de seguridad ni errores lógicos; sugirió mantener los claims acotados a la caracterización sintética fake-backed, como se documenta abajo.

## Límites

Fake-only: no acredita grants ni autenticación efectiva, PostgreSQL/CAS, atomicidad, rollback durable, persistencia o reinicio. No ejecuta storage, scanner, Next standalone/middleware, servicio API desplegado, proveedores externos, SQL/DDL, base persistente, Cloud ni personas/datos reales. La ruta exportada y el handler son código real invocado en un harness local; no es un runtime Next productivo ni un recorrido de navegador.
