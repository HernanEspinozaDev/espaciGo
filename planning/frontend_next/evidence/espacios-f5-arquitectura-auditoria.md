# F5 — auditoría de arquitectura de verificación

**Fecha:** 2026-10-28  
**Repositorio:** `/home/nandev/Documentos/projects/espaciGo`, rama `main`, HEAD observado `3956d2f3459b0bf4b15511f4e1a74ffd06a29975`; el worktree contiene cambios locales sin commit.  
**Alcance:** inspección estática read-only de verificación, documentos, booking y publicación. No se editaron fuentes durante la auditoría, no se inició API/DB/servicios y no se accedió a datos privados. La política vigente está en `espacios-f5-politica-owner-aprobada.md`.

## Hallazgos

- `internal/verification` y `internal/adapters/postgres/verification` implementan el flujo existente de casos, revisión, revocación, historial y elegibilidad. La evidencia actual se identifica como fixture local (`local-fixture-v1`, `fixture:`); no acredita identidad documental.
- `internal/adapters/postgres/booking` consulta la elegibilidad KYC sintética para ambas partes durante operaciones de reserva. No existe una fuente de verificación de identidad documental vigente independiente que satisfaga la política aprobada.
- La publicación de espacios ya no exige esa elegibilidad. Mantiene ownership, estado, zona horaria, conflicto de fixture e historial; el resultado del test aislado de publicación sin KYC está registrado en `espacios-f5-publicacion-sin-kyc-local.md`.
- No hay dominio/repositorio separado para distintivo de identidad o verificación documental por espacio; tampoco existe intake privado con cuarentena, scanner, ACL por propósito, prevención de auto-revisión, expiración y retención aprobada. El módulo de contratos y el almacenamiento de galería no son sustitutos seguros para documentos de identidad/espacio.
- No se debe convertir automáticamente una elegibilidad o aprobación antigua de fixture KYC/KYB en el distintivo nuevo. El distintivo requiere la verificación manual aprobada y vigente; el distintivo documental del espacio es otro alcance y no es requisito para publicar o contratar.

## Corte siguiente recomendado

Conservar separadas identidad documental y KYC sintético; añadir primero pruebas de contrato sintéticas para que el booking solo se confirme si ambas partes tienen identidad documental verificada vigente, y que la elegibilidad KYC de fixture por sí sola no satisface ese gate. Mantener la publicación independiente de identidad y documentos del espacio. Después diseñar contratos de dominio/repositorio separados para identidad y documentos del espacio, en cortes pequeños con permisos/retención/scanner fail-closed de la política aprobada. No aplicar DDL persistente ni iniciar API/DB hasta completar F6 y confirmar el target sintético aislado.

**Límite de evidencia:** auditoría de código, no prueba de booking/documentos contra PostgreSQL ni de flujo browser→BFF→Go→storage. El corte recomendado no fue implementado en esta auditoría.