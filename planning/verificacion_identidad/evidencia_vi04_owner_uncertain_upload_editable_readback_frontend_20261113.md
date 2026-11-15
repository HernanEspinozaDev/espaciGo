# VI-04 — caracterización de recuperación editable tras carga identity incierta

**Fecha:** 2026-11-13 (`America/Santiago`)

## Alcance

Se añadió una caracterización Playwright test-only en el checkout autorizado `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes/frontend`, rama `frontend-next-hermes`, base HEAD `cf6e28a365d2495e78bc64c01686ed55efb17c8a`. No se modificó código productivo en este subcorte.

`tests/identity-upload-uncertain-recovery.spec.ts` cubre la rama complementaria de recuperación de carga incierta:

1. La primera carga sintética usa `If-Match: "1"` y recibe `409 etag_conflict`.
2. La persona solicita explícitamente actualizar; el fake devuelve `requiere_antecedentes`, revisión 2 y ETag `"2"`, un estado que sigue permitiendo aportar evidencia.
3. Se comprueba que el mismo control conserva exactamente un `File` seleccionado, «Cargar evidencia» vuelve a estar habilitado y el contador de POST permanece en uno: no hubo reintento automático.
4. Solo después de un segundo clic deliberado se envía la misma parte multipart sintética (rol `identity_front`, nombre y bytes centinela) con `If-Match: "2"`. El fake devuelve 201 con evidencia `pendiente` y revisión 3; el CTA de envío a revisión permanece ausente porque la evidencia aún no está limpia.

Todas las rutas `/api/backend/**` están interceptadas en Playwright y usan respuestas, cuenta, expediente y archivos sintéticos. La prueba caracteriza la UI y la solicitud emitida por el navegador; no invoca el BFF ni Go.

## Verificación

- Playwright focal, ambas ramas de recuperación de carga incierta — PASS 2/2 en la versión final del test.
- Suite E2E completa — PASS 85, omitidas 45, total 130, con las aserciones finales y el tamaño fake calculado por `Buffer.byteLength(...)`.
- Playwright focal de ambas ramas de recuperación — PASS 2/2 tras las aserciones finales.
- `pnpm typecheck`, `pnpm lint` y `pnpm format:check` — PASS con el test final. El artefacto `.next/standalone` existente se había generado tras el cambio de UI anterior; no se reconstruyó porque este subcorte solo añadió una prueba.
- No se observó RED y no se afirma RED/GREEN: la rama editable ya estaba implementada; el cambio añade caracterización de la conducta existente.
- La revisión source-only inicial `deleg_14236fdd` no aprobó la primera versión y pidió hacer explícitas las comprobaciones de identidad del File, ausencia de reintento durante el refresh, ETag del detalle actualizado y bloqueo del submit por evidencia pendiente. Se añadieron esas aserciones y se repitieron los gates indicados. La revisión source-only final `deleg_d0d0183f` aprobó sin findings y confirmó los cuatro comportamientos, limitados a UI y red interceptada sintéticas.

## Límites

No demuestra el estado del BFF, respuesta real de Go, autorización/grants, almacenamiento, inspección, CAS ni persistencia. No se ejecutó SQL/DDL ni se accedió a bases persistentes, Cloud, proveedores externos o datos reales. No hubo commit, push, despliegue ni merge. VI-04 y el MVP siguen parciales en sus gates de persistencia y almacenamiento real.
