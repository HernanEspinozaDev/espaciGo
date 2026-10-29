# Ejecución por cortes — cierre de espacios, galería y documentos

Fecha: 2026-10-28. Checkout: `/home/nandev/Documentos/projects/espaciGo`, rama `main`, HEAD base `3956d2f3459b0bf4b15511f4e1a74ffd06a29975`. El trabajo previo sin commit se preserva.

## Alcance autorizado y límites

El owner pidió continuar el orden concreto de cierre y reasignar agentes al terminar cada alcance. Esta instrucción habilita continuar implementación y pruebas locales sintéticas; no sustituye los permisos específicos pendientes de F6/F7. No aplicar DDL a bases persistentes, recrear el target retirado, iniciar scanner/servicios nuevos, usar cuentas reales ni operar cloud por inferencia. Sin commit, push o deploy en este corte. No modificar `AGENT.md`/`AGENTS.md` ni leer secretos.

## Baseline ejecutada por el padre

- Rama/HEAD comprobados; worktree previo con cambios preservado.
- `ss` no mostró listeners en 15432, 18083, 4203 ni 4205 al inicio.
- Con variables opt-in DB retiradas, `go test -count=1 ./internal/spaces/... ./internal/adapters/postgres/spaces ./internal/identityverification/...` pasó.
- Este baseline no ejecuta integración PostgreSQL ni acredita recorrido autenticado.

## Asignación actual

| Corte | Dueño / unidad | Archivos exclusivos | Puerta de cierre |
| --- | --- | --- | --- |
| CAS publicación y contenido Go | `deleg_291c14c8`, tarea 0 | `internal/spaces` y repositorio PostgreSQL spaces; sólo endpoints publication/publication-content en este corte | RED→GREEN; parseo antes de body, comparación bajo lock, bump monotónico y ETag; tests y revisión del padre |
| Replacement lookup ready/error | `deleg_291c14c8`, tarea 1 | `listing-editor.tsx`, `phase7-host-listings.spec.ts` | Reconciliación, misma clave sin replay, portada/ETag y errores; padre ejecuta build único y Playwright |
| Ciclo documental Go | `deleg_291c14c8`, tarea 2 | Nuevo paquete `internal/documentverification` | Reglas de política, cuarentena, cuotas, revisión separada, vigencia/revocación y puerto CAS; tests ejecutables; no equivale a flujo operativo |
| Contrato/adapter CAS publicación | `deleg_3f9d3318`, tarea 0 | OpenAPI, adapter host-listings, cambios necesarios de facade, prueba unitaria nueva y derivados generated/inventory | If-Match requerido, ETag de respuesta preservado, 412 sin retry y tests RED→GREEN |

Un único dueño del build/Playwright: el padre. Ningún agente debe esperar un build futuro: entrega su alcance y declara navegador pendiente. El padre verifica archivos, ejecuta gates y solicita revisión independiente antes de dar por completado un corte. Los agentes no deben extender por su cuenta el ownership a otros archivos; reciben siguiente asignación después de la revisión.

## Cola de continuación

1. Finalizar y revisar las entregas anteriores; corregir regresiones hasta GREEN.
2. CAS tarifas: `pricing.Service.UpdateRate`, repositorio PostgreSQL pricing y ruta tariff en el handler spaces. Compartir la misma revisión del espacio; conservar snapshots de reservas. Serializar edición del handler con el dueño anterior.
3. Localizar y cerrar CAS de horario semanal en su ruta real y repositorio; no confundir consulta `occupancy.Availability` con el writer del horario. Preservar reglas de zona IANA/DST y bloqueos existentes.
4. CAS galería: portada, metadata/visibilidad, orden y retiro para caminos sintéticos y media reales. No confundir bump con comparación de revisión esperada; no degradar el fallback a un writer sin CAS.
5. Corpus sintético representativo del codec con dimensiones/pesos/orientación/metadatos y límites; no afirmar reducción de peso universal.
6. F5: después de revisar el contrato de dominio, implementar persistencia y permisos, API/BFF/UI, scanner real y ciclo de retención. El dominio aislado no cierra F5.
7. Resolver explícitamente target aislado, cuenta/canal sintético y scanner privado antes de integración autenticada F6. No reutilizar ni recrear recursos retirados sin revalidar autorización.
8. F7 sólo tras cierre F6 y autorización específica de backup/migración/release.

