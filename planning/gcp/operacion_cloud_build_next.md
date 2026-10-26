# Operación de Cloud Build y Cloud Run — EspaciGo Next

Este es el punto de entrada operativo para el checkout `/home/nandev/Documentos/projects/espaciGo`. No sustituye las reglas actuales ni autoriza migraciones, cambios IAM/Terraform, cuentas reales de ensayo o alteraciones de datos. El usuario autorizó push directo a `main` con `HernanEspinozaDev` y la normalización del despliegue existente. No editar `AGENTS.md` ni restaurar Angular para seguir instrucciones históricas.

## Inventario verificado con gcloud el 26 de octubre de 2026

| Recurso | Valor |
| --- | --- |
| Proyecto / número | `espacigo-testprod` / `447373903153` |
| Región | `southamerica-west1` |
| Cuenta operadora ya conectada | `pasteleriahijitos@gmail.com` |
| Repositorio GitHub | `HernanEspinozaDev/espaciGo` |
| Trigger regional | `espacigo-main-rollout` |
| ID del trigger | `548b8eff-e489-4307-a77f-455b56aa4e57` |
| Evento / configuración | push `^main$` / `cloudbuild.yaml` |
| Conexión Cloud Build | `projects/espacigo-testprod/locations/southamerica-west1/connections/espacigo-github/repositories/espacigo` |
| Identidad de builds | `espacigo-cloud-build-deployer@espacigo-testprod.iam.gserviceaccount.com` |
| Artifact Registry | `southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod` |
| Bucket privado de staging/lock/evidencia | `espacigo-testprod-cloud-build-447373903153` |
| API privada | `espacigo-test-api` — puerto 8080 |
| Web pública | `espacigo-test-public` — puerto 4000 |
| Web privada IAP | `espacigo-test-private` — puerto 4000 |
| Dominio público | `espacigo.testingpage.store` |
| Dominio privado | `espacigo-demo.testingpage.store` |
| Cloud SQL existente | `espacigo-testprod-db` / base `espacigo_dms_stage_20261020` |

Usar `--account`, `--project` y `--region` explícitos. No cambiar cuentas globales en paralelo ni deducir recursos desde un nombre de proyecto supuesto. GitHub y GCP son identidades distintas.

## Información recuperada del checkout anterior

Se auditó `/home/nandev/Documentos/projects/Proyectotitulo/espaciGo` en modo read-only. Su rama local era `codex/frontend/fix/local-login-error-state`, HEAD `59fd92ab5ab331dbe5a7a52c1c5a028bd1d11468`, con modificaciones pendientes Angular/Go/contrato: se conservaron intactas. En ese árbol de trabajo no estaban `cloudbuild*.yaml` ni los runbooks recientes; sí estaban en su referencia local `origin/main` (`99a1ee6ab2764b50a218b7042519c40e43684080`). Por tanto, buscar solo archivos visibles de aquella rama habría perdido nuevamente el contexto.

Se recuperaron y contrastaron las referencias de su historial Git —`cloudbuild.yaml`, `cloudbuild-images.yaml` y `planning/gcp/gcp08_cloud_build_transition_20261022.md`— con los archivos y recursos actuales. La carpeta actual ya contenía esa transición y procedimientos posteriores. Se traslada aquí el conocimiento operativo comprobado, no una copia regresiva de infraestructura, Angular, reglas antiguas, `.local`, secretos ni estado Terraform. Los nombres definitivos de lock/servicios se validan en el pipeline actual y con gcloud, no se deducen del resumen de un agente.

## Diagnóstico que explica el desfase

El trigger sí estaba conectado y recibió el commit remoto `12c2b7552ce6a0ad5217835be9792412a847bc4f`. Su build `32a84b0e-dc87-40b8-9ef1-15a8c8e67ecf` terminó `FAILURE` en `acquire-and-validate-deploy-lock`: `_APP_SCHEMA_EVIDENCE_URI`, `_APP_SCHEMA_EVIDENCE_GENERATION` y `_APP_SCHEMA_EVIDENCE_SHA256` llegaron vacíos. No se construyeron ni desplegaron las imágenes de ese commit. Es un bloqueo intencional de la publicación completa, no un problema de que falte GitHub Actions.

