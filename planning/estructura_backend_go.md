# Estructura lógica del backend Go — CORE-BE-01

**Estado:** propuesta documental para revisión. **Trazabilidad primaria:** RNF-033 (ES1 C). **Fuentes de diseño:** propuesta oficial ES2 v1.2, secciones 3.3 y 3.6; CORE-ARCH-01 y mapa de ownership ratificado. Este documento fija límites lógicos para implementar después: no crea código, configuración, API, DDL ni migraciones; tampoco demuestra cumplimiento o despliegue.

## Decisión de organización

Mantener un único monolito modular Go y una unidad de despliegue. M01–M11 son fronteras de negocio y ownership, no servicios ni contenedores separados. Los nombres/directorios siguientes son una guía para el crecimiento incremental; no se deben crear vacíos desde el inicio ni se selecciona aquí un framework HTTP.

```text
cmd/api/                              composición, arranque y cierre del servicio
internal/platform/config/             configuración tipada y validada al inicio
internal/platform/health/             comprobaciones acotadas de vida y readiness
internal/platform/httpserver/         ciclo de vida HTTP, límites y shutdown
internal/platform/observability/      logs, métricas y correlación minimizados
internal/platform/postgres/           pool, unidad de trabajo y runner transaccional
internal/platform/work/               ciclo de vida común de workers
internal/identity/                    M01: cuenta, roles, sesión y credenciales
internal/profile/                     M02: perfil, privacidad y solicitudes de titulares
internal/verification/                M03: KYC/KYB y revisión
internal/listings/                    M04: catálogo, publicaciones, tarifas y políticas
internal/search/                      M05: filtros, lectura de oferta y cotización
internal/booking/                     M06: reserva, ocupación, pago e inbox proveedor
internal/contracts/                   M07: contrato y firma
internal/operations/                  M08: check-in/out y evidencias
internal/communications/              M09: documentos, mensajes, reseñas y notificaciones
internal/finance/                     M10: disputa, garantía, liquidación y movimientos
internal/admin/                       M11: administración y auditoría
internal/adapters/postgres/<owner>/   persistencia concreta organizada por dueño
internal/adapters/storage/            almacenamiento tras un puerto
internal/adapters/providers/          pago, firma, identidad y correo tras puertos
api/openapi.yaml                      contrato HTTP (se define en CORE-API-01)
db/query/<owner>/                     SQL fuente revisable por dueño para sqlc
db/migrations/                        migraciones versionadas (fuera de esta tarjeta)
```

Cada módulo puede usar internamente `domain`, `application`, `ports` y `transport/http` cuando su tamaño lo justifique. Esos nombres no son un framework ni habilitan dependencias cruzadas. Los contratos entre módulos son interfaces/tipos mínimos y explícitos; no se comparten modelos SQL, structs generados por sqlc ni DTOs HTTP.

## Reglas de dependencias verificables

1. **Dominio:** tipos, invariantes, estados y errores propios; no importa HTTP, PostgreSQL, `pgx`, `sqlc`, SDKs cloud/proveedor, configuración global ni otros módulos de infraestructura.
2. **Aplicación:** implementa casos de uso sobre su dominio y puertos. Coordina autorización por recurso y la unidad transaccional; no ejecuta SQL ni importa adaptadores concretos.
3. **Transporte HTTP:** traduce contrato JSON a comandos/consultas de aplicación y resultados a respuestas. No contiene reglas de negocio ni accede a PostgreSQL. Rutas, esquemas y errores externos se fijan en CORE-API-01.
4. **Persistencia:** adaptadores implementan puertos y encapsulan `pgx`/`sqlc`. Cada carpeta `db/query/<owner>` y adaptador SQL solo lee/escribe datos del dueño lógico correspondiente. Un módulo no consulta ni modifica tablas ajenas directamente.
5. **Colaboración:** el consumidor invoca el contrato tipado del dueño. Las lecturas transversales requieren una operación explícita del dueño o una proyección aprobada; no se resuelven con SQL arbitrario.
6. **Grafo:** las dependencias van desde el punto de entrada hacia aplicación/dominio y desde aplicación hacia puertos; adaptadores implementan esos puertos en el borde. No hay ciclos entre módulos ni imports de módulos de negocio desde `platform`.
7. **Composición:** `cmd/api` es el único composition root: construye configuración, pool, adaptadores, casos de uso, handlers y workers, y los conecta por interfaces. Las dependencias de proveedores/cloud quedan detrás de puertos.

Cuando exista código, las reglas se pueden revisar con el grafo de imports (`go list -deps` y chequeo de ciclos) y comprobación de ownership de las consultas `db/query/<owner>`. Una lectura de datos ajenos debe tener contrato documentado. Esta tarjeta no ejecuta esos chequeos porque todavía no existe código de aplicación.

