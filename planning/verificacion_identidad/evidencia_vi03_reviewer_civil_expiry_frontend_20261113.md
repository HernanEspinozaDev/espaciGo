# Evidencia VI-03 — fecha civil de vencimiento en decisión reviewer

**Fecha:** 2026-11-13 (`America/Santiago`)

## Alcance

Se amplió únicamente la prueba unitaria `frontend/tooling/reviewer-identity-decision.test.mjs` en el worktree autorizado `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`, rama `frontend-next-hermes`, HEAD base `cf6e28a365d2495e78bc64c01686ed55efb17c8a`.

La prueba ejercita el builder público `buildReviewerIdentityDecisionRequest` y comprueba que:

- las fechas imposibles (`2031-02-29`), no canónicas (`2031-4-05`) y timestamp (`2031-04-05T23:59:59.999Z`) fallan cerradas tanto para aprobación como para solicitud de antecedentes;
- el día bisiesto válido `2032-02-29` se conserva sin convertirlo a timestamp ni cambiar su valor civil.

No se modificó código productivo. El contrato OpenAPI ya representa `document_expires_at_observed` como `format: date`; el adaptador frontend debe preservar la fecha civil exacta.

## Verificación

- `node --test tooling/reviewer-identity-decision.test.mjs`: 11/11 PASS.
- `pnpm test`: 114/114 PASS.
- `pnpm lint`, `pnpm typecheck` y `pnpm format:check`: PASS.
- En el checkout backend `/home/nandev/Documentos/projects/espaciGo-main-sync`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`, el mismo comando Go focal con `-race` pasó después de cambiar `TestReviewerIdentityApprovalHTTPPreservesCivilExpiryThroughService` para enviar `2032-02-29` por el handler y servicio reales; verifica frontera exclusiva en snapshot/auditoría y fecha civil original en DTO. Los otros tests filtrados cubren brecha DST, timestamp y fechas malformadas. Repositorio, grants y demás dependencias son fakes; no abrió conexión a base.
- Revisión source-only `deleg_93e8e348`: aprobó los casos frontend, con la limitación concreta de que el día bisiesto aún no estaba cubierto en Go. Se añadió la caracterización HTTP/servicio anterior para atenderla. Revisión source-only posterior `deleg_bd8b0f4f`: PASS; confirmó el flujo handler→servicio, reloj anterior al vencimiento, grant exacto y aserciones de frontera en snapshot/auditoría y fecha original en DTO. No ejecutó pruebas y señaló que el test Go es un archivo nuevo no seguido en HEAD.

## Límites

La prueba frontend valida el builder de dominio, no una solicitud navegador→BFF→Go. El test Go de ruta compone handler y servicio reales con datos, grants y repositorio fake; no acredita PostgreSQL, persistencia, atomicidad durable, autorización efectiva, runtime desplegado ni Cloud. No se ejecutó SQL/DDL, no se accedió a bases persistentes/Cloud ni a proveedores externos. No se ejecutó Playwright E2E en este subcorte. Sin commit, push, deploy o integración a `main`.
