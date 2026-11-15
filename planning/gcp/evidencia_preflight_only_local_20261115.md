# Evidencia — guardas locales del modo Cloud SQL preflight-only

- **Fecha:** 2026-11-15
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`
- **Rama/HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`

## Ejecución

Desde `scripts/gcp`, con entorno mínimo `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR"`, se ejecutaron cinco pruebas focales de `test_main_release.py`:

- `MainReleaseTests.test_preflight_only_cannot_reach_backup_apply_rollout_or_watermark`
- `MainReleaseTests.test_preflight_parent_cannot_fall_through_to_applying_release`
- `MainReleaseTests.test_preflight_failure_retains_lock_and_never_applies`
- `MainReleaseTests.test_preflight_requires_both_cli_and_parent_mode_to_match`
- `JobBoundaryTests.test_preflight_parent_cannot_provision_an_applying_job`

Comando: `python3 -m unittest -v` seguido por esos cinco identificadores. Resultado: **5 tests, PASS, 0 FAIL**, duración `0.066s`, exit `0`.

## Alcance probado

La suite ejercita el orquestador con `FakeCloud` y un directorio temporal. Caracteriza el modo preflight-only y sus guards: no se alcanza backup, Job aplicador, SQL, rollout ni watermark; un parent preflight no cae al flujo de aplicación; los errores conservan el lock simulado; y el provisioner rechaza un Job `apply` antes de solicitar su creación.

## Límites

No se invocaron Cloud Build, Cloud Run, Cloud SQL, `gcloud` ni red; no se creó un Job/lock remoto y no se leyó el esquema Cloud SQL. El `PASS` es evidencia unitaria del código local con dobles, no una ejecución del modo en el proveedor ni prueba del ledger/autoridad del destino. La ruta permanece local/no publicada y requiere sus gates propios antes de cualquier operación remota. Cloud SQL sigue **no verificado**. Esta evidencia no autoriza publicación, SQL/DDL, IAM, unlock ni despliegue; tampoco cambia el bloqueo independiente del test de persistencia nativa KYC.

Sin cambios de código, commit, push o despliegue en esta corrida.