La revisión pública observada al iniciar esta reparación, `espacigo-test-public-00026-tk9`, fue creada a las `2026-10-26T02:26:39Z`, antes de esos pushes, y servía el digest `sha256:06e89661fd631d7b7f94c90f84cc556eb7fa50b648cc12721f4607102f9517b6`. Que estuviese Ready y tuviese el 100% del tráfico no demostraba la publicación del HEAD actual. Importante: el historial contiene `36ac2763a985138c49f216f42104ceafe2a61b9c`, que revirtió la retirada de demos de `9874badf651b27db688c74a1e44e4231f6e89788`. Tanto el código actual como producción responden páginas 200 en `/saas` y `/pricing`: **no usar esas rutas como señal de despliegue obsoleto ni afirmar que el HEAD las redirige**. Esta reparación de entrega no modifica otra vez esas decisiones de producto.

El antiguo `scripts/gcp/deploy-frontend.sh` de este checkout apuntaba a `espacigo-448200` / `espacigo-frontend`, recursos que no forman parte de este inventario. No reutilizar ese procedimiento `run deploy --source`: usa otra ruta de build, otra cuenta de construcción por defecto y produjo 403 de `storage.objects.get`. No ampliar permisos de la cuenta Compute por ese error ni hacer público el servicio privado.

## Separación de publicaciones

La normalización distingue dos operaciones dentro del trigger existente:

1. **Web (`_DEPLOY_SCOPE=web`)**: publica únicamente Next.js en los servicios web existentes. Antes de hacerlo debe probar que las fuentes Go, dependencias, Dockerfile, migraciones y contrato del checkout son idénticos a los que produjeron la imagen de API que realmente recibe tráfico. La prueba usa proveniencia Cloud Build, archivo fuente de GCS fijado por generación y SHA256, y comparación de archivos. Si no coincide, falla explícitamente. No despliega Go, no consulta/escribe SQL y no obtiene contraseñas.
2. **Completa (`_DEPLOY_SCOPE=full`)**: conserva el gate de evidencia real de esquema de menos de seis horas, fijada por URI/generación/SHA256, y la comprobación SMTP. Cualquier cambio de Go/esquema/contrato requiere esta publicación. No eliminar el gate, modificar fechas de reportes antiguos o usar fixtures como evidencia remota.

El modo web se ejecuta en `web-only-rollout` mediante `scripts/gcp/rollout_web.py`; los pasos históricos restantes solo se ejecutan con `full`. `scripts/gcp/verify_web_backend.py` permite repetir únicamente la prueba de paridad en modo read-only. El executor web conserva red, identidad runtime, IAM/IAP y variables ajenas de los servicios existentes; actualiza imagen y variables de enlace Next→Go mediante `--update-env-vars`, prepara ambas revisiones con `--no-traffic`, comprueba digest/Ready/etiquetas y promueve cada revisión exacta. No recrea servicios ni usa `--set-env-vars` para borrar configuración ajena.

El modo web no acredita compatibilidad de nuevo backend; solo mantiene la API existente sin cambios. No es una autorización para migrar automáticamente. Una publicación completa necesita la evidencia vigente antes de ejecutarse. No conceder acceso SQL/Secret Manager a Cloud Build para automatizar esa captura por inferencia.

Prueba previa de esta reparación: el digest de API `sha256:612d771bf7f0c01c5f7fb2335d37bbd5678479e885ffb2abc352f4303fd2e5d7` procede del build `5305b375-8898-48a4-a0f1-58cdd614f92f`, SHA declarado `275c0bfad3378622fa36c230a5903d53243c565f`. Se verificó el archivo fuente real de esa proveniencia (generación `1792808035936172`, SHA256 `65ac05a2a6222c7df9dd5ec33e4fa539d4b661563fc17609292aa5d00095e069`): **447 archivos backend y cero diferencias** con el checkout actual antes de la reparación. Esto permite una entrega solo web sin tocar Cloud SQL.

## Preflight read-only y diagnóstico

```bash
cd /home/nandev/Documentos/projects/espaciGo
git status --short
git rev-parse HEAD
git ls-remote origin refs/heads/main
gh api user --jq .login

gcloud builds triggers describe espacigo-main-rollout \
  --account=pasteleriahijitos@gmail.com --project=espacigo-testprod \
  --region=southamerica-west1 --format=json

gcloud builds list --limit=5 \
  --account=pasteleriahijitos@gmail.com --project=espacigo-testprod \
  --region=southamerica-west1 \
  --format='table(id,status,createTime,substitutions.COMMIT_SHA,buildTriggerId)'
```

El script `scripts/gcp/deploy-frontend.sh` es la entrada de consola al trigger: consultar su `--help`. El uso sin argumentos debe ser read-only; iniciar otra publicación requiere opción explícita. Tras un push normal no ejecutar además un build manual salvo que se haya comprobado que falta el build de ese SHA: así se evitan carreras y locks innecesarios.

## Publicación completa y evidencia SQL

