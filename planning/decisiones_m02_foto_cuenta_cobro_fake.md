# M02 local: foto sintética y cuenta de cobro fake

Issue hija: #202, relacionada con #37–#43 y subordinada al cierre de privacidad #185/#40. Este corte amplía M02 para datos sintéticos; no cierra los padres.

## Decisiones y límites

- La foto no admite cargas del cliente. El Backend genera exclusivamente `synthetic-png-v1`, la guarda en el almacenamiento privado ya configurado y entrega contenido solo mediante una ruta autenticada del titular. Reemplazo y retiro marcan limpieza recuperable; el worker elimina archivos fuera de la transacción y reintenta fallos.
- La cuenta de cobro es una referencia `fake-local-v1` (`demo_<UUID>`). No se reciben RUT, cuenta bancaria, credenciales ni secretos de proveedor. No procesa transferencias.
- Alta o cambio requiere elegibilidad KYC sintética efectiva del titular. KYB no sustituye KYC para estas cuentas personales. La preferencia de uso no concede permiso.
- Lectura y escritura son por titular. Mutaciones requieren `Idempotency-Key`; el resultado previo se devuelve en reintentos compatibles y una clave reutilizada para otra acción entra en conflicto.
- Las escrituras toman el bloqueo de cuenta que comparte la baja. La baja revalida actividad y crea jobs de limpieza junto con la minimización; referencias fake se retiran y el historial sintético se conserva según la política ratificada.
- El ZIP propio incluye metadata y, cuando existe, el PNG privado propio y el historial/referencias ficticias propias. No exporta datos de otras cuentas. La baja bloqueada no elimina acceso a exportaciones propias.

## Dependencias

- Satisfechas: almacenamiento privado local, autenticación/ownership, consulta KYC efectiva, worker de limpieza recuperable, ZIP versionado y ejecución de baja con bloqueo de cuenta.
- No necesarias para este prototipo: proveedor de pagos real, RUT, permisos comerciales, imágenes reales, endpoint general de publicación. Esta entrega no desbloquea M04 ni completa M02.
- Pendiente: retención productiva y legal, integración real de pagos, gestión de documentos reales y criterios generales de #37–#43/#185/#40.

## Evidencia de revisión

La implementación, pruebas PostgreSQL desechables, límites y pasos de validación quedan en `planning/evidence/local-m02-photo-payout-20261008.md`. La automatización cubre aislamiento y serialización; el recorrido visual queda a disposición de quien revise el PR y no se declara ejecutado aquí.

## Persistencia

V30 añade filas sintéticas versionadas sin modificar migraciones previas. La tabla de cuenta de cobro conserva una sola activa por titular. Las referencias no son credenciales ni se consumen por un proveedor. Las fotos almacenan solo UUID/hash/tamaño/estado en DB; el PNG permanece fuera de rutas públicas.

Las reglas semanales, `ocupacion`, reservas y datos financieros permanecen fuera del modelo M02 y no se alteran.
