# Prototipo local M02 — perfil y solicitudes de privacidad

## Levantar y recorrer

Para levantar requiere Docker Compose. Para ejecutar toda la suite localmente se requieren Go, Node/npm y Python 3. Desde la raíz del repositorio:

```sh
scripts/dev-env.sh up -d
```

El comando conserva `pgdata` y secretos locales; el servicio `migrate` solo aplica migraciones pendientes. Abre <http://127.0.0.1:8081> y registra una cuenta con correo sintético `@ejemplo.invalid`; abre <http://127.0.0.1:8025> para copiar Token ID y Token del mensaje, verifica y entra.

En el mock, usa **Perfil y privacidad (M02)** para consultar/guardar nombre y teléfono opcional de nueve dígitos. La foto se genera como PNG sintético fijo en Backend; no se cargan archivos. La cuenta de cobro fake requiere elegibilidad KYC sintética y solo produce una referencia de ensayo, nunca una transferencia. Registra solicitud de acceso o supresión y consúltala; la de supresión permanece `en_revision` hasta su revisión administrativa. Ejecuta pruebas automáticas solo con `TEST_DATABASE_URL` apuntando a PostgreSQL desechable. No uses `scripts/dev-env.sh clean`.

## Límite del corte

La subentrega #202 añade exclusivamente foto sintética y cuenta de cobro fake usando el storage/KYC ya existentes; no modela una cuenta bancaria, RUT ni proveedor real. El archivo se elimina con trabajo recuperable al retirar/reemplazar o ejecutar una baja. La supresión completa y el tratamiento productivo de documentos, proveedores y pagos siguen fuera de este slice. La política local ratificada se documenta en `decisiones_m02_foto_cuenta_cobro_fake.md`; no afirma cumplimiento legal ni retención productiva. El inventario de respaldos y la limpieza del servidor #123 siguen aparte.
