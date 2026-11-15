# Propuesta acotada — transición nativa V76→V85

Fecha original: 2026-11-07. **Propuesta histórica de preparación/revisión**, subordinada a `cloudsql_migraciones_incrementales.md`; no es otro runbook Cloud SQL. La autorización y ejecución posterior del destino nativo se registran al final.

## Destino y evidencia

Único destino de esta propuesta: PostgreSQL nativo `127.0.0.1:5432/espacigo_dms_stage_20261020`, motor 18.6, administración peer mediante `sudo -u postgres`. No Cloud SQL, contenedores, F5, wizard ni demos retenidas.

- Ledger READ ONLY 16:49:29 -03: V1–V76, 76 nombres/checksums coincidentes con el candidato; V77–V85 pendientes. `evidence_native_candidate_v85_20261107.json`.
- Roles 16:41:23: runtime LOGIN restringido; faltan API login, migrator login, schema owner y helper. `evidence_native_authority_20261107_tarde.json`.
- Conexiones 16:52:46: runtime=0, api_login=0 y memberships runtime=0. Snapshot sin textos SQL ni filas de negocio; no garantiza ausencia de futuros reconectores ni autoriza parar procesos ajenos.
- Fuente V81 exige runtime NOLOGIN y las identidades/memberships exactas antes de aplicar. V85 no puede reparar ese requisito anterior ni justifica elevar runtime.

## Transición reproducida, solo sintética

El harness ahora reproduce el predecesor V76 con **solo runtime LOGIN**, sin API/migrator logins. Aplica el grant routine versionado para el perfil preexistente, conserva el prefijo y aplica V77–V80 con administrador del fixture. Cambia runtime a NOLOGIN preservando su OID y grants; crea API/migrator logins separados y la membership API→runtime con ADMIN=false/INHERIT=false/SET=true. Comprueba conexión del nuevo API login, SET ROLE y lectura sin filas. Luego provisiona schema owner/helper y aplica V81→V85 por las fronteras revisadas.

Resultado: `bash scripts/test-migration-runner-postgres.sh` **96 resultados requeridos PASS, sin skips**, incluido `upgrade-v76/native-role-transition-contract`. Último contenedor `espacigo-migration-ci-1537475`, imagen PostGIS 18 fijada, sin red/puertos/volúmenes, tmpfs/socket privado; retirado y ausencia verificada. El RED previo rechazó evidencia sin ese caso obligatorio.

**Límite:** el ensayo usa autenticación trust en un socket privado sin red. No prueba SCRAM/TCP, passfiles ni configuración de los consumidores nativos. No se cambia ni renombra el rol persistente, no se leen hashes/passwords y no se aplica un bootstrap de otro destino.

## Gates antes de una ejecución nativa

1. Completar revisión de fuente/consumidores y fijar SHA e inventario exactos. Mantener V1–V84 intactas; V85 candidata de seguridad no equivale a expand universal.
2. Inventariar configuración pública/launcher del consumidor local y preparar su cambio de usuario de conexión a `espacigo_api_login`, manteniendo `SET ROLE espacigo_runtime`. No imprimir ni leer credenciales existentes. Revisar el mecanismo privado para **credenciales nuevas**, sin resetear la contraseña anterior ni modificar cuentas de producto.
3. Revalidar destino/roles/owners/ACL/memberships y ledger bajo el lock del migrador. Falla ante cualquier drift: no adaptar el contrato a observaciones inesperadas ni crear/alterar roles por nombre para hacer pasar una comprobación.
4. Crear backup custom-format privado y off-tmpfs del destino exacto, validar archivo/TOC y parse completo antes de DDL. No restaurar sobre la base compartida ni copiar datos locales a Cloud SQL.
5. Revisar un bootstrap **versionado y específico de este destino nativo**, independiente de V85. Debe conservar OID/grants del runtime y ownership de tablas/datos; permitir solo los roles/memberships/ACL exactos del contrato V81, sin superusuario/CREATEROLE/CREATE DB para migrador o API. No implementar secretos en SQL versionado, argumentos, URLs o logs.
6. Preparar y comprobar el nuevo login API mediante el mecanismo privado revisado, en READ ONLY y con lectura sin filas; adaptar el consumidor controlado. Solo entonces convertir runtime a NOLOGIN con ausencia de sesiones/reconectores propios confirmada y revisión del impacto. No terminar sesiones ni deshabilitar una conexión ajena por inferencia.
7. Aplicar V77–V80 con la fase administrativa acotada del runner versionado; provisión y guard V81; después V81 y ventanas V82–V83 de CREATE/owner/grant option con revocación/readback. V84–V85 requieren CREATE y REFERENCES(id) efectivos; V85 aborta sin borrar si hay filas legacy. No omitir versiones, alterar ledger o reescribir SQL histórico.
8. Postcheck V85: nombres/checksums, objetos, permisos, revocación de ventanas y consultas API pertinentes. Sin levantar API con workers ni declarar el flujo KYC listo. Conservar backup y evidencia, registrar límites de autenticación y recuperación.

## Alcance de autorización y límites

Al redactar esta propuesta, el alcance vigente solo permitía preparación local y no autorizaba deshabilitar el login persistente usado por clientes. La autorización específica recibida el 2026-11-08 para el destino nativo y la transición de roles/configuración se registra en la revalidación siguiente. La publicación DB/pipeline y cualquier despliegue KYC siguen fuera de ese alcance; no publicar cambios KYC parciales del worktree como parte de esta reparación.

Al redactarse esta propuesta no existía un ejecutor revisado ni un bootstrap habilitado. La implementación posterior y la aplicación autorizada se registran al final; no se usó F5/wizard ni se ejecutó como superusuario para saltarse V81.

## Revalidación pre-DDL después de autorización — 2026-11-08 (histórica)

El usuario autorizó específicamente el destino PostgreSQL nativo `127.0.0.1:5432/espacigo_dms_stage_20261020`, incluida la transición de roles/configuración, backup y aplicación secuencial V77–V85; no Cloud SQL ni despliegue KYC. En el preflight inicial se observó V1–V76 y V77–V85 pendientes; esa instantánea quedó superada por la aplicación posterior.

En ese corte previo a DDL, PostgreSQL era 18.6 y el ledger V1–V76 continuo coincidía con los nombres/checksums del candidato; `espacigo_runtime` seguía LOGIN y faltaban los cuatro roles requeridos por V81. No se detectaron sesiones cliente en el clúster durante esa lectura. El backup custom privado pasó `pg_restore --list` y parseo completo; no fue un ensayo de restore ni contenía globals/roles.

En ese corte el gate persistente seguía abierto y no se había ejecutado DDL; el runner, credencial dedicada y ventanas temporales se prepararon y probaron después antes de aplicar. No se usó el modo F5/Cloud ni SQL manual.

## Resultado de aplicación — 2026-11-08

La ejecución autorizada completó V77–V85 en el PostgreSQL nativo, por orden y con backup validado antes de cada fase. El ledger actual fue releído como V1–V85 continuo, con nombres/checksums coincidentes y sin pendientes; las ventanas CREATE/REFERENCES quedaron revocadas y los owners, membresías, OID del runtime y grants de columnas pasaron el postcheck. El detalle de evidencia, checksums y backups está en [`evidence_native_v76_v85_preflight_20261108.md`](evidence_native_v76_v85_preflight_20261108.md).

Esto no incluye Cloud SQL ni despliegue KYC, no demuestra el flujo KYC end-to-end y no cierra la auditoría semántica (`semantic_audit_closed=false`).
