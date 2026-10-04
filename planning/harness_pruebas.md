# Harness automatizado de DB y API

## Estado y alcance

Este documento define el contrato del harness. CORE-TEST-01 se completó como definición documental mediante el PR #8 fusionado el 2026-10-02; ese PR no entregó un runner DB/API ni ejecutó integración PostgreSQL. La rama local CORE-ENV-02 contiene un bootstrap Go y pruebas de paquete, pero no una suite de DB integration con fixtures/cleanup ni jobs CI separados. Los comandos y checks marcados como aceptación siguen siendo gates para una futura implementación del harness; no se sustituyen con SQLite ni con resultados simulados.

## Capas y comandos

La implementación deberá conservar estos comandos estables desde la raíz del repositorio:

| Capa | Comando | Contenido / dependencias |
| --- | --- | --- |
| Unit | `go test ./...` | Dominio, validación y políticas puras; sin DB, red ni proveedor externo. Fakes deterministas. |
| DB integration | `go test -tags=integration ./...` | Repositorios, SQL, constraints, transacciones y migraciones contra PostgreSQL real. Requiere `TEST_DATABASE_URL`; jamás SQLite. |
| API contract | `go test -tags=contract ./...` | Servidor HTTP con `httptest`, OpenAPI versionado, status/body/headers, validación y autorización. Sin DB cuando el contrato se puede probar con dependencias fake; casos end-to-end se marcan además como integración. |
| Migración desde vacío | `go test -tags=integration ./internal/...` (paquete de migraciones) | Crear DB/scope vacío, aplicar todas las migraciones en orden, verificar extensiones/revisión del schema y luego comprobar una segunda ejecución idempotente. Ajustar el paquete cuando se decida la estructura del código, sin cambiar el gate. |

El runner CI debe ejecutar los tres primeros gates separadamente, publicar el resultado y terminar con código distinto de cero ante cualquier fallo. No ocultar errores con `|| true`, filtros que descarten fallos ni `continue-on-error`. El job de DB debe provisionar PostgreSQL con las extensiones objetivo PostgreSQL 18, PostGIS y `btree_gist`, y registrar versión/extensiones sin imprimir URL/contraseña. Fijar imagen/digest solo después de verificar disponibilidad en el entorno objetivo (CORE-ENV-01).

## Fixtures y aislamiento

- Datos completamente sintéticos, mínimos y deterministas. Usar IDs/seeds fijos por test; emails bajo dominio reservado `.test`; nombres y direcciones ficticios; nunca copiar dumps, tokens, credenciales, webhook payloads reales ni PII.
- Cada test crea sus fixtures y limpia lo que crea, incluso si falla. Preferir rollback de transacción cuando todo el caso comparte conexión; para pruebas que necesitan commit/concurrencia usar base/schema aislado por proceso o suite y eliminarlo en `defer`/cleanup del runner.
- Nunca limpiar una DB arbitraria: exigir `TEST_DATABASE_URL`, validar que el nombre de DB esté inequívocamente marcado para test y abortar antes de conectar si falta la variable, apunta a producción/DB compartida, o no satisface la guarda. No aceptar `DATABASE_URL` como sustituto.
- Para paralelismo, cada worker obtiene un scope único; no ejecutar `TRUNCATE` global compartido. Probar explícitamente que la limpieza elimina filas/objetos del scope del test y preserva otro scope centinela.
- Congelar reloj/azar en unitarias. En integración controlar orden y concurrencia con sincronización explícita, no `sleep` ni dependencia del orden incidental.
- Adaptadores externos fake por defecto. Sandbox solo en suite separada, con credenciales de CI protegidas y opt-in explícito; nunca necesario para pasar gates ordinarios.

## Migraciones y semántica PostgreSQL

La suite parte de una base vacía y utiliza el mismo ejecutor de migraciones previsto para la aplicación. Verifica orden/checksum según el contrato de migraciones, extensiones requeridas, constraints, índices y segundo arranque sin cambios. Añadir tests de regresión PostgreSQL para rangos/exclusiones, solapamiento y adyacencia; SQLSTATE pertinente, FK, unicidad, rollback y carreras. No modelar estas propiedades con SQLite ni asumir equivalencia de dialectos. Las migraciones aplicadas no se reescriben para arreglar tests: se agrega forward-fix.

## Contrato HTTP

El OpenAPI versionado es la referencia para request/response. Cada operación probada cubre al menos respuesta válida y errores documentados; donde corresponda 400/401/403/404/409/422/5xx, headers, formato de error común, request_id, paginación e idempotencia. Verificar que respuestas reales se ajusten al schema y que ejemplos sean sintéticos. Una ruta protegida prueba identidad ausente y acceso a recurso ajeno; no basta validar el código HTTP. No inventar rutas de negocio desde este harness: cada módulo incorpora las suyas a su OpenAPI y casos.

## Evidencia y diagnóstico

Por ejecución CI conservar: commit, versión Go y PostgreSQL/extensiones, comando/capa, seed o identificador de suite (no datos personales), resumen de tests y código de salida; adjuntar reporte JUnit cuando el runner lo soporte y logs redactados. Nunca registrar DSN, secrets, tokens, cuerpos privados completos ni fixtures con datos identificables. Reportes son artefactos de CI, no evidencia de aprobación de cumplimiento, SLA o rendimiento. Un fallo debe ser visible como job fallido y conservar diagnóstico saneado.

## Pruebas de aceptación del harness

Cuando exista el runner, demostrar en CI/local reproducible:

1. Unit, DB integration y API contract son jobs/comandos distinguibles; un fallo deliberado y aislado en cada capa hace que su comando/job falle visiblemente.
2. Smoke DB conecta a PostgreSQL real, verifica versión/extensiones esperadas, aplica esquema desde vacío y consulta el schema; sin credenciales o DB no disponible, falla claramente.
3. El test de limpieza crea fixture, ejecuta cleanup aun ante error inducido y confirma que no quedan sus filas/scope y que el centinela de otro scope permanece.
4. Suite de contrato valida OpenAPI y respuestas HTTP con `httptest`; un schema incompatible produce fallo.
5. Toda la suite funciona sin PII/secrets ni llamadas a proveedores externos.

Estado: CORE-TEST-01 quedó `done` por su alcance documental en PR #8. Las pruebas de paquete del bootstrap (`go test ./...`) y los smoke HTTP/red de CORE-ENV-02 no equivalen al harness de integración descrito aquí. Siguen pendientes la DB PostgreSQL desechable para la suite general, fixtures/cleanup de integración, validación de contrato OpenAPI automatizada como capa separada y jobs CI por capa; implementarlos requiere sus tarjetas y gates correspondientes.