## Estado de evidencia

Las tareas anteriores están asignadas, no completadas. La prueba futura debe registrarse con comandos y resultados reales.

### Reasignación por entregas incompletas

`deleg_291c14c8` devolvió: CAS sin implementar (RED retirado por el agente), mocks ready/error sin assertions de navegador y dominio documental con TOCTOU reconocido. Ninguna entrega se aceptó como terminada. El padre inspeccionó fuentes y reasignó en `deleg_5d0ce3fc`: CAS reducido a publication; assertions concretas ready/error; rediseño documental con transición atómica, autorización bajo snapshot y corrección de ownership/UUID/campos inyectados. El contrato adapter/OpenAPI de `deleg_3f9d3318` sigue separado.

En paralelo, el padre añadió y ejecutó seis escenarios sintéticos del codec: `TestConvertSyntheticScenarioCorpus` PASS. La medición reveló incumplimientos de SLO en textura y transparencia; ver `espacios-f4-codec-corpus-sintetico.md`. No se modificó el codec ni se declara completo el corpus fotográfico.

### Recuperación del CAS de publicación por el padre

La segunda entrega Go dejó el handler llamando una API inexistente. El padre retiró sólo esa modificación incompleta y conservó el resto del árbol; añadió `TestPublicationPreconditions` y observó RED real (sin header:400 en vez de428; tag malformado/stale:200; éxito sin ETag renovado). Implementó `RevisionAwarePublicationRepository` y `SetPublicationStateIfRevision`, wrapper PostgreSQL que compara tras `FOR UPDATE` antes de no-op/historial/writes, bump monotónico y ETag HTTP. El test enfocado, suite spaces/transport/repositorio/documentverification y vet pasaron en el padre. No se ejecutó PostgreSQL real. El siguiente corte publication-content se asignó en `deleg_4f456e31`; la revisión documental read-only en `deleg_8800ea3a`. Galería contiene assertions nuevas aún pendientes de build/Playwright del padre. Ninguna fase global se cierra por estos resultados. Las fases F3/F4/F5/F6 siguen parciales; F7 entrega bloqueada por secuencia y autorización. F1 está fijada como contrato, no implementada transversalmente. F2 conserva su cierre acotado al PostgreSQL nativo V68–V76; V77/V78 sólo tienen evidencia del target sintético retirado.

### Enmienda posterior — CAS tarifas (2026-10-28)

La ruta de tarifa ya no está pendiente de conectar: el padre integró `PUT /tariff` con `If-Match` 428/400/412 y ETag renovado, y el frontend/OpenAPI lo conectó. Evidencia detallada: `espacios-cas-tarifa-20261028.md`. Go focused suites/vet, frontend API check 183 ops, format/typecheck/lint/Node 267/267/build y Playwright `navegación por pestañas` 1/1 pasaron en el padre. Review Go independiente PASS. Una primera ejecución Playwright falló por mock de tarifa sin ETag; se corrigió el mock para reflejar contrato y renovación, luego el caso enfocado pasó. Revisión frontend independiente sigue pendiente. No se ejecutó DB real; CAS horario y mutadores galería siguen abiertos. Esto completa únicamente un corte de F1, no una fase global.

### Reanudación — CAS de metadatos/orden de galería (2026-10-28)

