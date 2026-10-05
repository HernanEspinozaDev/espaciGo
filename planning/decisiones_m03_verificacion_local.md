# M03 — contrato de verificación local

## Decisión implementable en este corte

KYC y KYB se representan como casos separados del titular, con estado `en_revision`, referencia de proveedor fija `local-fixture-v1`, referencia opaca `fixture:<uuid>`, clave idempotente por titular, revisor/código de motivo y marcas de creación/resolución. Las transiciones finales las ejecuta únicamente el rol `administrador`; un rechazo exige motivo codificado. Un reintento requiere que el titular sea dueño del caso rechazado, confirme corrección fixture y use nueva clave idempotente. No se borra ni sobrescribe el caso anterior.

El fake no declara una identidad acreditada ni habilita publicación/reserva. No se reciben RUT, número de serie, fotos, selfie, documentos, referencias de almacenamiento externo ni texto libre de identidad. PostgreSQL conserva solo la referencia sintética de caso. No se emite correo de resultado en este corte.

## Capacidades confirmadas vs pendientes

| Capacidad | Estado en prototipo |
|---|---|
| Casos KYC y KYB, consulta propia, propiedad y reintento de fixture | Implementado en entorno local |
| Cola y resolución manual con autorización de rol y motivo obligatorio para rechazo | Implementado localmente; el alta de rol administrador requiere acción explícita del desarrollador local |
| Consentimiento afirmativo versionado, límite de solicitudes y control de abuso | Pendiente; el submit de la fixture no se presenta como consentimiento legal y el cuerpo HTTP está acotado, pero no hay cuota M03 |
| Revisión de identidad/RUT, vigencia cédula, extracción OCR, inicio de actividades | Pendiente; no se consulta Registro Civil ni SII |
| Recepción, validación MIME/tamaño, almacenamiento privado, referencias de documento, selfie/biometría | Pendiente de decisión y servicio privado; los archivos reales no se aceptan |
| Aviso por correo de resultado | Pendiente de contrato de notificaciones durable y tratamiento del fallo |
| Retención y expiración de evidencia KYC/KYB | Pendiente de decisión legal/operativa; no se inventa plazo |
| Habilitar publicación/reserva por KYC/KYB | Fuera de este corte; el fixture no concede derechos de producto |

Trazabilidad: Issue #44–51, HU04, CU-10–14, RQF-038–059, RQF-192–194 y RQF-219–220; Anexo B `verificacion`. El diseño no afirma cumplimiento legal ni integración productiva.
