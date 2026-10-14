# Reglas de desarrollo y revisión de EspaciGo

Estas instrucciones se aplican al monorepo. Se complementan con `frontend/AGENTS.md`, las decisiones arquitectónicas y el plan vigente. Las instrucciones expresas del usuario prevalecen. No presentar decisiones/documentos propuestos como capacidades implementadas.

## Base y ramas

- `main` es la única rama permanente y la base de todos los cortes. No crear `develop`, ramas permanentes por área ni ramas personales.
- Antes de trabajar: comprobar cambios locales, `git fetch origin --prune` y actualizar `main` mediante fast-forward. Si tiene cambios ajenos, preservarlos y usar un worktree; no resetearlos ni mezclarlos en el PR.
- Una rama temporal por tarea/corte y un PR coherente hacia `main`. Reutilizar la rama del PR abierto para corregir su revisión; no crear una rama por cada comentario.
- Nombres: `codex/<área>/<tipo>/<identificador>-<descripción>`.
- Áreas: `marketplace`, `backend`, `frontend`, `database`, `infra`, `docs`. Tipos: `feature`, `fix`, `chore`. Descripción breve en minúsculas y guiones.
- Para V2 usar identificadores `mpv2-00`–`mpv2-13`. Un corte transversal usa `marketplace`; no crear tres ramas permanentes para DB, Go y Angular.
- Ejemplo: `codex/marketplace/feature/mpv2-01-public-catalog`.
- No crear ramas sin trabajo autorizado; no reabrir ramas de PR fusionados. Al fusionar, eliminar rama remota y local y archivar/retirar su worktree una vez conservado cualquier trabajo ajeno.
- No hacer push directo o force-push a `main`. Fusionar mediante PR con checks y revisión. No borrar ramas activas/PR abiertos sin autorización explícita. La limpieza histórica del 2026-10-13 fue una petición puntual, no una autorización permanente para eliminar todo.

## Identidades obligatorias

| Operación | Cuenta |
| --- | --- |
| Desarrollo, commits, push y creación/actualización del PR | `HernanMEC` |
| Revisión, aprobación, solicitud de cambios y fusión | `HernanEspinozaDev` |
| Administración/limpieza solicitada por el usuario | `HernanEspinozaDev` |

- Configurar mediante `scripts/git-role.sh development` o `scripts/git-role.sh review` en este repositorio. Verificar la identidad inmediatamente antes de publicar o revisar; la cuenta de un conector puede diferir de `gh` y de las credenciales de Git.
- Para publicar ejecutar `scripts/git-role.sh check-development`: comprueba `gh`, credencial efectiva de Git y autor configurado sin imprimir tokens. Cambiar `git user.name` no cambia la cuenta autenticada.
- La cuenta creadora debe comprobarse en GitHub después de abrir el PR. Solicitar revisión a `HernanEspinozaDev` y registrar autor, rama y HEAD.
- No publicar como `HernanEspinozaDev` ni aprobar como `HernanMEC`. Si la cuenta requerida no está disponible, informar el bloqueo; no sustituirla por otra.
- Aprobar solo después de revisar el HEAD actual y las pruebas pertinentes. Fusionar con el SHA esperado; verificar `merged_by`, merge SHA y cierre. No hacer autoaprobación.
- Instalar el hook local versionado con `git config --local core.hooksPath .githooks` en cada clon/worktree. No desactivarlo para saltarse una identidad incorrecta. El hook es un control local, no una protección remota de GitHub ni una garantía frente a clientes que lo omitan.

## MARKETPLACE-LOCAL-2

- Conservar `planning/marketplace_publico_v2/plan_desarrollo_local_v2.md`, `plan_perfeccionamiento_publico.md` y matriz como historial. MPV2-00–09 mantienen su historial; MPV2-10 se fusionó en PR #304 (`5029a1ab73f4e9c365d5352299849b7dbe80991f`), MPV2-11 en PR #305 (`2856a304fbbfdbf1b5dcb0544b7c6d504e574898`), MPV2-12 en PR #306 (`c797f0eb734e2c7eda37eb70a50d4d509e82677d`) y MPV2-13 en PR #307 (`28f63f3dcba5d1b6a57b47b0c229e0caf2bba7eb`). El usuario aceptó explícitamente el producto **solo** para el estado UX/demo/código `736f81a367eed79bd87950e2dc64716adc6469db` el 2026-10-14; no es aceptación global del marketplace, DOCKER-02 ni del cierre local. La rama de corte actual es LOCAL-CLOSE-00 sobre `origin/main` `28f63f3`.
- Orden dentro del corte: datos/migraciones → Go/OpenAPI → Angular → evidencia del recorrido. Preparar trabajo entre capas cuando el contrato lo permita, sin aceptar integración solo con respuestas simuladas.
- Mantener V1–V57 y toda migración aplicada byte a byte. MPV2-13 no prevé migraciones; si una necesidad imprescindible se demuestra, agregar solo V58 y probar base vacía y actualización desde V57, sin reescribir historial.
- Go conserva reglas y autorización; Angular usa features, hexagonal ligera selectiva, TypeScript estricto, Signals, adaptadores OpenAPI y SCSS/tokens. No hacer HTTP en presentación ni introducir rutas/transporte en fachadas de aplicación.
- No basta quitar guards: lecturas públicas necesitan DTO minimizado, política de publicación y acceso controlado a fotos. Mantener privadas reservas, identidad, contratos, pagos, evidencia y administración.
- Conservar las ocho categorías y unidades CLP hora/día/mes hasta una nueva decisión de producto. No presentar visitas/postulaciones, fotos reales, pagos reales o GCP como implementados.

