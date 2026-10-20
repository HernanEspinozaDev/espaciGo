# Plan GCP-07: publicar y desplegar frontend y backend de `main` en Cloud Run

**Estado:** GCP-07A está integrado y `main` conserva la protección con revisión independiente y checks obligatorios. GCP-07B se fusionó mediante PR #466 (HEAD revisado `dca7eed735acdbd307a0cbcea8aae0b653b73d0d`, merge `76a8ca731513010d8d40895059587122ffe40c57`); los cambios Terraform/IAM siguen sin aplicarse en Cloud. GCP-07C está en preparación local: las imágenes generan y verifican SBOM/provenance localmente; la publicación al Artifact Registry queda desactivada hasta superar el gate y recibir autorización explícita. El GitHub Environment `testprod` aún no está creado/configurado. No se publicaron imágenes ni se desplegó. GCP-07D–07E siguen como propuesta.
**Alcance:** automatizar la entrega de cambios de aplicación ya integrados y aprobados en `main` al entorno de ensayo `espacigo-testprod`.
**Región:** `southamerica-west1`.
**No incluye:** cambios a bases de datos/migraciones, transferencias G03O, Jobs de ensayo, producción ni aceptación del producto.

## Decisión propuesta

Usar **GitHub Actions** para ejecutar CI en los pull requests y para iniciar el release tras cada push a `main`. El evento `push` de GitHub Actions sucede después de que el PR se aprueba y se integra; la aprobación del PR no despliega por sí sola. Por tanto:

1. Todo cambio entra mediante PR a `main`, con revisión requerida y checks obligatorios.
2. Al integrarse el PR, Actions recibe `push` en `main`, construye y publica las imágenes que cambiaron.
3. El deploy usa las imágenes por digest y actualiza los servicios existentes mediante el root Terraform canónico de `infra/terraform/environments/testprod/gcp`.
4. Un control automático del plan detiene el release si Terraform propone algo fuera de actualizar la imagen de los servicios esperados.

No habilitar deploy desde PR, `pull_request_target`, ramas de trabajo o un push directo. La regla `main` debe impedir push directo y exigir aprobación; de ese modo el evento de deploy representa un cambio revisado e integrado. Una aprobación adicional en cada deploy sería opcional y no se recomienda para el comportamiento continuo solicitado; la autorización humana inicial para activar el pipeline y sus permisos sigue siendo un gate aparte.

## Inventario de aplicación y nombres

Las imágenes de backend y frontend son artefactos distintos. Cloud Run mantiene tres servicios porque la web tiene una superficie pública y otra privada:

| Componente | Imagen Artifact Registry | Servicio Cloud Run existente | Cambios que lo actualizan |
| --- | --- | --- | --- |
| API / backend Go | `espacigo-api` | `espacigo-test-api` | cambios Go/API o contrato compartido |
| Angular SSR / frontend | `espacigo-web` | `espacigo-test-public` | cambios de frontend o contrato compartido |
| Angular SSR / frontend | el mismo digest `espacigo-web` | `espacigo-test-private` | cambios de frontend o contrato compartido |

No se propone renombrar servicios a `backend`/`frontend`: los nombres actuales identifican que pertenecen al entorno test y distinguen las dos superficies web. La fuente actual confirma que Terraform declara los servicios y recibe `api_image` y `frontend_image` como referencias por SHA-256. La lista definitiva de destinos se validará contra estado remoto y configuración efectiva antes de activar el release; no se crean servicios nuevos.

## Flujo propuesto

### 0. Preparación única antes de activar deploy

