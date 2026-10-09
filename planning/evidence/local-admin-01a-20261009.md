# LOCAL-ADMIN-01A — evidencia del corte

Fecha: 2026-10-09. Issue operativa: [#226](https://github.com/HernanEspinozaDev/espaciGo/issues/226), en curso dentro de Project 1. Rama: `codex/local-admin-01a`.

## Alcance y dependencias

Entrega acotada de consulta administrativa M11 (#111–#118), con API read-only para listar y consultar reservas locales y sus hechos financieros/históricos mínimos. Dependencias reutilizadas e integradas: LOCAL-CORE-01/#180 (sesión, rol y auditoría), LOCAL-BOOK-02/#214 (reservas y snapshots), LOCAL-DIS-01/#222 (reclamo y resolución) y LOCAL-FIN-01/#224 (garantía/deducción fake). Project 1 conserva estas cuatro dependencias como Hecho y #226 como En curso. #185/#40 se relacionan solo por minimización y consulta de históricos retenidos; no se consideran dependencia técnica de este slice. #111–#118, #185/#40 y LOCAL-CORE-02 mantienen su estado/criterios generales; este corte no los cierra ni desbloquea.

No se agregó migración: V40 y las tablas/contratos existentes bastan para proyectar el listado, pagos fake, devolución, garantía, decisión y reclamo. Las lecturas no procesan vencimientos, no concilian, no llaman adaptadores ni cambian reservas, importes u ocupaciones. Una petición exitosa escribe únicamente un evento mínimo de auditoría para la consulta, no uno por fila. Campos de participantes retirados se proyectan `null`; no se reconstruyen. Mensajes, documentos, contactos, credenciales, tokens, evidencia privada y payloads no forman parte de la respuesta.

## Verificación ejecutada

- `go test ./internal/booking/... ./internal/adapters/postgres/booking/... ./cmd/api` — pasó.
- `GO_TEST_RUN=TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/booking` — pasó en PostgreSQL desechable (2.31 s). Incluye filtros por UUID/estado/fecha, dos páginas con cursor estable, rechazo de cursor ligado a otros filtros, detalle con historial/pago/garantía/reclamo, participantes minimizados, rechazo de cuenta sin rol, auditoría y comparación del estado/importe antes/después para confirmar que la consulta no muta negocio. El script limpia su instancia efímera; no usa el volumen de desarrollo.
- `go vet ./...` — pasó.
- `cd mock && npm run build && node --test test/admin-reservations-state.test.mjs` — pasó, 2/2. Cubre descarte de respuesta al perder sesión y volver a entrar con la misma cuenta, y controles de paginación según el resultado vigente.
- `python3` + PyYAML sobre `planning/openapi.yaml` — parseo correcto y presencia de las dos rutas admin.
- `git diff --check` — pasó.
- `bash scripts/dev-env.sh verify-http` — pasó: backend liveness/readiness, CORS, mock HTML/CSS/JS y API accesible desde el origen del mock.

No se ejecutó la suite global ni una migración nueva. El stack local existente se conservó (volumen `espacigo_pgdata` y secretos); la integración PostgreSQL usó un contenedor/base desechable. No hubo trabajo GCP ni proveedor real.

## Recorrido visual

Se recargó `http://127.0.0.1:8081/` y se confirmó que el mock sirve la nueva sección “Consulta administrativa de reservas y finanzas (solo lectura)”, inicialmente cerrada, con búsqueda, filtros, paginación y detalle. La implementación usa el rol de sesión para habilitar el acceso, texto DOM seguro y limpieza al logout/cambio de cuenta; la prueba unitaria cubre respuesta tardía de una sesión anterior.

La consulta autenticada desde el navegador con una cuenta admin sintética queda como comprobación manual para la revisión del PR. El contrato admin, proyección PostgreSQL, permisos y datos financieros fueron ejercitados por la prueba de integración desechable con principal administrador y rol no administrador; no se afirma que se haya recorrido esa UI con credenciales persistentes.

## Pasos de revisión manual

1. Levantar/conservar el entorno existente con `bash scripts/dev-env.sh up -d backend mock-frontend`; no ejecutar `clean` ni `down --volumes`.
2. Iniciar sesión en el mock (`http://127.0.0.1:8081/`) con una cuenta sintética que ya tenga rol administrador; abrir **Consulta administrativa de reservas y finanzas (solo lectura)**.
3. Consultar sin filtros; probar `reservation_id`, `state`, rango de creación, páginas y nueva consulta. Abrir una reserva con datos fake y revisar historial, pago/devolución, garantía/deducción y reclamo.
4. Confirmar que una cuenta sin rol recibe 403 en la API (cubierto por integración) y que al cambiar/perder sesión desaparecen listado, detalle y cursor.

## Límites

Este PR implementa solo la subentrega 01A. No resuelve gobierno de cuentas, consultas/reportes M11 restantes, outbox/continuidad completa, retención productiva ni los criterios amplios de #185/#40. No desbloquea LOCAL-CORE-02 ni declara M11 o LOCAL-ADMIN-01 completos. Las observaciones sobre operaciones inciertas son proyecciones de estado; no las concilian.
