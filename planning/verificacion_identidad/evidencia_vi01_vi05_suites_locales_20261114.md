# Evidencia de suites locales VI-01–VI-05 — 2026-11-14

## Contexto

- Repositorio: `/home/nandev/Documentos/projects/espaciGo-main-sync`
- Rama: `codex/marketplace/fix/kyc-06-save-identity-first`
- HEAD: `274f37e483a078890e92382a159fa49fd23c2fe8`
- Fecha local observada: `2026-11-14T22:05:19-03:00`
- Worktree compartido ya estaba modificado y contenía archivos no rastreados. No se editaron archivos de código ni se alteró el índice.

## Comprobaciones ejecutadas

Con entorno mínimo (`env -i`, conservando solo `PATH`, `HOME` y `TMPDIR`):

```sh
go test -race -count=1 \
  ./internal/documentverification/... \
  ./internal/identityverification \
  ./internal/verification/... \
  ./internal/adapters/documentcodec \
  ./internal/adapters/documentstorage
```

Resultado: **PASS** en los 10 paquetes devueltos por Go, incluyendo inspección, HTTP y codec/storage. Ninguna prueba de integración BFF con build tag fue incluida; se retiraron variables de entorno de DB/PG y de opt-in.

```sh
go vet \
  ./internal/documentverification/... \
  ./internal/identityverification \
  ./internal/verification/... \
  ./internal/adapters/documentcodec \
  ./internal/adapters/documentstorage
```

Resultado: **PASS**.

```sh
go build -o /dev/null ./cmd/api
git diff --check
```

Resultado: ambos **PASS**. Rama y HEAD permanecieron iguales a los identificados arriba.

## Alcance y límites

Son pruebas locales de código, dominio, handlers, inspección y adapters con fixtures/fakes; algunas pruebas HTTP usan servidores sintéticos locales. No se ejecutaron pruebas con tag `bffintegration`, pruebas opt-in de `cmd/api`, tests PostgreSQL, consultas a bases persistentes, DDL/DML, cambios ACL, reinicio de consumidor, restore, Cloud SQL, GCS, MFA efectiva ni pruebas de producto con personas reales. Estos resultados no cierran persistencia funcional ni reinicio.

El siguiente gate de VI-01/VI-02 sigue siendo el recorrido de persistencia nativa sintética. Permanece bloqueado por la autoridad de limpieza faltante en `evento_auditoria_local`; esta corrida no cambió ni amplió permisos y no habilita repetir el test funcional.

## Subcorte adicional del adapter PostgreSQL — pruebas no opt-in

Se inspeccionaron las entradas de integración del paquete `internal/adapters/postgres/documentverification`: no hay `TestMain`; las conexiones a PostgreSQL están detrás de variables opt-in/DSN y las variantes con socket F5 tienen su propia bandera explícita. Con `env -i` (solo `PATH`, `HOME`, `TMPDIR`), por tanto sin DSN, passfiles ni banderas de integración:

```sh
go test -race -count=1 -json ./internal/adapters/postgres/documentverification
go vet ./internal/adapters/postgres/documentverification
```

Resultado: `go test` **PASS**, exit 0: **427 PASS, 31 SKIP, 0 FAIL**; los omitidos fueron tests de integración PostgreSQL/ClamAV y requieren opt-ins no proporcionados. `go vet` **PASS**. No se conectó a la base nativa ni al PostgreSQL desechable, y no se ejecutó DDL/DML. El recuento corresponde a esta ejecución independiente; no se suma a otros runners.

## Subcorte `cmd/api` — journals y guards unitarios

