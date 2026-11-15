# N02/N03 — Cuenta sintética y persistencia KYC nativa (2026-11-09)

## Destino y alcance

Rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8`, cambios de trabajo sin commit. Único destino: `127.0.0.1:5432/espacigo_dms_stage_20261020`, OID 16388, PostgreSQL 180006, ledger V1–V88. No Docker/bases efímeras, Didit externo, Cloud SQL, API general, workers, publicación ni despliegue.

## Entrega reemplazada y cuenta real de ensayo

La delegación de `cmd/kyc-native-fixture` no entregó CLI funcional y retiró esa carpeta. El padre implementó un **test opt-in aislado**, no una CLI ni un nuevo perfil general de API:

- `cmd/api/identity_native_fixture_test.go`: `TestNativeIdentityPrepareAuthorizedFixture`.
- `AuthenticationService.Register` y `VerifyEmail` reales, repositorio PostgreSQL de identidad, bcrypt/generador normales y mailer de captura en memoria; ningún SMTP externo.
- Consentimientos desde `identity.PublicConsentDocuments()` y validación normal del servicio contra su repositorio. La supuesta imposibilidad de consultar el consentimiento como runtime quedó refutada por el registro/verificación reales: **no se concedieron permisos adicionales de consentimiento/registro**.
- Una sola cuenta nueva del corte, dominio sintético permitido; estado activo leído de DB. Política/inventario y receipt/credencial permanecen privados 0600 bajo `.local/testing/`, excluidos de Git; aquí no se copian correo, UUID, contraseña ni tokens.
- Snapshot de intención antes de Register y receipt con ID antes de VerifyEmail; no reintentar Register si un ID conocido desaparece, ni adoptar un correo existente sin intención registrada. Reuso de la cuenta activa por lectura, sin login POST ni reset/recovery. El límite privado sigue siendo máximo dos cuentas; solo una se creó.
- El test de preparación pasó al crear/verificar y volvió a pasar reutilizando esa misma cuenta.

Comando de preparación desde la raíz del checkout:

```sh
IDENTITY_DECLARATION_NATIVE_PREPARE_FIXTURE=1 GOTOOLCHAIN=local \
  go test -v -count=1 ./cmd/api -run '^TestNativeIdentityPrepareAuthorizedFixture$'
