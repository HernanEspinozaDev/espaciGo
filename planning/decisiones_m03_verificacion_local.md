# M03 — contrato de verificación local

## Decisión implementable en este corte

KYC y KYB se representan como casos separados del titular, con estado `en_revision`, referencia de proveedor fija `local-fixture-v1`, referencia opaca `fixture:<uuid>`, clave idempotente por titular, revisor/código de motivo y marcas de creación/resolución. Las transiciones finales las ejecuta únicamente el rol `administrador`; un rechazo exige motivo codificado. Un reintento requiere que el titular sea dueño del caso rechazado, confirme corrección fixture y use nueva clave idempotente. No se borra ni sobrescribe el caso anterior.

El fake no declara una identidad acreditada ni habilita publicación/reserva. No se reciben RUT, número de serie, fotos, selfie, documentos, referencias de almacenamiento externo ni texto libre de identidad. PostgreSQL conserva solo la referencia sintética de caso. No se emite correo de resultado en este corte.

### Excepción local temporal autorizada — 2026-10-05

Para el prototipo del Issue #47, se permite cargar **exclusivamente** el fixture `synthetic-png-v1`: la API lo genera, calcula SHA-256 y persiste sus metadatos en una tabla incremental. El archivo vive en un directorio privado local fuera del repositorio y fuera de rutas estáticas; el mock no acepta un selector ni bytes del usuario. La API permite consultar el caso/evidencia al titular y leer/eliminarla al rol administrador de revisión ya existente. El titular también puede consultar y limpiar explícitamente sus fixtures. La limpieza borra el archivo y sus metadatos puntuales; no elimina el volumen PostgreSQL ni el directorio completo.

Esta excepción no permite RUT, documentos, fotos, selfie, imágenes del usuario ni datos de personas reales. No selecciona proveedor productivo y no resuelve MIME/tamaño/antivirus para documentos reales, consentimiento productivo, retención, expiración, borrado legal ni acceso de terceros. Esos criterios se separan en la Issue vinculada #142 y permanecen abiertos.

## Capacidades confirmadas vs pendientes

| Capacidad | Estado en prototipo |
|---|---|
| Casos KYC y KYB, consulta propia, propiedad y reintento de fixture | Implementado en entorno local |
| Cola y resolución manual con autorización de rol y motivo obligatorio para rechazo | Implementado localmente; el alta de rol administrador requiere acción explícita del desarrollador local |
| Consentimiento afirmativo versionado, límite de solicitudes y control de abuso | Pendiente; el submit de la fixture no se presenta como consentimiento legal y el cuerpo HTTP está acotado, pero no hay cuota M03 |
| Revisión de identidad/RUT, vigencia cédula, extracción OCR, inicio de actividades | Pendiente; no se consulta Registro Civil ni SII |
| PNG de fixture fijo generado por la API, fuera del repo/web pública, con acceso por titular/revisor y limpieza puntual | Prototipo local sintético implementado bajo autorización temporal #47 |
| Archivos/documentos reales, MIME/tamaño/antivirus productivos, proveedor y referencias durables | Pendiente en #142; los archivos reales no se aceptan |
| Aviso por correo de resultado | Pendiente de contrato de notificaciones durable y tratamiento del fallo |
| Retención y expiración de evidencia KYC/KYB real | Pendiente de decisión legal/operativa en #142; no se inventa plazo |
| Habilitar publicación/reserva por KYC/KYB | Fuera de este corte; el fixture no concede derechos de producto |

Trazabilidad: Issue #44–51, HU04, CU-10–14, RQF-038–059, RQF-192–194 y RQF-219–220; Anexo B `verificacion`. El diseño no afirma cumplimiento legal ni integración productiva.

## Addendum ratificado — 2026-10-08, LOCAL-KYC-01

La decisión anterior de mantener la aprobación completamente fuera de elegibilidad se modifica **solo para el prototipo local sintético**: una aprobación concede elegibilidad persistente por tipo; no concede roles ni se calcula usando únicamente el caso más reciente. Casos nuevos pendientes/rechazados no revocan la concesión. Solo una acción administrativa explícita, estructurada y auditada la revoca; una nueva aprobación explícita puede restaurar el tipo. Reservas existentes e historiales no se alteran.

La consulta propia, revocación, historial e integración con reserva son parte del corte ampliado #200. El usuario ratificó KYC sintético vigente para anfitrión y arrendatario en nuevas reservas de cuentas personales; KYB no sustituye KYC y no se infiere de `use_preference`. Baja y reserva comparten bloqueos de cuenta. M04 aún no ofrece endpoint de publicación: su gate se implementará con ese ciclo, sin introducirlo en #200. Exportación y retención siguen la política local de verificación. #142 y los criterios generales de #45/#48–#50 continúan abiertos.
