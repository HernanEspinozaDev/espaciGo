# F5 — Contrato documental privado, preparación no operativa

**Fecha:** 2026-10-28  
**Repositorio:** `/home/nandev/Documentos/projects/espaciGo`, rama `main`, HEAD base `3956d2f3459b0bf4b15511f4e1a74ffd06a29975`.  
**Clasificación:** evidencia de diseño; no es una decisión legal/operativa ni una implementación habilitada.

> **Snapshot histórico al 2026-10-28, anterior a la aprobación del owner.** La tabla de decisiones pendientes refleja únicamente ese corte. Las decisiones vigentes están en `espacios-f5-politica-owner-aprobada.md`; las capacidades, ejecución de scanner y pendientes de integración se reconcilian en `espacios-f5-documentos-flujo-arquitectura.md` y `espacios-f5-delta-persistencia-integracion-20261029.md`. Este documento sigue sirviendo como evidencia de que sus rutas/namespace no se activaron en ese corte, no como lista actual de aprobaciones.

## Alcance completado

Se dejó explícito que el namespace de documentos descrito en §5.2 del plan es una propuesta todavía no aplicada. Las rutas ilustrativas bajo `users/{user_id}/spaces/{space_id}/documents/v1/...` y `users/{user_id}/documents/v1/...` no se han activado ni validado como política. El prefijo `staging/...` tampoco autoriza crear un flujo de carga o un recolector.

La revisión read-only no encontró un validador de claves documentales ni un endpoint genérico de carga de documentos en los árboles `internal/` o `cmd/api/` inspeccionados. Los adaptadores de media son específicos de fotos/WebP; no se deben ampliar a PDF por analogía. Consentimiento de registro, contratos sintéticos de reservas y evidencias sintéticas de verificación conservan finalidades y permisos propios; no aprueban un cargador documental de espacios.

## Restricciones técnicas que ya fija el plan (no activación)

- Mantener documentos fuera de la galería y del pipeline de imágenes WebP.
- En una futura función aprobada, derivar identidad/vínculo desde principal y DB, no desde valores de propietario aportados por el cliente; evitar PII y nombres de archivo del usuario en claves.
- Lectura solo por Go autorizado y respuesta privada `no-store`/`nosniff`; no habilitar bucket público, URL permanente ni acceso directo del navegador a GCS por inferencia.
- No reutilizar la retención de contratos sintéticos, replay de privacidad o artefactos de Cloud Build como política documental.

Estas restricciones son criterios de diseño del plan, no una aprobación para crear storage, tablas, validadores, rutas, controles de acceso o tareas de borrado.

## Decisiones que requieren al owner

| Decisión                                                                                           | Estado                                     |
| -------------------------------------------------------------------------------------------------- | ------------------------------------------ |
| Finalidad, clases permitidas y entidad asociada (usuario, espacio u otra)                          | Pendiente                                  |
| Quién carga, lee, revisa, aprueba/rechaza y audita; acceso de soporte                              | Pendiente                                  |
| MIME, tamaño, formatos, antimalware, cuarentena y fallos de inspección                             | Pendiente                                  |
| Estados de revisión y auditoría de acceso/cambios                                                  | Pendiente                                  |
| Cifrado de aplicación/storage y gestión de claves                                                  | Pendiente                                  |
| Retención por clase/estado, inicio del plazo, excepciones, retención legal y borrado lógico/físico | Pendiente                                  |
| Necesidad de bucket, IAM, lifecycle u otros cambios de infraestructura                             | Pendiente y requiere autorización separada |

## No implementado / no probado

No se añadieron uploads/downloads, tablas o migraciones, validadores de namespace, UI, ACL, scanner, lifecycle, recolector, bucket, IAM ni cambios GCP/Terraform. No hubo consulta ni escritura de DB ni tests de servicio. Por ello, F5 queda **parcial: contrato de preparación no operativo**; el despliegue de una función documental sigue bloqueado hasta que el owner resuelva las decisiones anteriores y autorice por separado cualquier cambio de infraestructura.

**Referencias:** `plan_creacion_espacios_galeria_documentos.md` §§5.2–6 y `espacios-auditoria-storage.md` (auditoría previa de storage; no prueba upload documental).
