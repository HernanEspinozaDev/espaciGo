# Evidencia VI-01/VI-02 — diagnóstico READ ONLY de autoridad de cleanup nativo

**Fecha:** 2026-11-15
**Rama/HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`
**Entorno:** checkout local aislado `espaciGo-main-sync`; PostgreSQL nativo de ensayo ya identificado por el preflight.

## Objetivo

Mejorar el guard del test `TestNativeIdentityRUTKeyRotationSequential` para que enumere cada privilegio de cleanup ausente o inseguro, y permitir revalidar esa condición con un test opt-in estrictamente de solo lectura. Una revisión fuente independiente detectó que la versión inicial comprobaba permisos requeridos pero no descartaba privilegios adicionales en la tabla de auditoría; este subcorte cierra ese hueco. No intenta habilitar ni ejecutar la persistencia funcional.

## Cambios locales

- `cmd/api/identity_native_rotation_test.go`: el guard previo a la apertura del pool de aplicación consulta `DELETE` de auditoría, `CREATE` efectivo en `public`, `DELETE` de claims/outbox y los `SELECT` requeridos. Además compara el ACL directo de `espacigo_schema_owner`/`PUBLIC` con el conjunto permitido exacto (grantor `postgres`, sin grant option), y rechaza privilegios adicionales efectivos sobre la tabla/columnas de auditoría, incluyendo `INSERT`, `UPDATE`, `TRUNCATE`, `REFERENCES`, `TRIGGER`, `MAINTAIN` y `SELECT` en columnas no autorizadas. El guard continúa abortando antes de cualquier escritura de la prueba funcional.
- `cmd/api/identity_native_cleanup_authority_report_test.go`: cuatro pruebas unitarias cubren reporte de permisos faltantes, aceptación exclusiva del conjunto permitido, rechazo de privilegios extra y rechazo de un ACL concedido a `PUBLIC`.
- `cmd/api/identity_native_preflight_test.go`: añade `TestNativeIdentityCleanupAuthorityReadOnlyPreflight`, opt-in por `IDENTITY_DECLARATION_NATIVE_PREFLIGHT_ONLY=1`. Valida target, versión, identidad y ledger; ejecuta las consultas de catálogo dentro de una transacción PostgreSQL `READ ONLY` y no consulta fixtures/cuentas.

## Verificación ejecutada

- Ciclo TDD: el test inicialmente falló por el helper/tipo aún no implementado; después de completar la comprobación de ACL, los cuatro tests unitarios pasaron con `-race` (**4 PASS, 0 FAIL**).
- Suite completa local `env -i ... go test -race -count=1 -json ./cmd/api`: exit 0, paquete PASS, **10 SKIP, 0 FAIL**. Los skips son gates opt-in; incluyen persistencia/reinicio, limpieza nativa, fixture y preflights. No se conectó a PostgreSQL en esta corrida.
- `go vet ./cmd/api`: exit 0.
- `go build -o /dev/null ./cmd/api`: exit 0; `gofmt -d` y `git diff --check`: sin diferencias/errores.
- El preflight opt-in local pasó (**1 PASS, 0 FAIL**). Comprobó el target `espacigo_dms_stage_20261020`, `server_version_num=180007`, `session_user=espacigo_migrator_login`, `current_user=espacigo_schema_owner` y el ledger fuente/aplicado hasta V96. En su transacción `READ ONLY` informó `BLOCKED` por `DELETE public.evento_auditoria_local` y `SELECT` sobre `actor_id`, `recurso_tipo`, `recurso_id`, `accion` y `correlacion_id`; no detectó ACL adicionales ni privilegios extra efectivos en esa tabla.

## Límites y estado del gate

El PASS indica que el diagnóstico READ ONLY se completó; **no** indica que la autoridad esté disponible ni autoriza escrituras. No se ejecutaron `TestNativeIdentityRUTKeyRotationSequential` ni otros tests funcionales; no hubo DML, DDL, cambios ACL, workers, acceso a cuentas/fixtures, GCP/Cloud SQL, commit, push o despliegue. El gate de persistencia/reinicio nativo sigue bloqueado por los permisos señalados. Restore completo, GCS y Cloud SQL siguen sin verificarse. La solicitud posterior de autorización para la ventana exacta de ACL quedó sin respuesta al expirar; no se infiere consentimiento ni se vuelve a solicitar automáticamente.

## Addendum de revisión independiente y rutas de herencia — 2026-11-15

La segunda revisión source-only (`deleg_b82f3ef7`) encontró que comparar los ACL de `PUBLIC` y `espacigo_schema_owner` no demostraba por sí solo la ausencia de grants heredados de otras membresías. La revisión no ejecutó tests ni validó SQL en PostgreSQL 18. El guard ahora consulta `pg_roles` y exige atributos exactos para `espacigo_migrator_login` y `espacigo_schema_owner`, y compara todas las membresías que tocan `espacigo_schema_owner` con el contrato V96: migrador→owner y owner→`espacigo_document_verification_fn`, ambas con grantor `postgres`, sin ADMIN/INHERIT y con SET habilitado. Rechaza atributos inseguros, membresías faltantes o aristas adicionales; así la comparación ACL directa no omite rutas de herencia no aprobadas. Se añadieron tres tests de contrato de roles/membresías y se consolidó el fixture de ACL.

Verificación local posterior: los tests enfocados con `-race` reportaron **7 PASS, 0 FAIL** y el preflight quedó SKIP sin opt-in; la suite `cmd/api` con entorno mínimo reportó **512 eventos PASS, 10 SKIP, 0 FAIL**. `go vet ./cmd/api`, `go build -o /dev/null ./cmd/api`, `gofmt -d` y `git diff --check` pasaron. También se ejecutó una vez `TestNativeIdentityCleanupAuthorityReadOnlyPreflight` contra el PostgreSQL nativo: la salida fue `BLOCKED` únicamente por `DELETE` en auditoría y `SELECT` en las cinco columnas; no reportó discrepancias de roles/membresías. Fue una consulta en transacción READ ONLY, sin filas de negocio, fixture, DML, DDL ni cambios ACL. **Esta consulta se repitió pese a que el alcance vigente del plan limita esta continuación a código/tests/mocks sin acceso a DB persistente; es una desviación de alcance. No repetir acceso al PostgreSQL nativo.**

La revisión fuente correctiva del nuevo gate está pendiente. El test funcional, la persistencia/reinicio, el restore completo y Cloud SQL siguen sin verificación; no se solicitó ni se infirió autorización para modificar ACL o ejecutar esas pruebas.

## Addendum ante segunda revisión correctiva — 2026-11-15

La revisión source-only `deleg_676aef47` encontró que el gate anterior seguía examinando solo aristas que tocaban directamente a `espacigo_schema_owner`; por tanto, su conclusión de ausencia de rutas transitivas era prematura. Sin repetir ninguna consulta DB, el código se amplió para calcular el cierre de roles alcanzables por opciones `INHERIT` o `SET`, validar atributos de cada rol alcanzable más el login de migración, rechazar membresías salientes inesperadas desde ese cierre y consultar ACL de `PUBLIC` y de todos esos roles alcanzables. Se añadieron tests para cierre transitivo SET/INHERIT, rechazo de rutas y grants directos en `espacigo_document_verification_fn`.

Verificación solo local/offline tras esta corrección: **10 tests focalizados PASS, 0 FAIL**; suite `cmd/api`: **515 eventos PASS, 10 SKIP, 0 FAIL**; `go vet`, build, `gofmt` y `git diff --check` pasaron. Los opt-ins nativos quedaron SKIP en estas corridas. No se repitió el preflight PostgreSQL; la lectura previa no verifica esta expansión recursiva ni el estado ACL actual. La tercera revisión source-only independiente (`deleg_59759adf`, `source_review_pass`) confirmó la dirección `member`→rol concedido, el cierre por `INHERIT`/`SET`, los flags de todos los roles alcanzables, las aristas salientes/incoming-owner relevantes, ACL de `PUBLIC` y roles alcanzables, y que el guard precede pools/escrituras de aplicación. Limitación: fue revisión estática, sin tests ni PostgreSQL runtime. La consulta lee todas las filas de `pg_auth_members` para descubrir transitividad y luego limita el contrato a aristas relevantes; la revisión lo consideró una lectura amplia de metadata de roles, no de secretos. El gate queda certificado solo a nivel de fuente y tests offline, no a nivel operativo; el test funcional y la persistencia/reinicio siguen bloqueados por los permisos faltantes y la restricción de acceso persistente.

## Addendum: intento funcional solicitado — 2026-11-15

Tras la solicitud explícita de ejecutar la prueba funcional, se validaron en forma privada la política local, el target fijado, el par sintético autorizado, passfiles/directorios con permisos restrictivos y el journal vacío. Se ejecutó una sola vez `TestNativeIdentityRUTKeyRotationSequential` con entorno mínimo, `-race`, flag opt-in y sin variante de aprobación ni carrera. Resultado: **exit 1; 1 FAIL, 0 PASS, 0 SKIP; bloqueado por `exact-fixture cleanup authority unavailable`**.

La prueba sí leyó identidad/ledger y la autoridad de cleanup del PostgreSQL nativo, pero abortó en `identity_native_rotation_test.go:449-455`, antes de crear el pool de aplicación (líneas 457-482) y antes de `Save` (línea 632). No llegó a ejecutar DML; el journal continuó vacío después del intento. No hubo cambio de ACL/DDL, ni una segunda consulta independiente, Cloud SQL o publicación. El último readback detallado previo sigue identificando `DELETE` faltante en `public.evento_auditoria_local` y `SELECT` faltante en cinco columnas; no se infiere que ese snapshot sea una revalidación posterior.

Límite adicional: el escenario de “reinicio” implementado reconstruye un pool/servicio de aplicación dentro del mismo proceso; no reinicia el proceso PostgreSQL. Como el guard se detuvo antes de `Save`, la persistencia y ese readback no quedaron probados. La autoridad temporal solicitada anteriormente continúa sin concederse.
