# Evidencia — LOCAL-M02-01 (Issue #202)

Estado: corte local aceptado tras PR #203 fusionado; Issue #202 cerrada y Hecho en Projects. Padres #37–#43 y #185/#40 permanecen abiertos. La decisión técnica está en `planning/decisiones_m02_foto_cuenta_cobro_fake.md`.

## Entrega

- V30 incremental crea almacenamiento de metadata para PNG fijo, cuenta de cobro fake, historial y reintentos idempotentes.
- Backend expone rutas autenticadas de foto propia y cuenta fake. La foto se produce con el generador PNG ya existente; el handler rechaza bytes enviados por el cliente. Los archivos permanecen en el storage privado ya configurado.
- El alta/cambio de cuenta fake comprueba KYC sintético efectivo dentro de la transacción y bajo el bloqueo de cuenta compartido con baja/revocación KYC. No se aceptan datos bancarios, RUT ni secretos de adaptador.
- Reemplazo/retiro marca la limpieza de archivo como pendiente; el worker reintenta. La baja encola todos los archivos restantes dentro de su transacción, minimiza referencias de cobro y conserva el historial sintético.
- El ZIP propio añade metadata/historial y los PNG propios todavía disponibles. La prueba comprueba la entrada PNG, la referencia fake propia y la exclusión del hash de credenciales y correo de otra cuenta.
- Mock existente añade generación/consulta/reemplazo/retiro de la foto y alta/consulta/cambio/revocación de la cuenta fake. Conserva claves idempotentes tras errores y descarta resultados/archivos al cambiar o perder sesión.

## Validación ejecutada

Las pruebas PostgreSQL se ejecutaron en la instancia desechable creada por `scripts/test-m04-attributes-postgres.sh`; cada caso migró una base temporal desde cero con V30 y las operaciones de servicio se ejecutaron mediante `SET ROLE espacigo_runtime`.

```text
GO_TEST_RUN='TestSyntheticPhotoPayoutOwnershipIdempotencyAndEligibility|TestPhotoWriteLosesRaceWithSuppressionAccountLock|TestM02LocalSuppressionMinimizesSyntheticAccountAndRecoversFileCleanup' \
  bash scripts/test-m04-attributes-postgres.sh ./internal/m02local ./internal/adapters/postgres/identity
PASS: titularidad, idempotencia/reemplazo/revocación, KYC, ZIP, carrera de escritura con baja y limpieza recuperable de ambos recursos.

go test ./internal/m02local ./internal/adapters/postgres/ownerexport ./internal/dbbootstrap ./cmd/api
PASS

go vet ./internal/m02local ./internal/adapters/postgres/ownerexport ./internal/adapters/postgres/identity ./internal/dbbootstrap ./cmd/api
PASS

npm --prefix mock run build
PASS (TypeScript compilado a internal/mockserver/static/app.js)

python3 -c 'import yaml; yaml.safe_load(open("planning/openapi.yaml"))'
PASS

git diff --check
PASS
```

Esta evidencia de PR conserva el hecho de que no se repitió la suite general. La comprobación posterior a merge se registra abajo.

## Ajustes de revisión #203

- La descarga de PNG vuelve a comprobar token, cuenta y generación de sesión después de resolver `response.blob()` y antes de reemplazar la imagen o metadata. El flujo de generación repite la comprobación después de `loadM02Photo()` y antes de mostrar resultado o mensaje. La generación distingue logout/login aunque vuelva la misma cuenta.
- Foto y cuenta fake usan el sobre común `{error:{code,message,request_id}}`; `request_id` coincide con `X-Request-ID`, y las respuestas 401 incluyen `WWW-Authenticate: Bearer`.
- Pruebas enfocadas: respuesta de PNG retrasada tras logout/login de la misma cuenta; respuesta tardía tras cambio de cuenta; generación nueva aunque token/cuenta se reutilicen; errores 401 y 422 en ambas rutas con comprobación del cuerpo y encabezados.

```text
go test ./internal/m02local -run 'TestM02(PhotoAndPayoutErrorsUseCommonHTTPContract|PhotoEndpointRejectsClientProvidedImageBytes)$'
PASS

npm --prefix mock run test:profile-races
PASS (TypeScript compilado; 25 pruebas del mock, incluidas 3 de sesión/foto)

git diff --check
PASS
```

Estas son pruebas enfocadas de los cambios solicitados; no se repitieron PostgreSQL, suites Backend ni recorrido manual de navegador, y no se tocó el volumen persistente ni los secretos.

## Aceptación posterior al merge #203

- Merge confirmado: `ca64f11860dfd8c60e3998cc2754b3b4915ece26`. V30 se aplicó de manera incremental; se conservaron `espacigo_pgdata`, secretos y datos anteriores.
- Salud local confirmada después del arranque y migración. Se usaron dos cuentas sintéticas independientes: una con KYC efectivo y otra sin KYC.
- Prueba real por API/HTTP: creación de cuenta de cobro sin KYC respondió 409 `eligibility_required`; con KYC se comprobó foto sintética (crear, consultar, reemplazar y retirar) y cuenta fake (crear, consultar, cambiar y revocar).
- El ZIP descargado se abrió y su manifiesto/JSON incluyeron ambas secciones propias y el PNG activo; la consulta de archivo verificó la firma PNG. No hubo proveedor productivo ni movimiento de dinero.
- La aceptación fue registrada y #202 cerrada/Hecho. #37–#43 y #185/#40 continúan abiertas por sus criterios generales.

## Recorrido manual de revisión

1. Aplicar migraciones normalmente con `scripts/dev-env.sh up -d` y abrir <http://127.0.0.1:8081>.
2. Iniciar sesión con la cuenta sintética titular que tenga KYC vigente.
3. En M02, generar la foto, consultar/recargarla, reemplazarla y retirarla. El contenido mostrado siempre lo genera el Backend.
4. Crear y consultar la cuenta fake; cambiarla y revocarla. El mock muestra “ENSAYO LOCAL — SIN TRANSFERENCIA REAL”. Una cuenta sin KYC obtiene el error de elegibilidad al crear/cambiar.
5. Exportar el ZIP propio para revisar la sección `synthetic_profile_photo`, `synthetic_payout_accounts` y `files/profile/`.

No hay proveedor productivo, movimiento de dinero, imagen real ni documento/RUT en este corte.
