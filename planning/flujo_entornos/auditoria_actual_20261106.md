# Auditoría del flujo de datos, desarrollo y despliegue

Fecha: 2026-11-06. Fuente inspeccionada: `origin/main`, SHA `771134d165046b290b1f5ad3a1074a6357c2327f`. Auditoría estática: no se consultaron APIs GCP, bases de datos, secretos, facturación ni aplicación online. Las líneas corresponden a ese SHA. La conversación compartida se recibió finalmente como texto pegado por el usuario; no se pudo extraer del enlace.

## 1. Respuestas a las preguntas de la conversación

### ¿Qué tecnología y qué estado tenemos?

Backend Go con pgx, SQL explícito en `db/migrations/`, CLI propia `cmd/dbmigrate` y aplicación desplegada mediante Cloud Build/Artifact Registry/Cloud Run. El frontend actual de main es Next.js/React (`frontend/package.json:31`), aunque quedan instrucciones e historia Angular: el nuevo pipeline debe probar el producto que realmente contiene cada candidato.

La importación inicial con DMS es histórica. La evidencia vigente del runbook registra PostgreSQL 18.6, ledger V1–V83 y cero pendientes en Cloud SQL (`planning/gcp/cloudsql_migraciones_incrementales.md:101`). No es una lectura remota nueva. Los 83 SQL están en el árbol inspeccionado. No corresponde reimportar local, establecer otro baseline ni cambiar de migrador sin necesidad demostrada.

### ¿Qué comandos soporta el migrador?

- `-dir`: directorio de SQL; ejecución normal aplica pendientes.
- `-plan-only`: inventario JSON de aplicadas/pendientes en transacción READ ONLY, valida historial y guards de objetos. No crea ledger ni toma el advisory lock; no simula ejecutar el DDL pendiente (`cmd/dbmigrate/main.go:15`, `internal/migrator/runner.go:98`).
- Opciones administrativas específicas F5 y Cloud Owner V76–V80: `-apply-runtime-acl`, `-recover-runtime-acl`, confirmaciones exactas y `-cloud-owner-migration`. Son excepciones delimitadas, no modos genéricos de CI (`cmd/dbmigrate/main.go:16`).
- No hay subcomandos independientes `status`, `validate`, `dry-run`, `down` o rollback automático. La validación forma parte del plan/ejecución.

### ¿Cómo funciona el lock, un fallo o una interrupción?

La ejecución toma `pg_advisory_lock` de sesión con la pareja constante ESPG/MIGR antes de validar/aplicar. Permanece sobre la conexión de ejecución hasta liberar o cerrar la sesión (`internal/migrator/runner.go:234`, `:332`). Otro ejecutor cooperante espera; no impide SQL administrativo que ignore el lock ni serializa por sí solo todo el release de aplicación.

Cada archivo tiene su propia transacción: DDL e INSERT en `schema_migrations` se confirman juntos (`internal/migrator/runner.go:305`). Si falla, se revierte ese archivo; los archivos anteriores ya confirmados permanecen. Ante pérdida de conexión durante COMMIT puede haber resultado incierto para el cliente: leer ledger/checksums y objetos antes de reintentar. No se revierte todo el lote. El timeout configurable de la CLI no sustituye una política explícita de duración y bloqueos del DDL.

### ¿Cómo detecta diferencias entre Git y ledger?

`DiscoverMigrations` exige nombres `V` + seis dígitos, SQL no vacío, versiones únicas y contiguas desde 1; calcula SHA256 de bytes (`internal/migrator/migrator.go:36`). `verifyHistory` rechaza huecos, nombres cambiados, SQL alterado y versiones aplicadas que no existen en el checkout (`:85`). Los guards revisan objetos prescritos, no prueban equivalencia universal de todo el catálogo. El ledger también registra checksum del ejecutable (`internal/migrator/runner.go:281`).

Esto exige coordinar números entre ramas. No renumerar archivos ya aplicados en una base retenida; un conflicto de versiones debe resolverse antes de aplicar, con inventario de destinos afectados.

### ¿Cómo sabe el backend a qué versión conectarse?

La configuración selecciona instancia/base/login/rol, no una copia de la base por versión. Cloud Build fija destino y secretos; la API crea su pool y asume runtime (`cloudbuild.yaml:20`, `:191`; `cmd/api/database_pool.go:12`). Cloud Run ya usa Direct VPC egress con rangos privados y el Cloud SQL Connector; el tráfico de la API no depende del bastión (`cloudbuild.yaml:189`).

**Sí existe una barrera parcial de esquema:** el modo `full` exige un reporte de destino, ledger/checksums y propiedades/ACL, fijado por URI, generación y hash; el verificador limita antigüedad a seis horas (`cloudbuild.yaml:73`, `:95`; `scripts/verify_img01_schema_release_evidence.py:110`). Por tanto, la afirmación histórica «no existe ninguna barrera» ya no describe todo el flujo.

