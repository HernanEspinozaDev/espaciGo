# LOCAL-PRIV-01A4 — Exportación completa local de datos propios

Issue hija #196 de #185 y relacionada con #40. Extiende la exportación limitada de identidad aceptada en #187. No termina la exportación integral del producto, #185/#40 ni la supresión.

## Contrato

`GET /api/v1/privacy/export/archive` requiere una sesión vigente y responde `application/zip` con `Content-Disposition: attachment`. Incluye `manifest.json`, `data.json` (`schema_version: 1`) y archivos sintéticos propios que sigan disponibles. La exportación es solo lectura y permanece accesible cuando una solicitud de supresión está bloqueada.

`data.json` mantiene el bloque de identidad existente y añade proyecciones de perfil y derechos, verificaciones, evidencias, borradores/características, tarifas/simulaciones, cotizaciones, reservas/historial, pagos fake/eventos aplicados, devoluciones, disputas/historial, mensajes escritos por el titular y cursores propios. Las proyecciones de reservas y disputas convierten actores a `self`, `counterparty` o `system`; omiten IDs personales de terceros. Los mensajes solo incluyen aquellos cuyo autor es el titular. Las consultas de cada módulo tienen un predicado de ownership explícito.

El manifiesto declara secciones, archivos incluidos y exclusiones. Un archivo sintético propio ausente se omite y se indica como exclusión; no se inventa ni se vuelve a generar durante la descarga. El JSON no incluye contraseñas/hashes, sesiones, tokens, secretos, claves de idempotencia, payloads del fake/proveedor, contenido administrativo restringido, direcciones de espacios ajenos ni mensajes escritos por terceros.

La lectura se compone mediante proyecciones de módulos en una solicitud autenticada; cada proyección es de solo lectura. No hay transacción global entre dominios, por lo que `exported_at` indica el momento de armado y no promete una instantánea serializable global entre módulos. La exportación no cambia estados, cursores, historial, auditoría ni solicitudes de derechos.

## Dependencias trazables

Implementaciones fusionadas reutilizadas: identidad y exportación inicial (#187); evidencia sintética privada (#143); borradores y atributos versionados (#134/#141); tarifas/simulaciones (#147); reservas/cotizaciones/pagos fake y devoluciones (#149/#172/#174); mensajes y cursores (#157/#159); disputas sintéticas (#190); baja local y retención/reaplicación (#193/#195). Estas dependencias están satisfechas para los datos existentes en el prototipo. Los padres #185/#40 siguen abiertos y la exportación de dominios no implementados permanece fuera.

## Recorrido y comprobaciones

1. Iniciar dos cuentas sintéticas en el mock, preparar sus exportaciones y descargar el ZIP de cada titular.
2. Abrir `manifest.json` y `data.json`; verificar que cada ZIP contiene únicamente los archivos sintéticos vinculados al titular y las proyecciones admitidas.
3. Comprobar exclusión de email/IDs personales de la contraparte, credenciales, tokens, secretos, mensajes ajenos y payloads internos.
4. Evaluar una solicitud de supresión bloqueada y repetir la descarga: debe seguir disponible.
5. Comparar conteos/estados relevantes antes y después para confirmar que la exportación no escribe.

La integración PostgreSQL debe ejecutarse en la instancia desechable del repositorio con rol `espacigo_runtime`. La base persistente `espacigo_pgdata`, secretos y datos no se usan para preparar fixtures de prueba.

## Pendientes

No se exportan entidades que aún no estén modeladas, datos administrativos, documentos reales ni respaldos. Esta entrega no determina retenciones nuevas, no purga datos ni afirma anonimización/supresión integral. La revisión de contenido y completitud corresponde a la aceptación de #196; #185 y #40 conservan sus criterios generales.
