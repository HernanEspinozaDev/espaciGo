# F5: equivalencia de baseline ACL después de restaurar el cluster

Fecha: 2026-11-04. Rama backend `codex/backend/feature/local-close-02-espacios-galeria-documentos`, HEAD base `d7fb5c45759ff527e9187bb5aebda431a9033e92` más el diff local registrado aquí. Solo entorno sintético `127.0.0.1:15434/espacigo_f5_gate_20261101`, server `172.28.0.2`, PostgreSQL 18.6. No cloud/main.

## Por qué hay dos fingerprints

El cluster tmpfs original se perdió. El archive pre-V83 conserva V1–V82, owners y ACL; no contiene globals ni las cuentas posteriores. Se restauraron seis roles y tres memberships del contrato revisado, reutilizando credenciales existentes sin cambiarlas. La restauración con owners/ACL completos terminó en una transacción; se excluyeron únicamente los CREATE SCHEMA tiger/topology que ya pertenecían a las extensiones instaladas. Después `cmd/dbmigrate` aplicó solo V83 y el plan read-only confirmó 83/83 fuentes y cero pendientes.

La huella histórica concatena OIDs de objetos y grantor: por ello no identifica semánticamente el mismo esquema después de un restore. El test read-only ACL rechazó inicialmente la nueva huella; no se hizo replay ni GRANT para satisfacerlo.

| Baseline | PUBLIC count | PUBLIC hash | default count/hash |
|---|---:|---|---|
| Histórico | 5762 | `527e02ed8bf413337ae39caf6e4edaa7` | 0 / `d41d8cd98f00b204e9800998ecf8427e` |
| Cluster restaurado | 5762 | `37f89420b0338cdd11a978e57a04204c` | 0 / `d41d8cd98f00b204e9800998ecf8427e` |

La segunda huella no se acepta por ser simplemente la observada. Su fundamento es la comparación reproducible de schema completo contra el archive conocido y los deltas exactos de migraciones inmutables. Ambas huellas quedan fijas; no se aprende ni acepta automáticamente una tercera. Se conservan las guardas de base, engine, cardinalidad y defaults. El baseline histórico permanece válido para sus previews históricos, que no se vuelven a ejecutar sobre roles ya restaurados.

## Artefactos y comprobación

- Archive original: `/home/nandev/.local/share/espacigo-f5-backups/f5-pre-v83-20261103T031238Z.dump`; 646453 bytes; SHA-256 `ef9d6b15d439227104fb0ca2ba916b711d4680b5570f3740b8aff409adb7ae54`.
- Referencia schema-only de `pg_restore`: scratch `f5-original-v82-schema.sql`; SHA-256 `32d476597a74186b11144177891a79c1444f8499d28d9714768d978ed4c9194c`.
- Schema-only de `pg_dump` del target restaurado: scratch `f5-postrestore-schema.sql`; SHA-256 `053f75b8e8f90425efc42db192b70e2bdb4026bef2d27849d8fc872a02e96810`.
- Backup post-V83: `/home/nandev/.local/share/espacigo-f5-backups/f5-restored-post-v83-20261104T073400Z.dump`; 646886 bytes; SHA-256 `3bfef93ec7afcc5805b75d61d967e42ca2e236f463bed00f26c42ade09a91ef5`; TOC y parse schema-only PASS.

Ejecutado por el padre:

```sh
python3 -m unittest scripts.test_verify_f5_restore_schema -v
python3 scripts/verify_f5_restore_schema.py \
  --before-schema /home/nandev/.hermes/cache/scratch/f5-original-v82-schema.sql \
  --after-schema /home/nandev/.hermes/cache/scratch/f5-postrestore-schema.sql \
  --migration-dir db/migrations
```

