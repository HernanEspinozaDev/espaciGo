# Plan GCP-07: publicar y desplegar frontend y backend de `main` en Cloud Run

**Estado:** propuesta para revisión; no implementada ni autorizada para operar Cloud.
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

- El job de publicación/deploy declara `environment: testprod`, restringido a `main`, y se autentica con OIDC→WIF. `id-token: write` queda solo en ese job; no usar llaves JSON ni claves de cuenta de servicio.
- Ejecutar `terraform plan` desde `infra/terraform/environments/testprod/gcp`, con el backend GCS ya usado por ese root y los nuevos `api_image`/`frontend_image` por digest. Guardar el plan generado como artefacto de la corrida y aplicar exactamente ese plan, nunca volver a resolver tags entre plan y apply.
- Antes del apply, evaluar el JSON del plan con una política de allowlist. Aceptar únicamente actualizaciones en sitio del campo de imagen de `google_cloud_run_v2_service.api` y/o `google_cloud_run_v2_service.web["public"|"private"]`, según el componente construido. Rechazar creación, eliminación, reemplazo o cualquier cambio de IAM, tráfico, URLs, ingress, Secret Manager, variables de entorno, SQL, red, Jobs u otros recursos. No usar `-target` para esconder drift.
- Si el plan detecta drift o requiere otros cambios, detener el release y preparar un PR/plan de infraestructura por separado. No aplicar automáticamente un plan amplio solo porque se originó desde `main`.
- Mantener `api_image` y `frontend_image` actualizados en el mismo state Terraform que administra esos servicios. No usar `gcloud run deploy` como camino de rutina, porque dejaría un estado remoto distinto a la configuración declarativa.

### 4. Verificar y registrar

- Después del apply, releer los tres servicios y confirmar revisión nueva `Ready=True`, imagen digest esperada, asignación de tráfico prevista y cero fallos de operación.
- Ejecutar readiness/health checks no mutantes para las rutas públicas aprobadas. La superficie privada requiere una estrategia de prueba autenticada aprobada; el job no debe guardar ni emitir tokens, cookies o cabeceras. Si no existe esa estrategia, marcar la comprobación privada como pendiente y no simular éxito.
- Publicar SHA de `main`, digest por imagen, servicios/revisiones, checks ejecutados, resultado y límites. No registrar DSN, payloads de Secret Manager, credenciales ni datos de usuario.
- Si se detecta fallo, detener pasos posteriores y conservar la revisión estable anterior. La reversa consiste en aplicar mediante un plan revisado los digests anteriores; no revierte migraciones ni cambios de datos.

## Esqueleto de workflows

Se propone mantener dos responsabilidades separadas para que un PR nunca pueda acceder a Cloud:

1. `application-quality.yml`: `pull_request` hacia `main`; checks Go y Angular requeridos, sin WIF.
2. `application-release.yml`: `push` a `main`; build/publicación, job Terraform con WIF, control de diff, apply del plan guardado y verificaciones.

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

En el repositorio hay checks Angular para PR y `main`, y workflows aislados GCP-06/G03O; no existe hoy CI general de backend ni workflow de publicación/deploy de aplicación. El Terraform raíz declara WIF, deployer, Artifact Registry e imágenes por digest, pero esta lectura local no comprueba sus recursos aplicados, permisos efectivos, protecciones de GitHub, state actual, cuota/costo ni que el deploy funcione.

Esta propuesta no añade workflows, no cambia Terraform, no modifica GitHub, no inicia sesión en GCP, no publica imágenes y no actualiza Cloud Run. Es un plan de diseño para la siguiente fase autorizada, no evidencia de que el despliegue esté funcionando.

## Referencias oficiales consultadas el 2026-10-20

- [GitHub Actions: eventos que disparan workflows](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow) (`push` puede limitarse a `main`; los environments pueden añadir una aprobación de job).
- [GitHub: ramas protegidas](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches) (revisión y checks requeridos para integrar a una rama protegida).
- [Google Cloud: Workload Identity Federation para pipelines](https://docs.cloud.google.com/iam/docs/workload-identity-federation-with-deployment-pipelines) (OIDC de GitHub y restricción por claims del repositorio/ref).
- [Google Cloud: roles de Cloud Run](https://docs.cloud.google.com/run/docs/reference/iam/roles) (los roles pueden otorgarse a nivel de servicio; `roles/run.developer` también permite crear/eliminar/ejecutar Jobs).
- [Google Cloud: despliegue de imágenes a Cloud Run](https://docs.cloud.google.com/run/docs/deploying) (las revisiones son inmutables y las imágenes con tags se resuelven a digest).
- [Google Cloud: permisos de Artifact Registry](https://docs.cloud.google.com/artifact-registry/docs/access-control) (el acceso puede limitarse al repositorio).
