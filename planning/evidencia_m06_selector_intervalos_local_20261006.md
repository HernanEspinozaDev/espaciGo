# Evidencia M06-LOCAL: selector de intervalos disponibles

Issue #166 · PR de la subentrega.

## Resultado

Por fecha local del espacio, el Backend consulta la tarifa vigente para generar candidatos de hora, día o mes, procesa retenciones vencidas mediante el mecanismo existente y quita candidatos solapados con `ocupacion`. La respuesta incluye solo espacio, zona, tarifa y UTC de intervalos libres. La cotización conserva su endpoint actual y vuelve a validar disponibilidad.

No se agregaron migraciones ni tablas: `ocupacion` mantiene el ownership del calendario. El horario de apertura no está modelado; para fixtures sintéticos se generan candidatos durante el día local completo, sujetos a bloqueos/ocupaciones. No se ejecutó `clean`, ni se recreó o vació `espacigo_pgdata`, ni se reemplazaron secretos.

## Verificaciones técnicas

- `go test ./...`: PASS.
- `go vet ./...`: PASS.
- `bash scripts/test-m06-local-booking-postgres.sh`: PASS con base efímera en tmpfs, runtime `espacigo_runtime`; cobertura de hora/día/mes, bloqueos, intervalos adyacentes, retención vencida, autorización, ausencia de escrituras y ocupación posterior a la consulta que provoca conflicto de reserva.
- `npm --prefix mock run build`: PASS.
- `npm --prefix mock run test:profile-races`: PASS, incluidas 3 pruebas del estado del selector para respuestas tardías/cambio de contexto y selección del UTC exacto.
- `python3 -c 'import yaml; yaml.safe_load(open("planning/openapi.yaml"))'`: PASS.
- `git diff --check`: PASS.
- Se habilitaron los fixtures de hora/día/mes para `m05-pagination-host-20261006@ejemplo.invalid` y `m05-pagination-renter-20261006@ejemplo.invalid`. La segunda ejecución de `scripts/enable-local-interval-selector-fixtures.sh` confirmó idempotencia; conservó los datos existentes.
- El stack local está arriba; `espacigo_pgdata` y los dos archivos locales de secreto se conservaron. No se ejecutaron comandos `clean` ni se eliminó información.
- El recorrido de navegador aún está pendiente: el mock abre correctamente, pero la sesión está cerrada y las credenciales existentes no están disponibles en esta conversación. Se completará tras iniciar sesión en la pestaña local; no se registran credenciales en esta evidencia.

## Recorrido local

1. Levantar stack local con `bash scripts/dev-env.sh up` (conserva volumen/secretos existentes).
2. Habilitar fixtures de demostración para las dos cuentas sintéticas ya verificadas (comando ejecutado dos veces):
   ```sh
   bash scripts/enable-local-interval-selector-fixtures.sh \
     --host-email m05-pagination-host-20261006@ejemplo.invalid \
     --renter-email m05-pagination-renter-20261006@ejemplo.invalid
   ```
3. Iniciar sesión como una cuenta participante; en el catálogo, abrir un detalle, elegir uno de los próximos siete días o una fecha hasta 90 días, consultar horarios, seleccionar un intervalo y pulsar **Crear cotización snapshot**.

Los fixtures y bloqueos sintéticos son datos permanentes de la base de desarrollo hasta una limpieza explícita. La consulta es orientativa; cotización y reserva vuelven a confirmar disponibilidad.
