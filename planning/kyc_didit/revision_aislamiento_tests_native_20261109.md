# Hallazgo de aislamiento de tests nativos — 2026-11-09

## Detección y evidencia

Durante el readback de N08-B, el outbox Didit global tenía50filas, frente a46 observadas en un corte anterior; el harness N08-B pasó su comparación de baseline antes/después. No se borraron registros para hacer coincidir totales.

Inspección source-only del adaptador identificó un acceso implícito a PostgreSQL en dos tests:

- `internal/adapters/postgres/identitydeclaration/rut_claim_test.go`: `resolveRutClaimDSN` buscaba un passfile privado hardcoded al faltar la variable DSN; construía una conexión con credencial descubierta. `TestRutClaimPostgresClaimLifecycle` usaba UUIDs fijos y `SET ROLE` sin verificar el error, y borraba claims preexistentes de esos IDs. No se autoriza repetir ese harness sin revisar el scope/fixtures/roles/journal, aunque se agregue opt-in.
- `session_deletion_queue_test.go`: `resolveRuntimeDSN` buscaba passfiles automáticamente y hacía fallback al resolver anterior. `TestSessionDeletionQueuePostgresLifecycle` creaba dos sesiones sintéticas UUID nuevas, las reconocía como ACK y las retenía en outbox al terminar; carecía de journal/cleanup y podía ejecutarse en la suite por defecto.

Consulta READ ONLY parental agregada, sin IDs/keys/PII impresos: outbox Didit total50; patrón sintético exacto de ese test (`didit-sess-unit-a-*`/`didit-sess-unit-b-*`)50; entregadas50; pendientes0. Esto identifica el patrón de origen, no una autorización de DELETE por prefijo. Ningún worker ni llamada de borrado remoto fue ejecutado para vaciarlo.

Se verificó además solo presencia de variables opt-in/DSN relevantes en el entorno del padre: ninguna. La brecha no era un opt-in humano activo, sino descubrimiento automático de credenciales. No se leyeron ni imprimieron valores de passfiles durante este diagnóstico; la inspección fue de fuente/catálogo agregado.

## Impacto sobre evidencia previa

Las afirmaciones anteriores de que la suite default Go no hacía mutaciones nativas quedan **corregidas para estos dos tests**: sus resolvers permitían acceder a la base sin opt-in y el ciclo outbox retenía filas sintéticas. Los PASS de paquetes no acreditan que todo fuera unitario/DB-free ni que se limpiara globalmente la base.

No se anulan las aserciones realmente ejecutadas por N07/N08-B sobre su propia declaración journaled: sus preflights y cleanups independientes pasaron y conservaron el baseline del outbox. Tampoco se atribuye un check de login/runtime a un test que usó owner o credenciales autoelegidas. El viejo lifecycle de claims requiere nueva revisión antes de habilitarlo; se mantienen por separado N03/N06 y los recorridos HTTP nativos que verifican explícitamente login y rol.

Las50filas ACK se conservan como artefactos sintéticos detectados. No hay evidencia de un journal previo que permita retirarlas automáticamente; no borrar por patrón ni ampliar el cleanup de N07 para adoptarlas. Una eventual retirada necesitará inventario privado por ID, validación/backup y un procedimiento revisado, sin procesar colas ajenas. Retención de estos registros no es éxito de limpieza global.

## Contención y siguiente acción

- Suspender suites amplias de este adaptador hasta verificar la corrección de sus resolvers. No volver a ejecutar los native lifecycle antiguos por inferencia.
- Delegación acotada `deleg_6d819ad1` sobre ambos archivos de test y regresión sintética nueva: exigir `ESPACIGO_NATIVE_ADAPTER_TESTS=1` **y** DSN explícita específica de cada test; sin búsqueda de archivos, sin extracción de contraseñas ni fallback entre tests. Tests de config con valores sintéticos y RED→GREEN, sin DB.
- Esa nueva flag es una barrera técnica, no aprobación operacional de fixtures/cleanup del harness antiguo. En la suite default ambos native lifecycle deben aparecer **SKIP**, no PASS nativo.
- Padre revisa fuente antes de correr la suite posterior; conservar snapshot del conteo50 y confirmar que no crezca por defecto. Datos/identidades actuales no se sustituyen.

Implementación de la contención en curso al registrar esta evidencia. No se declara resuelta ni se elimina cobertura histórica por anticipado.

## Cierre de contención — verificación parental posterior

- La entrega retiró ambos scanners/búsquedas de passfile y el fallback entre tipos de test. Los resolvers consultan únicamente configuración explícita: enable exacto `ESPACIGO_NATIVE_ADAPTER_TESTS=1` y su variable DSN específica; por defecto retornan vacío y el lifecycle se omite.
- El ejecutor agregó inicialmente un helper en `native_optin.go` fuera del scope test. Padre lo trasladó a `native_optin_test.go` y retiró solo ese archivo nuevo del ejecutor: la contención queda íntegramente en tests, sin configuración de producción.
- El RED inicial reportado por el ejecutor fue compilación por helper ausente, no una regresión conductual aceptable. Padre verificó sensibilidad con overlay Go en scratch que elimina solo el guard de enable, manteniendo los resolvers ya libres de lectura de secretos: `TestNativeResolversRequireExplicitOptIn` falló por retornar la DSN sintética sin opt-in. Se ejecutó únicamente ese test de configuración, sin DB ni retorno a los scanners inseguros.
- GREEN parental sobre fuente definitiva, con las tres variables de opt-in/DSN retiradas del entorno del runner: `go test -race -count=1 -json ./internal/verification/... ./internal/adapters/postgres/identitydeclaration/... ./cmd/api ./cmd/kyc-fake-provider` PASS. Parseo de eventos JSON, excluyendo subtests: **161tests top-level PASS,10SKIP,0FAIL**, seis paquetes PASS. `TestRutClaimPostgresClaimLifecycle` y `TestSessionDeletionQueuePostgresLifecycle` aparecen explícitamente SKIP, no PASS nativo.
- `go vet` de esos paquetes y `git diff --check` PASS. Búsqueda fuente confirma ausencia de `pgpass`/`os.Open`/scanner/QueryEscape en los resolvers corregidos. No se descubrió ni imprimió material privado.
- Readback independiente READ ONLY después de la suite: outbox50, ACK50, pendientes0, **sin crecimiento**. Preflight autorizado posterior PASS. No se eliminó ninguna de las50filas por patrón.
- Regresión frontend posterior: TypeScript PASS, ESLint0errores/4advertencias anteriores, formato de los archivos del corte PASS, tooling484casos:483PASS/1SKIP/0FAIL. Sin rebuild necesario para estos cambios de test; el build de producto N07 no cambió.

**Contención del acceso implícito resuelta.** Se pueden ejecutar las suites default con estos guards y reportar los skips. Los lifecycle nativos viejos siguen operativamente detenidos: fixtures/identidad/limpieza/journal deben revisarse antes de un opt-in explícito futuro. Las50filas históricas se conservan; no se declara limpieza global ni aceptación retroactiva de esas ejecuciones.
