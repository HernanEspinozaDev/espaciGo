# Plan de evolución — etapa inicial de desarrollo sin staging

Fecha: 2026-11-06. Revisión 3. Estado: **plan de ejecución inicial; componentes pendientes de implementar**. Base técnica auditada: `771134d165046b290b1f5ad3a1074a6357c2327f`. Este corte actualiza documentación; no ejecuta migraciones, despliegues, cambios IAM ni eliminación de recursos.

## Decisión vigente y alcance

El usuario establece que el producto continúa en desarrollo y todavía no tiene usuarios reales. En esta etapa los cambios se integran directamente a **main** y se publican al **Cloud Run y Cloud SQL actuales**, sin develop ni ambiente intermedio. Las migraciones dejan de depender del portátil/túnel y pasan a un Cloud Run Job en la VPC. El bastión se retira al completar y verificar la sustitución.

Esta decisión sustituye la preferencia anterior de incluir staging bajo demanda en la ejecución inmediata. Staging, su segunda instancia SQL, sesiones/TTL, encendido/apagado, controlador y vigilante quedan **diferidos**, fuera del costo y de los gates de esta etapa. Su propuesta se conserva en [staging futuro](staging_futuro_20261106.md), para prepararla antes de incorporar los primeros usuarios reales.

Main sigue siendo la única rama permanente. No se introducen PRs, revisiones remotas ni promoción entre ambientes. Se pueden preparar cambios en ramas/worktrees temporales para revisión local y evitar activar el trigger con código incompleto; su destino de integración y despliegue es main. La petición actual autoriza **planificar** este flujo completo: no ejecuta por sí sola las operaciones cloud descritas aquí ni publica ahora cambios de pipeline en main.

La ausencia de usuarios reales es una declaración humana de contexto, no una inspección de cuentas ni permiso de borrar datos existentes. Conservar demos, respaldos, volúmenes y migraciones aplicadas. El recorrido funcional online continúa a cargo del usuario conforme a la regla vigente; no se habilita CUA ni uso de cuentas reales por esta planificación.

## Resultado esperado

```text
Desarrollo de código Go/frontend + nuevo SQL versionado
                  ↓
Revisión/pruebas locales → commit integrado a main
                  ↓
Cloud Build fija el SHA y espera controles de calidad del mismo candidato
                  ↓
Construcción y verificación de imágenes API/web/migrador por digest
                  ↓
Lock de release del único entorno de desarrollo cloud
                  ↓
Preflight READ ONLY + backup/recuperación comprobados
                  ↓
Cloud Run Job → Direct VPC → Cloud SQL actual: solo migraciones pendientes
                  ↓
Postcheck de ledger/checksums/objetos y compatibilidad de aplicación
                  ↓
Revisión Cloud Run sin tráfico → controles técnicos → tráfico
                  ↓
Prueba funcional del usuario en el entorno actual + observación/evidencia
```

