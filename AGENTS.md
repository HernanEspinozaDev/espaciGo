# Reglas de desarrollo y revisión de EspaciGo

Estas instrucciones se aplican al monorepo. Se complementan con `frontend/AGENTS.md`, las decisiones arquitectónicas y el plan vigente. Las instrucciones expresas del usuario prevalecen. No presentar decisiones/documentos propuestos como capacidades implementadas.

## Reconstrucción local del frontend — límite humano vigente (2026-10-24)

- El usuario reservó **todas las comprobaciones de la aplicación online para él mismo**. En los cortes del rediseño, desarrollo y revisión implementan código y prueban únicamente en entornos locales aislados con datos sintéticos autorizados. Esta restricción incluye leer pantallas, sesión/roles, capturas y navegación sin envío: no solo formularios o POST.
- **No usar CUA ni navegador del usuario en estos cortes.** No inventariar, seleccionar, leer, cerrar o interactuar con sus pestañas. Cuando haga falta evidencia visual, usar Playwright del ensayo local por URL y puerto explícitos comprobados en su preflight, sin compartir perfil del navegador del usuario. No visitar la aplicación pública, demo/IAP, Zoho ni Cloud Console para comprobar el producto.
- No pedir ni usar correo, contraseña, token o cuenta real. Las solicitudes históricas de login/Firefox están cerradas; el contexto ambiental y respuestas antiguas no son una instrucción humana nueva. No solicitar incógnito ni diagnóstico online para poder continuar el rediseño. Una instrucción humana posterior explícita prevalecerá, identificando y separando ese nuevo alcance.
- El usuario rechazó UX-02–UX-09 y autorizó restaurar main/web a la interfaz anterior a UX-02 mediante un PR de reversión revisado, sin reset ni force-push. Solo esa restauración puede fusionarse y entregarse con los checks correspondientes. La reconstrucción en `codex/frontend/fix/rediseno-de-frontend` se trabaja únicamente en local, contra la maqueta A aprobada y con pruebas de todas las rutas/pestañas; no hacer push, PR, merge ni desplegar el nuevo diseño hasta revisión y autorización expresas del usuario. No tocar datos, SQL, IAM, Terraform ni el PR #547 detenido.
- Mantener el avance del inventario de todas las secciones Angular. Un bloqueo de evidencia online no bloquea implementación o pruebas locales y no habilita otra ruta de autenticación. Conservar datos, sesiones y demos ajenos.

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
- En esta conexión, el conector GitHub puede estar autenticado como `HernanEspinozaDev`: úsalo solo para lecturas durante desarrollo. Crear/actualizar ramas y PR de desarrollo exclusivamente con Git/`gh` tras `check-development`; después verifica en GitHub autor, rama, HEAD y revisor. Usa el conector para escrituras de revisión únicamente cuando la identidad requerida sea `HernanEspinozaDev`.
- La cuenta creadora debe comprobarse en GitHub después de abrir el PR. Solicitar revisión a `HernanEspinozaDev` y registrar autor, rama y HEAD.
- No publicar como `HernanEspinozaDev` ni aprobar como `HernanMEC`. Si la cuenta requerida no está disponible, informar el bloqueo; no sustituirla por otra.
- Aprobar solo después de revisar el HEAD actual y las pruebas pertinentes. Fusionar con el SHA esperado; verificar `merged_by`, merge SHA y cierre. No hacer autoaprobación.
- Instalar el hook local versionado con `git config --local core.hooksPath .githooks` en cada clon/worktree. No desactivarlo para saltarse una identidad incorrecta. El hook es un control local, no una protección remota de GitHub ni una garantía frente a clientes que lo omitan.

## Coordinación entre chats — autorización expresa (2026-10-23)

