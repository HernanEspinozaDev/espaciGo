# F5 — Diseño propuesto del ciclo documental privado

**Fecha:** 2026-10-28  
**Repositorio:** `/home/nandev/Documentos/projects/espaciGo`, rama `main`; cambios locales sin commit.  
**Estado:** diseño no operativo basado en la política aprobada en `espacios-f5-politica-owner-aprobada.md`. No autoriza storage, DDL, API, scanner ni infraestructura.

## 1. Principio de separación

Implementar dos expedientes independientes, con clases de acceso y distintivos separados:

1. **Identidad de cuenta:** caso del titular autenticado, documentos mínimos de cédula o pasaporte y revisión manual. Su resultado vigente puede habilitar contratación cuando ambas partes cumplen; no bloquea explorar, preparar ni publicar.
2. **Antecedentes del espacio:** caso asociado por el servidor a un espacio y su cuenta administradora. Acepta la clase habilitante que aplique (propiedad, autorización, otro antecedente o permiso/certificado pertinente). Solo puede otorgar distintivo del espacio; no es requisito para publicar o contratar.

No compartir objetos, casos, estados, ACL ni evidencia de identidad con galería, KYC/KYB fixture, contratos sintéticos u operaciones. Una elegibilidad antigua de fixture nunca migra automáticamente a identidad documental aprobada.

## 2. Límites aprobados por clase

| Dimensión | Identidad | Espacio |
|---|---|---|
| Solicitud abierta | Máximo una por cuenta | Máximo una por espacio |
| Archivos | JPEG, PNG, PDF; 10 MiB por archivo; máximo 3 | JPEG, PNG, PDF; 10 MiB por archivo; máximo 10 |
| Páginas PDF | Máximo 5 | Máximo 20 |
| Revisión | Manual; personal con capacidad de revisar identidad | Manual; capacidad separada de revisión de espacio; sin autoaprobación |
| Vigencia | Hasta 24 meses desde revisión y nunca después del vencimiento documental | Hasta 12 meses y nunca después del vencimiento del antecedente |
| Visibilidad externa | Solo distintivo «identidad contrastada mediante documentación» y su alcance | Solo distintivo del espacio y su alcance |

Reemplazos y cargas pendientes consumen cuota. La interfaz y el API no deben exigir todos los tipos de antecedentes: se presenta solo la alternativa aplicable.

## 3. Arquitectura lógica propuesta

- **Dominio:** entidades `IdentityVerificationCase` y `SpaceDocumentCase` independientes; cada caso referencia objetos documentales por identificadores opacos y sin nombre original/PII en la clave. Separar estado de caso, estado de inspección por archivo y resultado vigente de verificación.
- **Autoridad:** Go deriva sujeto, espacio, cuenta administradora y capacidades de revisión de la sesión/datos autorizados. No aceptar del cliente propietario, reviewer, `space_id` arbitrario como prueba de permiso ni clave de almacenamiento. Los prefijos de objetos nunca son ACL.
- **Persistencia:** repositorios separados por propósito; historial de cambios mínimo e inmutable para actor, fecha, acción, objeto lógico, resultado y código de motivo. No guardar bytes, tokens, enlaces privados ni documentos en eventos de auditoría.
- **Storage:** almacenamiento privado separado por propósito y cifrado en tránsito/reposo; objeto nuevo en cuarentena, claves opacas, sin URL permanente ni lectura directa desde navegador. La capa Go autoriza cada carga, revisión y descarga por sujeto/recurso/capacidad; solo descarga archivos limpios por respuesta `private`, `no-store`, `nosniff` y `Content-Disposition: attachment`. La decisión concreta bucket/mount queda pendiente de validar en el target autorizado; este diseño no ordena crear uno.
- **Revisión:** capacidades `review_identity` y `review_space` independientes, con prohibición de revisar/aprobar el propio caso. El personal autorizado puede leer solo expedientes dentro de su capacidad; soporte y roles de publicación/arrendamiento no heredan acceso documental.
- **Distintivos:** proyecciones públicas minimizadas, sin enlaces, fechas/documentos precisos ni estados internos. Identidad de cuenta y antecedente de espacio generan distintivos distintos. La aprobación de espacio requiere además identidad de cuenta vigente; la revocación/vencimiento retira el distintivo correspondiente sin afectar publicación por sí misma.

El diseño no fija nombres definitivos de tablas, rutas, bucket, IAM o variables: deben revisarse contra el target local aislado existente y no se crean por este documento.

