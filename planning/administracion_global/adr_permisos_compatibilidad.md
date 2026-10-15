# ADR GADM-00 — permisos de administración global y compatibilidad

- Estado: propuesto para revisión en GADM-00; **no implementado**.
- Fecha: 2026-10-14.
- Base examinada: `4cabb943a295569cadd330ed656d9d941e4f9517` (PR #311 en `main`).
- Alcance: documentar el inventario y la decisión de autorización para GADM-01–04. Este ADR no concede roles, no crea cuentas y no autoriza migraciones ni cambios de producto.

## Contexto observado

El rol de aplicación `administrador` es una cadena explícita y no hay herencia entre roles. `internal/identity/domain.go` define `RoleAdministrator`; `RegistrationRoles()` solo devuelve `arrendatario`. `AuthenticationService.Authorize` valida sesión y cuenta bajo bloqueo, vuelve a consultar `tx.Roles(...)` y compara cada rol por igualdad con el rol solicitado. Para operaciones de usuario, modo restringido y cuenta bloqueada también afectan la autorización. Esto da una base para revocación en peticiones posteriores, aunque cada mutación debe revalidar dentro de la transacción que la realiza.

El contrato persistente también está cerrado: `db/migrations/V000001__m01_identity.sql` limita `rol_usuario.rol` a `arrendatario`, `arrendador` y `administrador`. V1–V57 son inmutables. Si GADM-01 requiere el nuevo valor, la migración deberá ser nueva (V58 como siguiente versión libre tras revalidar el historial), con rollback/upgrade/base vacía cubiertos en ese corte.

La autorización administrativa está repartida y es literal en varias fronteras:

| Frontera | Comprobación actual | Consecuencia para el nuevo rol |
| --- | --- | --- |
| HTTP/servicios Go | Handlers de `adminlocal`, `booking`, `damageclaim`, `dispute`, `identity`, `localnotice`, `reputation` y `verification` pasan `identity.RoleAdministrator` o la cadena `administrador` a `Authorize`. | Cambiar solo el middleware/enum no alcanza. Cada operación debe recibir capacidad explícita y probar sus actores permitidos. |
| SQL dentro de servicios/repositorios | `internal/adminlocal/service.go:193` vuelve a exigir `rol='administrador'`; `internal/adapters/postgres/{damageclaim,localnotice,reputation}/repository.go` contienen validaciones similares. `identity/retention.go` exige administrador para una operación de retención. `V000036` selecciona administradores para avisos operativos. | Los nuevos gates SQL deben consultar el mismo evaluador/política transaccional; no relajar los gates existentes en bloque. Las consultas auxiliares y triggers también requieren inventario en GADM-01. |
| Esquema de datos | `rol_usuario_rol_ck` en V1 excluye `administrador_global`. | Mantener el historial; añadir una migración aditiva y mínima solo después de revisar V1–V57. |
| Contrato Go / sesión | `Role`, `Principal.Roles`, login/sesión retornan roles actuales. El alta concede solo `arrendatario`. | Añadir el rol a tipos/validadores de forma cerrada y conservar la prohibición de autopromoción en registro/edición de perfil. |
| OpenAPI / SDK | Los esquemas de roles tienen enums cerrados en `planning/openapi.yaml` (incluye modelos de identidad y cuenta). | Actualizar contrato y SDK en el mismo corte que lo soporte; no mandar un `role` arbitrario. |
| Angular | `identity.guard.ts` compara `requiredRole` exactamente; rutas de administración, navegación, `AdminFacade` e identidad comprueban `roles.includes('administrador')`. Los modelos de sesión repiten el union literal. | La UI solo refleja sesión/capacidades de servidor. Debe limpiar estado sensible y manejar respuestas antiguas al cambiar permisos; no decide autorización. |

## Decisión propuesta

1. Introducir en Go un catálogo cerrado de capacidades por operación y denegar por defecto. Cada handler/comando declara la capacidad exacta, además de validar cuenta activa, bloqueo, modo restringido, actor, recurso y estado de dominio. La UI y el nombre de la ruta no son autoridad.
2. Mantener `administrador` con su superficie histórica exacta. Su mapeo a capacidades será explícito y documentado a partir del inventario endpoint por endpoint, incluyendo requisitos de contexto y acciones de escritura. Ninguna capacidad nueva de administración global se añade a ese mapeo por conveniencia.
3. `administrador_global` será un rol distinto asignado solo por bootstrap local auditable de un UUID ya registrado/verificado del propietario. El bootstrap es de un solo uso/reejecutable de forma controlada, valida entorno/destino y no es una ruta pública de promoción. El rol otorga capacidades adicionales explícitas, no un bypass universal ni privilegios de PostgreSQL.
4. Operaciones globales nuevas se exponen en rutas/servicios dedicados o mediante una adaptación con política explícita y pruebas de compatibilidad. No cambiar silenciosamente los permisos de endpoints heredados. Cuando una operación común admita ambos roles, su tabla de política enumera capacidades aceptadas y se prueba administrador previo/global por separado.
5. El servidor consulta roles y permisos vigentes en cada petición relevante. Las mutaciones críticas vuelven a validar actor/capacidad y condición del recurso dentro de la transacción. Revocar rol, bloquear cuenta o revocar sesión debe invalidar accesos siguientes aun si el cliente conserva un Bearer previo. Proteger la condición de último administrador global con bloqueo transaccional y cubrir actualizaciones concurrentes.
6. Bootstrap y recuperación no contienen contraseña fija, cuenta compartida, secreto versionado ni autofacilitación de rol. El maestro de ensayo vive en DB/credenciales aisladas propias. La cuenta definitiva no se activa hasta que el propietario identifique su UUID/correo por el canal acordado.
7. Las capacidades de soporte quedan fuera de este ADR y de la implementación GADM-00. No se crean roles, tickets ni endpoints de soporte.

## Catálogo inicial a refinar

Se conserva como vocabulario propuesto del plan; todavía no es contrato implementado:

- Cuentas: `accounts.list`, `accounts.read`, `accounts.update`, `accounts.block`, `accounts.roles`.
- Publicaciones: `publications.list`, `publications.read`, `publications.update`, `publications.moderate`, `publications.retire`.
- Operación: `operations.read`, `operations.manage`.
- Finanzas: `finance.read`, `finance.operate`.
- Auditoría: `audit.read`, `audit.export`.
- Indicadores: `kpi.read`.
- Administración de accesos: `admin_access.manage`.

La matriz adjunta deberá enlazar cada operación existente/nueva con actor, capacidad, regla de recurso, auditoría y caso positivo/negativo antes del primer cambio de permisos en GADM-01.

## Consecuencias y límites

La autorización continúa en Go y en la transacción propietaria de cada mutación. Postgres sigue siendo ejecutado por la identidad runtime existente, no por una identidad superusuario concedida a la aplicación. El rol global nunca permite leer credenciales, editar ledger/contratos/historial libremente ni impersonar a otro usuario. Los flujos de privacidad/retención siguen rigiendo bajas y datos obligatorios. El permiso de lectura sensible y su motivo auditado se separan del acceso agregado y de los DTO minimizados.

La aceptación de este ADR solo aprobaría el diseño documental de GADM-00. No significa que las capacidades estén implementadas, que exista una cuenta maestra o que cualquier administrador actual tenga permisos globales.
