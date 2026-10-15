# GADM-04 — inventario y entrega local de cuenta maestra

Estado: documentación de entrega y regresión; no constituye activación ni aceptación de producto. Código de referencia: `cmd/gadm-admin/main.go`, `internal/adminglobal/`, `internal/identity/domain.go` y `internal/adminglobal/transport/http/handler.go`.

## Inventario verificado de capacidad

El rol `administrador_global` tiene `legacy.administration`, `accounts.list/read/update/block/roles`, `publications.list/read/update/moderate/retire`, `admin_access.manage`, `audit.read/export` y `kpi.read`. Go calcula estas capacidades desde roles actuales en cada petición; una sesión anterior no conserva permisos revocados. El rol `administrador` solo recibe `legacy.administration`, no capacidades globales. Registro y edición de perfil no conceden el rol global.

| Capacidad disponible | Superficie y efecto | Límite/auditoría y recuperación |
| --- | --- | --- |
| Sesión, directorio, detalle y recursos de cuenta | `/api/v1/admin/global/me`, `/overview`, `/accounts`, `/accounts/{id}`; búsqueda/paginación y ficha minimizada | Requiere global activo y fuera de modo restringido. Los bloqueos/revocaciones invalidan peticiones siguientes; errores 401/403/404 mantienen los límites de acceso. |
| Administración de cuentas | `/accounts/{id}/profile`, `/roles/{role}`, `/block`, `/sessions/revoke`, `/resources/{kind}[/{resourceID}]` | Allowlist de perfil y roles ordinarios; bloquear/desbloquear revoca acceso y sesiones. Mutaciones registran actor, destino y motivo; versionado/conflictos responden sin sobrescribir cambios. No hay contraseña visible, elevación a global desde ficha ni borrado físico que omita obligaciones. La protección del último global se serializa. |
| Inventario y administración de publicaciones | `/publications`, `/publications/{id}/state`, `/content`, `/retire`, `/schedule`, `/gallery`, `/gallery/{photoID}/visibility`, `/cover`, `/remove` | Conserva dueño, reservas, contratos, tarifas y snapshots históricos; valida atributos/horarios y obligaciones. Las fotos usan las capacidades existentes, no storage real nuevo. |
| KPI | `/api/v1/admin/global/overview` y `/api/v1/admin/global/kpis` | Totales consultados desde DB, ventanas delimitadas y CLP exacto. El alcance disponible no instrumenta visitas ni DAU; cifras de pagos/liquidaciones son de ensayo. |
| Auditoría y exportación | `/api/v1/admin/global/audit-events` y `/audit-events/export` (JSON/CSV) | Paginación/filtros acotados; actor y objetivo pueden diferir. Lecturas y exportaciones quedan auditadas; export no cambia permisos ni publica archivos. CSV protege celdas tipo fórmula. |
| Superficie heredada | Rutas `/api/v1/admin/...` existentes protegidas por `legacy.administration` | Global conserva compatibilidad de administrador para colas y operaciones antiguas; administrador previo conserva únicamente esa superficie. No supone capacidad global sobre toda ruta o tabla. |

### Brechas de decisión

`operations.read/manage` y `finance.read/operate` figuran en el plan de producto, pero no son capacidades del catálogo Go ni permisos otorgados por GADM-01–03. Las operaciones heredadas de reservas/finanzas continúan disponibles por sus reglas antiguas de administrador; no existe una API global nueva que represente esos dos permisos. Antes de añadirlas, el propietario debe decidir qué comandos de dominio entran, si el maestro los puede ejecutar o solo leer, y qué autenticación reciente/motivo requieren. No se amplía su alcance en este corte.

La cuenta del propietario y el destino local definitivo no están identificados en el material autorizado. No se promovió una cuenta, no se cambió `espacigo_pgdata` y no se habilitó un bootstrap genérico para bases de aplicación. Siguiente paso exacto: proporcionar UUID/correo de la cuenta EspaciGo ya verificada y el nombre de la base/entorno local elegido; revisar identidad/estado del destino y diseñar el comando explícito con confirmación de ese objetivo antes de activarlo. No se debe inferir el usuario desde las cuentas GitHub ni transferir silenciosamente el bootstrap único.

## Procedimiento disponible para ensayos

El comando actual **solo** acepta `GADM_BOOTSTRAP_ENV=local-test`, una URL loopback y DB llamada `espacigo_gadm01_<8–12 hex>`. Lee `.local/testing/gadm01.env` con modo `0600`; verifica que `current_database()` coincida con `EXPECTED_DATABASE`; exige `bootstrap --account-id UUID` o `recover --account-id UUID`. Bootstrap solo enlaza un UUID activo y elegible si aún no existe titular; recuperación solo restaura ese mismo UUID. Cada intento queda en `administrador_global_bootstrap_evento`, con actor `cli_local`, acción, resultado, motivo y correlación. No conectar este CLI a una base persistente ni usarlo para la cuenta real: no soporta ese entorno.

Para preparar el archivo de ensayo, copiar la URL de la DB efímera asignada por el runner y declarar `EXPECTED_DATABASE` con su nombre exacto; establecer modo `0600`, ejecutar desde el repositorio y retirar el archivo cuando el runner limpie el proyecto. Nunca añadir el archivo a Git, logs, capturas o artefactos compartidos. Los E2E y la integración de bootstrap siembran únicamente identidades sintéticas propias en una DB/volumen Compose desechable.

## Regresión obligatoria

`internal/adminglobal::TestGlobalBootstrapDirectoryAndLastMasterConcurrency` forma parte de `scripts/local-required-integrations.txt`; el validador exige que se ejecute y pase, y rechaza omisiones o skips. `scripts/test-local-qa-backend.sh` combina suite Go sin servicios, `go vet` y paquetes de integración con Postgres/PostGIS y Mailpit desechables. El test de migración GADM-03 comprueba DB vacía y upgrades desde V56 y V58, preservando los checksums de migraciones ya aplicadas. GADM-04 no cambia archivos `db/migrations`; V1–V59 permanecen intactas.

Para el E2E GADM-02/03 servido, este selector hace reproducible la selección y falla si no encuentra los títulos: `FE14_TEST_GREP='GADM-02|GADM-03|Go-backed role navigation, private JSON/ZIP exports, and responsive evidence|Anonymous visitor explores the Go-backed public catalog and minimized detail'`. Ejecutar con `FE14_EVIDENCE_OUTPUT_DIR=planning/administracion_global/evidencia/gadm_04 bash scripts/test-frontend-e2e.sh`. El runner crea cuentas sintéticas, verifica resultados seleccionados y afirmaciones PostGIS y retira recursos propios; las capturas y sidecars de medidas se guardan en esa carpeta. `LOCAL-CLOSE-02` necesita ejecución separada con `FE14_PUBLIC_ONLY=1 FE14_LOCAL_CLOSE02_CLOCK=1`; una invocación normal no conduce el reloj de negocio y no verifica ese cierre financiero. No afirmar FE-14 completo desde un grep parcial.

Las capturas válidas deben registrar SHA, viewport 390×844 y 1365×900, dimensiones reales, ruta, foco/teclado y recuperación error→reintento. Una descarga del navegador solo se declara escrita en disco si se inspeccionó el archivo descargado y se registraron sus dimensiones/hash cuando corresponda. El volumen de demo LOCAL-CLOSE-02 retenido y `espacigo_pgdata` quedan fuera del ensayo GADM-04.