- El contrato `updateOwnSpaceMediaPhoto` quedó actualizado: `If-Match` requerido, ETag renovado en 200 y respuestas 400/412/428; generación de tipos e inventario actual (`183 operaciones`) verificados con `api:check`.
- El corte frontend previo a la simplificación de la operación pasó `api:check`, `format:check`, `typecheck`, `lint`, `pnpm test` (281/281) y `pnpm build`. Playwright local `@phase7` terminó 23 passed/1 skipped (solo la prueba nativa opt-in omitida). La primera corrida encontró una expectativa de mock obsoleta: el DELETE debía usar el ETag nuevo tras el reordenamiento; se corrigió la aserción y la repetición completa pasó. Puerto local 4203 quedó libre. Todo este E2E es mock/local y no acredita backend real.
- La regresión RED HTTP para `PUT /api/v1/spaces/{spaceId}/gallery/{photoId}` sin `If-Match` se observó: respondió 200 y llamó al fake de mutación en vez de 428 sin mutar. La implementación backend CAS de servicio/repositorio y handler está en curso, con archivos de propiedad separados; sin DB ni DDL.
- La auditoría read-only confirmó que el wizard crea un espacio general pero manda horarios al endpoint fixture-only. Sigue pendiente decidir si restringir el wizard o implementar un API general de horarios; no se cambió contrato ni SQL.
- Se detectó que la versión frontend secuencial podía dejar un ETag local obsoleto tras un fallo parcial. Se inició un corte TDD para que cada movimiento adyacente use un solo PUT de posición CAS; requiere repetir format/typecheck/lint/unit/build y Playwright en el padre al integrarlo.
- F5 documental sigue limitado a dominio/diseño por gates de persistencia, storage/ACL, scanner y rutas; F6 autenticado y F7 cloud siguen sujetos a target/autorización. No hubo DB, commit, push ni deploy.

### Continuación — API owner-scoped de horarios semanales (2026-10-29)

- Se implementó `GET/PUT /api/v1/spaces/{spaceID}/weekly-hours`, independiente del feature flag/pagos de booking-trial; se conserva sin cambios la ruta fixture-only histórica. GET/PUT requieren sesión autenticada y el repositorio limita por `espacio_id + propietario_id`, cuenta activa, espacio no eliminado y tarifa `hora`; no exige el KYC de lectura pública. La ruta devuelve el ETag de revisión compartida del espacio; PUT usa `If-Match`, responde 428/400/412 y renueva ETag sólo al éxito. La implementación no aplica DDL.
- RED adicional observada: `Content-Type: application/json; charset=utf-8` daba 415. El decoder ahora analiza el media type y admite parámetros, rechaza formatos incompatibles, limita el body a 16 KiB, bloquea campos desconocidos y JSON final adicional. Test focal GREEN.
- Padre verificó `env -u TEST_DATABASE_URL -u IMG01F_TEST_DATABASE_URL -u DATABASE_URL -u PGHOST -u PGPORT -u PGDATABASE -u PGPASSWORD go test ./cmd/api ./internal/booking ./internal/booking/transport/http ./internal/adapters/postgres/booking -count=1`, `go vet` en esos paquetes y `git diff --check`; todos pasaron. Las pruebas PostgreSQL gated quedaron omitidas porque las variables de integración se retiraron; no hubo conexión ni evidencia PostgreSQL. La fuente V61 está tracked, pero no se comprobó su aplicación y no se ejecutó migración.
- La regresión gated dentro de `TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry` ahora crea un espacio horario sintético separado sin fila en `reserva_ensayo_local_fixture`; comprueba que siga sin fila antes y después de lectura/escritura, y valida lectura, escritura CAS, conflicto por revisión stale y aislamiento de otro owner. Compila y pasa el package test, pero su cuerpo PostgreSQL se omitió con `TEST_DATABASE_URL` unset, por lo que no es evidencia de ejecución real.
- Frontend/contrato: `api:generate` y `api:check` verificaron 185 operaciones; `format:check`, `typecheck`, `lint` y `pnpm test` pasaron (281/281). `pnpm build` pasó. Playwright `@phase7 --project chromium-local --workers 1`, contra standalone local 127.0.0.1:4203, pasó 23 y omitió 1 prueba nativa opt-in. Esa suite usa mocks para el recorrido de host; no acredita Go/BFF/DB real. El puerto 4203 quedó libre.
- No commit, push ni deploy. F1 continúa parcial: contrato/adapter/handler/repositorio conectados localmente, pero persisten sin resolver la verificación de esquema aplicado y la integración real autorizada.