## 3.1 Matriz de acceso propuesta — 2026-10-29 (no operativa)

Esta matriz traduce la política aprobada en una propuesta verificable de permisos; no asigna roles a personas ni autoriza rutas, persistencia, storage, scanner o infraestructura.

| Actor | Enviar / consultar caso | Leer o descargar archivos | Revisar / decidir | Visibilidad pública |
|---|---|---|---|---|
| Titular — identidad | Solo su propio expediente | Propuesta pendiente: solo sus archivos limpios; definir acceso posterior al cierre | No | Solo su distintivo minimizado |
| Administrador del espacio | Solo expediente del espacio que administra, según autoridad del servidor | Propuesta pendiente: solo archivos limpios del expediente propio; definir acceso posterior al cierre | No | Solo distintivo y alcance de ese espacio |
| Personal `review_identity` | No enviar; leer casos de identidad dentro de alcance asignado, aún por definir | Solo archivos limpios del caso autorizado | Solo identidad: aprobar, rechazar o pedir antecedentes, sin auto-revisión | No privilegio adicional |
| Personal `review_space` | No enviar; leer casos de espacio dentro de alcance asignado, aún por definir | Solo archivos limpios del caso autorizado | Solo espacio: aprobar, rechazar o pedir antecedentes, sin auto-revisión | No privilegio adicional |
| Personal con ambas capacidades | No enviar | Solo archivos limpios de los casos dentro de cada alcance | Ambos propósitos; permitir esta combinación requiere confirmación | No privilegio adicional |
| Soporte / administración general | Ninguno por defecto | Ninguno | Ninguno | Igual que cualquier público; break-glass no definido |
| Escáner | No | Propuesta: acceso técnico mínimo a objetos en cuarentena; ACL exacta pendiente | No | Ninguna |
| Otro usuario público | Ninguno | Ninguno | Ninguno | Solo distintivo minimizado y su alcance, si existe |

**Reglas base aprobadas:** las dos finalidades y sus capacidades de revisión son distintas; revisión manual; no auto-aprobación; autorización en Go; archivos pendientes/bloqueados/con error no son revisables ni descargables; `limpio` no equivale a caso aprobado; aprobar un antecedente de espacio requiere identidad vigente; terceros solo ven el distintivo minimizado, nunca documentos ni estado interno.

**Decisión pendiente sobre conflicto de interés:** la matriz propone que quien administra actualmente un espacio no pueda revisar/revocar el expediente de ese mismo espacio, aunque el sujeto histórico sea otro. La política aprobada prohíbe auto-revisión y exige nueva revisión cuando cambia titular/administrador, pero no especifica este caso límite. El servicio en memoria hoy bloquea solo `SubjectID == actor`; `authorization_parent_test.go` espera permitir una revisión cuando `SpaceOwnerID == actor` pero `SubjectID` es distinto. No cambiar esa semántica hasta que el owner resuelva si el administrador vigente puede recibir `review_space` y decidir casos del propio espacio. El modelo persistente también deberá invalidar/exigir nueva revisión tras cambios de titular/administrador; no basta confiar en el `SubjectID` histórico.

El alcance/cola de asignación de casos a revisores también sigue pendiente.

### Inventario de RBAC y decisiones por aprobar

- `internal/identity/domain.go` define capacidades administrativas generales (`accounts.*`, `publications.*`, `audit.*`, `kpi.read`, `admin_access.manage`). Ninguna capacidad de revisión documental está asignada por `CapabilitiesForRoles` a los roles actuales.
- `internal/documentverification/service.go` define `review_identity` y `review_space` y los valida en su autorización de dominio; esto no demuestra asignación productiva, rutas, ACL persistente ni acceso a bytes.
- Requiere decisión explícita del owner: qué roles/personas pueden recibir cada capacidad, alcance/cola por caso, si se permite que una persona tenga ambas, descarga propia antes/después de decisión y cualquier excepción de soporte/break-glass.
- Sigue sin autorización operativa: identidad técnica del scanner y permisos de cuarentena, rutas, target PostgreSQL, storage privado/ACL, cifrado y gestión de claves. No se crea ni opera ninguno en este documento.

## 4. Flujo de caso y de archivo

El estado del archivo y el estado de la solicitud son ejes distintos. Propuesta de flujo del caso:

