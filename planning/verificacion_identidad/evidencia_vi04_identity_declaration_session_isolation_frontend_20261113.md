# Evidencia VI-04: aislamiento de declaración entre cuentas

**Fecha:** 2026-11-13 (`America/Santiago`)

## Checkout y alcance

Checkout frontend autorizado: `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`, rama `frontend-next-hermes`, HEAD `cf6e28a365d2495e78bc64c01686ed55efb17c8a`. El test `frontend/tests/identity-declaration.spec.ts` caracteriza A→logout→B→logout→A y logout final con `page.route` y cuentas/declaraciones sintéticas. Los cambios permanecen en el worktree; no se atribuyen al commit HEAD.

## Comportamiento comprobado

- Registra todos los GET de declaración, incluso anónimos, y comprueba la secuencia autenticada A,B,A; no hay GET adicional tras el logout final.
- Verifica los cuatro valores de cada declaración autoritativa en A y B, ausencia de datos cruzados, y borradores no guardados de nombres/apellidos aislados.
- Al volver a A, comprueba nombre, apellidos, fecha y RUT autoritativos; al logout final, comprueba ausencia de datos privados, campos, consentimiento, formulario, controles de edición/guardado/consulta y estado guardado.
- Revisión read-only final `deleg_526ba34d`: PASS, sin hallazgos; limita expresamente el resultado a la prueba mock.
- Ejecución parent-owned: `pnpm test:e2e --grep 'identity declaration stays isolated across account changes and logout' --workers=1 --output test-results/identity-declaration-a-b-a-reviewed-final-20261113` — PASS, 1/1.

## Límites

La red del E2E se intercepta: no demuestra autenticación, aislamiento ni persistencia reales en backend/Go/PostgreSQL. La caracterización separada de respuesta tardía pasó parent-owned (focal 1/1; suite 88 PASS/45 omitidas/0 FAIL, 133); su revisión source-only independiente está en curso y consta en `evidencia_vi04_identity_declaration_late_response_frontend_20261113.md`. No se ejecutó SQL/DDL ni se accedió a bases persistentes, Cloud, proveedores, sitios públicos, cuentas o datos reales. Sin commit, push, despliegue o integración a `main`.