### Continuación — CAS de metadatos/orden de galería (2026-10-29)
- CAS real-media de metadata/visibilidad/posición está conectado en servicio, PostgreSQL y handler. El review independiente no encontró fallas de scope/orden transaccional/ETag, pero sí que el repositorio aceptaba posición no positiva si se invocaba directamente. Se añadió guard previo a `Begin` y regresión para 0/-1; RED observado como panic por pool nulo antes del guard y GREEN después. `go test ./internal/media ./internal/adapters/postgres/media -count=1`, `go vet`, `gofmt` y `git diff --check` pasaron con variables DB unset.
- El test HTTP real-media cubre `If-Match` ausente (428), weak/wildcard/múltiple/malformado (400), espacio ajeno y revisión stale del mismo espacio (412; el caso stale sí llega al CAS fake), éxito (200 + ETag fuerte renovado y llamada CAS, sin writer legacy). Padre verificó 2 pruebas y 7 subtests con `go test -v ./internal/gallery/transport/http -run '^TestRealMediaMetadata' -count=1`, además de vet, formato y diff-check. Evidencia con fake; PostgreSQL no ejecutado. Review independiente `deleg_941fac42` PASS para el guard defensivo, sin hallazgos; la prueba sólo acredita rechazo temprano antes de `Begin`, no transacción PostgreSQL.
- El reorder frontend de galería usa un único PUT por movimiento adyacente; el padre adaptó la regresión de facade al método singular tras detectar una referencia obsoleta a `reorderPhotos`. Gates actuales: `api:check` 183 operaciones, typecheck, lint, `pnpm test` 281/281, `format:check` y `pnpm build` pasaron. Playwright en servidor local construido, sin banderas nativas, pasó el caso enfocado 1/1 y la suite `@phase7` 22/22; el mock verifica photo ID, posición, If-Match, ETag renovado, 412 sin retry ni cambio local. Es evidencia sintética UI, no backend/DB. Puerto 4204 quedó libre.
- CAS de publicación: servicio/repositorio para fotos sintéticas implementados con revisión esperada bajo lock, no-op sin bump y actualización+bump transaccionales. OpenAPI usa `setOwnSpaceGalleryPhotoPublication`, documenta If-Match fuerte, ETag, 400/412/422/428 y respuesta `oneOf` sintética/multimedia; `api:generate` y `api:check` pasan con 183 operaciones. Review `deleg_cf79f794` confirmó el orden CAS/lock y encontró que `validUUID` aceptaba whitespace al validar pero pasaba el original. Se añadió regresión RED para owner/space/photo y `validUUID` ahora valida el token exacto sin trim; GREEN, `go test ./internal/gallery ./internal/adapters/postgres/gallery -count=1`, vet, gofmt y diff-check pasan con `TEST_DATABASE_URL` unset. Follow-up review independiente `deleg_0a8fa7ae` PASS, sin hallazgos; sugirió como opcional una prueba directa del helper. No hay evidencia PostgreSQL. La integración HTTP CAS de publicación para fotos sintéticas y fallback multimedia está implementada. El handler valida If-Match antes del JSON, responde 428/400/412 según el caso, usa `SetPublicInCatalogCAS`/`UpdateSpaceCAS`, retorna ETag renovado y evita writers legacy. Tests cubren missing/malformed/weak/wildcard/multiple/stale/success sintético y fallback multimedia success/stale; se añadieron assertions de owner/space/photo/revisión en ambas rutas, JSON inválido sin llamadas a CAS/writers y ningún ETag en error. Review independiente `deleg_a2c190de` PASS sin hallazgos de seguridad/lógica; sus sugerencias de cobertura quedaron atendidas por el padre. Padre rerun: `env -u TEST_DATABASE_URL go test ./internal/gallery ./internal/adapters/postgres/gallery ./internal/gallery/transport/http -count=1`, `go vet` en los tres paquetes, gofmt check y diff-check; todo pasa. Evidencia fake/unit/package solamente, no PostgreSQL.


