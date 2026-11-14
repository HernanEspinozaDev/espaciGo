# DEMO-00 — reconciliación aislada completada

## Fuentes y preservación

- Worktree original: `/home/nandev/Documentos/projects/espaciGo-rent-published-space`.
- Rama original: `codex/marketplace/feature/rent-published-space-flow`.
- HEAD original: `75c8c21d30c70a4fd63f6109664d5c2a063410df`.
- `origin/main` después del fetch: `f3671834727417733a11132aef783fa9941ba012`.
- Relación verificada con Git: original ancestro de main, 0 commits exclusivos originales y 13 de main.
- Candidato: `/home/nandev/Documentos/projects/espaciGo-rental-demo-reconciled`, HEAD desacoplado sobre el main indicado. No se movió ninguna rama, no se creó commit ni se publicó.
- Se preservaron 116 archivos rastreados modificados y nuevos mediante copia privada y manifiesto SHA-256 bajo scratch. Se rechazaron symlinks y rutas privadas en la importación. No se copiaron `.local/`, secretos ni recursos de ejecución.
- Readback por hash del worktree original: 116 archivos, cero diferencias contra el manifiesto después de generar los artefactos del candidato.
- La importación usa merge de tres vías para cambios superpuestos: base original, main y archivo local. Se resolvieron tres conflictos en repository de catálogo, sus tests de servicio y transporte HTTP. La revisión del padre confirmó que el delta productivo del repository respecto de main es únicamente la interfaz de pool para tests: se conservan alias/orden de columnas cover_source, proyección de media, COALESCE y cierre de filas vigentes. Los fixtures se adaptaron al scan de main; se conservaron ambos grupos de pruebas.

## Validación ejecutada sobre el candidato

- Dependencias frontend: `pnpm install --offline --frozen-lockfile` PASS; 471 paquetes reutilizados, cero descargas.
- Regeneración del OpenAPI combinado: `node tooling/api-generate.mjs && node tooling/generate-api-inventory.mjs && node tooling/generate-api-allowlist.mjs && node tooling/api-check.mjs` PASS. Inventario 203 operaciones (53 directas, 106 por familia/prefijo, 44 no verificadas); clasificación generada no acredita 203 rutas activas.
- `go test -race -count=1 ./internal/calendar ./internal/pricing ./internal/identityverification ./internal/rentalflow/... ./internal/openapicontract` PASS. Variables de conexión/proveedor retiradas; sin servidores ni base de datos.
- Frontend: `pnpm typecheck` PASS; `pnpm lint` exit 0 con cuatro advertencias preexistentes; `pnpm test`: 508 tests, 507 PASS, 1 SKIP opt-in de baseline, cero fallos.
- Advertencias lint: img en listing-gallery; variable retainAuthLifecycle en authenticated-operation.test; queryPath en public-verification-refresh.test; expresión en wizard-native-https. No se cambiaron archivos ajenos para ocultarlas.
- Validación independiente posterior del padre: `go test -race -count=1 ./internal/publiccatalog/...` PASS; selección fake `TestListCoverImageSourceMatchesPublicContract|TestPublicListQueryFailureDoesNotLeakPartialItems|TestDetailScanFailuresDoNotLeakPartialPublicDTO` PASS con race; `go build -o /dev/null ./cmd/api` PASS; `git diff --check` PASS. No se ejecutó API, SQL, Playwright ni build frontend en DEMO-00.
- DEMO-00 queda cerrado como reconciliación local en candidato, no como commit/push ni integración a main. El worktree original conserva sus archivos y su HEAD. DEMO-01 continúa con contrato explícitamente separado; sus pruebas y cambios posteriores no se atribuyen al baseline reconciliado.

## Alcance posterior

Rige A–G del plan actualizado: DEMO-01–05 usan una composición local Go separada, estado sintético compartido y reiniciable, dos calendarios mensuales conectados y resultado de pago simulado. Los guards reales KYC/contrato/pago permanecen intactos. Ningún éxito en memoria prueba SQL, pago, identidad legal ni reserva comercial. No se requiere DB para esta demo.