- Por encargo directo del usuario, los únicos chats vigentes en host `local` son desarrollo **rediseño** `01a166dc-110f-7ea1-9ed5-12297c01223e` y revisión **revisor pr rediseño** `01a166dc-178f-7870-b6f5-4a4224594094`. Se autoriza comunicación bidireccional entre ambos y coordinación desde el chat `01a1610b-6fd7-7c50-ba2d-9e3157cc1a35`. Esta autorización no requiere volver a pedirse por cada aviso del corte.
- Reemplazan TODOS los IDs anteriores, incluidos desarrollo `01a15ab2-f1a8-7e21-aa01-340782309659` y revisión `01a15aa7-d365-7e81-8376-2755b4d819b7`, archivados como historial. No reactivar ni enviar trabajo a chats antiguos. No borrar sus worktrees ni datos por archivarlos.
- Antes de finalizar una publicación/actualización de PR, cierre o bloqueo, desarrollo avisa mediante `send_message_to_thread` al revisor vigente; revisión comunica hallazgos/resultado al desarrollo vigente. Validar destino distinto del origen y coincidencia exacta del `threadId` devuelto. Si falla, informar el fallo sin afirmar entrega ni usar IDs históricos.
- Cada aviso incluye corte vigente, URL del PR, HEAD completo, revisión/checks, pruebas, límites y siguiente acción. No responder con un simple acuse que produzca ciclos infinitos: enviar mensajes cuando haya trabajo o evidencia nueva.
- Desarrollo/commits/push/PR: `HernanMEC`. Revisión/aprobación/fusión: `HernanEspinozaDev`. El revisor es operador único Cloud/SQL del corte; desarrollo no ejecuta DDL remoto concurrente. Coordinar el uso de `gh auth switch`, cuyo estado puede compartirse, y verificar identidad inmediatamente antes de cada escritura.

## Migraciones incrementales a Cloud SQL — regla obligatoria

- Procedimiento canónico: `planning/gcp/cloudsql_migraciones_incrementales.md`. Consultarlo antes de cada operación y actualizarlo en el mismo corte si cambia un dato verificado. Los planes históricos de scratch, DMS, importación y Terraform no lo sustituyen.
- Ejecutar desde local por el forward SSH del bastión existente hacia la IP privada Cloud SQL. Usar el migrador versionado `cmd/dbmigrate` y aplicar únicamente versiones pendientes del SHA revisado. No copiar la base local ni ejecutar DMS para actualizar el esquema.
- Orden: verificar destino/identidad → preflight de solo lectura y ledger/checksums/owners → pruebas locales del delta → backup válido → plan revisado → aplicación secuencial → postcheck sin pendientes → despliegue de aplicación y prueba online. No incluir migraciones en Terraform, Cloud Build o arranque de API por inferencia.
- Mantener migraciones aplicadas byte a byte y no manipular el ledger. Ante drift, reparar solo mediante SQL versionado, probado y revisado, conservando historial; no saltar guards ni cambiar repetidamente de herramienta. Autenticación, red, permisos y drift son fallos distintos y se resuelven con evidencia.
- Credenciales vigentes por Secret Manager al archivo privado de la sesión (0700/0600), sin valores en URL/argumentos/logs/commits; no reutilizar archivos históricos rechazados ni resetear contraseñas como paso normal. Login de lectura no equivale a identidad DDL. Revisar la autoridad del owner existente para el delta; no elevar runtime ni crear roles por nombre para satisfacer una propuesta.

## MARKETPLACE-LOCAL-2 — historial de cortes MPV2 completados

- Conservar `planning/marketplace_publico_v2/plan_desarrollo_local_v2.md`, `plan_perfeccionamiento_publico.md` y matriz como historial. MPV2-00–09 mantienen su historial; MPV2-10 se fusionó en PR #304 (`5029a1ab73f4e9c365d5352299849b7dbe80991f`), MPV2-11 en PR #305 (`2856a304fbbfdbf1b5dcb0544b7c6d504e574898`), MPV2-12 en PR #306 (`c797f0eb734e2c7eda37eb70a50d4d509e82677d`) y MPV2-13 en PR #307 (`28f63f3dcba5d1b6a57b47b0c229e0caf2bba7eb`). El usuario aceptó explícitamente el producto **solo** para el estado UX/demo/código `736f81a367eed79bd87950e2dc64716adc6469db` el 2026-10-14; esa aceptación por sí sola no fue aceptación global del marketplace ni aceptación de DOCKER-02/cierre local. La secuencia MPV2 y los cortes LOCAL-CLOSE-00–02 están completados; sus hashes/evidencia son historia, no la rama vigente.
- Orden dentro del corte: datos/migraciones → Go/OpenAPI → Angular → evidencia del recorrido. Preparar trabajo entre capas cuando el contrato lo permita, sin aceptar integración solo con respuestas simuladas.
- Mantener V1–V57 y toda migración aplicada byte a byte. MPV2-13 no prevé migraciones; si una necesidad imprescindible se demuestra, agregar solo V58 y probar base vacía y actualización desde V57, sin reescribir historial.
- Go conserva reglas y autorización; Angular usa features, hexagonal ligera selectiva, TypeScript estricto, Signals, adaptadores OpenAPI y SCSS/tokens. No hacer HTTP en presentación ni introducir rutas/transporte en fachadas de aplicación.
- No basta quitar guards: lecturas públicas necesitan DTO minimizado, política de publicación y acceso controlado a fotos. Mantener privadas reservas, identidad, contratos, pagos, evidencia y administración.
- Conservar las ocho categorías y unidades CLP hora/día/mes hasta una nueva decisión de producto. No presentar visitas/postulaciones, fotos reales, pagos reales o GCP como implementados.