Seguir `planning/gcp/cloudsql_migraciones_incrementales.md` para el transporte ya existente, revalidando los recursos. La captura de catálogo es una operación **read-only distinta de migrar**:

- `scripts/capture_img01_schema_release_evidence.sh` usa el SQL versionado `db/query/img01_schema_release_evidence.sql` y `PGPASSFILE` 0600; no acepta contraseña en argumentos ni `PGPASSWORD`.
- `scripts/verify_img01_schema_release_evidence.py` exige destino, ledger/checksums, objetos, permisos y frescura. No alterar el reporte para satisfacerlo.
- Subir el reporte validado al bucket privado con nombre `img01-schema-evidence/<sha256>.json`, sin sobrescribir otros objetos. Verificar generación remota y digest.
- Ejecutar el trigger para un SHA completo de `main` con `_DEPLOY_SCOPE=full` y los tres valores `_APP_SCHEMA_EVIDENCE_*`. Nunca fijar un reporte de seis horas como default permanente del trigger.
- Ante divergencia SQL, detener el full rollout y preparar la reparación/migración aprobada. No resetear la base ni importar la base local.

## Verificación obligatoria de entrega

La cadena aceptable es **SHA remoto → build del trigger SUCCESS → build hijo/proveniencia → digest Artifact Registry → revisión Ready con ese digest → tráfico de esa revisión → HTTP del dominio correcto**.

```bash
# Sustituir BUILD_ID por el ID obtenido, no inventar uno.
gcloud builds describe BUILD_ID \
  --account=pasteleriahijitos@gmail.com --project=espacigo-testprod \
  --region=southamerica-west1 \
  --format='json(status,substitutions.COMMIT_SHA,substitutions._DEPLOY_SCOPE,failureInfo)'

gcloud builds log BUILD_ID \
  --account=pasteleriahijitos@gmail.com --project=espacigo-testprod \
  --region=southamerica-west1

for service in espacigo-test-public espacigo-test-private; do
  gcloud run services describe "$service" \
    --account=pasteleriahijitos@gmail.com --project=espacigo-testprod \
    --region=southamerica-west1 \
    --format='json(status.traffic,status.latestReadyRevisionName,spec.template.spec.containers[0].image)'
done
```

Inspeccionar también la revisión exacta: imagen/digest, condición Ready, etiquetas de SHA/build. No usar `--to-latest` sin identidad porque otro build puede moverla. Verificar los dos servicios por separado. No afirmar recorrido privado IAP/E2E si solo se comprobaron metadata y readiness.

El dominio público usa el URL map `espacigo-testprod-edge` → backend `espacigo-testprod-public-web` → NEG `public-web-serverless` → `espacigo-test-public`. El dominio privado usa backend `espacigo-testprod-private-web` → NEG `private-web-serverless` → `espacigo-test-private`. No confundir URLs `run.app` adivinadas con esos destinos. No se usa domain mapping de Cloud Run para estos dominios.

Para HTTP usar clientes que fallen ante errores y comprobar código, URL final y contenido esperado. `curl ... | grep ... || echo VERIFIED` es una comprobación inválida: imprime VERIFIED incluso si curl falla. La ausencia de un nombre de componente React en HTML no demuestra su retirada, especialmente en un menú móvil cerrado.

## Retomar después de cambiar de agente/modelo

1. Leer este documento, las reglas actuales y el estado de Git; no continuar instrucciones antiguas de Angular/roles de PR.
2. Consultar el trigger regional correcto y el último build del SHA remoto.
3. Identificar el paso y error reales. No crear otro workflow, servicio, proyecto o cuenta como sustituto.
4. Si cambia backend, mantener full fail-closed hasta contar con evidencia real vigente; no tratar web-only como aceptación del backend nuevo.
5. Verificar la cadena completa antes de decir «desplegado» y registrar SHA, build, digest, revisiones, tráfico, pruebas y límites en una evidencia de release.

## Referencias

- Historial de transición: `planning/gcp/gcp08_cloud_build_transition_20261022.md`.
- Procedimiento SQL existente: `planning/gcp/cloudsql_migraciones_incrementales.md`.
- Pipeline versionado: `cloudbuild.yaml`, `cloudbuild-images.yaml`; imagen Next standalone: `frontend/Dockerfile`.
- Documentación Google Cloud: sustituciones de Cloud Build (`cloud.google.com/build/docs/configuring-builds/substitute-variable-values`) y promoción de tráfico Cloud Run (`cloud.google.com/run/docs/rollouts-rollbacks-traffic-migration`).

La evidencia de ejecución de esta reparación se registra aparte; este runbook no convierte una implementación propuesta o un push en un despliegue exitoso.
