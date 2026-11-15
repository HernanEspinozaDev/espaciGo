# Evidencia — VI-03/VI-04, revocación identity sintética

- **Fecha:** 2026-11-11
- **Estado:** subcorte local de código/fixtures sintéticos completado; VI-04 global permanece parcial.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty antes de este subcorte; se preservaron los cambios ajenos. Solo se atribuyen las pruebas y este registro.

## Cobertura añadida

- `TestIdentityRevokeRequiresExactGrantAndAuditsTransition` construye un caso identity sintético aprobado cuya validez inicial se comprueba con `validSnapshot`. Cuatro escenarios ejercitan el servicio `Revoke`: sin grant, scope de sujeto incorrecto, grant ligado a otro reviewer y grant identity exacto para el sujeto.
- En los tres escenarios denegados, la expectativa es `ErrCapability` y el test compara el snapshot completo y la auditoría antes/después para asegurar que no hubo mutación.
- Con el grant exacto, comprueba `CaseRevoked`, incremento de revisión 4→5, `RevokedAt` del reloj inyectado, reviewer responsable y campos esperados de la auditoría (`revoked`, estado resultante y motivo sintético).
- `TestReviewerAccessRejectsRevokedIdentityCaseBeforeStoreRead` usa un snapshot terminal revocado con grant exacto y tres roles clean sintéticos. La descarga devuelve `ErrNotFound`, sin bytes ni MIME y con cero lecturas en los stores identity y space.
- Ambas son caracterizaciones de comportamiento existente; pasaron en primera ejecución. No hubo ciclo RED/GREEN ni cambio de producción en este subcorte.

## Verificaciones ejecutadas

Con variables de conexión PostgreSQL/DB retiradas del entorno:

- Tests focales: `go test ./internal/documentverification -run '^(TestIdentityRevokeRequiresExactGrantAndAuditsTransition|TestReviewerAccessRejectsRevokedIdentityCaseBeforeStoreRead)$' -count=1` — PASS.
- `go test -race ./internal/documentverification/... -count=1` — PASS en `documentverification`, `inspection` y `transport/http`.
- `go vet ./internal/documentverification/...` — PASS.
- `gofmt -d` para los dos archivos Go tocados — sin diferencias.
- `git diff --check` — PASS.

- **Revisión independiente read-only del subcorte de servicio/store:** `deleg_631d2ce0` aprobó esos dos tests y su documentación, sin hallazgos de seguridad ni errores lógicos. Sugirió considerar campos de auditoría adicionales si el contrato los requiere. Se verificó que `Service.Revoke` construye únicamente `Action`, `ResultCode` y `ReasonCode`; son los campos aplicables a esta transición y están afirmados por el test. Los campos de observación/checklist corresponden a la decisión de revisión identity, no a la revocación.

## Addendum — frontera HTTP de revocación (2026-11-11)

- `TestReviewerRevokeUsesAuthenticatedPrincipalAndReadsBackRevokedCase` pasa una solicitud `POST /revoke` mediante el handler real y un service fake. Comprueba el actor del principal autenticado, ID de expediente, `PurposeIdentity`, motivo canónico `solicitud_titular` y revisión esperada `7`; después de éxito, exige `Load`, ETag `"8"` y DTO fixture con estado `revocada` y revisión `8`.
- `TestReviewerRevokeDenialDoesNotReadBackCase` hace que el service fake devuelva `ErrCapability`; la respuesta es 404 neutral, no serializa el error y el handler no llama a `Load`.
- Se corrigió el motivo sintético en el test de endpoints existente y en el test que inyecta un campo `actor`, de `owner_request` a `solicitud_titular`. El primer intento focal falló por ese valor incorrecto del fixture; el dominio define `ReasonOwnerRequest` como `solicitud_titular`. Fue un fallo de preparación del test, no un RED de producción. Con el valor correcto, test focal y regresiones de ruta/campo actor pasaron.
- Verificación final: test focal de handler PASS; `go test -race ./internal/documentverification/... -count=1`, `go vet ./internal/documentverification/...`, `gofmt -d` y `git diff --check` PASS, con variables DB/PG retiradas.
- Alcance: la respuesta revocada es una fixture preconfigurada del fake; se caracteriza el cableado HTTP→service fake→Load, no la transición del estado dentro del servicio/repository, auth real, PostgreSQL, transacción, persistencia ni reinicio. No se cambió código de producción. Revisión independiente read-only `deleg_cb240b2c`: aprobada sin hallazgos de seguridad, lógica ni sugerencias; confirmó el alcance fake-only.

## Límites y siguiente gate

- La cobertura usa servicios/repositorios/stores in-memory. No prueba grant real, transacción/trigger PostgreSQL, persistencia tras reinicio ni descarga real.
- El repositorio PostgreSQL tiene una rama de actualización que libera el vínculo de declaración al pasar un caso identity a estado terminal; este subcorte solo inspeccionó esa fuente. El fake de `Service.Revoke` no emula dicha mutación del adaptador y no se ejecutó SQL para comprobarla.
- No se ejecutaron SQL/DDL, migraciones, consultas ni pruebas conectadas a bases persistentes; tampoco se accedió a Cloud, proveedor, sitio público o datos reales. No hubo commit, push, integración a `main` ni despliegue.
- VI-04 permanece parcial: siguen pendientes las pruebas autorizadas de integración y persistencia que requieren un entorno/destino explícitamente permitido, además de los demás gates del plan. Este recibo no constituye aceptación de producto.