**Brecha:** no se identificó un contrato general de compatibilidad por imagen que pruebe simultáneamente API anterior/nueva, cambios destructivos y rollback. El pool no compara un contrato de release con el esquema. Una condición Cloud Run Ready no demuestra esa compatibilidad funcional.

Además, `_DEPLOY_SCOPE` es `web` por defecto (`cloudbuild.yaml:8`): un push no significa publicar siempre backend y SQL. El wrapper frontend exige main remoto limpio y el rollout valida que el backend siga igual. El modo `full` construye API/web, verifica provenance y despliega por digest; no aplica SQL.

### ¿Qué pruebas existen y cuáles se ejecutan obligatoriamente?

Hay pruebas de base vacía, actualizaciones desde versiones previas, checksums, rollback por archivo, recuperación entre commits y concurrencia, incluso procesos independientes (`internal/migrator/integration_test.go:1325`, `:1350`, `:1373`, `:1418`, `:1452`). Los ensayos V75→V83 y recuperación están documentados en los archivos `espacios_cloudsql_v83_*` de `planning/gcp/`.

`application-quality.yml:26` ejecuta `go test ./...`, vet y build; también existen controles frontend y de imágenes/política. Sus triggers cubren main/PR a main, no todos los pushes de ramas de tarea. Hay pruebas PostgreSQL que se omiten sin `TEST_DATABASE_URL` (`internal/migrator/integration_test.go:788`). El workflow no aprovisiona allí una base PostgreSQL/PostGIS ni impone que estas pruebas hayan corrido: test verde no equivale a regresión SQL ejecutada. Tampoco se identificó en el rollout activo una dependencia explícita de éxito de ambos workflows para el mismo SHA antes de desplegar.

No se ejecutaron suites de aplicación en esta auditoría documental.

### ¿Hay staging, aprobación, backups/PITR y recuperación?

- Se identifican testprod, web pública y demo privada; compartir API/base entre dos webs no constituye staging aislado.
- Los contenedores F5 y ensayos SQL son entornos locales distintos; no acreditan staging cloud.
- La evidencia registra trigger `espacigo-main-rollout` para `^main$`, junto a un flujo de validación local/autorización operativa. No se comprobó su estado actual ni una aprobación cloud obligatoria por ambiente.
- Hay backup gestionado `1793899104062` SUCCESSFUL y dump con checksum/lectura comprobada registrados en el runbook (`:114`). Esto desmiente «no existe ningún backup»; no prueba política automática actual, retención, PITR habilitado ni recuperación cloud ensayada.
- El bastión quedó TERMINATED según evidencia (`:116`). El costo actual no se puede inferir de que históricamente se encendiera: discos/IP/snapshots y uso efectivo requieren inventario y Billing.

### ¿Qué impide ejecutar el migrador como Cloud Run Job?

La CLI ya soporta `DATABASE_CONNECTION_MODE=cloudsql`, `DATABASE_URL` sin contraseña, `CLOUD_SQL_INSTANCE_CONNECTION_NAME`, `CLOUD_SQL_PRIVATE_IP=true`, `DATABASE_PASSWORD_FILE` y `DATABASE_SET_ROLE=espacigo_schema_owner`. La ejecución normal exige `espacigo_migrator_login` (`cmd/dbmigrate/config.go:31`, `:117`). El conector proporciona transporte TLS autenticado; no crea una ruta de red hacia IP privada por sí mismo.

Los obstáculos concretos son:

1. **Imagen incompleta:** `Dockerfile.runtime-migrator:13` copia V1–V69, no V70–V83. `Dockerfile.dbmigrate` es el ejecutor histórico scratch V1–V61, no se debe reutilizar para el delta actual. Hay que reconstruir/verificar imagen vigente, dependencias y manifest de todos los SQL antes de cualquier Job.
2. Falta definir/probar Job actual con VPC/subred adecuada, identidad propia, secreto fijado, tareas/paralelismo 1, timeout, política de reintentos y destino allowlist por ambiente.
3. La cuenta Cloud Build documentada no tiene `run.jobs.*`, acceso SQL ni payloads de secretos. Hay que diseñar autoridad para aprovisionar/configurar Job y ejecutarlo; no añadir permisos amplios por comodidad. El poder de reemplazar un Job que migra es autoridad efectiva para ejecutar DDL.
4. Debe comprobarse que el login migrador tiene exactamente la autoridad requerida por el próximo delta. Los repairs históricos V81–V83 no habilitan ejecutar reparaciones privilegiadas genéricas en CI.
5. Faltan orquestación que espere éxito, postcheck, evidencia del mismo SHA/digest y bloqueo de release completo. No basta enviar la ejecución y continuar.
6. El procedimiento canónico y los validadores actuales prohíben migraciones en build/release por defecto: deben evolucionar juntos mediante una decisión explícita, conservando su cobertura.