```text
borrador → enviada → en_revision → aprobada | rechazada
                              ↘ requiere_antecedentes → en_revision

aprobada → vencida | revocada
borrador/enviada/en_revision/requiere_antecedentes → cierre lógico por política/acción autorizada (sin agregar un estado persistido nuevo sin validar el modelo)
```

Estados técnicos por archivo: `pendiente`, `limpio`, `bloqueado`, `error`. Todo archivo empieza en cuarentena; solo inspección exitosa puede producir `limpio`. Un scanner caído o con error no equivale a limpio; `bloqueado` no se puede revisar ni descargar. Un archivo limpio no aprueba el caso.

La inspección debe validar contenido real, no confiar en MIME/extensión; rechazar bytes corruptos, PDF cifrado, contenido activo y adjuntos incrustados; retirar metadatos innecesarios de imágenes, incluido GPS. Mantener PDF e imágenes fuera de la normalización WebP de galería. No enviar documentos a servicios públicos de análisis; usar motor privado/controlado con firmas actualizadas.

## 5. Vigencia, booking y concurrencia

- La revisión de identidad vence al primero entre el vencimiento del documento y 24 meses desde la revisión; la de espacio al primero entre vencimiento del antecedente y 12 meses. Cambio de titular/administrador exige nueva revisión; cambios relevantes de ubicación/identidad suspenden distintivo de espacio. Cambios ordinarios de precio/disponibilidad/descripción no lo suspenden.
- La decisión manual registra alcance, responsable, instante y vencimiento. Cualquier revocación registrada invalida de inmediato; no se admiten revocaciones futuras programadas.
- El corte actual en `booking.Service.Decide(aprobar)` es un preflight fail-closed, no una garantía transaccional: `Get` → gate → `Repository.Decide` son llamadas separadas. La implementación persistente debe volver a verificar ambas cuentas dentro de la transacción/bloqueo que cambia la reserva, serializando revocación/vencimiento contra aprobación. Nunca declarar booking confirmado usando solo KYC fixture ni una lectura previa susceptible a carrera.
- `cmd/api` aún no tiene repositorio documental cableado; el perfil devuelve `identity_verification_unavailable` al aprobar. Este bloqueo es intencional hasta que existan persistencia autorizada y prueba integrada.

## 6. Retención y eliminación aprobadas

| Clase/estado | Acción y plazo |
|---|---|
| Borrador abandonado | Eliminar tras 24 h; respetar cargas activas |
| Archivo malicioso/bloqueado | Eliminar en un máximo de 24 h |
| Error de inspección | Reintentos limitados; eliminar a 7 días |
| Solicitud con antecedentes requeridos sin respuesta | Cerrar a 30 días; eliminar archivos durante los 30 días siguientes |
| Identidad, decisión final | Eliminar documentos a los 30 días; conservar solo constancia mínima |
| Espacio, aprobado | Conservar durante vigencia; eliminar 30 días después de vencimiento, revocación o sustitución |
| Rechazado/sustituido | Eliminar a 30 días |
| Auditoría mínima | 12 meses desde cierre/fin de vigencia; sin contenido documental |

Toda excepción anticipada requiere responsable, motivo y fecha de revisión. El job de limpieza debe reconciliar DB y todas las generaciones/versiones del objeto, ser idempotente y registrar el resultado mínimo. No prometer borrado instantáneo de backups; describir solo el comportamiento probado del storage autorizado.

## 7. Gates para implementación y evidencia

El código y pruebas unitarias pueden avanzar con archivos sintéticos, pero el recorrido real queda sujeto a F6. Para cerrar F5 se requiere, en target local aislado existente y autorizado: carga y cuota por clase; aislamiento entre cuenta/espacio; scanner fail-closed y casos maliciosos/corruptos/error; revisión manual sintética con anti-autoaprobación; distintivo y retiro por revocación/vencimiento; booking con revalidación atómica de ambas partes; retención/eliminación verificadas incluyendo versiones; y evidencia de auditoría sin PII/contenido.

Pendientes operativos no inferibles: identificación/autorización del target existente, motor/servicio privado de inspección (no se encontró ClamAV local), configuración concreta del mount/namespace privado y semántica efectiva de versiones/backups del storage. No crear DB, container, volumen, bucket, IAM ni servicio para resolverlos por inferencia.

**Resultado:** arquitectura separa identidad, documentos de espacio y galería; conserva la política de publicación sin KYC/documentos; no implementa ni activa flujo documental. F5 continúa parcial.
