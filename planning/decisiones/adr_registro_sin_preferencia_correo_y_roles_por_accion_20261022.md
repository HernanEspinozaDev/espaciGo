# ADR: Registro sin preferencia, correo verificable y roles por primera acción

Estado: propuesto para revisión en el corte de onboarding/IMG-01  
Fecha: 2026-10-22

## Decisión

- El registro no pregunta si la persona arrendará u ofrecerá, no persiste esa selección y no concede roles de negocio. Se conservan las preferencias y membresías históricas.
- La primera creación correcta de borrador concede `arrendador`; la primera solicitud de reserva correcta concede `arrendatario`. Cada concesión ocurre en la misma transacción que la acción y escribe un evento único de auditoría. Las membresías se acumulan; no se concede administración.
- El perfil local sigue usando Mailpit. `cloud-test` usa SMTP autenticado con TLS validado y credencial desde un archivo montado; si falta configuración, el API permanece disponible y el envío falla de forma visible, sin recurrir al sumidero Mailpit.
- El enlace de verificación se sirve desde el origen público de EspaciGo. El token va en el fragmento URL, se copia al formulario y se quita del historial del navegador antes de que el usuario lo envíe. El backend conserva token de un solo uso y vencimiento existentes.
- SMTP aceptado significa aceptación del servidor para entrega, no recepción comprobada en la bandeja.

## Hallazgo de nube confirmado y límite de migración

El operador informó errores de PostgreSQL para `GET /api/v1/public/spaces` en el proyecto `espacigo-testprod`, instancia `espacigo-testprod-db`, base `espacigo_dms_stage_20261020`, revisión API `00012`: SQLSTATE `42P01` (`public.space_media` no existe) y `42883` (`public.claim_private_media_cleanup(unknown, unknown)` no existe). El código espera objetos de V68 y V73. Esto confirma ausencia de objetos usados por la aplicación; por sí solo no demuestra el estado del ledger ni autoriza a volver a ejecutar una migración.

No se ejecuta SQL, migración, despliegue ni cambio de Cloud desde este corte. Antes de preparar un paso de migración, el operador debe aportar por canal aprobado el inventario secuencial del ledger con nombre/checksum, comprobarlo contra V1–V73 y confirmar que no hay checksum drift. Si V68 o V73 aparecen aplicadas pero sus objetos faltan, detenerse: no editar el ledger ni repetir esas migraciones; preparar recuperación/corrección revisada.

El runner `cmd/dbmigrate` usa un advisory lock de sesión, exige versiones contiguas y compara nombre/checksum antes de ejecutar archivos pendientes. El Cloud Build actual valida proyecto/instancia/base contra sustituciones permitidas, pero no ejecuta el runner ni verifica el ledger, backup/restauración o privilegios runtime antes de desplegar. El backup automático más reciente informado es de 2026-10-22 04:43Z y PITR está deshabilitado; no se considera evidencia de restauración previa a un cambio. No se debe convertir el despliegue regular en un aplicador automático. Cualquier migración futura requiere un paquete operacional serial separado, destino exacto permitido, ledger/checksum observado, backup restaurable y concesión mínima revisada.

## Configuración SMTP pendiente para release

Remitente y destinatario autorizados para un ensayo: `contacto@hotusoft.com`, nombre visible `EspaciGo (pruebas)`, origen público `https://espacigo.testingpage.store`. No modificar DNS, MX, SPF, DKIM, otros buzones ni dominios.

Faltan verificar en la configuración del buzón el host Zoho exacto (no inferirlo del MX), puerto `587` con STARTTLS o `465` con TLS implícito, y SMTP AUTH. El dueño debe crear una app password si Zoho la exige y pasarla directamente al operador por canal seguro; nunca en chat, repo o logs. El operador debe crear un secreto independiente de Secret Manager, montarlo como archivo de solo lectura en `/var/secrets/auth-smtp/password`, dar acceso `secretAccessor` solo a la cuenta runtime API y configurar estas variables no secretas: `AUTH_SMTP_HOST`, `AUTH_SMTP_PORT`, `AUTH_SMTP_SECURITY`, `AUTH_SMTP_USERNAME=contacto@hotusoft.com`, `AUTH_SMTP_FROM=contacto@hotusoft.com`, `AUTH_SMTP_PASSWORD_FILE=/var/secrets/auth-smtp/password`. La creación del secreto y su IAM son una dependencia externa a este PR; no se añade Terraform ni se crea secreto en el corte. Si se habilitan las sustituciones, Cloud Build solo consulta metadatos de la versión y su política IAM para comprobar que está habilitada y que la SA API tiene `secretAccessor`; no lee el valor. Las sustituciones guardadas en el repo quedan vacías. Mientras falte, el API responde a registro pero entrega 202 pendiente y muestra advertencia; el correo no se envía. No se usa el correo de producción ni se prueba una contraseña desde este workspace.

## Verificación de extremo a extremo pendiente

Se necesita autorización separada de la política privada de ensayo para crear una cuenta sintética de testprod con `contacto@hotusoft.com`; la política actual es local-only. La cuenta real de la captura y las credenciales de ensayo local quedan excluidas. Una vez permitida la cuenta y configurado SMTP, verificar registro → aceptación SMTP → mensaje recibido → verificación → acceso → primer borrador/primera reserva y roles acumulables. La respuesta HTTP 202 de registro significa cuenta creada pendiente de verificación y envío fallido; la interfaz debe mostrar advertencia y ofrecer reenvío, nunca éxito verde.