**«Directo a Cloud SQL» significa Job dentro de la VPC hacia la instancia privada actual**, sin bastión, DMS ni importación del PostgreSQL local. No significa exponer un puerto SQL público. Cloud Run Jobs admite Direct VPC egress; el pipeline debe esperar y comprobar el resultado de su ejecución. [Red privada para Jobs](https://docs.cloud.google.com/run/docs/configuring/vpc-direct-vpc), [ejecución y resultados](https://docs.cloud.google.com/run/docs/execute/jobs).

No hay transacción única entre SQL y aplicación. Si la migración falla se detiene el release antes de publicar el backend nuevo; si la migración terminó y falla la aplicación, el esquema ya pudo avanzar. La revisión anterior debe seguir siendo compatible durante la transición.

## Punto de partida y destino único

| Elemento | Evidencia de la auditoría | Acción inicial |
| --- | --- | --- |
| Git | main es la única rama permanente; trigger documentado `^main$` | Mantener main como integración y origen del despliegue |
| Backend/frontend | Go + pgx; frontend de main Next.js/React | Probar el código realmente presente en el SHA candidato |
| Cloud | `espacigo-testprod`, `southamerica-west1` | Revalidar metadata/identidad sin renombrar recursos |
| SQL | Instancia `espacigo-testprod-db`, base `espacigo_dms_stage_20261020`; evidencia previa PostgreSQL 18.6/V83 | Revalidar ledger/ACL/extensiones; conservar datos y nombre actual |
| Migrador | CLI Go con plan READ ONLY, checksums, advisory lock y transacción por archivo | Mantener herramienta y controles |
| Imagen migrador | `Dockerfile.runtime-migrator` solo incluye hasta V69 en la base auditada | Corregir inventario/empaquetado antes de crear o ejecutar Job |
| Cloud Build | Modo web por defecto; full con evidencia SQL externa, sin aplicar DDL | Añadir secuencia integrada conservando modo web y sus validadores |
| Backup | Evidencia histórica de backup exitoso; PITR/retención/restore cloud no acreditados actualmente | Verificar y probar recuperación antes de automatizar escrituras |
| Bastión | Ruta manual del runbook; último cierre documentado TERMINATED | Sustituir conectividad de migración/diagnóstico, después retirar recursos exactos |

Estos valores proceden de [la auditoría estática](auditoria_actual_20261106.md) y del runbook, no de lecturas cloud de esta revisión. Verificar si main o el esquema avanzaron antes de implementar; V84 no se reserva ni se crea por redactar este plan.

## Qué se mantiene y qué se posterga

| Ahora, desarrollo inicial | Después, antes de primeros usuarios reales |
| --- | --- |
| Main → entorno cloud actual, sin promoción entre ambientes | Incorporar staging bajo demanda y producción final separados |
| PostgreSQL efímero local/CI para probar antes de tocar el destino actual | Mantener CI y añadir validación de candidato en staging |
| Un Job migrador con identidad propia y SQL privado existente | Jobs/identidades por ambiente y promoción de artefactos |
| Backup, recuperación y reversión compatibles con el entorno actual | Revisar objetivos productivos de continuidad/recuperación |
| Sin segunda instancia SQL permanente ni controlador de apagado | Cotizar/aprobar su ciclo de vida y guard de costo |

Un PostgreSQL temporal para una prueba no es un entorno staging. Se retira al terminar y no recibe tráfico de producto. El único destino cloud de release durante esta etapa es el actual. Ensayos de restore podrán necesitar un recurso temporal aislado autorizado; tampoco constituyen un staging permanente.

## Comportamiento por tipo de cambio en main

| Cambio | Calidad obligatoria | Operación en el destino cloud |
| --- | --- | --- |
| Solo documentación acreditada por clasificador | Validación documental/política afectada | No desplegar ni ejecutar DDL |
| Solo frontend sin cambio de contrato/API | Formato/lint/tipos/tests/build frontend y guards web-only | Actualizar web; verificar backend/esquema requeridos, sin migrar por rutina |
| Go/API/contrato compartido sin SQL nuevo | Go tests/vet/build, integración PostgreSQL afectada y frontend/contrato cuando corresponda | Verificar compatibilidad/esquema; Job puede resultar sin pendientes; desplegar componentes afectados |
| SQL nuevo, con o sin backend | Base vacía y actualización desde baseline destino, compatibilidad anterior/nueva, permisos y fallos/reanudación | Backup según política, Job, postcheck; después aplicación correspondiente |
| Pipeline/IAM/roles/red/imagen migrador | Tests de política y empaquetado + revisión local del delta exacto | Corte operativo explícito de activación/cambio; no tratarlo como release SQL ordinario |

El clasificador incluye código compartido, consultas, Dockerfiles, dependencias y configuración. Impacto ambiguo exige la ruta completa o detención explicada, nunca asumir web-only. Un push a main **no** habilita ejecutar SQL arbitrario: solo el delta versionado incluido en el manifest del candidato, sobre el destino fijado, bajo la política activada.

## Diseño del release integrado

### 1. Calidad y candidato fijo

- Usar SHA completo del push a main. Construir manifest con SHA, digests API/web/migrador según alcance, inventario SQL con nombres/checksums, provenance y contrato de compatibilidad. No usar latest ni volver a resolver tags mutables al desplegar.
- Conectar calidad y despliegue: el pipeline espera éxito de los controles requeridos para ese mismo SHA, o los ejecuta en su propia cadena. Un workflow verde de otro commit no permite publicar. Un test fallido puede dejar main sin desplegar; corregir mediante otro commit normal, sin force-push.
- Aprovisionar PostgreSQL 18 con extensiones requeridas en CI efímero y exigir que las pruebas SQL seleccionadas se ejecuten realmente; no aceptar omisión silenciosa por falta de `TEST_DATABASE_URL`. Fixtures y credenciales son sintéticos/privados, fuera de logs/commits.
- Ejecutar base vacía y actualización desde baseline equivalente al destino, con metadata de roles/owners necesaria. Conservar V1–V83 y todo SQL ya aplicado byte a byte. Coordinar numeración antes de aplicar en cualquier base retenida.

### 2. Exclusión, preflight y backup

- Tomar lock del **release completo** antes de modificar Job/esquema/revisiones/tráfico. Conservar advisory lock del migrador. Rechazar ejecuciones obsoletas y comprobar generación/propiedad del lock antes de liberar; no cancelar un Job que escribe para reemplazarlo por un push más nuevo.
- Verificar proyecto/región/instancia/base/login/SET ROLE, ledger/checksums, objetos/ACL y requisitos del delta mediante inspección READ ONLY. El plan sin lock no congela la base; el aplicador vuelve a validar bajo su lock.
- Verificar backup satisfactorio y antigüedad permitida; obtener backup nuevo antes de los deltas que lo requieran y esperar éxito. Registrar política/retención/PITR realmente presentes y método de restauración ensayado; un archivo listado por pg_restore no acredita por sí solo restore cloud.
- No vaciar/recrear la base compartida ni cargar seeds de CI. Un drift se repara mediante cambio versionado y revisado, sin editar ledger ni reutilizar excepciones F5/Cloud Owner.

### 3. Job de migración sin bastión

- Empaquetar todos los SQL actuales en la imagen migradora; verificar inventario/bytes contra el árbol exacto y probar construcción/ejecución local. No reutilizar `Dockerfile.dbmigrate`, que conserva el runner histórico V1–V61.
- Reutilizar la conectividad Direct VPC y Cloud SQL Connector tras validar subnet/rutas/IP disponible y política TLS. El conector autentica/cifra; no crea la ruta de red privada.
- Configuración del Job: imagen por digest, una tarea, paralelismo 1, timeout acotado, reintentos inicialmente 0, cuenta de servicio dedicada y secreto montado por versión. Destino allowlist exacto. CLI ordinaria con login `espacigo_migrator_login` y `SET ROLE espacigo_schema_owner`; nada de ejecutar migraciones desde el arranque de cada API.
- Separar identidad de provisionamiento/configuración del Job y autoridad de lanzamiento rutinario. Cloud Build no necesita leer contraseña SQL; poder cambiar imagen/args/secretos del Job equivale a autoridad efectiva de DDL y debe revisarse como tal. No habilitar overrides arbitrarios.
- Si preflight descubre una falta de autoridad SQL, preparar bootstrap/reparación mínimo con ejecutor administrativo **separado**, SQL exacto y temporal, dentro de VPC. La ruta inicial no debe depender de abrir el bastión para «preparar el Job». No elevar runtime ni crear un shell administrativo general. Verificar y retirar privilegios transitorios.
- Esperar finalización y comprobar ID de ejecución, digest/configuración efectivos y éxito de todas sus tareas; no basta que se acepte el lanzamiento. Conservar resultado aunque el padre se cancele; readback antes de reintentar.

### 4. Postcheck, despliegue y prueba directa

- Verificar cero pendientes del inventario esperado, ledger/checksums, objetos/ACL y contrato API–esquema. La evidencia ya existente del modo full se adapta a este resultado controlado, no se elimina para que el pipeline pase.
- El contrato declara requisitos de esquema y qué revisiones anteriores pueden volver. No basta `max(version)`. Una ampliación compatible puede admitir una API anterior; una contracción debe retirar explícitamente su rollback. Usar expandir → backfill → cambiar consumidores → contraer en otro corte.
- Publicar revisión API/web por digest sin tráfico, comprobar configuración/health/contrato y promover en orden compatible. Sin atomicidad entre servicios; no iniciar consumidores de fondo inesperados en preflight.
- Después de promover, ejecutar la validación funcional en el entorno cloud **actual** con cuentas/datos de ensayo autorizados. El usuario mantiene la navegación/QA online; evidencia técnica del pipeline y aceptación funcional se registran por separado. La falta de QA no debe figurar como aceptación automática.
- Un release ordinario del circuito ya activado pasa automáticamente de calidad y preflight a migración y despliegue; no introduce una aprobación manual nueva en cada push. Operaciones destructivas, repairs o cambios de autoridad fuera de la política se detienen para un corte específico.
- Registrar SHA, digests, IDs de build/Job/revisiones, esquema antes/después, backup, pruebas y resultado. Liberar lock solo tras readback. No apagar el SQL actual al terminar: sigue siendo la base del entorno de desarrollo compartido.

## Pruebas obligatorias y recuperación

| Escenario | Resultado exigido antes de activar el flujo |
| --- | --- |
| Imagen con SQL omitido, nombre/checksum alterado o hueco | Rechazo antes de aplicar DDL |
| Calidad omitida o de SHA diferente | Sin migración ni despliegue |
| Dos pushes cercanos | Releases serializados, sin mezcla de Job/imagen y sin promoción obsoleta |
| SQL falla a mitad de archivo | Rollback de ese archivo y su ledger; anteriores confirmados permanecen |
| Conexión/cancelación durante COMMIT | Leer ledger/objetos para resolver resultado incierto; no reintento ciego |
| Job correcto y API nueva fallida | Recuperar revisión anterior compatible; no revertir SQL automáticamente |
| Destino o identidad incorrectos; secreto faltante; red inaccesible | Fallo cerrado antes de escribir; no cambiar destino ni elevar permisos |
| Backup/restauración no válidos para el delta | Bloquear escritura; reparar la capacidad de recuperación |
| Release sin SQL nuevo | Resultado sin pendientes compatible, sin tocar archivos históricos |
| Solo documentación | Cero despliegues y cero ejecuciones SQL |

El runner transaccional no admite `CREATE INDEX CONCURRENTLY` dentro de un archivo normal. Si surge esa necesidad, diseñar una fase no transaccional explícita con readback/recuperación. Backfills grandes requieren lotes y checkpoints, no una transacción DDL sin límites. Fijar timeouts de bloqueo/operación por delta.

La recuperación distingue: rollback de aplicación a digest compatible, corrección hacia adelante mediante nueva migración y restauración de datos a destino aislado con conmutación autorizada. No usar DROP/importación local como solución de un fallo. Fijar inicialmente retención, RPO/RTO adecuados al desarrollo y probar la recuperación antes de habilitar escrituras automáticas, sin imponer por inferencia un SLA de producción.

## Activación gradual sobre el circuito actual de main

1. **Preparar localmente:** código, Dockerfile, CI, contrato y orquestación, con pruebas de política y ensayos sintéticos; mantener el circuito actual hasta completar esas piezas. Puede usarse una rama temporal; no añade un entorno ni altera el destino main.
2. **Inventariar/provisionar lo mínimo:** Job y cuentas/roles/secretos necesarios mediante cambio cloud acotado y aprobado. Probar conexión/plan READ ONLY desde el Job sin bastión. Si se requiere bootstrap privilegiado, usar el mecanismo separado descrito antes; no extender la cuenta deployer o API como solución genérica.
3. **Cerrar una ventana de transición:** fijar SHA/destino e impedir pushes concurrentes; comprobar que no hay build/release activo. Pausar el trigger actual antes de integrar a main el nuevo comportamiento. Registrar estado inicial y reversión, sin asumir que pausa cancela builds existentes.
4. **Integrar y activar una sola ruta:** llevar el código completo a main por historia normal, actualizar runbook/AGENTS/validadores para declarar la nueva ruta canónica y habilitar un único trigger/orquestador. Evitar tener simultáneamente un deploy antiguo y otro nuevo para el mismo push. Realizar primera ejecución controlada del SHA integrado.
5. **Primer recorrido:** plan cloud y cero pendientes pueden demostrar transporte/permisos sin inventar V84. Verificar un delta real solo cuando exista un cambio funcional revisado que lo necesite. Ensayar aplicación/reanudación con SQL sintético en local; no agregar DDL vacío al destino compartido para «probar el Job».
6. **Normalizar:** siguientes commits de producto a main disparan la ruta clasificada automáticamente; ante error no continuar a backend. La reversión del pipeline impide nuevos despliegues y recupera configuración conocida, sin borrar migraciones ya confirmadas ni reactivar bastión por defecto.

Actualizar el procedimiento canónico `planning/gcp/cloudsql_migraciones_incrementales.md`, reglas y pruebas de `validate_cloud_build_transition.py` en el mismo corte de activación. Hasta que ese corte esté implementado y verificado, el plan no presenta Jobs como ruta operativa ya disponible ni elimina la regla vigente de transporte por editar este documento.

## Fases ejecutables de la etapa inicial

La auditoría FLUJO-00 se conserva. Las fases pendientes FLUJO-01–07 se redefinen para **un solo destino de desarrollo**, sustituyendo sus gates anteriores de staging. El orden es secuencial; preparar entregables locales no requiere crear primero un segundo entorno cloud.

| Fase | Trabajo y entregable | Criterio de cierre |
| --- | --- | --- |
| FLUJO-00 | Auditoría existente y revisión 3 del plan inicial | Alcance main→cloud actual y staging diferido registrados; documentación de este corte |
| FLUJO-01 | Inventario read-only de SQL/red/roles/IAM/trigger, backup/PITR/recuperación y costo del bastión | Destino exacto revalidado; plan mínimo de Job y recuperación; listar recursos/dependencias a retirar; sin crear staging |
| FLUJO-02 | CI PostgreSQL efímero obligatorio + manifest/contrato de compatibilidad | Pruebas desde cero y upgrade/fallo/concurrencia ejecutadas; omisiones y SHA incorrecto bloquean release |
| FLUJO-03 | Corregir imagen migrador y preparar Job/identidades/red/orquestación | Inventario completo por digest; tests locales de permisos/destino/fallos; plan de aprovisionamiento mínimo y bootstrap si corresponde |
| FLUJO-04 | Habilitar Job en VPC contra el SQL actual y comprobar recuperación | Preflight cloud READ ONLY y sin bastión, identidad correcta; backup/restore ensayados; evidencia de acceso y autoridad mínima, sin DDL incidental |
| FLUJO-05 | Integrar pipeline en main y sustituir cuidadosamente el trigger | Una sola ruta, calidad→backup→Job→postcheck→aplicación; casos docs/web/backend/SQL probados; canónico y validadores actualizados |
| FLUJO-06 | Primer release directo del circuito, verificación y QA en el entorno actual | SHA/digests/ledger consistentes, tráfico correcto, resultado técnico y QA humano registrados; SQL real solo si existe delta revisado |
| FLUJO-07 | Retirar bastión y cerrar operación sin dependencia manual | Nueva ruta y diagnóstico/recuperación sin túnel ensayados; dependencias identificadas resueltas; borrado exacto autorizado y verificado, costo residual y documentación registrados |

Estados al iniciar ejecución: FLUJO-00 documental actualizado; FLUJO-01 inventario cloud y ledger READ ONLY comprobados, con límites de recuperación/costos; FLUJO-02 CI remoto y verificadores implementados, compatibilidad funcional/wiring pendientes; FLUJO-03 imagen y Job de plan publicados y probados, Job aplicador pendiente; FLUJO-04 preflight real iniciado, restore pendiente; FLUJO-05–07 pendientes. Evidencia y resultados: [avance inicial](avance_inicial_20261106.md). Cada fase registra SHA, comandos realmente ejecutados, pruebas, límites y recursos propios usados. No se afirma CI remoto, backup actual, migración aplicada ni aceptación de producto sin su evidencia.

## Retirada del bastión y operación posterior

El resultado final de esta etapa **no conserva un bastión como requisito habitual ni como fallback permanente**. Antes de borrar, comprobar si aún sirve a DMS, forward inverso, otras sesiones o recursos retenidos. Resolver esas dependencias explícitamente; no borrar el material histórico de la importación.

Inventariar VM, discos, IP reservada, snapshots, claves/reglas/bindings exclusivos por identificador y propietario. Presentar el conjunto exacto para eliminación autorizada, conservando backups requeridos. No hacer limpieza global ni borrar subred/VPC compartida con API/SQL. Verificar baja, recursos residuales y facturación posterior sin prometer ahorro no medido.

El diagnóstico normal utiliza metadata/logs y un ejecutor de inspección READ ONLY autenticado dentro de VPC. Restauración y reparaciones son operaciones separadas, auditadas y con permisos temporales; no se exige volver a abrir SSH para continuar el desarrollo. Un fallo de autenticación/red/permisos/drift se diagnostica por su causa, sin alternar herramientas para saltar controles.

## Costo y alcance de infraestructura inicial

Se reutilizan Cloud SQL, Cloud Run, VPC y registro actuales. Se añade únicamente lo necesario para Job/inspección, identidades y evidencias, CI efímero y recuperación acordada. No presupuestar ahora otra instancia staging, Scheduler de apagado ni controlador de sesiones. Un restore aislado temporal requiere costo/limpieza propios definidos antes de crearlo.

Comparar gasto real del bastión encendido/detenido y sus recursos asociados frente a minutos de Job/build/CI, logs, imágenes y costo adicional de backup/PITR. Mantener SQL actual disponible; no trasladar a él el apagado automático previsto para staging. No cambiar mínimos de API/workers sin revisar sus consumidores. Las cifras de lista y Terraform histórico no sustituyen Billing/inventario real; no se ofrece un ahorro garantizado.

## Hito de salida a producción final

Antes de incorporar los primeros usuarios reales se abre el corte de producción final y se retoma [staging bajo demanda](staging_futuro_20261106.md). Ese corte separará pruebas/productivo, revisará acceso/proveedores/datos, recuperación y operación, y validará promoción de artefactos. No es requisito para completar ahora FLUJO-01–07 ni se crea infraestructura futura por adelantado.

La decisión de continuar en main y omitir staging **ya está tomada para esta etapa**. Quedan por concretar durante FLUJO-01 recursos/permisos del Job, política de backup/recuperación, costos y ventana de activación; no volver a pedir elegir develop o staging para avanzar en este alcance.

## Evidencia y límites de esta revisión documental

- Fuente: instrucción humana posterior de mantener el desarrollo en main y Cloud SQL actual, retirar bastión y posponer staging a producción final.
- Se conserva la auditoría original; revisión 2 en historial y resumen futuro enlazado. No se ejecutaron lecturas SQL/GCP, migraciones, IAM, Terraform, despliegues ni pruebas online en esta revisión.
- Validación documental: revisión completa del diff, enlaces locales/bloques Markdown y `git diff --check`. No corresponde ejecutar suites de aplicación por este cambio de planificación.
- Publicar estos documentos en la rama de tarea no activa el pipeline propuesto ni modifica main. La implementación posterior seguirá la secuencia de activación de este plan.