- Confirmar en GitHub que `main` requiere PR, al menos una aprobación independiente, checks obligatorios, aprobación del último push revisable y que se descartan aprobaciones obsoletas al cambiar el PR. Deshabilitar push/force-push directo y bypass no justificado.
- Completar CI de backend: formato, `go vet`, pruebas Go seleccionadas para cambios de aplicación y `go test ./...` según el costo y la compatibilidad real del runner. Mantener las pruebas de integración que requieren PostgreSQL/PostGIS en un servicio de ensayo efímero. No usar GCP ni datos persistentes en CI.
- Mantener CI de frontend: lint, verificación API/arquitectura, pruebas, build y auditoría ya presentes. Revisar filtros por ruta para que cada check obligatorio siempre publique un resultado en PR y no quede pendiente por un workflow omitido.
- Pin de acciones de GitHub a SHA completo, como ya se hace en los workflows existentes. Fijar también toolchains, dependencias y las imágenes base que hoy solo tengan tag mutable antes de confiar en reproducibilidad.
- Confirmar que el state GCS del root Terraform canónico tiene el prefijo correcto, locking disponible y una copia/restauración operativa. Actions usará ese mismo state; no crear un segundo state para los mismos servicios.
- Revisar la configuración WIF/IAM real en GCP. La configuración versionada restringe actualmente GitHub al repositorio configurado y `refs/heads/main`, pero no demuestra que el proveedor esté aplicado ni que la identidad efectiva sea mínima. Endurecer el proveedor para limitar también por ID inmutable del repositorio y por el workflow de despliegue si los claims disponibles lo permiten.
- Reducir permisos del deployer antes de activarlo. El Terraform versionado actualmente da `roles/run.developer` a nivel de proyecto y `actAs` sobre las cuentas API, web pública, web privada y migrator. Diseñar un principal de despliegue que pueda actualizar/leer solo los tres servicios seleccionados, subir imágenes solo al repositorio de aplicación y actuar únicamente como las cuentas API/web. Quitar `actAs` de migrator. No conceder permisos para ejecutar Jobs, borrar imágenes, leer secretos ni cambiar IAM desde el pipeline.
- Identificar la configuración no secreta que requiere el root Terraform y proporcionarla desde variables de entorno GitHub protegidas o archivos versionados aprobados. Los valores secretos seguirán en Secret Manager; el workflow no leerá su contenido ni imprimirá secretos.

**Gate de activación:** PR separado de implementación aprobado, plan Terraform exacto revisado, WIF/IAM efectivos comprobados y autorización explícita para el primer cambio Cloud. Hasta superar el gate, los workflows pueden validar y construir localmente en CI, pero no federarse ni publicar.

### 1. PR: validación sin credenciales Cloud

- Ejecutar calidad y pruebas para backend y frontend en todo PR a `main`.
- Construir los targets de las imágenes en modo CI para detectar fallos de Docker, sin iniciar sesión en Google, publicar, obtener token OIDC ni desplegar.
- No exponer credenciales a código de PR. Los permisos del workflow de PR serán de solo lectura (`contents: read`); `id-token: write` no estará disponible.
- Si un cambio afecta contrato Go/OpenAPI/Angular, ejecutar checks de ambos componentes y preparar el release conjunto.

### 2. Push a `main`: construir y publicar

- El workflow de release se dispara con `push.branches: [main]` después de merge; usa el SHA exacto del commit que activó la corrida.
- Ejecutar una sola corrida por SHA de `main`, con concurrencia serializada y sin cancelar un deploy en curso. No aceptar tags mutables como fuente de despliegue.
- Construir `Dockerfile --target api` para Go y `frontend/Dockerfile` para Angular SSR. En cambios compartidos de contrato, construir ambos. En cambios solo de backend o frontend, publicar y desplegar únicamente el componente afectado; una política alternativa de rebuild completo requerirá decisión en el PR de implementación.
- Publicar en el Artifact Registry existente `southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod` con etiquetas informativas ligadas al SHA completo y capturar el digest devuelto por el registro. Terraform y Cloud Run recibirán solo `@sha256:<64 hex>`.
- Adjuntar al resumen de la corrida: SHA fuente, digest publicado, target Docker, resultado de pruebas y vínculo al release. Generar SBOM/provenance y escaneo de vulnerabilidades antes de habilitar el deploy automático.

### 3. Deploy: Terraform como fuente declarativa

