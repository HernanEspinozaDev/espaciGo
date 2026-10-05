# Prototipo local M03 — casos KYC/KYB sintéticos

## Levantar y probar el flujo

Requiere Docker Compose. Desde la raíz del repositorio:

```sh
scripts/dev-env.sh up -d
```

El servicio de migración agrega V3/V4 al PostgreSQL persistente sin reiniciarlo. Abre <http://127.0.0.1:8081>, registra y verifica una cuenta sintética desde Mailpit (`http://127.0.0.1:8025`) e inicia sesión. En **Verificación M03** envía una solicitud KYC o KYB y consulta sus casos: el resultado queda en revisión con referencia fixture. No ingreses RUT ni adjuntes documentos reales.

Para probar autorización administrativa, crea e inicia sesión con una segunda cuenta sintética. En otra terminal, asigna explícitamente el rol local usando el correo de esa cuenta:

```sh
scripts/dev-grant-kyc-admin.sh admin@ejemplo.invalid
```

Vuelve a iniciar sesión para cargar el rol actualizado. La cuenta administradora puede ver la cola, aprobar la fixture o rechazarla con un motivo. Para probar reintento, vuelve a la cuenta titular, toma el ID de su caso rechazado, confirma que corrigió los antecedentes fixture y reintenta. La API no publica identidad verificada ni habilita reservas. El rol agregado queda en la base persistente hasta que el desarrollador local lo revoque explícitamente.

## Alcance y límites

El adaptador `local-fixture-v1` no llama Registro Civil/SII; no se persiste RUT, fotos, selfie ni documento. La verificación real, tratamiento de archivos privados, correo de resultado, retención, KYC de cobro y autorización de operaciones comerciales siguen pendientes. M02 también mantiene abiertos los pendientes registrados en #37–43. DB02-09 y la limpieza del servidor #123 permanecen independientes.

Este mock HTML/CSS/TypeScript compilado consume exclusivamente HTTP/JSON. Usa una clave `Idempotency-Key` nueva por operación, guarda la sesión solo en memoria y se sirve en su contenedor separado.