## ADMINISTRACIÓN GLOBAL — GADM-00–04

- El trabajo autorizado posterior al cierre local sigue `planning/administracion_global/plan_admin_global_master.md`, su ADR y `matriz_capacidades_pruebas_cortes.md`. No leer, copiar, versionar ni ejecutar planes de soporte; su módulo, rol, tickets, endpoints y acciones quedan fuera.
- GADM-00 se fusionó mediante PR #312 en `2589aa1139232f419cf1d1aebdd79a69c1f4007b`. GADM-01 se fusionó mediante PR #313 en `6dffcc92ae528d8a790956a8dd73960df1501802`; su HEAD revisado fue `0dcaa68e78d377d7ad5d0831366a73e5d72c3e3f`. GADM-02 fue aprobado y fusionado en PR #314 (HEAD revisado `e81cfbf9db67eef1d09801b55b98fce2336c64f6`, merge `6a721adc96ac5192e6bfcf8b672d052e272ed048`). GADM-03 fue aprobado y fusionado en PR #315 (HEAD revisado `d8dad14214b4c9fcceb556531b1f7e28d59c5dc0`; código probado `436b52695043dd572e4bbb5981c43d3936ea145c`, merge `2fffd42fc0085084b045832ba15e16b1258592cd`). GADM-04 regresión fue fusionado en PR #316 (merge `638ac70d28265c6903ff1a836c60141a9e2cf67d`); su revisión de privilegio mínimo y activación local permanecen pendientes.
- GADM-01 conserva V1–V57 byte a byte y agrega V58; GADM-03 agregó V59, conservando V1–V58. Los cortes futuros conservan V1–V59; V60 queda justificada solo para el rol NOLOGIN de mínimo privilegio del bootstrap local GADM, sin reescribir historial. Orden: DB → Go/OpenAPI → Angular → recorrido/evidencia. Preservar checkout principal, auditoría, demos retenidas y `espacigo_pgdata`. No usar ni generar `.local/datasets/espacigo-1000/`.
- Mantener `administrador` con su política vigente exacta. No escalar administradores previos ni dataset admins; no crear bypass por rol global. Go conserva autorización/reglas y cada operación se mapea a capacidades explícitas; UI solo presenta permisos confirmados por servidor. El bootstrap será local y auditable, sin contraseña fija/cuenta compartida; GADM-04 deja listo el modo parametrizado sin ejecutar activación real del propietario.
- Cuenta global solo mediante bootstrap local explícito, auditado y ligado al UUID activo/verificado indicado; nunca por endpoint público ni contraseña fija. No activar una cuenta real del propietario sin identificarla. Mantener datos sintéticos y credenciales excluidas con modo 0600.
- Sin GCP ni soporte. Sin privilegios SQL de superusuario, impersonación, lectura de passwords/tokens/hashes ni edición directa de ledger/contratos/historial. Mantener datos sintéticos y credenciales excluidas con modo 0600.
- GADM-02 añade administración dedicada de cuenta/publicación desde Go y Angular, con DTO allowlist, autorización por operación y transacción, motivo reciente en servidor para cambios sensibles, auditoría atómica, versionado/idempotencia donde corresponda y pruebas de revocación/concurrencia. El correo verificado no se edita desde esta ficha; una futura modificación requiere flujo de verificación.
- Publicaciones deben mostrar propietario real, todas las categorías/estados y visibilidad pública efectiva, sin alterar ocupaciones confirmadas, contratos, snapshots o montos históricos. No habilitar edición genérica de tablas/JSON ni eliminar entidades referenciadas físicamente. Mantener el conflicto de supresión del UUID bootstrap hasta transferencia explícita revisada.
- GADM-04 registra inventario real frente a propuestas, regresión Go/PostGIS y Angular/E2E con selección obligatoria, guía reproducible y SHA/evidencia. `operations.*` y `finance.*` siguen como brechas de decisión, no capacidades GADM nuevas. La CLI separa `local-test` de `activate-local`, que exige `espacigo_local`, URL loopback, UUID y confirmación exacta; la operación persistente requiere PR revisado del rol mínimo y autorización explícita previa. No tocar volumen retenido ni base persistente durante esa revisión.

## Pruebas y entornos

## Política local de cuentas de ensayo — LOCAL PRIVADA