Resultado: 12 tests PASS y comparación real PASS. El checker valida checksums V82 `b53fe7f6f03af4d9081e4a378053ad0780d8b532962326488108201eb098ed96` y V83 `405c2f02f8a29e962421a44cbbd07e6e7fae4aa4696483e9d45ab209e82e9333`. Exige que ambos cuerpos de `document_verification_space_owner_changed` coincidan exactamente con su fuente respectiva y conserva firma/settings al comparar. Permite únicamente ese cuerpo y la línea adicional exacta `GRANT UPDATE(id)` al runtime en reviewer_grant. No elimina funciones completas ni permite statements adyacentes arbitrarios. Conserva contenido de literales/dollar bodies, owners, grants, revokes y defaults. Solo normaliza comentarios externos de pg_dump, líneas vacías externas y tokens restrict/unrestrict externos. No ejecuta SQL.

El ejecutor inicial dejó un checker cuyo positivo fallaba y cuya regla de grant no correspondía a V83. El padre sustituyó esa regla, añadió conservación léxica y corrigió fixtures; no se considera suficiente la primera entrega ni los negativos que pasaban por ese fallo previo.

## Gates separados

- RED de `TestRuntimeACLF5RestoredBaselineRetainsAllOtherGuards` observado antes de incorporar la segunda huella; GREEN posterior. Paquetes `internal/dbbootstrap` y `cmd/dbmigrate` PASS sin DSN.
- `TestPostgresV83ReviewerGrantLockPrivilegeIsMinimal` real PASS.
- La comparación schema-only acredita objetos de aplicación/owners/ACL restaurados, no por sí sola todas las clases ACL del engine ni ACL de conexión a base. Estas siguen sometidas al readback completo, incluyendo PUBLIC/default, pg_init_privs, roles, ownership y privilegios efectivos. No sustituir ese gate por este checker.
- Revisión independiente `deleg_d7356e05`: PASS, sin falso positivo concreto; 12/12 tests Python y tests Go focalizados repetidos por el revisor. El padre ejecutó además `TestVerifyRuntimeACLReadbackF5Gate` en transacción de lectura contra el target exacto restaurado: PASS (0.757 s). No se hizo replay ni se modificaron grants para conseguirlo. El revisor no conectó DB; el resultado live corresponde al padre. F5-A3/F6 permanecen abiertas por sus otros criterios.
- La suite completa `env -i ... go test -mod=readonly -timeout=300s ./... -count=1` pasó después del cambio de baseline, sin DSN (pruebas opt-in omitidas). No hubo migraciones/grants adicionales.
- API propia worker-free arrancada como proceso `proc_7993807676c8`, PID inicial `2591692`, en `127.0.0.1:18082`, credencial runtime y `SET ROLE espacigo_runtime`; `/health/ready` y `/api/v1/auth/terms` HTTP 200. Catálogo READ ONLY confirma su conexión únicamente a la DB F5 como `espacigo_api_login`; cero jobs scan/retention activos al iniciar. No se contactaron puertos protegidos 18083/18084.
- Registro/verificación normal de únicamente las dos cuentas de reemplazo autorizado y en ejecución delegada. Se preserva el inventario anterior y se escribirá uno nuevo privado; el backup post-V83 no sustituye el backup posterior a su registro/verificación. Antes de la suite committed/colas hay que detener solo esta API propia y verificar ausencia de consumidores.

## Nota de publicación de la reparación (no migración)

El SQL de preparación `db/repair/f5_restore_v82_roles.sql` fue aplicado/revisado con SHA-256 `c64fb2fe82290f86d5938958e2a754786cb30da388c1452ae7eb3f6bcfc4b475`. El gate `git diff --cached --check` detectó dos líneas vacías finales en el archivo nuevo: se retiraron únicamente esas dos líneas antes de publicar. La fuente publicada tiene SHA-256 `66e1fc49a6206c29ef1a0c0fa07b3881be6ced3199bc6b2f43577f5e1deb2f47`. Comprobación byte a byte en el padre: agregar exactamente dos bytes newline a la fuente publicada reproduce el SHA aplicado; ningún statement cambió ni se volvió a ejecutar SQL. Esta diferencia no afecta ninguna migración ni su ledger: V1–V83 permanecen intactas. Los estados pendientes del cuerpo anterior son snapshots históricos; el cierre posterior está en `f5-f6-recovered-live-20261104.md`.