## 2. Hallazgos por prioridad

| Prioridad | Hallazgo | Tratamiento |
| --- | --- | --- |
| Alta, bloquea automatización | Imagen runtime omite migraciones posteriores a V69 | Corregir empaquetado y probar inventario contra árbol exacto antes de crear Job |
| Alta | Sin contrato general API/esquema y secuencia integrada SQL→tráfico | Manifest de release, compatibilidad anterior/nueva y postcheck bajo lock de ambiente |
| Alta | Recuperación/PITR/retención reales no comprobados | Inventario read-only y restore aislado antes de escrituras automatizadas |
| Alta | Pruebas DB pueden omitirse y rollout no acredita gate de calidad por SHA | Integración SQL explícita, fail si omitida; gates en cadena para el mismo candidato |
| Media | Un único testprod conocido; nombre stage no prueba aislamiento | Decidir clasificación del destino actual y crear/seleccionar staging aislado |
| Media | Dependencia operativa del túnel/portátil | Job dentro de VPC; retirar bastión tras validar recuperación y dependencias |
| Media | API mínimo 1/CPU siempre asignada y recursos cloud fijados a testprod | No clonar configuración sin analizar tareas de fondo, costos y separación |
| Media | Runbook mezcla estado V83 y secciones históricas PR/V67/plan-only | Normalizar autoridad vigente por fecha, sin borrar historia ni tratar PR antiguo como gate actual |

## 3. Verificaciones cloud pendientes, solo lectura

Los siguientes son ejemplos de inventario para una fase operativa autorizada; **no se ejecutaron**. No muestran secretos ni consultan datos de negocio. Usar identidad/proyecto explícitos, salida privada y redactar metadata sensible antes de versionarla.

```bash
gcloud auth list --filter=status:ACTIVE --format='value(account)'
gcloud sql instances describe espacigo-testprod-db --project=espacigo-testprod --format='yaml(name,region,databaseVersion,settings.tier,settings.availabilityType,settings.backupConfiguration,settings.ipConfiguration,settings.activationPolicy)'
gcloud sql backups list --instance=espacigo-testprod-db --project=espacigo-testprod --limit=10
gcloud run services describe espacigo-test-api --project=espacigo-testprod --region=southamerica-west1 --format='yaml(metadata.annotations,spec.template.metadata.annotations,spec.template.spec.serviceAccountName,status.traffic)'
gcloud run jobs list --project=espacigo-testprod --region=southamerica-west1
gcloud compute instances describe espacigo-dms-bastion-v67 --project=espacigo-testprod --zone=southamerica-west1-a --format='yaml(name,status,machineType,disks,networkInterfaces,scheduling)'
gcloud builds triggers describe 548b8eff-e489-4307-a77f-455b56aa4e57 --project=espacigo-testprod --region=southamerica-west1 --format='yaml(name,disabled,repositoryEventConfig,serviceAccount,approvalConfig)'
```

Añadir inventario acotado de discos/IP/snapshots asociados por identificadores exactos, IAM efectivo del ejecutor y facturación por recurso. Revisar logs de ejecuciones existentes, si las hubiera. El ledger/ACL requieren una lectura SQL autorizada adicional; no iniciar bastión, Job, dump, restore ni pruebas online como parte de este inventario.

## 4. Evidencia del corte documental

- Worktree aislado desde main actualizado por fast-forward y fetch: rama `codex/docs/chore/flujo-00-plan-entornos-migraciones`.
- El hook rechazó el primer nombre sin identificador de corte; se corrigió a `flujo-00`, conservando hooks e historial local.
- Revisión local íntegra de los dos documentos nuevos; `git diff --check` PASS.
- Comprobación de enlaces locales, bloques Markdown y alcance de archivos PASS; inventario estático V1–V83 contiguo PASS.
- Consulta de documentación pública oficial de Google Cloud/PostgreSQL, enlazada en el plan. Sin pruebas funcionales, build de imagen ni suites SQL ejecutadas: este corte solo añade documentación.
- Ninguna lectura/escritura de DB, llamada GCP, acceso a secretos o navegación de producto. Sin recursos de ensayo ni credenciales creados.
- La publicación en rama de tarea no implementa la propuesta, integra main ni dispara intencionalmente un despliegue del producto. Las fases FLUJO-01–07 siguen pendientes.

Plan propuesto: [plan de evolución](plan_evolucion_20261106.md).
