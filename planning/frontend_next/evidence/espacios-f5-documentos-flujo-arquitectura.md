# F5 — Diseño propuesto del ciclo documental privado

**Fecha:** 2026-10-28  
**Repositorio:** `/home/nandev/Documentos/projects/espaciGo`, rama de trabajo `codex/backend/feature/local-close-02-espacios-galeria-documentos`; cambios F5 posteriores al commit `c50a2ef` en curso.
**Estado al 2026-10-29:** diseño no operativo complementado con reglas de dominio, grants explícitos actor/capacidad/scope, política de descarga owner-only y prueba real del adaptador ClamAV contra el scanner sintético local. No hay persistencia documental ni wiring en `cmd/api`; por tanto aún no existe flujo operativo de carga/revisión/descarga. La aprobación del owner sí autoriza el trabajo local sintético descrito en `espacios-f5-politica-owner-aprobada.md` y `espacios-f5-clamav-prueba-local-20261029.md`; se mantienen los límites de no tocar DB persistente/cloud ni documentos reales.

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
| Titular — identidad | Solo su propio expediente | Solo sus archivos limpios mientras el caso está abierto; sin descarga al cerrar | No | Solo su distintivo minimizado |
| Administrador del espacio | Solo expediente del espacio que administra según autoridad del servidor | Solo archivos limpios del expediente propio mientras el caso está abierto; transferencia exige caso nuevo | No puede revisar ni revocar ese expediente, incluso si su sujeto histórico es distinto | Solo distintivo y alcance de ese espacio |
| Personal `review_identity` | No enviar; solo casos de identidad con grant explícito del actor en el scope autorizado | Solo archivos limpios del caso autorizado | Solo identidad: aprobar, rechazar o pedir antecedentes; sin auto-revisión | No privilegio adicional |
| Personal `review_space` | No enviar; solo casos del espacio con grant explícito del actor para el `space_id` | Solo archivos limpios del caso autorizado | Solo espacio: aprobar, rechazar o pedir antecedentes; current admin del espacio excluido | No privilegio adicional |
| Personal con ambas capacidades | Denegado hasta aprobación específica; grants duales se rechazan | Ninguno mientras no se apruebe la combinación | Ninguno mientras no se apruebe la combinación | No privilegio adicional |
| Soporte / administración general | Ninguno por defecto | Ninguno | Ninguno | Igual que cualquier público; break-glass no definido |
| Escáner | No | Propuesta: acceso técnico mínimo a objetos en cuarentena; ACL exacta pendiente | No | Ninguna |
| Otro usuario público | Ninguno | Ninguno | Ninguno | Solo distintivo minimizado y su alcance, si existe |

**Reglas base aprobadas:** las dos finalidades y sus capacidades de revisión son distintas; revisión manual; no auto-aprobación; autorización en Go; archivos pendientes/bloqueados/con error no son revisables ni descargables; `limpio` no equivale a caso aprobado; aprobar un antecedente de espacio requiere identidad vigente; terceros solo ven el distintivo minimizado, nunca documentos ni estado interno. Titular actual descarga solo limpio mientras el caso está abierto.

**Conflicto de interés — decisión del owner 2026-10-29:** quien administra actualmente un espacio no puede revisar ni revocar el expediente de ese espacio, incluso si `SubjectID` histórico es otra persona. La transferencia de titular/administrador exige un caso nuevo; el anterior no se hereda. Estas reglas sustituyen la propuesta y la pregunta pendientes de versiones anteriores de esta nota.

**Asignación — decisión del owner 2026-10-29:** asignar explícitamente las capacidades separadas a personas; el modelo local requiere actor autenticado, capacidad y scope exacto. Para este corte, `review_identity` se limita al `SubjectID` y `review_space` al `SpaceID`; cada grant debe indicar el `ReviewerID` que coincide con el actor autenticado y el servicio rechaza dos capacidades en un mismo actor mientras la combinación no esté aprobada. No existe roster ni persistencia de asignaciones, por lo que el servicio en memoria no acredita grants productivos.

### Inventario de RBAC y decisiones por aprobar

- `internal/identity/domain.go` define capacidades administrativas generales (`accounts.*`, `publications.*`, `audit.*`, `kpi.read`, `admin_access.manage`). Ninguna capacidad de revisión documental está asignada por `CapabilitiesForRoles` a los roles actuales.
- `internal/documentverification/service.go` verifica grants de persona/capacidad/scope y deniega dual grants sin aprobación; esto aún no demuestra asignación productiva, rutas montadas ni ACL persistente.
- Continúa sin definirse roster, workflow/cola de asignación y semántica de scopes organizacionales por encima del recurso individual. El corte actual usa solo `SubjectID` para identidad y `SpaceID` para espacio.
- No hay repositorio/storage documental productivo ni identidad técnica de scanner en `cmd/api`; no se inicia ningún runtime documental hasta tener esos componentes y target aislado autorizado.

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

El código y pruebas unitarias pueden avanzar con archivos sintéticos, pero la integración real F5 requiere **un target F5 aislado, específicamente autorizado y distinto de F6**. F6 no es prerrequisito para reutilizarse como base de F5 ni autoriza V79+. Para cerrar F5 se necesita probar: carga/cuotas, aislamiento, scanner fail-closed, revisión manual, distintivos/revocación/vencimiento, booking con revalidación atómica de ambas partes, retención/eliminación con versiones y auditoría mínima sin PII/contenido.

Pendientes operativos no inferibles: repositorio documental/ACL transaccional, asignación persistida de reviewer grants, cableado en `cmd/api`, storage privado F5 dedicado y configurado, política productiva de antigüedad de firmas, permisos del scanner sobre cuarentena, UI/BFF, retención y reconciliación de versiones/backups. ClamAV sintético local ya está disponible y probado; sus recursos existentes, estado de retención y cleanup acotado constan en `espacios-f5-clamav-prueba-local-20261029.md`. No crear DB, volúmenes, buckets, IAM ni servicios adicionales para resolver pendientes por inferencia.

**Resultado al 2026-10-29:** arquitectura separa identidad, documentos de espacio y galería; dominio en memoria y descarga HTTP se probaron compuestos (handler → caso de uso/política → repositorio y store privado, con fakes locales). El adaptador scanner real se ejerció con muestras sintéticas. La ruta no está montada en `cmd/api`; no se activa flujo documental por falta de repositorio/ACL y storage autorizados. F5 continúa parcial. Delta propuesto de tablas, transacciones, API, target de integración y cleanup ClamAV: `espacios-f5-delta-persistencia-integracion-20261029.md`.
