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
- Se habilitó una segunda pareja sintética mediante registro y verificación Mailpit. El helper se repitió para ambas parejas. La consulta de control mostró los 14 fixtures que ya pertenecían a la primera pareja (incluidos 3 fixtures M06 con sus 3 bloqueos M06) y 3 fixtures M06 con 3 bloqueos propios para la pareja nueva; no se duplicaron bloqueos. El helper identifica el bloqueo por `espacio_id`, no por UUID compartido.
- Prueba con reloj inyectado en `America/Santiago`, `2026-09-05 23:30` local: el 6 de septiembre se conserva como fecha calendario aunque `time.Date` normalice su medianoche al día anterior. El primer candidato es 01:00; día de 1/2/3 fechas produce 23/47/71 horas transcurridas respectivamente; el mes inicia el 6-sep a la primera hora válida y termina el aniversario del 6-oct. Todas las duraciones son positivas.
- El stack local está arriba; `espacigo_pgdata` y los dos archivos locales de secreto se conservaron. No se ejecutaron comandos `clean` ni se eliminó información.
- Recorrido completado en el mock con la pareja sintética nueva: inicio de sesión, búsqueda, detalle de fixture horario, selección de mañana, consulta de intervalos, selección de un intervalo de 1 hora y creación de cotización snapshot. La cotización devolvió el mismo intervalo UTC seleccionado para el fixture y su tarifa/huso. No se solicitó reserva ni se creó ocupación.

## Recorrido local

1. Levantar stack local con `bash scripts/dev-env.sh up` (conserva volumen/secretos existentes).
2. Habilitar fixtures de demostración para dos cuentas sintéticas verificadas:
   ```sh
   bash scripts/enable-local-interval-selector-fixtures.sh \
     --host-email m06-interval-host-20261006@ejemplo.invalid \
     --renter-email m06-interval-renter-20261006@ejemplo.invalid
   ```
3. Iniciar sesión como una cuenta participante; en el catálogo, abrir un detalle, elegir uno de los próximos siete días o una fecha hasta 90 días, consultar horarios, seleccionar un intervalo y pulsar **Crear cotización snapshot**.

Los fixtures y bloqueos sintéticos son datos permanentes de la base de desarrollo hasta una limpieza explícita. La consulta es orientativa; cotización y reserva vuelven a confirmar disponibilidad.
