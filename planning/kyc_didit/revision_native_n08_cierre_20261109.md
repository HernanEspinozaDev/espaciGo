# N08 — revisión de los gates de cierre nativo (2026-11-09)

## Alcance y baseline

Checkout `espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8` más árbol dirty conservado. El SHA no representa los cambios sin commit. Sin publicación, integración, despliegue, Didit externo ni datos reales.

La auditoría independiente de criterios `deleg_c2fcc245` fue source-only: C08 requiere supersesión/nueva revisión y aprobación sintética SANDBOX sin badge LIVE; C10 debe precisar purga, outbox y retención por capa, no inventar SLA operativo. N08-A–J conservan su evidencia y sus límites. Una variante no ejecutada no se convierte en PASS por reducir la matriz.

## Prerrequisito encontrado para retirar una aprobación purgada

El padre trazó `DiditService.ProcessWebhook` → `UpdateAttemptStatusAndCompleteEvent` y V90. La aprobación autoritativa separa el intento de su declaración y purga los snapshots. El CHECK aplicado `identity_provider_attempt_detached_approved_ck` admite una declaración NULL únicamente con status APPROVED y metadata cifrada NULL. La revocación posterior intenta persistir REJECTED/EXPIRED: ese candidato contradice el CHECK. Esta revisión no presenta una inferencia de fuente como un webhook nativo ejecutado.

Readback administrativo READ ONLY del 2026-11-09 20:43 -03: destino exclusivo `127.0.0.1:5432/espacigo_dms_stage_20261020`, socket `/var/run/postgresql`, OID16388, motor180006, ledger90. Catálogo confirma el CHECK descrito y owner `espacigo_schema_owner`; schema CREATE=false, audit DELETE=false, outbox DELETE=false. Outbox Didit50 filas/0pendientes; puertos18090/4201/4219/19091 sin listeners. No se escribió en la base.

Se prepara un delta append-only V91, exclusivamente para permitir retirada terminal de intentos ya separados y sin datos cifrados. No modifica V1–V90 ni crea roles, grants o funciones SECURITY DEFINER. Su aplicación aún requiere revisión independiente, runner exacto, backup y postcheck; este documento no acredita aplicación.

Backup privado previo: `.local/testing/kyc-native/pre-n08-v91-20261109.dump`, custom format, 8323938 bytes, modo0600, ext4; SHA-256 `9dc7d2edf79543580e9dac2e0ae8cae29df7c2bb5607a24c3fd5a485b5b09bd1`. `pg_restore --list` y parse completo `--exit-on-error --file=/dev/null` terminaron exit0. No es un ensayo de restauración ni backup de roles globales. Sudo se ejecutó directamente en terminal Hermes usando su autenticación de entorno, sin imprimir ni transmitir `SUDO_PASSWORD`.

## Fixtures y proveedor de ensayo

- Fake separado `NewSyntheticApprovalHandler`, habilitado solo con `KYC_FAKE_SYNTHETIC_APPROVAL=1`; default continúa In Review. Solo acepta la identidad sintética allowlisted del código; almacena vínculos sesión/workflow/correlación, no expected_details del request.
- Padre añadió evaluación conductual de la respuesta del fake con `verification.EvaluateDecision`: APPROVED sin flags/warnings. `go test -race -count=1 ./cmd/kyc-fake-provider`, vet, build scratch y diff-check PASS. Esto es prueba de código/fake, no aprobación biométrica real ni gate nativo.
- Aprobación/purga pierde la FK a declaración. Se requiere journal de intento confiable **previo** a PrepareAttempt y cleanup limitado a esos IDs, además de las declaraciones. No adoptar intentos por cuenta, prefijo o respuesta HTTP.
- Revisión parental detectó que el primer helper delegado exigía correlación idéntica de declaración e intento; producción genera dos correlaciones distintas. Se añadió fixture que conserva esa separación; RED/GREEN parental pendiente de congelar las fuentes de cmd/api.

## Estado inicial (histórico, previo a ejecución K–N)

**N08 sigue abierta.** Nuevos probes y helpers están en preparación source-only. No hay DDL V91, aprobación HTTP nativa, supersesión ni retirada post-aprobación acreditados por este informe inicial. Mantener grants cerrados hasta revisar todos los artefactos y no ejecutar workers generales ni reclamar outbox ajena.

## Disposición final de esta revisión

El estado inicial anterior quedó superado por `evidencia_native_n08_aprobacion_20261109.md` (V91 aplicada/readback, K/L aprobados), `evidencia_native_n08_reintento_ttl_20261109.md` (M) y `evidencia_native_n08_purga_acotada_20261109.md` (N). `deleg_31c3f5b9` revisó criterios canónicos y evidencia de cierre source-only: approvedlocalclosure=true, sin criterio canónico local pendiente; no ejecución DB por el revisor. Sus observaciones sobre el archivo de cierre aún no creado y estados parciales históricos se resolvieron con el recibo definitivo y addenda explícitos.

**N08 cerrada localmente** según `cierre_native_n08_20261109.md` y plan§25.34. Se conservan fallos/correcciones y límites: no aceptación humana, jurídico/privacidad, proveedor externo, scheduler/SLA ni publicación/despliegue. Postflight: ledger91, grants cerrados, outbox50/0, cuentas conservadas sin residuos y servicios propios retirados.