- Antes de activar la publicación, crear y configurar el GitHub Environment `testprod` con restricción de despliegue a `main` y las reglas de protección decididas para este entorno; verificar su configuración efectiva. Hoy ese environment no existe/configurado. El job de publicación/deploy declara `environment: testprod` y se autentica con OIDC→WIF solo después de superar ese gate. `id-token: write` queda solo en ese job; no usar llaves JSON ni claves de cuenta de servicio.
- Ejecutar `terraform plan` desde `infra/terraform/environments/testprod/gcp`, con el backend GCS ya usado por ese root y los nuevos `api_image`/`frontend_image` por digest. Mantener el plan binario y cualquier JSON derivado únicamente dentro del job confiable de deploy, en el workspace efímero del runner, con acceso restringido (archivo `0600` cuando aplique). Aplicar exactamente ese plan dentro del mismo job; nunca volver a resolver tags entre plan y apply.
- No subir el plan binario ni JSON completo a Actions artifacts, caché, outputs, comentarios del PR, logs u otro almacenamiento descargable: los planes pueden contener valores sensibles en claro incluso cuando Terraform los marca como sensibles. No imprimirlos con `terraform show` ni volcar el JSON a logs. Limpiar los archivos temporales al terminar y depender del descarte del runner efímero.
- Antes del apply, evaluar localmente el JSON del plan con una política de allowlist. Aceptar únicamente actualizaciones en sitio del campo de imagen de `google_cloud_run_v2_service.api` y/o `google_cloud_run_v2_service.web["public"|"private"]`, según el componente construido. Rechazar creación, eliminación, reemplazo o cualquier cambio de IAM, tráfico, URLs, ingress, Secret Manager, variables de entorno, SQL, red, Jobs u otros recursos. No usar `-target` para esconder drift.
- El resumen publicable de la corrida contendrá solo datos de allowlist explícita: SHA fuente, digest de imagen, resultado/código de salida y conteos de acciones permitidas. No incluir valores del plan, variables, nombres de recursos no aprobados ni fragmentos del JSON. Si se necesita revisión humana de cambios fuera de la allowlist, detener el deploy y abrir un flujo de infraestructura separado; no distribuir el plan sensible como artefacto.
- Si el plan detecta drift o requiere otros cambios, detener el release y preparar un PR/plan de infraestructura por separado. No aplicar automáticamente un plan amplio solo porque se originó desde `main`.
- Mantener `api_image` y `frontend_image` actualizados en el mismo state Terraform que administra esos servicios. No usar `gcloud run deploy` como camino de rutina, porque dejaría un estado remoto distinto a la configuración declarativa.

### 4. Verificar y registrar

- Después del apply, releer los tres servicios y confirmar revisión nueva `Ready=True`, imagen digest esperada, asignación de tráfico prevista y cero fallos de operación.
- Ejecutar readiness/health checks no mutantes para las rutas públicas aprobadas. La superficie privada requiere una estrategia de prueba autenticada aprobada; el job no debe guardar ni emitir tokens, cookies o cabeceras. Si no existe esa estrategia, marcar la comprobación privada como pendiente y no simular éxito.
- Publicar SHA de `main`, digest por imagen, servicios/revisiones, checks ejecutados, resultado y límites. No registrar DSN, payloads de Secret Manager, credenciales ni datos de usuario.
- Si se detecta fallo, detener pasos posteriores y conservar la revisión estable anterior. La reversa consiste en aplicar mediante un plan revisado los digests anteriores; no revierte migraciones ni cambios de datos.

## Esqueleto de workflows

Se propone mantener dos responsabilidades separadas para que un PR nunca pueda acceder a Cloud:

1. `application-quality.yml`: `pull_request` hacia `main`; checks Go/Angular y build local con verificación de SBOM/provenance, sin WIF.
2. `application-release.yml`: `push` a `main`; build/publicación por digest detrás del gate de activación. El deploy Terraform con WIF se añade en GCP-07D; ningún workflow de PR recibe `id-token: write`.

El release puede usar jobs `build-api`, `build-web` y `deploy`, conectados por `needs`. La concurrencia será `testprod-cloud-run` y conservará la corrida activa. Las acciones de terceros se fijan por SHA completo y se limita `permissions` por job.

## Alcance de la cuenta y límites del cambio

