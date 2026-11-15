# Evidencia local — gate de autoridad DDL V84

Fecha: 2026-11-07; actualización posterior al ensayo de las 15:54 -03.
Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`
Rama: `codex/marketplace/fix/kyc-06-save-identity-first`
HEAD: `274f37e483a078890e92382a159fa49fd23c2fe8`
Estado: cambios sin commit; KYC y cambios ajenos preservados. **Gate de código/ensayo V84 verificado localmente; autoridad del destino Cloud SQL NO verificada.** No es autorización de release.

## 1. Harness frente al Job: discrepancia aclarada

| Aspecto | Harness desechable | Job privado |
|---|---|---|
| V001–V080 | Conexión administrativa; instala extensiones y ejecuta las fronteras administrativas históricas | No instala desde cero: `migration_execution.contract` rechaza prefijos menores que V83 |
| V081 | Roles y ledger preprovisionados por el fixture; login migrador asume schema owner | Usa roles preexistentes; no los crea ni repara |
| V082–V083 | Ventana explícita de CREATE, owner de función y grant option; después revoca/restaura | Exige baseline revisado; el harness no acredita las ACL reales del destino |
| V084 | `usuario` y ledger pertenecen a `postgres`; se concede CREATE de `public` y ahora **REFERENCES(id)**, sin grant option. Se prueban permisos ausentes y revocación | Login `espacigo_migrator_login`, `SET ROLE espacigo_schema_owner`; no concede ni revoca esa autoridad temporal |
| Limpieza | Revocación con readback; defer registrado antes del primer grant V84; contenedor propio eliminado y ausencia verificada | No existe una ventana administrativa de grants/revokes V84 en el Job. No se le atribuye la limpieza del harness |

La necesidad de REFERENCES depende de owner/ACL efectivos: si el schema owner ya posee `usuario`, no necesita un grant explícito. La concesión del fixture no prueba ausencia de permisos en Cloud SQL ni explica un fallo histórico de Job. Solo el destino sintético tiene diagnóstico SQLSTATE observado en este corte.

## 2. Contrato implementado y límites

`runner.go`, `authority_catalog.go`, `release.go`, `migration_release_manifest.py` y `v84_authority.py`:

- El plan read-only informa identidad, roles, owners, privilegios efectivos, esquemas efectivos, memberships y ACL semánticas; el runner reconsulta y valida dentro de la transacción inmediatamente antes del SQL V84, sin conceder permisos.
- Se exige login migrador restringido, schema owner NOLOGIN restringido y helper `espacigo_document_verification_fn` NOLOGIN restringido.
- Memberships exactas, en ambos sentidos y sin edges adicionales que toquen esos tres roles: login → schema owner; schema owner → helper. `ADMIN=false`, `INHERIT=false`, `SET=true` en ambos casos. No se rechazan memberships ajenas del clúster.
- Orden efectivo de lookup: `pg_catalog`, `public`; rechaza otros esquemas/temporales. Las consultas nuevas califican funciones de catálogo con `pg_catalog`.
- Profiles de owners cubiertos: `public=pg_database_owner`; `usuario` y ledger con el mismo owner, `postgres` (fixture) o `espacigo_schema_owner` (contrato de ownership). Otros owners quedan bloqueados para revisión: no se aprenden del target ni se alteran automáticamente.
- ACL de base, schema `public`, tablas `usuario`/ledger, sus columnas y defaults de tablas del rol creador en `public`/global. Se conserva owner/grantee/grantor/privilegio/grant option, incluidos PUBLIC y defaults implícitos para ACL NULL. No se llama `aclexplode` sobre ACL de columna vacía.
- Se rechazan exposiciones PUBLIC no previstas, grants/defaults no autorizados en ese alcance, grant options inesperadas y CREATE de base. Privilegios naturales del owner se distinguen de grants adicionales.
- El perfil acepta REFERENCES a nivel tabla o `id` para reconocer la semántica existente; **el harness positivo utiliza solo `id`**. Otros permisos runtime/worker se registran, pero este gate no redefine su política.
- El reporte de release conserva también `ddl_authority` leído dentro de la transacción. El verificador Python exige que coincida con el preflight revisado; una diferencia no autoriza grants, reintentos ni éxito. Esa comparación final es posterior al commit: si falla, inspeccionar el estado, nunca reaplicar automáticamente.

**Alcance deliberado:** esto no certifica mínimo privilegio de todo el clúster, otras tablas/funciones/secuencias, ACL compartidas del sistema o capacidad de todos los workers. V84 solo crea tablas/índices y referencia `usuario`; defaults de funciones/secuencias no se presentan como auditados. Los dos profiles de fuente no prueban cuál corresponde al Cloud SQL real; su compatibilidad y cualquier grant option histórica requieren preflight read-only y revisión del destino exacto. No activar grants temporales persistentes sin un mecanismo separado de duración/revocación autorizado y probado.

## 3. Fallo de integración corregido

`release_jobs.execute_job` reconocía únicamente seis campos en el JSON de plan. Al añadir `ddl_authority`, descartaba el reporte y esperaba logs hasta timeout. Se observó RED con `test_release_plan_authority`; ahora reconoce ambas formas exactas y aplica validación semántica. Reportes inválidos siguen bloqueados. No se ejecutó ningún Job remoto.

## 4. Ejecución PostgreSQL real, aislada

Comando: `bash scripts/test-migration-runner-postgres.sh`.

Último contenedor del ensayo completo: `espacigo-migration-ci-1468668`. Imagen local fijada:
`postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677`.

- `network=none`, sin puertos publicados, sin volúmenes Docker; PGDATA tmpfs y socket privado bajo `$TMPDIR/espacigo-migration-ci.G4XsOD/socket`.
- Bases `migrator_test_<nonce>` generadas únicamente dentro de ese clúster. No es `127.0.0.1:5432`, otra demo ni Cloud SQL.
- Fresh V001→V084 y upgrades V075/V076→V084 por fronteras de autoridad explícitas. Prefijos anteriores conservados; no-op posterior y catálogo/contrato API comprobados.
- Release V083→V084 ejecutado con `runReleaseWithConn` e identidad restringida, no solo `RunWithRole`.
- Negativos: sin CREATE ni REFERENCES, solo CREATE y solo REFERENCES. Runner rechaza antes del DDL; ledger sigue V83 y no quedan tablas V84. Diagnóstico directo separado, con transacción revertida: SQLSTATE `42501`, schema `public` al faltar CREATE y tabla `usuario` al faltar REFERENCES. No extrapolarlo a un Job real.
- Negativos de catálogo en cada baseline: membership saliente/entrante adicional, helper BYPASSRLS, UPDATE directo inesperado, grant option de REFERENCES(id), SELECT para PUBLIC y defaults de tablas que exponen SELECT a PUBLIC. Cada caso vuelve a su baseline y el runner conserva V83.
- Evidencia real PostgreSQL → Go → JSON → validador Python. Ledger drift, huecos, rollback, recuperación y concurrencia cubiertos por la suite real del runner.
- **PASS: 56 resultados obligatorios, ninguno omitido ni fallido.** El número incluye subpruebas requeridas, no significa 56 bases ni cobertura independiente de las 84 migraciones.
- Retirada del contenedor propio verificada; no quedaron contenedores `espacigo-migration-ci-*`. No se retiraron demos/volúmenes ajenos.

## 5. Otras verificaciones y fallos conservados

- `go test -race ./internal/migrator -run '^(TestRequireV84|TestV84)' -count=1`: PASS.
- `go vet ./internal/migrator`: PASS.
- `/usr/bin/python3 -m unittest discover -p 'test_*.py'` desde `scripts/gcp`: **PASS, 80 tests**, sin skips reportados. Log local: `$TMPDIR/espacigo-v84-python-suite.log`. Es offline: los mensajes de release/Cloud Build de esos tests son mocks, no evidencia remota.
- Primer discovery con Python de Hermes: falló por falta de PyYAML; no se instaló nada ni se ocultó el fallo. Se usó `/usr/bin/python3`, que ya tiene PyYAML. Discovery emite ResourceWarnings de HTTPError sintéticos de tests; no confundirlos con llamadas reales al proveedor.
- RED observado: faltaba cobertura obligatoria V76; lector descartaba `ddl_authority`; gate aceptaba membership no revisada; reporte omitía autoridad transaccional. Los subagentes aportaron una implementación parcial que no se consideró terminada; se completó y verificó por el padre.
- `git diff --check`: PASS. V001–V084 y `db/release-policy.json` no se modificaron. Los 84 nombres/versiones/hashes del TSV coinciden individualmente con checkout y HEAD.
- Nota de inventario: la raíz histórica del encabezado TSV no tiene receta de serialización documentada reproducida en este corte; no se usa como fingerprint de release. El algoritmo real `inventoryFingerprint` (`checksum + dos espacios + nombre + LF`) da `3921ff747e4a7e7f49de443de7333dad03535f0687078b93b8a3cac6042a209a`. Se conserva el manifiesto original sin alterar checksums; documentar su serialización queda pendiente de reconciliación de fuente, no de permisos V84.

## 6. Gate siguiente: ledger y autoridad por destino

**No se consultó ni modificó ninguna base persistente ni Cloud SQL en este corte.** Sí hubo SQL/DDL/roles/ACL exclusivamente sintéticos en el contenedor desechable autorizado. Sin credenciales reales, backups, bootstrap persistente, despliegue, commit, push, merge, reset o cambio de rama.

**Estado al cierre de esta evidencia local (antes de la posterior inspección recovery):** el siguiente gate era una ruta privada que no continuara a backup/apply/rollout; Cloud SQL no estaba verificado por este ensayo. Esa lectura ya fue realizada separadamente: véase `evidence_cloudsql_readonly_report_20261107.json` y `evidence_recovery_storage_operational_20261107.json` en esta misma carpeta. La lectura posterior encontró V1–V83 exactas/V84 pendiente y CREATE/REFERENCES ausentes para el actor observado, sin cambiar permisos. No convierte este ensayo en permiso de aplicar DDL, no actualiza el snapshot nativo V76 y no cierra la auditoría semántica independiente.
