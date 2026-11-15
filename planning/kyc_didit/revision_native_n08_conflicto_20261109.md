# N08-G — revisión previa del conflicto al guardar (2026-11-09)

## Baseline y límite

Checkout `espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8` más árbol dirty conservado. Revisión de fuente y preparación local, **sin ensayo nativo de conflicto ejecutado todavía**. No commits, push, migraciones, integración a main ni despliegue.

## Gates de reanudación verificados

- Preflight READ ONLY con los launchers existentes PASS sobre `espacigo_dms_stage_20261020`, loopback5432, OID16388, PostgreSQL180006, ledger90.
- Suite parental sin caché con `-race`: cmd/api, internal/verification/... (incluido crypto/HTTP), adaptador identitydeclaration y fake PASS; vet y diff-check PASS. Legacy native adapter lifecycle sin opt-in, sin ejecutarlos para obtener cobertura.
- Readback administrativo READ ONLY: auditDELETE=false, outboxDELETE=false, ownerCREATE=false, auditSELECT=false; outbox Didit50/pendientes0. No permisos temporales abiertos.
- Backup privado fresco `pre-n08-conflict-20261109.dump`, modo0600, 8,323,117bytes, SHA-256 `4791d3ca67ab3ceda0f0f42a1084846571934d8ec43e4cdde87bc0dd374f6058`; TOC y consumo completo con pg_restore PASS, sin restore.
- Se detectó build frontend obsoleto: el snapshot anterior conservaba copy de conflicto distinto de la fuente actual. Nuevo build aislado `espacigo-kyc-n08g-yy47yl6i` PASS (compilación, TypeScript y generación). Next4201/HTTPS4219/fake19091 arrancados con entornos separados y allowlist, health Next/HTTPS200; API18090 no arrancada. Recursos propios retenidos para este ensayo pendiente, responsable: ejecución parental KYC local. No navegador/perfil del usuario ni proveedor externo.

## Adjudicación de revisión independiente

Revisión `deleg_3e51e4a9`, exclusivamente fuente, sin DB ni cambios:

1. **No se acepta la premisa de que B no pueda tener journal si el guardado termina en conflicto.** La cadena real es handler → DeclarationService.Save → wrapper.CreateRevision → repositorio. El wrapper registra el input antes de delegar, incluso si el repositorio revierte una transacción y devuelve ErrConflict. El test unitario Aexitoso/BErrConflict demuestra precisamente ese límite. No retirar el requisito de journal B para fabricar PASS.
2. El callback de verificación B está registrado después del cleanup; LIFO lo ejecuta **antes** de borrar B/A. Es el orden requerido. No se exige una transacción única entre ambos cleanups de fixtures independientes: cada uno debe validar su journal/scope, confirmar eliminación y mantener journals hasta comprobar el baseline global. Sí se debe reforzar que A permanezca íntegra antes del cleanup y acreditar la causa específica del409.
3. `getByText(string)` usa coincidencia parcial salvo `exact:true`; la objeción a un prefijo por sí sola no prueba un falso positivo. Se conservará o endurecerá la aserción sin atribuir un defecto inexistente.
4. La observación de copy quedó obsoleta: fuente actual usa mensaje neutral `No se pudo registrar este RUT. Revisa los datos ingresados o solicita ayuda.` La aserción del probe todavía espera el texto anterior; corrección test-only delegada por separado, sin volver atrás el producto.

## Brecha real encontrada por el padre

El requisito del plan §23/§24 es unicidad **al guardar**, con declaración/auditoría/claim en una transacción. La fuente actual **no lo implementa**:

- `DeclarationService.Save` pasa solo input de revisión/cifrado a `CreateRevision`, sin fingerprint.
- `identitydeclaration.Repository.CreateRevision` inserta revisión y auditoría, pero no claim.
- `RutClaimStore.Claim` se invoca desde `DiditService.StartOrResume`, después del guardado y en otra transacción.

Por tanto, el B409 esperado por el probe no está respaldado por esta cadena. No ejecutar la mutación para presentar el mismo defecto como un problema de journal ni degradar el test para aceptar201. El cierre histórico N05 conserva su prueba del ciclo durable de claims; **no acredita el guardado atómico**. C09/N08-G permanecen parciales.

## Corrección secuencial autorizada en curso

1. Servicio/runtime: derivar HMAC versionado dedicado desde el RUT normalizado y llevar solo fingerprint en el input de revisión; fallar cerrado si runtime carece de configuración. Pruebas unitarias antes de integración, sin llamada al proveedor.
2. Adaptador: integrar claim con revisión/auditoría dentro de la misma transacción; conflicto revierte todos los artefactos B. No usar un Claim separado que pueda confirmar antes/después del guardado. No editar V89/V90 ni tocar cuentas ajenas/colas expiradas globales para satisfacer el ensayo.
3. Revisar/endurecer harness y causa409, comprobar integridad A y ausencia de artefactos B antes del cleanup.
4. Solo con fuentes congeladas, regresión/revisión y preflight PASS: ventana temporal mínima, ensayo HTTPS/BFF/Go/DB autorizado, cleanup por ambos journals, finalización y readback independiente. Aún pendiente.

## Revisión parental de entregas parciales

- Probe: copy neutral actualizado y revisado; sintaxis/Prettier/ESLint PASS y contrato adaptador7/7PASS. Padre añadió prueba del payload real enviado por B contra la identidad sintética seleccionada (RUT ya normalizado por la UI), sin imprimir el body. No atribuye todavía un409 a una ejecución real.
- Harness: verificación READ ONLY previa al cleanup exige A con su declaración exacta, actor/ID/correlación de auditoría y claim ligado a esa revisión; B sin artefactos. Gofmt/diff-check PASS; aún sin ensayo nativo del cambio.
- Servicio/runtime: primera entrega añade fingerprint y constructor HMAC dedicado, runtime failclosed. La suite parental amplia **falló** en tres lifecycle unitarios de Didit y dos pruebas HTTP de guardado: el constructor legado no configura HMAC, pero Save lo exigía incondicionalmente. No es fallo DB/autenticación ni un PASS integral. Corrección de compatibilidad y validación estricta de versión delegadas; no se modifican los tests de otros consumidores para ocultarlo.
- Adaptador transaccional: implementación acotada delegada sobre input `RutFingerprint` ya existente. Sin call al helper Claim que confirma por separado o barre expirados ajenos. Fuentes compartidas pendientes de congelar antes de regresión parental conjunta; permisos temporales permanecen cerrados.

### Compatibilidad recuperada; prueba transaccional todavía pendiente

Segunda entrega de servicio preservó el constructor legado y dejó el constructor HMAC obligatorio en runtime. Suite parental amplia cmd/api/verificación/crypto/HTTP/adaptador/fake con race y vet PASS al congelarse los archivos. Padre detectó además que la validación estricta del identificador HMAC se había aplicado indebidamente al token de consentimiento: añadió una regresión conductual que falló (tokens históricamente válidos rechazados y longitud21 aceptada), restauró el contrato original de consentimiento y separó la validación HMAC. Después verificación/crypto/HTTP con race y vet PASS.

La entrega del adaptador fue **parcial**, sin las pruebas transaccionales requeridas. Su fallo de compilación durante edición concurrente quedó superado por la suite parental posterior; no se mantiene como bloqueo vigente. Se asignó únicamente el helper transaccional llamado por CreateRevision y sus pruebas conductuales de commit/rollback/error/filas afectadas. No declarar completo el fix por compilar ni abrir ventana nativa hasta verificar esa entrega.
