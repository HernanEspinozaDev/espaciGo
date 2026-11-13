# Gates pendientes de publicación — observación del 2026-11-13

Candidato: `codex/marketplace/chore/spaces-main-preparation`, HEAD base `647cd491985d619370990d3f226b5df61e36b42b`, origin/main releído `5830c9ce2dc61d4df13a4139da984b4cafd6d9ba`.

## Metadata cloud consultada sin ejecutar Jobs

- Job de plan histórico `espacigo-db-p-2dc002a95831471780646432b98d3e01` existe, UID `e6efbb5b-d615-42d0-8742-adb68f817967`, Ready. Argumentos `-dir /app/db/migrations -plan-only`; runtime `espacigo-db-migrator@espacigo-testprod.iam.gserviceaccount.com`, SET ROLE `espacigo_schema_owner`. Digest migrador `sha256:d9787d1f56d612a8e780f892f037f55e9c7145136ecf5ff752002f7d2a8e3052`. No se reejecutó ni modificó.
- Ejecución aplicadora `espacigo-db-r-2dc002a95831471780646432b98d3e01-v62zb` permanece Completed=False, NonZeroExitCode, failedCount1, UID `3c86092e-476c-4fd4-9377-a57d87acdace`. No hubo nueva ejecución ni cancelación.
- Trigger `548b8eff-e489-4307-a77f-455b56aa4e57` está habilitado, branch `^main$`, `cloudbuild.yaml`, identidad `espacigo-cloud-build-deployer`. Un push main activaría el circuito full; no se publicó para forzar el paso.

## IAM directo actual observado

Para `serviceAccount:espacigo-release-job-builder@espacigo-testprod.iam.gserviceaccount.com`:

- En proyecto: `projects/espacigo-testprod/roles/espacigoReleaseJobProvisioner` y `roles/logging.logWriter`.
- En bucket Build: `roles/storage.objectViewer` condicionado solo a `source/` y `releases/`, título `flujo_release_source_requests`.
- No se encontró en esos bindings directos una excepción de lectura para el lock raíz o `recovery-inspections/`. Esto no demuestra denegación global heredada; no se ampliaron permisos ni se habilitó una API para demostrarla.

## Decisiones no recibidas

Se preguntó por separado mediante el formulario de autorización:

1. Lectura temporal `storage.objects.get` acotada a objetos exactos de la nueva inspección y lock, con revisión y revocación verificadas, sin SQL/ACL/unlock.
2. Regresiones focales y controles obligatorios de publicación, sin cuentas ni base persistente local.

El formulario terminó `timed_out` con ambas respuestas `unanswered`. No se interpreta como aprobación ni rechazo. Se conserva la restricción de no nuevas pruebas y no cambios inferidos de controles de seguridad. Resolver por source el pendiente frontend puede continuar; fresh preflight/DDL/desbloqueo/publicación no se declaran completados.
