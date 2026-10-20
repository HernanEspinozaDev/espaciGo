# Propuesta: CI/CD de GitHub Actions a Cloud Run para testprod

**Estado:** propuesta local para revisión. No se añadió un workflow de despliegue, no se publicaron imágenes por esta propuesta y no se autenticó ni desplegó desde GitHub Actions.

## Estado verificado en el repositorio

- `.github/workflows/frontend-quality.yml` ejecuta calidad Angular en PR y `main` para las rutas indicadas.
- `.github/workflows/g03o-v64-transfer.yml` ejecuta pruebas del ejecutor de transferencia en PR; no despliega.
- No hay workflow que construya imágenes de aplicación, publique digest o actualice servicios Cloud Run. Terraform describe imágenes fijadas por digest y dice que CI es responsable de construir/subirlas.
- Terraform ya declara un Workload Identity Pool/Provider restringido al repositorio configurado y a `refs/heads/main`, una cuenta `deployer`, permiso `roles/run.developer` a nivel de proyecto, `roles/artifactregistry.writer` limitado al repositorio y `roles/iam.serviceAccountUser` sobre las cuentas runtime API, web público, web privado y migrator. Esto permite autenticar una ruta de CI si la infraestructura está aplicada y vigente; no prueba que el workflow exista, que la federación esté funcional o que el deployer tenga alcance mínimo.
- Cloud Build API se reportó deshabilitada en la revisión read-only anterior. Este documento propone GitHub Actions, no habilitar Cloud Build.

## Flujo mínimo propuesto

1. **PR / push a ramas:** ejecutar lint, checks de arquitectura/API, pruebas backend relevantes, pruebas frontend, build de contenedores y escaneo de dependencias. No conceder `id-token: write`, no autenticarse en Google Cloud, no publicar imágenes y no desplegar desde PRs.
2. **Push a `main`:** después de checks requeridos, construir imágenes API y Angular SSR desde el mismo SHA, publicar en `southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod`, resolver y registrar digest OCI inmutable y provenance de commit/herramientas. Acciones de terceros fijadas por SHA.
3. **Autenticación:** GitHub OIDC→WIF, sin llave JSON. `id-token: write` se limita al job de deploy; `contents: read` al workflow. Conservar la restricción de repositorio/ref, y considerar mapear/restringir `repository_id`, `workflow_ref` y environment para evitar depender solo del nombre mutable del repo.
4. **Despliegue:** actualizar servicios Cloud Run declarados (`espacigo-test-api`, `espacigo-test-public`, `espacigo-test-private`) usando referencias `@sha256`, no tags mutables. Ejecutar health checks públicos/privados aprobados sin exponer tokens. El cambio de imágenes se mantiene coherente con Terraform; no permitir drift permanente entre la fuente declarativa y el estado desplegado.
5. **Evidencia:** publicar en el resumen del workflow commit, digest(s), servicios/revisiones resultantes, URLs/checks y resultado. No imprimir variables secretas, DSN, tokens, cuerpos de solicitudes ni contenido de Secret Manager.

## IAM y controles pendientes antes de implementar

El deployer existente tiene permisos a nivel de proyecto para Run y puede adjuntar cuatro cuentas runtime. La propuesta debe reemplazar ese alcance por permisos de actualización/lectura solo sobre los tres servicios web/API que se desplieguen y `actAs` solo para sus cuentas runtime. No permitir `actAs` de migrator al pipeline de aplicación. `roles/artifactregistry.writer` permanece ligado únicamente al repositorio de contenedores; no se requiere permiso para borrar artefactos. Mantener el despliegue sujeto a revisión de identidad/cuenta y plan Terraform antes de aplicar cualquier reducción IAM.

Antes de activar un workflow hay que comprobar que la condición WIF realmente restringe al repositorio y a `main`, auditar el alcance de IAM vigente, decidir si se mantendrán tres servicios en Terraform como fuente declarativa o se migrará el control de imagen al pipeline, y añadir pruebas de configuración/política. No dar por sentado que un archivo YAML por sí solo despliega con éxito.

## Decisiones de producto/operación requeridas

- Confirmar si cada merge a `main` debe desplegar automáticamente a testprod una vez verdes los checks, o si el primer despliegue debe requerir aprobación manual de un Environment de GitHub.
- Confirmar si deploy se permite solo desde `main` o desde tags de release; la WIF actual solo permite `main`.
- Acordar estrategia de rollback por digest y disponibilidad del servicio API durante actualización. Los cambios de schema y las migraciones de datos permanecen en releases aparte; el pipeline de aplicación no debe ejecutar migraciones de esquema o transferencia G03O automáticamente.
- Aprobar lista de checks obligatorios por imagen y plan de observabilidad del release. El estado de revisión de PR no debe confundirse con la aceptación final del producto.

## Límites

Este documento no crea/activa GitHub Environments, secrets, WIF, service accounts o bindings; no aplica Terraform; no habilita APIs; no publica imágenes; no actualiza Cloud Run y no ejecuta migraciones. Implementar CI/CD necesita un PR separado con el workflow y una revisión del IAM efectivo. El primer despliegue requiere su propio plan/release y verificación del proyecto `espacigo-testprod`.

## Referencias oficiales consultadas el 2026-10-20

- [Workload Identity Federation con pipelines de despliegue](https://docs.cloud.google.com/iam/docs/workload-identity-federation-with-deployment-pipelines): claims y condiciones para restringir GitHub Actions por repo/ref.
- [Roles de IAM de Cloud Run](https://docs.cloud.google.com/run/docs/reference/iam/roles): permisos para deploy, lectura de Artifact Registry y `actAs` de la identidad de servicio.
- [Control de acceso de Artifact Registry](https://docs.cloud.google.com/artifact-registry/docs/access-control): `roles/artifactregistry.writer` puede limitarse al repositorio.
