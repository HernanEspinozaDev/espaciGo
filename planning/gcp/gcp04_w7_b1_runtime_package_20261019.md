# GCP-04-W7 B1 — paquete local de runtime

**Estado:** propuesta local reproducible y Terraform OFF. No existe release aprobado ni digest publicado. Este documento no autoriza invocar APIs, conceder IAM, crear recursos, ejecutar SQL/Job o publicar la imagen.

## Artefactos y límites

- `cmd/gcp04w7b1probe` es un proceso runtime separado del operador `cmd/gcp04w7b1`. No crea clientes de Cloud SQL Admin o Secret Manager ni llama inventarios `users.list`, `secret.get` o `versions.list`.
- El único permiso de control de plano runtime propuesto es `roles/cloudsql.client` condicionado a `projects/espacigo-testprod/instances/espacigo-testprod-pg`, además de `roles/secretmanager.secretAccessor` sobre el secreto de contraseña exacto. No se asignan `cloudsql.users.create`, `cloudsql.users.delete`, `cloudsql.users.list`, `secretmanager.secrets.get` ni `secretmanager.versions.list` al Job.
- El release se valida como `b1_probe`, con `Runs=1`, login `espacigo_gcp04_w7_bootstrap`, base `espacigo`, rol solicitado explícito `cloudsqlsuperuser`, versión numérica, ETags, expiración, ID de operación y SHA-256 SQL fijado en el paquete B1. `cloudsqlsuperuser` es el rol de base Cloud SQL solicitado; no se presenta como atributo PostgreSQL `SUPERUSER`.
- La imagen parte de Go `1.27.1-alpine3.24` por digest. El query ya está protegido por `CompileQuery` y tiene SHA-256 `9f9427086cb31d446edeb64e007e1f5e24999feee4cb245930964e1507ec3826`.
- Cloud Run Job monta el secreto como archivo root-owned `0400`. El contenedor corre como `root` explícitamente, valida que el mount sea archivo regular `0400`, lo copia con `os.CreateTemp` a `0600` y usa el `ReadCredentialFile` existente sin debilitar su contrato. La copia temporal se elimina al salir. La compatibilidad se basa en documentación de Cloud Run/Provider; no está probada en Cloud.
- `RunProbeOnce` conserva lease y journal `0600`, pero se ubican en `/tmp` efímero del task. No son idempotencia durable entre nuevas ejecuciones. La separación por nombre de Job/run ID y el procedimiento externo de una sola ejecución reducen errores operativos, pero no impiden una segunda llamada a `jobs execute`. Una solución durable y su IAM mínimo son gate bloqueante para afirmar “single run” enforced; no se concede Storage en este paquete.

## Release operacional pendiente

Mantener todos los campos sin aprobar hasta una liberación firmada/revisada fuera del repositorio. El dossier de fase deberá registrar conjuntamente:

1. Identificador de release y run, `Runs=1`, plazo UTC, nombre de Job único y decisión explícita `b1_probe`.
2. Proyecto `espacigo-testprod`, instancia/conexión `espacigo-testprod:southamerica-west1:espacigo-testprod-pg`, IP privada `10.189.0.3`, DB `espacigo`, login y rol solicitado exactos.
3. Versión numérica del secreto y ETags del shell/versión, sin revelar payload; operación de creación asociada y su estado reconciliado.
4. SHA-256 de query, binario, Dockerfile, imagen local/publicada por digest y plan Terraform; conservar por separado el digest OCI local y el remoto.
5. Nombre/ID de SA runtime, concesiones exactas, red/subred, una tarea, paralelismo/concurrencia 1, reintentos 0, deadline máximo 120 s, 1 vCPU/512 MiB.
6. Procedimiento de publicación y ejecución: son acciones futuras sujetas a autorización y revisión separadas. Confirmar que el monitor de Billing/cuotas y una alternativa de rollback están definidos antes de release.
7. Después de una ejecución futura: guardar ID de ejecución, fecha/actor, digest, salida saneada, resultado, costs/usage visibles, recurso eliminado y limitaciones; no hacer retry ante salida ambigua sin reconcile.

El `release_json` de Terraform es `sensitive` para reducir exposición casual, pero variables de entorno de Job y state de Terraform pueden conservar el contenido. No se debe introducir allí ningún password/token; el release JSON solo contiene metadata y ETags. El state remoto propuesto requiere bucket/prefix de alcance run (`gcp04-w7-b1/runs/<release-id>`), creado fuera de este root, con acceso restringido y retención definida. `backend.tf` queda vacío y no se inicializa contra GCS en este corte.

