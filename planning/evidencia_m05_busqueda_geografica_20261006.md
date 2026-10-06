# Evidencia — M05-LOCAL búsqueda geográfica

Fecha: 2026-10-06. Issue #162. PR de implementación pendiente de publicación.

## Implementado

- V018 añade una tabla incremental ligada únicamente a fixtures habilitados, coordenadas explícitamente sintéticas, `geography(Point,4326)`, índice GiST y límites de latitud/longitud. No modifica V1–V17 ni consulta direcciones.
- El bootstrap deja `SELECT` como único permiso de `espacigo_runtime` sobre las ubicaciones. La migración y el comando administrativo de habilitación asignan muestras por categoría.
- El catálogo autenticado combina proximidad con categoría, disponibilidad, atributos versionados y precio. Distancia/radio se calculan en PostGIS en metros; las respuestas exponen solo `distance_km` a un decimal y `distance_kind: direct`, nunca coordenadas, punto ni dirección.
- Las muestras de los fixtures persistentes de este entorno quedaron: `sala_multiproposito:1` en el centro y `bodega:1` a aproximadamente 2,5 km. En el mock, el centro sintético y radio 1 km ilustran un positivo y una exclusión; 5 km incluye ambos. El resultado explica que es distancia directa y no ruta por carretera.
- La consulta no crea cotizaciones, reservas u ocupaciones; el flujo de vencimientos que ya aplica al catálogo permanece intacto.

## Validación ejecutada

Comprobaciones completadas en esta rama:

```text
bash scripts/test-m06-local-booking-postgres.sh  PASS
go test ./...                                   PASS
go vet ./...                                    PASS
npm --prefix mock run test:profile-races         PASS (13 pruebas)
npm --prefix mock run build                      PASS
python3 -c 'import yaml; yaml.safe_load(open("planning/openapi.yaml"))'  PASS
git diff --check                                 PASS
bash scripts/dev-env.sh up                       PASS; migración V018 incremental
```

La prueba PostgreSQL corre sobre su instancia desechable, con rol `espacigo_runtime`; cubre los radios 1/3/5/10/25 km, inclusión exacta de frontera, punto a 1.001 km excluido del radio 1 km, filtros AND, orden por distancia sin redondear, igualdad estable, respuestas sin ubicación privada, fixture sin coordenadas, aislamiento de tercero, grants de solo lectura y ausencia de escrituras en cotizaciones/reservas/ocupaciones.

La cuenta de la base local reportó una ubicación sintética para sala y una para bodega. `docker volume inspect` confirmó el volumen `espacigo_pgdata` presente. `scripts/dev-env.sh up` reconstruyó y levantó los servicios sanos sin limpiar el volumen ni regenerar/borrar secretos.

## Smoke del mock y pasos para revisión

Tras aplicar la migración, la página `http://127.0.0.1:8081/` presentó los controles nuevos: activar cercanía, latitud/longitud, botón **Cargar centro sintético de ejemplo**, radios 1/3/5/10/25 con 5 km por defecto y aviso de distancia directa. El estado del navegador se perdió al recargar el mock para recoger los assets nuevos; por eso no se atribuye aquí una búsqueda autenticada interactiva.

Para repetir el recorrido con una de las cuentas ya autorizadas en el fixture local:

1. Inicia sesión en `http://127.0.0.1:8081/` con una de las dos cuentas sintéticas allowlisted.
2. En **Catálogo sintético y reserva local**, elige **Todas las categorías**, pulsa **Cargar centro sintético de ejemplo** y deja el radio en **5 km**. Buscar muestra los fixtures autorizados y una distancia directa aproximada.
3. Cambia el radio a **1 km** y vuelve a buscar: la muestra próxima permanece y la bodega a unos 2,5 km queda fuera. En 5 km vuelve a aparecer.
4. Para filtros combinados, fija categoría, atributos o intervalo/precio y confirma que los resultados cumplen todos; la búsqueda no cotiza ni retiene horarios.
5. Para entrada incompleta o inválida, usa la API autenticada, por ejemplo `GET /api/v1/local/booking-trial/catalog?latitude=-33.456`; la respuesta esperada es `422`. La suite HTTP automatizada cubre coordenadas no finitas/fuera de rango y radios no permitidos.

No se ejecutó `clean`, `down --volumes` ni limpieza de mensajes, fixtures, cuentas o datos sintéticos.