Con `env -i` y selección explícita, sin `TestMain`, perfiles de runtime ni variables de conexión, se ejecutaron en `cmd/api` los casos unitarios de journals de declaración/attempt/supersession, rechazo de cuenta/scope/entrada insegura, conservación del intent ante resultado incierto, limpieza sin autoridad y guard de artefactos. Comando focal: `go test -race -count=1 ./cmd/api -run '^(TestNativeDeclarationTrackedRepository(PersistsIntentBeforeWrite|NeverWritesWithoutJournal|KeepsIntentAfterUncertainWrite)|TestNativeDeclarationCleanupRejectsMissingAuthorityOrScope|TestNativeKYCNoRevisionArtifactGuard|TestNativeDeclarationIntentPair(JournalsEachAccountBeforeDelegateWrite|RejectsUnknownAndPreservesOccupiedJournals|ConstructorRejectsUnsafeInputs)|TestNativeAttemptJournal(PersistsBeforeDelegateAndReadback|RejectsDeclarationMismatchBeforeDelegate|RejectsOccupiedJournalAndUnsafeParent|RejectsUnknownAccount|RetainsIntentOnDelegateErrorAndRejectsBadRows)|TestNativeSupersession(JournalsTwoSequentialWritesAndConsumesUncertainSlot|RejectsUnknownAccountAndTracksExactlyTwo)|TestNativeApprovalDetachedCleanupBounds)$' -v`.

Resultado observado: **16 tests principales PASS**, 0 FAIL, paquete `cmd/api` PASS en 1.140 s. Son fakes y filesystem bajo `t.TempDir()`; no prueban cleanup ni persistencia PostgreSQL. No se ejecutó el test opt-in de persistencia/rotación y no se accedió a una base.

## Adapter de declaraciones — suite sin opt-in

Antes de ejecutar se inspeccionaron los dos tests de ciclo PostgreSQL: ambos exigen `ESPACIGO_NATIVE_ADAPTER_TESTS=1` y su DSN dedicado; la ejecución con `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR"` no habilitó esos opt-ins. No hay `TestMain` ni `init` en el paquete. Se ejecutó `go test -race -count=1 -json ./internal/adapters/postgres/identitydeclaration`: **26 PASS, 2 SKIP, 0 FAIL**. Los omitidos fueron exactamente `TestRutClaimPostgresClaimLifecycle` y `TestSessionDeletionQueuePostgresLifecycle`. `go vet ./internal/adapters/postgres/identitydeclaration` terminó exit 0. No se abrió pool ni se accedió a PostgreSQL; los casos ejecutados son unitarios/fakes y no prueban persistencia.

## Almacenamiento privado local — suite documentfs

Se inspeccionó el paquete para descartar setup de DB/integración (`TestMain`, `init`, DSN, procesos externos); las pruebas usan filesystem local temporal. `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 -json ./internal/adapters/documentfs` terminó exit 0: **22 PASS, 0 SKIP, 0 FAIL**. `go vet ./internal/adapters/documentfs` terminó exit 0. Esta corrida cubre comportamiento local/sintético del store, incluidos scopes, borrado y symlinks; no demuestra persistencia duradera tras reinicio, GCS, generaciones, ni borrado/recuperación cloud.

## Suite local completa de `cmd/api` con entorno mínimo — 2026-11-15

Antes de ejecutar se inspeccionaron los puntos de conexión PostgreSQL de las pruebas y los entrypoints opt-in: runtime PostgreSQL, rotación, fixture/HTTP harness, cleanup journal y preflights requieren flags/DSN explícitos; `TestMain`/`init` no aparecen en el paquete. La corrida usó solo `PATH`, `HOME` y `TMPDIR` mediante `env -i`, por lo que esos opt-ins quedaron deshabilitados. `go test -race -count=1 -json ./cmd/api` terminó exit 0: **143 PASS, 9 SKIP, 0 FAIL**; `go vet ./cmd/api` terminó exit 0. Los nueve skips fueron `TestIdentityDeclarationRuntimePostgreSQLSaveRestartReadback`, `TestNativeIdentityCleanupPersistedIntent`, `TestNativeIdentityPrepareAuthorizedFixture`, `TestNativeKYCAPIWithBFF`, `TestNativeIdentityExternalReadOnlyPreflight`, `TestNativeKYCFixtureReadOnlyPreflight`, `TestNativeIdentityRUTKeyRotationSequential`, `TestLocalKYCProfileStartupPreflight` y `TestWizardStartupDiagnostic`. No se abrió conexión PostgreSQL ni se escribieron fixtures; esta suite local no cierra persistencia nativa, reinicio ni cleanup.
