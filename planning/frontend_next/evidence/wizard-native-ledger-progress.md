# Ensayo wizard: ledger y bloqueo real en V60

Estado observado: 2026-11-06 21:17:36 -03:00. Checkout `codex/frontend/fix/mejoras-creacion-espacios`, base `771134d165046b290b1f5ad3a1074a6357c2327f`; árbol de trabajo con cambios locales, no una release publicada.

## Autorización y destino

El usuario autorizó explícitamente la preparación versionada del ledger vacío con propietario `postgres`, SELECT/INSERT para el rol efectivo migrador, el contrato mínimo previo a V81 y la aplicación V1–V83 mediante identidad migradora limitada, exclusivamente en `espacigo_wizard_20261106`.

Destino: `127.0.0.1:15436` mediante proxy TCP Python propio hacia el contenedor aislado `espacigo_wizard_20261106-db`. Sin SSH, bastión, Cloud SQL, base persistente ni recursos de demos ajenas.

## Evidencia ejecutada por el padre

- `002_empty_ledger.sql`: revisión local y cinco pruebas estáticas aprobadas. Preview transaccional terminó en ROLLBACK; lectura independiente confirmó ausencia del ledger. Después, ejecución real terminó en COMMIT y otra conexión confirmó cero filas, propietario `postgres`, únicamente SELECT/INSERT no otorgables a `espacigo_schema_owner`, grantor `postgres`. El preview no se cuenta como commit.
- Las 83 migraciones coinciden byte a byte con el manifiesto independiente BEFORE conservado fuera de Git. Se congeló el manifiesto completo y se prepararon copias privadas del prefijo V1–V80 para detenerse antes de la transición administrativa V81. No se alteró `db/migrations`.
- Backup custom nuevo anterior a las migraciones: 9.299 bytes, SHA-256 `70c87dbaa2abfa25effc42b56ee16ff6c48e6151279cc4c5805637c50fe237dc`; `pg_restore --list` y parseo completo hacia `/dev/null` aprobados. Artefacto privado excluido bajo `.local/testing/wizard-improvements/`.
- Binario construido desde el árbol revisado: `cmd/dbmigrate`. Login `espacigo_migrator_login`, `SET ROLE espacigo_schema_owner`, passfile privado; nunca API como administrador.
- Primer commit V1 confirmado por `-plan-only` desde conexión nueva: applied `[1]`, 82 pendientes.
- El intento de ensayo de interrupción por advisory lock no observó el bloqueo esperado y falló su aserción. **No es evidencia aprobada de interrupción controlada.** El migrador alcanzó V59 antes de fallar por permisos en V60. Una nueva ejecución del runner reprodujo `execute migration V000060 ... ERROR: permission denied (SQLSTATE 42501)`.
- Lectura administrativa explícita READ ONLY posterior: 59 entradas, min=1, max=59, 59 versiones distintas. La función `public.reject_gadm_bootstrap_identity_change()` y el trigger `usuario_gadm_bootstrap_identity_immutable`, creados por V60, están ausentes. Esto confirma ausencia de esos efectos de V60; no demuestra todavía recuperación exitosa ni todo su delta.

## Estado y límites

V1–V59 están comprometidas; V60–V83 permanecen pendientes. El diagnóstico de la sentencia/autoridad concreta de V60 está abierto. No elevar roles, improvisar grants, editar bytes de migraciones ni utilizar el bypass F5 para resolverlo.

`003_pre_v81_contract.sql` es un checker de solo lectura. `004_prepare_v81.sql` es preparación mutante propuesta y continúa en revisión; no se ejecutó y requiere V80 verificada. No confundir pruebas estáticas con validez de ejecución PostgreSQL.

ACL runtime no aplicada. API, Next nativo y cuentas sintéticas todavía no iniciados para este ensayo. Las pruebas frontend controladas no prueban este destino. No se realizó commit/push, integración a main ni despliegue de este conjunto.

## Addendum: autorización administrativa y guard V81

- El usuario autorizó posteriormente una ruta administrativa versionada y exclusiva para V60, V64 y V66 en este mismo ensayo. El resto mantiene el migrador limitado. La autorización exige revisar owners/SECURITY DEFINER, backup y readback; no autoriza elevar roles de aplicación ni utilizar el bypass F5. Implementación y revisión de esa ruta todavía pendientes de cierre; no ejecutada.
- `004_prepare_v81.sql`: corregidos los límites de los bloques DO. El padre repitió cuatro pruebas estáticas y comparó los 80 checksums incorporados con las fuentes; todos coinciden.
- Prueba negativa real del bootstrap V81 sobre el estado V59: rechazó el prefijo antes del DDL de helper. Una conexión independiente READ ONLY confirmó 59 versiones, max=59 y ausencia del rol helper y su esquema privado. Esto verifica ese guard negativo, no una preparación V81 aprobada ni un commit de V81.

## Addendum: rollback nativo aprobado y avance hasta V81