```

No crear otro fixture por inferencia ni borrar el receipt para forzar un alta.

## Recorrido de declaración efectivamente ejecutado

`TestIdentityDeclarationRuntimePostgreSQLSaveRestartReadback` **PASS, con -race**, autenticando TCP como login API y haciendo SET ROLE runtime en cada pool. Se ejecutó de nuevo después de endurecer guards y volvió a pasar:

1. Preflight READ ONLY autenticado por el migrador: target/OID/motor/roles y todos los nombres/checksums V1–V88.
2. Política privada con ID propio marcado verified; existencia y estado activo comprobados como runtime.
3. Handler real de declaración → servicio real → adaptador PostgreSQL, POST sintético separado de cualquier inicio Didit.
4. Respuesta 201, correlación, ID/revisión y campos normalizados comprobados sin imprimirlos.
5. Fila leída como runtime: ciphertext opaco de longitud válida, sin nombre/RUT en claro.
6. Nuevo pool y nuevo handler/servicio → GET 200 → mismos ID/revisión/campos/consentimiento que el guardado.
7. Cleanup con migrador→schema_owner y transacción propia, limitado a cuenta/declaración/correlación del ensayo. Conteos restantes verificados a cero.

**Límites:** es reconstrucción de pool/handler, no reinicio de un proceso completo. La autenticación HTTP del handler se inyecta para la cuenta sintética; no demuestra JWT/sesión/BFF ni recorrido visual. No se arrancó `cmd/api`. Aislamiento entre dos cuentas en PostgreSQL y rotación nativa de clave aún no se ejecutaron en este ensayo; su cobertura previa de unidad no se transforma en evidencia nativa. N03 no se cierra íntegramente mientras falte aislamiento.

## Fallo real y corrección de limpieza

El primer ensayo guardó/releyó correctamente, pero falló al limpiar con SQLSTATE 42501 sobre `evento_auditoria_local`; no se reportó como PASS. La tabla es de postgres y schema_owner carecía de SELECT/DELETE. No se elevó runtime ni se ejecutó limpieza de negocio como superusuario.

Antes de la ventana de ACL se creó backup custom privado `.local/testing/kyc-native/pre-fixture-cleanup-20261109.dump`: **8,290,536 bytes**, 0600, SHA-256 `f84109afc144b3be66d4d010dca65581473774ccb1721c6206b5221197858c8e`. Listado y parse completo pg_restore exit 0; no restore real ni backup de roles globales.

Preparación/finalización versionadas:

- `db/repair/prepare_native_kyc_fixture_cleanup.sql`: guard de destino/OID/motor/administrador/ledger88 y ausencia previa de los permisos; concede temporalmente SELECT solo de `actor_id,recurso_tipo,recurso_id,accion,correlacion_id` y DELETE sobre esa tabla al schema_owner.
- `db/repair/finalize_native_kyc_fixture_cleanup.sql`: retira exactamente esos permisos y comprueba ausencia efectiva, ejecutado por trap del shell principal tanto en fallo como en éxito.
- **Riesgo acotado, no RLS:** DELETE es permiso de tabla durante la ventana; el límite de filas lo aplica el cleanup revisado por cuenta/declaración/correlación, no una política SQL por fila. No dejar abierta la ventana ni presentarla como ACL de producción.
- Un primer intento de retirar el residuo usó un nombre de tabla erróneo; falló al primer DELETE, la transacción se revirtió y el finalizador retiró permisos. Se releyó V85/cleanup y se usaron los identificadores reales `identity_provider_event` → `identity_provider_attempt` → `identity_declaration_revision` y acción `kyc.identity_declaration.save`.
- Se retiró únicamente una declaración y su evento de auditoría del fixture propio del primer ensayo; se conservaron la cuenta, su registro/verificación y cuentas/datos ajenos. Luego el ensayo completo pasó.
- Se agregó guard previo de autoridad de cleanup **antes de cualquier POST/declaración**. Prueba negativa nativa sin permisos: rechazo esperado, sin escritura de declaración. Prueba positiva con preparación/trap: PASS.

No correr el test de persistencia sin esta preparación/finalización ni conceder privilegios permanentes para evitarla. Los DSN/ID de opt-in se cargaron programáticamente desde configuración/política local sin imprimirlos; API y migrador usan passfiles explícitos distintos.

## Readback independiente tras el cierre

Conexión administrativa nueva, transacción READ ONLY, solo conteos/metadata sin imprimir valores privados:

- Revisiones propias restantes: **0**.
- Intentos de proveedor propios restantes: **0**.
- Auditorías propias de guardado de declaración restantes: **0**.
- DELETE temporal schema_owner: **false**.
- SELECT de las cinco columnas temporales: **false**.
- Owner CREATE: **false**; runtime EXECUTE de purga: **false**.
- Ledger: **88 filas / máximo 88**, sin aplicar V89.
- Política, receipt y passfile API: 0600; receipt ignorado por Git.

## Regresión fuente y pendientes

Pasaron `go test -race -count=1` sobre `cmd/api`, `cmd/dbmigrate`, `internal/migrator`, `internal/adapters/postgres/identitydeclaration` e `internal/verification/...`; también vet y diff-check. Integraciones opt-in omitidas en esa suite default no se cuentan como pruebas DB; las nativas positivas/negativa de arriba se invocaron explícitamente.

Tests de conductor comprueban receipt pendiente antes de VerifyEmail y conservación ante fallo de verificación. Tests de resume rechazan adoptar otra identidad, ID cambiado o recrear un ID conocido ausente. La revisión independiente de estos cambios está en curso; no se afirma aprobada todavía.

Siguiente dependencia: completar N03 aislamiento nativo con segunda cuenta sintética autorizada, sin duplicar bases; luego N05 claim RUT atómico con revisión/auditoría, revisión/aplicación de V89 candidata, reconciliación durable y retención/outbox. Persistir una declaración no cierra esos gates ni el producto KYC completo.