El pipeline solo cubre la actualización de código de la app. No ejecuta migraciones, Jobs Cloud Run, transferencia de datos, cambios de IAM, cambios de tráfico manual, operaciones de SQL ni `terraform destroy`. Los cambios de esquema permanecen en su release revisado propio; la API solo se actualiza cuando la migración ya tiene su autorización/evidencia correspondiente. No desplegar datos reales, producción o capacidades aún no implementadas.

## Secuencia de cortes

| Fase | Entregable | Condición para avanzar |
| --- | --- | --- |
| GCP-07A | CI obligatorio de backend + frontend en PR y política de `main` documentada/verificada | CI verde y protección de `main` activa |
| GCP-07B | Baseline state/config, WIF endurecido y deployer con mínimo privilegio | PR aprobado y plan IAM exacto revisado |
| GCP-07C | Build/push de imágenes por SHA, digest, SBOM/provenance | publicación verificada en Artifact Registry sin deploy |
| GCP-07D | Deploy automatizado mediante plan Terraform acotado a imágenes | gate inicial autorizado y plan exacto sin cambios fuera de allowlist |
| GCP-07E | Smoke checks, historial de revisiones y rollback ensayado con digest previo | evidencia reproducible y límites documentados |

Los sucesores no se activan por anticipado: cada fase se revisa antes de iniciar la siguiente. El primer deploy necesita una autorización separada para el plan exacto. Tras esa habilitación, los merges aprobados a `main` pueden desplegarse automáticamente mientras el plan siga dentro de la allowlist.

## Estado y límites de esta propuesta

El repositorio tiene CI de backend, frontend e imágenes, además de workflows aislados GCP-06/G03O; GCP-07C prepara publicación con WIF protegida, aún no activada. El Terraform raíz declara WIF, deployer, Artifact Registry e imágenes por digest, pero una lectura local no confirma el estado aplicado, la cuota/costo ni que el deploy funcione.

La lectura de estado y recursos descrita aquí no es evidencia de que el despliegue funcione. GCP-07B preparó cambios Terraform, pero no aplicó IAM en Cloud. GCP-07C agrega generación/verificación local de attestations y un flujo de publicación protegido; no publica imágenes ni actualiza Cloud Run.

### GCP-07B — baseline observado y límites del plan

La lectura de solo consulta del proyecto `espacigo-testprod` confirmó el state GCS canónico del root, versión Terraform 1.16.6, serial 25, los tres servicios Cloud Run esperados, el repositorio Artifact Registry existente y los recursos WIF/deployer. No se cambió ni se copió el state. El proveedor aplicado aceptaba nombre del repositorio y `refs/heads/main`; el principal deployer tenía `roles/run.developer` a nivel de proyecto, `roles/artifactregistry.writer` limitado al repositorio existente y `roles/iam.serviceAccountUser` para API, web pública, web privada y migrator. No se observaron grants directos del deployer sobre los servicios. PR #466 integró el endurecimiento versionado de WIF/IAM, pero no aplicó esos cambios a Cloud.

El ID numérico del repositorio y el ID del propietario se verificaron con la API pública de GitHub. El Terraform de 07B propone mapear esos IDs más `workflow_ref` y `environment`, exigir `application-release.yml` en `main` con environment `testprod`, retirar el rol de proyecto de Cloud Run, conceder un rol custom con solo `run.services.get`/`run.services.update` sobre los servicios de API y web existentes, retirar `actAs` del migrator y conceder acceso de objetos al deployer mediante un managed folder `espacigo/testprod/gcp/`. El bucket contiene 34 objetos y 32 están fuera de ese prefijo, por lo que el grant no se hace al bucket. UBLA y versionado están activos. El writer de Artifact Registry sigue limitado al repositorio.

