# Evidencia M05-LOCAL-02 — búsqueda por características y precio

Fecha: 2026-10-06. Issue de entrega: #160. PR: pendiente de publicación.

## Alcance comprobado

La búsqueda del catálogo acepta categoría, intervalo y rango inclusivo del total CLP estimado, además de filtros AND generados desde el perfil de categoría seleccionado. Cada resultado con intervalo muestra total estimado, tarifa/unidad y zona horaria. La estimación usa el mismo cálculo de unidades y zona horaria que la cotización. Buscar no crea cotizaciones ni ocupaciones. Crear una cotización vuelve a verificar disponibilidad y tarifa vigente; crear la reserva repite las validaciones transaccionales.

Los filtros respetan el perfil/versionado guardado del fixture. Un atributo ausente no satisface el filtro y `false` se conserva como un valor JSON presente. La consulta queda limitada a fixtures habilitados explícitamente.

## Pruebas ejecutadas

- `go test ./...` — pasó.
- `go vet ./...` — pasó.
- `npm --prefix mock run test:profile-races` — pasó, 13 pruebas.
- `bash scripts/test-m06-local-booking-postgres.sh` — pasó en PostgreSQL desechable con el rol de runtime. Incluye combinación AND de categoría/disponibilidad/atributos/precio, límites inclusivos, booleano falso frente a campo ausente, versión de perfil, ausencia de escrituras en cotizaciones/ocupaciones y comparación del subtotal estimado contra la cotización para el mismo intervalo. También verifica conflicto al cambiar la tarifa antes de reservar.
- `git diff --check` — pasó.
- `planning/openapi.yaml` parseado con PyYAML — pasó.

## Comprobación del mock local

Se reconstruyó y levantó el entorno existente mediante `scripts/dev-env.sh up`. El backend y el mock quedaron saludables y las migraciones incrementales no introdujeron una migración nueva. En el mock, autenticado con la cuenta sintética participante, se cargaron los controles del perfil de Bodega; un filtro booleano `false` devolvió cero resultados para snapshots actuales `{}` (atributo ausente). Al quitarlo y buscar Bodega para 2030-06-01 10:00–11:00 America/Santiago con mínimo y máximo inclusivos de 8.000 CLP, el mock mostró un resultado con “Tarifa 8000 CLP/hora”, zona horaria y “Estimación total 8.000 CLP”. Al quitar filtros se mostraron ambos fixtures habilitados. La búsqueda solo hizo consultas GET. No se crearon cotizaciones, reservas u ocupaciones ni se alteraron fixtures.

Los fixtures persistentes actuales tienen perfiles de atributos vacíos, por lo que la comprobación positiva de filtros tipados y la estimación con datos que declaran características se ejecutó contra la base PostgreSQL desechable del script de integración. Allí el resultado y la cotización produjeron el mismo subtotal.

## Persistencia local

Se conservaron `espacigo_pgdata` y los secretos locales. No se ejecutó `clean`, no se eliminaron volúmenes, secretos, mensajes, cursores ni fixtures. El ensayo PostgreSQL usó recursos desechables y se limpió al terminar su script. Esta evidencia solo cubre M05-LOCAL-02; Issues generales permanecen abiertas.