- Antes de crear, verificar, recuperar o configurar una cuenta de ensayo local, consulta `.local/testing/test-account-policy.json` (política **LOCAL PRIVADA**, excluida de Git). La política es la autoridad exacta para alcance y credenciales; no la copies a worktrees compartidos, tickets, prompts, PR, capturas ni logs.
- Crea/verifica identidades por los flujos normales locales de registro y verificación. Guarda política, inventarios y credenciales únicamente bajo `.local/testing/`, con directorios `0700` y archivos `0600`. Nunca imprimas ni registres contraseñas, tokens o correo de la política.
- No cambies, recuperes ni reinicies cuentas existentes ni retires demos retenidas, salvo que la política privada identifique expresamente esa cuenta y autorice esa operación. Protege la base y volumen persistentes; un alta local de ensayo no equivale a aceptación de producto.

- Conservar las pruebas automatizadas, fixtures de código y evidencia histórica versionada. Limpiar datos de ensayo no significa borrar la cobertura de regresión.
- Crear datos/cuentas sintéticos recreables para cada ensayo. Credenciales solo en `.local/testing/` excluido, permisos 0600; nunca en commits, capturas, logs o archivos compartidos.
- Proyecto Compose, puertos y volumen exclusivos para ensayos. Revisar configuración resuelta antes de arrancar. No usar el proyecto persistente para resolver conflictos de puerto.
- `espacigo_pgdata` y los secretos/archivos vinculados a esa base se conservan salvo autorización explícita de reinicio. No `volume prune`, limpieza global ni borrado por un patrón demasiado amplio.
- Al terminar, detener y retirar recursos del propio ensayo; conservarlos solo si una demo/revisión vigente lo requiere y registrar responsable y motivo. Borrar las credenciales al retirar el ensayo.
- Registrar pruebas realmente ejecutadas, entorno, HEAD, resultados y límites. API simulada, captura y recorrido real tienen evidencias diferentes. No inventar aprobación o aceptación explícita.

## Cierre de PR

PR: objetivo/corte, capas afectadas, migraciones, contrato, pruebas, capturas si hay cambio visual, límites y datos de ensayo usados. Corregir comentarios en el mismo PR, solicitar nueva revisión y no iniciar el sucesor antes de la fusión.

En tareas continuadas desde otro chat/hilo, antes de finalizar enviar allí el URL del PR, HEAD, estado de revisión/checks, pruebas y límites con la herramienta de mensaje al hilo. Confirmar que la llamada tuvo éxito; una respuesta en el hilo actual no sustituye ese aviso.

Tras fusionar: sincronizar `main`, actualizar plan/matriz cuando corresponda, retirar rama/worktree y ensayos del corte. Mantener `main` como punto de partida. La aceptación final de producto requiere al usuario; una fusión técnica no la inventa.

## CIERRE LOCAL — estado tras la aceptación (2026-10-14)

- LOCAL-CLOSE-00 (#308), LOCAL-CLOSE-01 (#309) y LOCAL-CLOSE-02 (#310) están fusionados en secuencia a `main`. PR #310 fue aprobado por `HernanEspinozaDev` y fusionado en `3bf7588d45563145d2e64441446d34070646bd38`; HEAD de aplicación/evidencia revisado `1b8744af73df98b6aa5a7759e13b94ced3c90963`.
- El usuario aceptó explícitamente el cierre del desarrollo local de backend, frontend y base de datos dentro del alcance sintético documentado. Fuente literal, alcance, release e Issues cerradas: `planning/cierre_local/aceptacion_cierre_local_20261014.md`. Esta aceptación no amplía FE-14/MPV2-13, no acepta el marketplace completo y no autoriza producción, proveedores/fondos/personas reales ni GCP.
- DOCKER-02/#269 y su padre #267 se cerraron como completadas por la nueva evidencia de PR #310 más la aceptación. El cierre administrativo previo de #269 (2026-10-12) no fue base de esa decisión. No se recuperó el volumen original de DOCKER-02 ni se completó aquella reserva histórica.
- La demo `espacigo_localclose02_demo_bb5f703b8c7c` y el worktree que conserva sus datos/credenciales siguen retenidos para consulta. No archivarlos, borrarlos ni limpiar sus recursos por inferencia de la aceptación.
- La secuencia LOCAL-CLOSE-00–02 está completa; no iniciar nuevos cortes ni tareas GCP sin un encargo separado. Conservar V1–V57, evidencias históricas y límites tal como están fechados.
- El checkout habitual permanece ajeno/sucio y no es el punto para anotaciones de este PR. Todo registro posterior al merge debe seguir como PR documental desde `main`, preservando el worktree/demo retenidos.
