# GCP-06 Phase B: preparación de contrato runtime y revisión

**Estado:** preparación local para revisión. No habilita ni ejecuta el cambio de runtime.
**Base:** merge de PR #460, `aaea4fb8c82da78324a36a80028c40f61d88922c`.
**Alcance:** API de ensayo, base candidata `espacigo_gcp06_candidate_20261020-gcp06a` y conservación de los dos frontends actuales. Sin cambios en Cloud SQL, Secret Manager, Cloud Run ni tráfico.

## Contrato preparado

- El API conserva `DATABASE_USER=espacigo_api_login` y `DATABASE_SET_ROLE=espacigo_runtime`, con conexión a la instancia y base candidatas fijadas por `20261020-gcp06a`. La identidad login y el rol runtime deben existir y tener privilegios mínimos verificados antes de que una revisión API pueda conectarse.
- El cambio de destino no cambia imagen/digest del API, imágenes de los frontends, origen API ni configuración de los servicios web. Se añadieron aserciones al test de arquitectura Terraform para fijar estos invariantes.
- El inspector SQL de solo lectura amplía su inventario de catálogos para mostrar propietario de base/esquema, existencia y privilegios efectivos de login/rol en base y esquema, acceso por relación y secuencia, ejecución por rutina y ACL por defecto agregadas. Limita sus consultas a `public`, usa roles/objetos fijos y nombres de propietarios/grantees allowlistados o resumidos como `other`; no lee filas de aplicación ni cargas de credenciales.
- El informe no decide qué grants deben aplicarse. Aún hace falta observar el catálogo de la candidata con una identidad preexistente autorizada y revisar los resultados frente al contrato de privilegio mínimo. El inventario nuevo tampoco concede permisos ni ejecuta migraciones.

## Plan Terraform de revisión

El plan remoto se generó con refresh, backend GCS de testprod y sin apply. Su archivo binario y JSON quedan en `/tmp/gcp06-phaseb-plan-20261020/` con permisos privados; SHA-256 del plan dirigido: `492a7b70295bdc4e7b5f415d61ae6d8361e9fc1bdb8cad45d05eef8b5591f61d`.

El plan dirigido contiene exactamente estas acciones:

| Recurso | Acción |
| --- | --- |
| `google_cloud_run_v2_service.api[0]` | update |
| `google_secret_manager_secret.runtime["database-password-gcp06"]` | create |
| `google_secret_manager_secret_iam_member.api_secret_access["database-password-gcp06"]` | create |

La referencia candidata reportada por Terraform es el secreto `espacigo-test-database-password-gcp06`, proyecto `espacigo-testprod`; el plan no lee ni contiene su carga. La variable numérica de versión es prospectiva y no acredita una versión activa. El plan requiere targeting explícito de esos tres recursos porque el plan completo también propone destruir el contenedor `gcp06-sql-inspector-credential` que permanece en el estado remoto tras el cierre del observador. Esa eliminación ajena al cambio API quedó fuera del plan dirigido y debe resolverse en una revisión separada; no se debe aplicar el plan completo ni inferir que esa limpieza fue aprobada.

Este plan dirigido es evidencia de wiring, no es aplicable: el secreto runtime candidato no está disponible con una versión confirmada y no se verificó el login `espacigo_api_login`/rol `espacigo_runtime` ni sus ACL. También falta la observación SQL de ownership, grants, secuencias, rutinas y ACL por defecto. La configuración no debe apuntarse al candidato hasta cerrar esas brechas y regenerar/revisar un plan exacto con versión real del secreto y estado actualizado.

## Verificaciones y límites

- `go test ./internal/gcp06sqlinspect ./cmd/gcp06sqlinspect`: pasa.
- `terraform fmt -check infra/terraform/environments/testprod/gcp/architecture.tftest.hcl`: pasa.
- `terraform -chdir=infra/terraform/environments/testprod/gcp test -no-color`: 14/14 pasan.
- `git diff --check`: pasa.
- La prueba de consultas contra PostgreSQL 18 desechable no pudo ejecutarse: el runtime local Docker falló al iniciar el proceso del contenedor. La prueba de integración quedó opt-in mediante `GCP06_INSPECTOR_SQL_TEST_DSN` y se omite si no se configura un PostgreSQL 18 aislado. No se conectó a Cloud SQL.
- No se aplicó Terraform, DDL ni DML; no se rotó ninguna contraseña, creó secreto/rol/login, cambió servicio, publicó imagen ni ejecutó E2E. No se consultaron filas ni se alteró una cuenta.

## Siguiente gate

Revisar este HEAD y el inventario ampliado; obtener evidencia read-only de catálogos de la candidata con un canal e identidad aprobados; definir los grants mínimos respaldados por esos resultados; confirmar el login runtime y una versión real de Secret Manager por el procedimiento autorizado; resolver aparte el recurso observer retirado del estado; regenerar el plan y revisar su SHA y alcance. Solo una autorización posterior, explícita y basada en ese plan exacto, puede habilitar la aplicación del cambio Cloud.
