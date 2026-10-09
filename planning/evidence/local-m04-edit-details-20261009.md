# LOCAL-M04-EDIT-02 — edición de detalles propios

Issue: [#208](https://github.com/HernanEspinozaDev/espaciGo/issues/208), subissue de #55. Depende de slices aceptados #204/#206.
Estado: implementación en curso en `codex/local-m04-edit-details`; revisión pendiente.

## Alcance

- Backend y API admiten cambios parciales de descripción, capacidad y reglas de uso en espacios propios `activa` u `oculta`, además de los campos ya aceptados en #206.
- Validaciones reutilizan los límites de borrador: descripción con al menos 100 caracteres, capacidad positiva, reglas no vacías de hasta 250 caracteres.
- La operación mantiene bloqueo de cuenta y fila de espacio, preserva estado y tarifa, y no modifica snapshots de cotizaciones/reservas.
- OpenAPI declara los nuevos campos del body. El mock carga valores actuales y solo envía las propiedades cambiadas.
- No se requiere migración; V5/V31 ya contienen las columnas y estados.

## Comprobaciones ejecutadas

- `go test ./internal/spaces/... ./internal/adapters/postgres/spaces ./internal/spaces/transport/http` — PASS.
- `npm --prefix mock run build` — PASS.
- `node --test mock/test/published-content-state.test.mjs` — PASS (2 pruebas de campos parciales, no-op, límites de capacidad y precisión CLP).
- `go vet ./internal/spaces/... ./internal/adapters/postgres/spaces ./internal/spaces/transport/http` — PASS.
- `git diff --check` — PASS.
- `GO_TEST_RUN='^TestLocalPublicationRequiresEffectiveKYCAndRecordsOwnerTransitions$' bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/spaces` — PASS con PostgreSQL/PostGIS desechable. El mismo recorrido verifica edición de tarifa desde 8.000 a 12.000, envío posterior solo de título desde un formulario antiguo sin revertir la tarifa, edición de descripción/capacidad/reglas y conservación de snapshots anteriores. También confirma que el fixture habilitado mantiene su conflicto y que el recorrido fixture sigue disponible tras ese rechazo. No usó el volumen `espacigo_pgdata`, secretos o datos locales persistentes.

## Decisiones y pendientes

Se deja fuera cambiar modalidad tarifaria porque interactúa con el selector/calendario y el comportamiento disponible ya vigente. Galería/archivos también queda aparte: este corte no amplía la autorización sintética previa de evidencia a imágenes de espacios. No hay cambio de requisitos generales ni GCP. #208 se aceptará solo por estos criterios; #55 y #52–#61 permanecen abiertos.