## Persistencia y límite de transacción

Se propone `pgx/v5` con `pgxpool` y `sqlc` para SQL estático, tipado y revisable; cualquier consulta dinámica debe usar parámetros enlazados y allowlist de filtros/orden. Versiones y configuración concreta se fijan y verifican en la tarjeta de implementación que incorpore cada dependencia; esta propuesta no afirma que estén instaladas ni elige un framework HTTP.

La aplicación del módulo dueño delimita la transacción local mediante un puerto de unidad de trabajo/runner. La transacción concreta y `pgx.Tx` permanecen dentro de plataforma/adaptador; no se filtran al dominio ni a otro módulo. Por ejemplo, M06 consume la cotización vigente mediante el contrato tipado de M05 y, en su propia transacción PostgreSQL, registra únicamente sus hechos de reserva/ocupación/transición y el outbox que corresponda. No lee ni escribe directamente tablas de M04/M05. Si se necesita coordinar persistencia adicional, se explicita mediante contratos; no se inventa una transacción distribuida.

La llamada a un proveedor ocurre fuera de la transacción PostgreSQL. Si el commit local ya ocurrió, un fallo o timeout externo se registra/conciliará como otro hecho idempotente; no se modela como rollback remoto. Un evento outbox necesario para el cambio local se escribe en el mismo commit y se publica después con entrega recuperable, no exactamente una vez.

## Configuración, salud y operación

- Cargar y validar configuración tipada al arrancar el proceso; pasarla explícitamente a constructores, sin leer variables globales desde dominio/casos de uso.
- Referenciar secretos desde el gestor del entorno de ejecución; nunca guardarlos en código, muestras versionadas, mensajes de error o logs.
- Separar liveness (el proceso puede continuar) de readiness (dependencias imprescindibles, con timeout acotado). Las respuestas de salud no revelan DSN, valores de configuración, consultas ni secretos. Ruta/protocolo se acuerdan en CORE-API-01.
- Emitir logs estructurados con correlación y datos técnicos mínimos. Omitir credenciales, tokens, documentos, payloads completos y contenido sensible.
- Iniciar y detener workers junto con el proceso; el trabajo pendiente, intentos, disponibilidad y lease deben ser durables en PostgreSQL según el contrato de persistencia aprobado. Reclamar transaccionalmente (por ejemplo, con lease vencible y `SKIP LOCKED`), hacer el efecto idempotente, guardar resultado/checkpoint y permitir recuperación por otra instancia.
- En shutdown, dejar de reclamar trabajo, terminar o liberar de forma segura los leases y cerrar servidor/pool. Memoria, goroutines y timers no son fuente de durabilidad ni garantizan exactamente-una-vez.

El esquema físico de trabajo/outbox, retenciones, plazos y consultas quedan en sus tarjetas DB. La guía no prescribe columnas ni sustituye la revisión de cada dueño.

## Hallazgos de M01 que permanecen abiertos

CORE-DB-02 cerró la revisión y dejó DB02-09 abierto en `revision_diccionario_datos.md`. Esta estructura no resuelve ni debe ocultar esas brechas:

- **RQF-213 — preferencia de uso:** una preferencia de onboarding no equivale a un rol de autorización; no se añade campo ni semántica por inferencia.
- **RQF-217 — historial de claves:** el requisito de evitar reutilización en tres meses no define persistencia, minimización ni retención del historial; no se inventa estructura ni regla adicional.
- **RQF-218 — notificación:** la capacidad genérica M09/M11 no define el evento ni el contrato específico del cambio M01; no se presume resuelto por la existencia de tablas comunes.

Antes de cualquier DDL o implementación que dependa de estos puntos, AUTH-ARCH-01 debe registrar la decisión necesaria y el trabajo debe quedar trazado en tickets con los RQF/CU/HU afectados. Mientras tanto se conservan como hallazgos abiertos y no bloquean esta tarjeta puramente arquitectónica.

## Comprobación de aceptación de CORE-BE-01

- El mapa cubre los once módulos, API/composición, plataforma, adaptadores y consultas SQL agrupadas por owner.
- Las reglas indican explícitamente dependencias permitidas/prohibidas y cómo revisar aciclicidad y encapsulación SQL.
- El ejemplo M05→M06 conserva el ownership y separa contrato de lectura de la transacción de escritura.
- El límite transaccional distingue commit PostgreSQL de cualquier efecto de proveedor.
- Configuración, salud y workers durables se describen sin escoger framework ni crear código/DDL.
- Validación documental de esta entrega: inspección de las reglas de importación, ownership, ejemplo M05/M06, red externa y recuperación durable contra CORE-ARCH-01 y ES2 §3.3/3.6. `git diff --check` se ejecuta antes de publicar. No se repiten pruebas de entorno ni se afirman pruebas de aplicación.