El root canónico también administra infraestructura ajena al cambio de imagen. Un plan Terraform completo requiere refrescar su inventario y leer el state; por eso no basta con conceder al deployer permisos de Cloud Run sobre esos tres recursos. Antes de 07D se debe demostrar el acceso de lectura necesario para planificar el root y el acceso al objeto/state-lock sin dar permisos de mutación adicionales ni exponer valores de state. El state puede contener datos sensibles y debe tratarse como tal. El script `scripts/gcp07-verify-state-backend.sh` comprueba `terraform init` y el listado del workspace predeterminado sin mostrar contenido ni nombres del state; se validó contra el backend remoto con la identidad local existente, no con el deployer. La prueba con el deployer queda pendiente de aplicar el grant tras revisar y autorizar el plan exacto. No se ha generado ni aplicado un plan remoto para este corte; tampoco se habilitó OIDC, publicación, deploy ni una identidad de planificación.

La aplicación del plan 07B modificaría IAM/WIF en Cloud y requiere revisión del plan exacto y autorización explícita previa. El PR de código, por sí solo, no ejecuta esa operación.

### GCP-07C — imágenes, digest y supply chain

`application-quality.yml` construye ambos targets en modo local con BuildKit y genera attestations SPDX SBOM y provenance mínima; `scripts/verify_gcp07_attestations.py` verifica que ambas estén presentes en cada archivo OCI temporal sin mostrar su contenido. Los archivos quedan solo en el runner y se eliminan al finalizar. Los PR no tienen permiso `id-token: write`, no inician sesión en GCP y no publican imágenes. El tag de Go está fijado al digest del índice multi-plataforma; las otras bases ya tenían digest.

`application-release.yml` atiende pushes a `main`, clasifica cambios API/web y publica el target afectado con un tag informativo `sha-<commit>` únicamente cuando la variable GitHub `GCP07C_PUBLISH_ENABLED` vale `true`. El job declara `environment: testprod`, pero al 2026-10-20 la lista de GitHub Environments del repositorio está vacía; por ello todavía no existe ese gate ni aplican protecciones de entorno. Antes de configurar/activar la variable, se debe crear `testprod`, restringirlo a `main` y configurar las reglas de protección acordadas. No se crean ni modifican settings en este corte. Tras configurar ese environment, la publicación usaría WIF de 07B, permisos de escritura limitados al repositorio Artifact Registry y attestations SBOM/provenance. El resumen contiene SHA fuente y digest; este workflow no despliega.

La variable de publicación no está configurada. El Environment `testprod` tampoco está creado/configurado, por lo que no hay protecciones de environment activas. No se hizo push de imágenes, no se verificó un digest remoto y WIF/IAM de 07B no se ha aplicado. Antes de habilitar el flag, crear y proteger el environment, revisar el plan IAM exacto de 07B y recibir autorización explícita para aplicar ese plan y publicar; luego se comprobarán los digests y attestations en Artifact Registry. GCP-07D no debe comenzar hasta que la publicación 07C se verifique y 07C se fusione.

## Referencias oficiales consultadas el 2026-10-20

- [GitHub Actions: eventos que disparan workflows](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow) (`push` puede limitarse a `main`; los environments pueden añadir una aprobación de job).
- [GitHub: ramas protegidas](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches) (revisión y checks requeridos para integrar a una rama protegida).
- [Google Cloud: Workload Identity Federation para pipelines](https://docs.cloud.google.com/iam/docs/workload-identity-federation-with-deployment-pipelines) (OIDC de GitHub y restricción por claims del repositorio/ref).
- [Google Cloud: roles de Cloud Run](https://docs.cloud.google.com/run/docs/reference/iam/roles) (los roles pueden otorgarse a nivel de servicio; `roles/run.developer` también permite crear/eliminar/ejecutar Jobs).
- [Google Cloud: despliegue de imágenes a Cloud Run](https://docs.cloud.google.com/run/docs/deploying) (las revisiones son inmutables y las imágenes con tags se resuelven a digest).
- [Google Cloud: permisos de Artifact Registry](https://docs.cloud.google.com/artifact-registry/docs/access-control) (el acceso puede limitarse al repositorio).
- [Docker: Build attestations](https://docs.docker.com/build/metadata/attestations/) (BuildKit adjunta SBOM y provenance a imágenes OCI).
- [Docker: SBOM attestations](https://docs.docker.com/build/metadata/attestations/sbom/) (BuildKit genera SBOM SPDX y permite fijar el scanner generator).
