# Prototipo local M02 — perfil y solicitudes de privacidad

## Levantar y recorrer

Para levantar requiere Docker Compose. Para ejecutar toda la suite localmente se requieren Go, Node/npm y Python 3. Desde la raíz del repositorio:

```sh
scripts/dev-env.sh up -d
```

El comando conserva `pgdata` y secretos locales; el servicio `migrate` solo aplica migraciones pendientes. Abre <http://127.0.0.1:8081> y registra una cuenta con correo sintético `@ejemplo.invalid`; abre <http://127.0.0.1:8025> para copiar Token ID y Token del mensaje, verifica y entra.

En el mock, usa **Perfil y privacidad (M02)** para consultar/guardar nombre y teléfono opcional de nueve dígitos. Registra solicitud de acceso o supresión y consúltala; la de supresión permanece `en_revision` y no elimina datos. Ejecuta pruebas automáticas solo con `TEST_DATABASE_URL` apuntando a PostgreSQL desechable. No uses `scripts/dev-env.sh clean`.

## Límite del corte

Foto, cuenta de cobro con RUT verificado y su integración dependen de definir storage/proveedor y de KYC (M03). La decisión final de supresión depende de comprobar reservas, pagos, disputas, copias/derivados y reglas de retención; no se automatiza. Historial de claves y avisos durables siguen pendientes de DB02-09. La limpieza del servidor #123 sigue aparte.