- La prueba opt-in `TestWizardAdministrativeV60RollbackNative` se ejecutó realmente y pasó: SQL original V60, normalización de owners y comprobaciones transaccionales, fallo inyectado antes del registro y rollback real; una conexión nueva confirmó prefijo V59 y reversión de objetos, comentario, membresías y ACL. Los primeros intentos fallaron antes del DDL por consultas del test y no se cuentan como rollback aprobado.
- Después se aplicaron V60, V64 y V66 por la ruta administrativa exclusiva autorizada; los objetos nuevos se normalizaron a `espacigo_schema_owner` dentro de la misma transacción, antes del ledger y COMMIT. Readbacks separados confirmaron esos owners, flags SECURITY DEFINER esperados y ausencia de elevación de los roles de aplicación. V61–V63, V65 y V67–V80 se aplicaron con login migrador y SET ROLE limitado.
- Backup previo a operaciones administrativas sobre V59: 471.380 bytes, SHA-256 `144fb3b87497ff4de51f767e31df861cb45f5b98e15ce9faed1179df89dd0455`, listado y parseo completo aprobados.
- Backup V80 previo al bootstrap V81: 614.565 bytes, SHA-256 `7edfb8254280e086b0fd852c07fa455b5468705fafef2bb90a42e95976f8f4c0`, listado y parseo completo aprobados.
- `004_prepare_v81.sql`: preview real con rollback y verificación independiente de reversión del helper; luego COMMIT real y checker `003` desde otra conexión aprobado. El checker read-only que termina en ROLLBACK no sustituye ese commit.
- V81 se aplicó con migrador limitado. V82 falló por falta de CREATE en public y no se registró. Plan independiente posterior: 81 versiones aplicadas y pendientes `[82,83]`.
- El usuario autorizó posteriormente CREATE temporal en public exclusivamente a `espacigo_schema_owner` para V82–V83, mediante preparación versionada y revocación inmediata verificada tanto al terminar como al fallar. SQL y runner de cleanup están en implementación/revisión; el permiso todavía no se concedió.

## Addendum: V83 y ACL runtime comprometidas

- `005` pasó preview PostgreSQL con rollback después de corregir sus identidades a las cuatro funciones reales de V81. Diez pruebas del runner aprobaron; adicionalmente SIGTERM real en un harness sin PostgreSQL comprobó invocación de cleanup durante llamadas simuladas de concesión/aplicación.
- Backup V81: 621.520 bytes, SHA-256 `ecd6418bd896246a2e16a05185c665bada697abebfceee32c2abd7979ac06743`, listado y parseo completo aprobados.
- Prueba negativa real de cleanup: GRANT temporal comprometido y observado en una conexión independiente, fallo intencional antes del SQL de migración, revocación mediante finally y readback independiente `81|false|false` (ledger, CREATE owner, CREATE runtime). No se confunde este fallo con aplicación migratoria.
- Ejecución real posterior del runner: V82–V83 aplicadas por identidad migradora limitada, plan completo comparado con el manifiesto independiente y sin pendientes, revocación de CREATE realizada. Conexión administrativa nueva READ ONLY: 83 filas, min=1, max=83; CREATE false para schema-owner, runtime y API.
- El primer intento ACL posterior rechazó una expectativa de rol helper mal escrita antes de conceder permisos. Se corrigió exclusivamente el identificador a `espacigo_document_verification_fn`, sin crear roles alternativos ni elevar permisos.
- Ejecución ACL real posterior: `ACL completed: ledger 1..83 verified; repository runtime grants committed and independently read back`. Contrato de roles post-V81 y readback core se verificaron antes y después del commit.
- API worker-free, cuentas sintéticas y recorrido nativo visible siguen pendientes. No hay publicación ni aceptación online de estas mejoras.

## Addendum: servicios y cuenta sintética operativos

- La API worker-free se lanzó en `127.0.0.1:18086`. El diagnóstico con el entorno exacto del launcher verificó `database=espacigo_wizard_20261106`, `session_user=espacigo_api_login`, `current_user=espacigo_runtime`. Health/live y ready respondieron 200. Se mantuvo el guard seguro de passfile; se retiró únicamente escritura de grupo (0775→0755) del directorio del worktree propio que impedía su recorrido seguro.
- Next standalone sirve por HTTPS en `127.0.0.1:4206`, mediante proxy local al proceso interno `127.0.0.1:4209`, con entorno sin credenciales PostgreSQL y API existente `18086`. Health y acceso respondieron 200; catálogo protegido 401 sin sesión.
- Mailpit aislado usa la imagen fijada `axllent/mailpit@sha256:b68349e3a014b90c5610bfb26b2ae36f3892d7b8cf25ee140c6c71c98d2fcf48`, etiqueta del ensayo y red interna existente. Forwards propios limitados a `127.0.0.1:18087` HTTP y `127.0.0.1:11036` SMTP; no acceso externo ni reutilización de correo ajeno.
- Tras revisar la política privada del checkout correcto, se creó exactamente una cuenta sintética por registro normal con los dos consentimientos activos obtenidos del API. Recibo durable privado 0600 antes del POST, entrega de exactamente un correo al destinatario propio y POST normal de verificación 204. Inventario y recibo verificado permanecen exclusivamente bajo `.local/testing/`; no se reproducen identificadores ni credenciales.
- Comprobación independiente de acceso: login 200, GET sesión 200 sin modo restringido, logout 204. Otra sesión diagnóstica obtuvo catálogo 200 y perfil de inmuebles 200; se cerró también con 204. Es evidencia de API real, no todavía de creación visible en el wizard.
- El test nativo hora/día/mes continúa en revisión antes de mutaciones: fixtures/rutas corregidos, pendiente recibo previo por intención para impedir duplicados si un resultado queda incierto. No se ejecutó todavía creación nativa de espacios ni despliegue de estas mejoras.
