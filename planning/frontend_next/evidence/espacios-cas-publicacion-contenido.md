# CAS de publicación y contenido — corte local

Fecha: 2026-10-28. Base `main` / `3956d2f3459b0bf4b15511f4e1a74ffd06a29975`, cambios sin commit preservados. Alcance: PUT `publication` y `publication-content`; no cierra CAS de tarifas, horarios o galería.

## Implementación actual

- HTTP requiere un único If-Match fuerte antes de decodificar JSON: ausente 428, malformado 400, recurso/tag obsoleto 412; éxito expone ETag.
- Servicio usa interfaces versionadas sin fallback legacy. Revisión vacía se rechaza antes de invocar repositorio.
- PostgreSQL mantiene lock de propietario activo y lock de espacio; compara revisión antes de historial/insert de tarifa/cambios, incluso antes de no-op. Un cambio efectivo usa `spaceversion.Bump` en la misma transacción sin reemplazar previamente el timestamp por `now()`.
- Wrappers legacy internos se conservan; no son el camino de los dos endpoints protegidos.
- OpenAPI declara precondición, ETag y 400/412/428, y elimina el requisito obsoleto de KYC para publicar. El adapter requiere tag, preserva ETag renovado y propaga 412 sin retry. El editor envía la revisión visible al publicar/ocultar.
- Acciones del listado: obtienen detalle por ID antes de cada acción y envían su revisión en una única mutación. El padre añadió `tests/host-listings-publication-cas.spec.ts`: publicar→ocultar usa tags renovados; 412 no reintenta y GET503 no envía mutación. Test focal PASS y suite `@phase7` posterior 23 passed/1 opt-in native skipped. La revisión independiente del listado está pendiente; no se afirma prueba específica de cambio tardío A→B por este test.

## Pruebas ejecutadas por el padre

- Publication: RED observado para ausencia antes de body (400 vs428), tag inválido/stale (200) y ausencia de ETag renovado. `TestPublicationPreconditions` pasó tras implementar.
- Contenido: la entrega inicial del agente no estaba GREEN; el padre corrigió el doble de repositorio y requests explícitos, eliminó el helper que inventaba tags desde `time.Now()`, y observó RED wrong-resource (400 vs412). `TestPublishedContentPreconditions` pasó con error correcto, ausencia/malformado/stale/current y ausencia de mutación ante rechazo.
- Service contenido: RED revisión cero alcanzaba repositorio; corregido a rechazo antes de llamar. Regresiones de ambas operaciones verifican propagación de revisión/stale y fallo cerrado sin repositorio versionado.
- `go test -count=1 ./internal/spaces/... ./internal/adapters/postgres/spaces` con variables opt-in DB retiradas: PASS; `go vet` de esos paquetes: PASS.
- Frontend API check: 183 operaciones; formato, tipos, lint, unit tests 264/264 y build: PASS en el padre antes del último endurecimiento ETag.
- E2E edición tras build: 1/1 PASS. El primer intento mostró ausencia de revisión en caller del editor; se corrigió y reconstruyó antes de volver a probar. El mock ahora aserta que publication recibe ETag vigente.
- Validación del adapter: se añadió RED de lista de tags, espacio y DEL; se exige un único tag opaco válido, y la regresión enfocada `node --test tooling/host-listings-publication-cas.test.mjs` pasó 4/4 tras el endurecimiento. Se deben repetir gates globales/build tras ese cambio.

## Límites

No se ejecutó PostgreSQL real para estas dos nuevas rutas CAS, ni navegador autenticado contra Go/BFF. Los tests de repositorio del paquete no acreditan concurrencia/rollback/permisos runtime en DB. La revisión independiente de los dos endpoints está asignada; este documento no la anticipa. F3/F4/F6 siguen parciales. No migraciones, cuenta real, cloud, commit/push/deploy.