## Pruebas y entornos

- Conservar las pruebas automatizadas, fixtures de código y evidencia histórica versionada. Limpiar datos de ensayo no significa borrar la cobertura de regresión.
- Crear datos/cuentas sintéticos recreables para cada ensayo. Credenciales solo en `.local/testing/` excluido, permisos 0600; nunca en commits, capturas, logs o archivos compartidos.
- Proyecto Compose, puertos y volumen exclusivos para ensayos. Revisar configuración resuelta antes de arrancar. No usar el proyecto persistente para resolver conflictos de puerto.
- `espacigo_pgdata` y los secretos/archivos vinculados a esa base se conservan salvo autorización explícita de reinicio. No `volume prune`, limpieza global ni borrado por un patrón demasiado amplio.
- Al terminar, detener y retirar recursos del propio ensayo; conservarlos solo si una demo/revisión vigente lo requiere y registrar responsable y motivo. Borrar las credenciales al retirar el ensayo.
- Registrar pruebas realmente ejecutadas, entorno, HEAD, resultados y límites. API simulada, captura y recorrido real tienen evidencias diferentes. No inventar aprobación o aceptación explícita.

## Cierre de PR

PR: objetivo/corte, capas afectadas, migraciones, contrato, pruebas, capturas si hay cambio visual, límites y datos de ensayo usados. Corregir comentarios en el mismo PR, solicitar nueva revisión y no iniciar el sucesor antes de la fusión.

Tras fusionar: sincronizar `main`, actualizar plan/matriz cuando corresponda, retirar rama/worktree y ensayos del corte. Mantener `main` como punto de partida. La aceptación final de producto requiere al usuario; una fusión técnica no la inventa.

## CIERRE LOCAL — LOCAL-CLOSE-00–02

- Seguir `planning/cierre_local/plan_cierre_desarrollo_local.md` y `planning/cierre_local/matriz_cierre_local.md`. LOCAL-CLOSE-00–02 están autorizados como secuencia, un PR a la vez y con base en `main`; no iniciar el corte sucesor hasta fusionar el PR previo. Tras cada fusión, desarrollo espera el encargo del chat de revisión para el siguiente corte, sin pedir otra autorización para el trabajo ya encargado. Avisar por `send_message_to_thread` al chat de revisión al publicar, corregir, terminar o bloquearse. LOCAL-CLOSE-00 es solo documental.
- Conservar las aceptaciones explícitas de FE-14 (2026-10-13, `d02f9c8`) y MPV2-13 (2026-10-14, UX/demo/código `736f81a`) dentro de sus alcances. No extrapolarlas a DOCKER-02 ni al cierre local integral, que siguen pendientes de evidencia/aceptación propia. DOCKER-01 (#268) y DOCKER-03 (#270) quedaron aceptados por sus PR #271 y #273. #269 se cerró administrativamente con aceptación funcional incompleta; #267 sigue abierta. Verificar criterios, no inferir estado por cierre administrativo.
- Estado actual FE-15: Angular reemplazó al mock en PR #292. Evidencia, pruebas o guías anteriores que mencionen el mock conservan su fecha y entorno; los nuevos recorridos usan Angular→Go→PostgreSQL.
- El checkout habitual `main` observado el 2026-10-14 está en `c797f0e`, cinco commits detrás de `origin/main`, con cinco archivos frontend modificados y `planning/auditorias/` sin tracking. Preservación excluida en `.local/preservation/local-close-00-20261014/`. No hacer fast-forward, stash, reset, cherry-pick ni copiar esos cambios al PR sin una revisión/decisión explícita. Para LOCAL-CLOSE-00 el árbol limpio desde `origin/main` `28f63f3` en `codex/docs/chore/local-close-00-baseline-plan` es la base canónica documentada.
- Conservar la auditoría original y su evidencia local; publicar únicamente resúmenes revisados, nunca logs/JSON completos sin revisión sensible. Preservar V1–V57, demos y `espacigo_pgdata`. No iniciar GCP ni modificar proveedores/producto en LOCAL-CLOSE-00.
- Para tareas de cierre futuras: una demo/proyecto/puertos/volumen/credenciales aislados por corrida, configuración Compose resuelta antes de arrancar; secretos sintéticos en `.local/testing`, modo 0600. El cierre integral local necesita su propia aceptación explícita, aunque exista aceptación previa del marketplace.
