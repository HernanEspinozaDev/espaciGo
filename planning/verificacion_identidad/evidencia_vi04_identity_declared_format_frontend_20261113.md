# VI-04 — declaración de formato identity en UI con fixtures sintéticos

**Fecha:** 2026-11-13 (`America/Santiago`)

## Alcance

Caracterización test-only en el checkout frontend autorizado `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes/frontend`, rama `frontend-next-hermes`, base HEAD `cf6e28a365d2495e78bc64c01686ed55efb17c8a`. Los archivos afectados ya estaban dirty por trabajo compartido; se conservaron los cambios existentes y solo se ampliaron las aserciones del flujo de creación identity y su test de normalización de dominio.

- `tests/phase-f6-document-verification.spec.ts`: la creación identity verifica que el selector ofrece exactamente `legacy` («Formato anterior»), `new` («Formato nuevo») y `unsure` («No estoy seguro»); muestra el aviso de que el formato observado lo determina la revisión humana y que el formato declarado no acredita autenticidad. Selecciona `legacy` y exige el cuerpo exacto de creación sintética con `document_format_declared: "legacy"`, consentimiento explícito y su versión. El DTO fake de respuesta refleja el formato declarado.
- `tooling/document-verification-domain.test.mjs`: el normalizador acepta y conserva cada uno de `legacy`, `new` y `unsure`, y rechaza el valor no admitido `digital`.

No se cambió código productivo. No se afirma RED/GREEN: las tres opciones ya existían y esta adición caracteriza el comportamiento implementado. El test no prueba la admisibilidad legal de documentos vencidos o vigentes ni el servicio Go real.

## Verificación ejecutada

- Test Node focal `node --test tooling/document-verification-domain.test.mjs` — PASS 2/2.
- Playwright focal `identity creation shows role-aware capture without exposing a download` — PASS 1/1.
- `pnpm test` — PASS 107/107.
- `pnpm typecheck`, `pnpm lint` y `pnpm format:check` — PASS.
- Suite E2E completa — 84 PASS, 45 omitidas, 1 FAIL de 130. El fallo es `tests/phase8-bookings.spec.ts:823`, Axe color-contrast en botones con colores `#278e6f` (4.05:1) y `#eb2730` (4.31:1). La prueba focal aislada de esa misma ruta repitió el fallo. No pertenece a los archivos ni al alcance de este subcorte y no se cambió.
- No se reconstruyó `.next/standalone`: solo cambiaron tests; el artefacto local existente corresponde al código UI ya construido en el corte anterior.
- Revisión source-only independiente `deleg_39394b10` — PASS, sin hallazgos; confirmó opciones, aviso de revisión humana, payload `legacy` exacto y límites de los mocks.

## Límites

El E2E intercepta `/api/backend/**` y usa cuenta/caso/respuestas sintéticas. La prueba de dominio ejecuta solo el normalizador frontend. No demuestra BFF→Go, OAuth/proveedor, validación legal, autorización, persistencia, almacenamiento, lectura automática o ausencia de automatización fuera del cliente probado. No se ejecutó SQL/DDL ni se accedió a bases persistentes, Cloud, proveedores externos o datos reales. Sin commit, push, despliegue o merge. El criterio completo de formato/aceptación del documento y los gates persistentes de VI-04/MVP siguen abiertos.
