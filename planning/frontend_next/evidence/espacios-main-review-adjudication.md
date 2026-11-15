# Revisión de preparación: adjudicación del padre

## Base y método

Candidato: `/home/nandev/Documentos/projects/espaciGo-spaces-main-preparation`, base `647cd491985d619370990d3f226b5df61e36b42b`; origin/main observado `5830c9ce2dc61d4df13a4139da984b4cafd6d9ba`. Revisión source-only: no pruebas nuevas ni consultas SQL.

## Hallazgos confirmados

1. El JSON PlanResult Go incluye `session_user` y `current_user`, mientras `release_jobs.py` y `migration_release_manifest.py` esperan exactamente seis claves. Deben sincronizarse ambos consumidores, conservando validación de identidad cuando se recibe el nuevo contrato. Corrección asignada exclusivamente en el candidato.
2. Terraform permite sufijos de bucket distintos del nombre fijado en el release. Debe fijarse el sufijo revisado para los gates de media.
3. La alternancia de gates Terraform usa dos direcciones para el mismo bucket y binding IAM. Debe conservarse una dirección canónica: las direcciones `_only[0]` ya aplicadas, sin ejecutar moved/import remoto. Corrección asignada exclusivamente en el candidato.

## Afirmaciones descartadas tras verificación independiente

- Un revisor afirmó que origin/main tenía 83 migraciones y que V84 era adicional. El padre ejecutó `git show origin/main:db/migrations/V000084__didit_identity_verification_schema.sql`, que tuvo éxito; el manifiesto programático verificó 84 migraciones idénticas. El revisor comparó con el checkout principal atrasado, no con el ref fijado. No se atribuye a la preparación un delta V84 nuevo.
- Un revisor afirmó que el candidato revertía h2 a h1 en verificaciones. `git diff origin/main -- frontend/src/app/(workspace)/mi-cuenta/verificaciones/page.tsx` muestra exactamente lo contrario: main tiene h1 y el candidato cambia a h2. No se aplica una reversión basándose en esa afirmación.
- `git merge-base --is-ancestor origin/main HEAD` pasó en el candidato. No existe una divergencia Git que obligue a sobreescribir código de main o a rebase; las correcciones deben ser acotadas sobre este candidato.

## Pendiente no resuelto por esta revisión

La recuperación parcial usa un booleano de módulo y marca pending antes de revalidar identidad tras createDraft (`listing-wizard.tsx:422-435`). La UI limita el enlace por accountId (`:484-486`), pero eso no demuestra aislamiento de la señal global en la carrera de cambio de cuenta. Se conserva como observación de revisión pendiente de reproducción autorizada; no se afirma fuga de ID ni se cambia el comportamiento a ciegas. No es aceptable declarar todo el plan verificado solo por los tests históricos con mocks.

## Límites de entrega

Ninguna revisión source-only autoriza ni acredita DDL, desbloqueo, publicación o despliegue. Continúan pendientes el preflight fresco de Cloud SQL, autoridad de V84, resolución controlada del lock y los gates de integración del plan. El usuario mantiene su instrucción de no ejecutar más pruebas.
