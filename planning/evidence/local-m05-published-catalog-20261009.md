# LOCAL-M05-DISC-01 — descubrimiento de publicaciones activas

Issue #212, hija de #65. Alcance de prototipo local: integrar publicaciones M04 activas y con elegibilidad KYC sintética efectiva al catálogo/detalle M05 ya existente. Los fixtures de ensayo expresamente habilitados conservan su autorización por pareja. Esto no completa M04/M05 ni cierra #52–68.

## Implementación

- `Repository.Catalog` une espacios y fixtures sin duplicar filas. Incluye drafts únicamente cuando el fixture está habilitado para una de sus dos cuentas; incluye ofertas activas cuando el titular mantiene KYC efectivo; deja fuera drafts privados, espacios ocultos, activos sin KYC y espacios propios del actor.
- Detalle e intervalos disponibles usan las mismas reglas de visibilidad. No se devuelven dirección, coordenadas, propietario ni galería privada.
- La cotización captura categoría/perfil, tarifa, zona y condiciones del espacio seleccionado. La solicitud mantiene los locks de cuentas/espacio y vuelve a validar KYC de ambas partes, publicación activa, tarifa, horario y ocupación; cambios posteriores producen conflicto.
- La galería no se publica. Precio, filtros, PostGIS, cursor firmado y reservas son los contratos existentes; no se añade esquema ni se modifica migración.
- El mock ya usa estos endpoints. Se actualizó su aviso para distinguir ofertas activas elegibles de fixtures explícitos, sin presentar KYC como rol ni borradores como públicos.

## Evidencia enfocada

Comando ejecutado en base PostgreSQL desechable, aislada del volumen de desarrollo:

```sh
GO_TEST_RUN='TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry' \
  bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/booking
```

Resultado: PASS. El escenario agrega una publicación activa con KYC vigente, una publicación oculta y una activa sin KYC; además conserva una fila de fixture deshabilitada sobre el espacio publicado para comprobar que no lo oculta ni genera duplicados. Comprueba búsqueda/detalle y ausencia de datos privados, cotización/reserva del espacio seleccionado, aislamiento del propietario, y que ocultos/no elegibles no generen cotizaciones, reservas ni ocupaciones.
El mismo escenario pasa por los handlers HTTP existentes para listar, abrir detalle seguro y cotizar el ID seleccionado; el mock consume esas rutas y no mantiene una implementación de búsqueda paralela.

Comprobaciones adicionales:

```sh
go test ./internal/booking/...
go test ./internal/spaces/...
npm --prefix mock run build
git diff --check
```

Las cuatro terminaron con código 0. No se repitieron suites globales ni comprobaciones ambientales. El recorrido existente de fixture, paginación, filtros, geografía y selector no se modificó; solo se ejecutó la integración de booking que contiene la nueva aceptación junto con el ciclo central.

Tras reconstruir los servicios locales sin cambiar migraciones, `scripts/dev-env.sh verify-http` confirmó readiness del Backend, CORS y assets compilados del mock.

Para revisión manual en el mock: ejecutar `scripts/dev-env.sh up`; iniciar sesión con una cuenta sintética distinta del titular; en “Catálogo sintético y reserva local”, filtrar Oficina y buscar; seleccionar una publicación activa; usar un intervalo futuro UTC mostrado en la zona del espacio y preparar la cotización. El detalle y el precio deben corresponder al mismo `space_id` seleccionado. Los borradores no habilitados no aparecen.

## Límites

La búsqueda solo autoriza oferta local activa con KYC sintético; no define publicación comercial, comisiones, garantías, proveedores ni rendimiento cloud. Cotizar no garantiza que la oferta continúe vigente: reservar vuelve a comprobarla transaccionalmente. No se necesitó una migración. #55/#56/#62–68 siguen abiertas. No se hizo trabajo de GCP.