## Infraestructura propuesta

La raíz `infra/terraform/environments/testprod/gcp04-w7-b1-runtime` se protege con `enable_runtime=false` y `confirm_release=false`; image, release, secret version y run ID vacíos. No consulta APIs ni crea recursos en modo apagado. Al activarse con todos los parámetros revisados propone solo Service Account, dos binding IAM indicados y Cloud Run Job; usa red y subnet existentes con egress privado. No otorga acceso Artifact Registry al Job; Cloud Run service agent pull access y cualquier necesidad de decrypt/service-agent deben verificarse fuera de la identidad runtime y revisarse antes de plan habilitado.

El runtime no determina si la asignación SQL de `cloudsqlsuperuser` es aprobable; el paquete B1 mantiene el rol solicitado explícito. La creación de usuario/secreto sigue siendo una fase de operador distinta, con derechos y release propios, y no la ejecuta el Job.

## Coste marginal referencial (USD, consulta 2026-10-19)

- Cloud Run Jobs cobra instance-based durante toda la vida de la instancia, mínimo 1 minuto. Para 1 vCPU y 0.5 GiB, se estima **$0.0015 por corrida de 60 s** fuera de free tier (cálculo conservador Tier 2: 60 × `$0.000024/vCPU-s` + 30 × `$0.0000025/GiB-s`). Para 120 s: **$0.0030**. La región Santiago está en Tier 2; el free tier se agrega entre proyectos por billing account y usa precios Tier 1.
- Secret Manager: 1 operación de acceso en este flujo; hasta 10,000/mes gratis por billing account, luego $0.03 por 10,000. Versiones activas: seis gratis por mes/billing account, luego $0.000082192 por versión-hora. La retención de una versión bootstrap activa podría superar el costo de una corrida; esta propuesta no fija TTL ni destruye secreto.
- Artifact Registry: 0.5 GiB gratis por billing account, luego $0.000136986/GiB-hora. No se conoce el tamaño real del artefacto remoto porque no se publica aquí. Transferencia a servicios Google Cloud en la misma región se lista como gratuita; Internet egress no se espera para SQL por la ruta privada.
- VPC: no se estima cargo por transferencia de datos entre recursos Google Cloud en la misma región según tarifa general; cualquier interzona/egress o NAT que resulte necesario debe medirse/revisarse. No se creó IP/NAT/connector nuevo.
- No se incluyó el costo ya existente de Cloud SQL, operaciones API de fase de creación, Cloud Build, carga de Artifact Registry ni consumo agregado del billing account. La cifra es margen unitario, no presupuesto aprobado.

Referencias tarifarias oficiales consultadas el 2026-10-19: Cloud Run Jobs/tiers en [pricing](https://cloud.google.com/run/pricing), [Secret Manager pricing](https://cloud.google.com/secret-manager/pricing), [Artifact Registry pricing](https://cloud.google.com/artifact-registry/pricing), transferencia en [VPC pricing](https://cloud.google.com/vpc/pricing). El montaje root-owned de segunda generación y los requerimientos de identidad siguen [Cloud Run Job secrets](https://docs.cloud.google.com/run/docs/configuring/jobs/secrets?hl=en) y [Terraform google_cloud_run_v2_job](https://github.com/hashicorp/terraform-provider-google/blob/main/website/docs/r/cloud_run_v2_job.html.markdown).

## Verificación local de este corte

La imagen puede construirse solo localmente con `scripts/build-gcp04-w7-b1-runtime-local.sh`, que guarda tar y hashes bajo `.local/testing/gcp04-w7-b1/runtime-build` con permisos privados. No hacer `docker push`, `gcloud`, `terraform apply`, `jobs execute`, consultas remotas ni restore. El gate de revisión siguiente debe comprobar código/runtime, build reproducible, hash, Terraform default plan sin backend y validación con mocks si aplica, además de resolver el bloqueo de single-run durable. Todo resultado de mock o build local es evidencia local y no Cloud.

Artefacto local generado en este checkout: Docker image config ID `sha256:adb4cbaea2490a3f27825179278aa4661a09958c37f06842ff05de512185f74a`; tar `gcp04-w7-b1-probe.tar` SHA-256 `78e6bfe56a366b72b592e9598af65a39251f5b34325aac8be76379f5092a1328`; guardado en `.local/testing/gcp04-w7-b1/runtime-build` con modo 0600. Es identificador/hash del artefacto local, no digest remoto publicable ni aprobación de imagen.
