# Evidencia KYC-08 — contención y seguridad básica

- Fecha: 2026-11-07.
- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- Rama: `codex/marketplace/fix/kyc-06-save-identity-first`.
- HEAD base verificado: `274f37e483a078890e92382a159fa49fd23c2fe8`.
- Estado: cambios locales sin commit ni push; preserva los cambios frontend/plan preexistentes.
- Alcance: Go de verificación Didit y pruebas automatizadas; no se aplicó SQL ni se iniciaron sesiones Didit.

## Cambios

- La creación de sesión ya no fabrica IDs/URLs de Didit. Sin API key/workflow responde 503; fallo o respuesta incompleta del proveedor responde 502; la URL debe ser HTTPS, sin userinfo/puerto y tener host exacto `verify.didit.me` o el host legacy `verification.didit.me`.
- Intentos de este handler llevan el `AccountID` autenticado y la consulta oculta con 404 intentos ajenos.
- Webhook sin secreto configurado falla 503; se eliminó el secreto fallback embebido. Se rechazan firma ausente, timestamp ausente/malformado/fuera de ventana y cuerpo mayor a 1 MiB. `X-Signature-V2` usa HMAC-SHA256 del JSON canonicalizado (orden recursivo de claves por punto de código, Unicode sin ASCII-escape, enteros preservados y floats normalizados); si V2 no valida se prueba el HMAC raw completo (`X-Signature`/`X-Didit-Signature`). Los pares `*-Previous` solo validan con `DIDIT_WEBHOOK_PREVIOUS_SECRET`; la variable opcional no ejecuta ni autoriza una rotación. Se exige frescura en ambos timestamps y, al validar V2, igualdad entre `X-Timestamp` y el timestamp firmado del body.
- Se eliminó la atribución de evento al único intento global. Correlación ausente/mal emparejada no cambia una intención y no se confirma con 2xx.
- `Approved` superior sin objeto `decision` deja el intento `IN_REVIEW`; `EvaluateDecision(nil, nil)` devuelve `IN_REVIEW` y `MISSING_DECISION`, sin panic.

## Verificación ejecutada

Comandos: `go test -count=1 -race ./internal/verification/...` y `go vet ./internal/verification/...`.

Resultado observado: ambos paquetes finalizaron `ok` y `go vet` terminó con exit 0. Incluye regresiones de fallo cerrado para proveedor no configurado/fallido, aislamiento de consulta entre cuentas, secreto ausente, timestamp ausente/malformado/vencido, correlación desconocida, aprobación sin decisión y decisión nula; canonicalización con orden de claves Unicode, enteros grandes y floats; golden V2 sintético; fallback raw completo; secreto de rotación previo opcional y binding de timestamp V2. El golden se calculó localmente con un secreto sintético; no es vector publicado ni entrega de Didit.

Comando: `git diff --check`

Resultado observado: exit 0, sin salida.

## Límites abiertos

- Es evidencia unitaria Go con almacén en memoria y un proveedor HTTP sintético; no prueba persistencia, reinicio, múltiples instancias, inbox durable, deduplicación ni transacciones PostgreSQL.
- No se verificó Didit sandbox/live, aplicación/workflow remoto, GCP, release ni interfaz online.
- La rotación externa del secreto previamente embebido en el código no se ejecutó; requiere cambio autorizado del secreto runtime y verificación separada. No se incluye ni reproduce el valor.
- Se implementó la canonicalización V2 indicada en la documentación pública consultada; queda pendiente contrastarla con entrega real sandbox/vector del proveedor y probar todas las variantes en KYC-11. No se ejecutó rotación ni se proveyó secreto anterior. El evento todavía se reconcilia con memoria, no inbox durable; KYC-09/11 siguen siendo gates separados.
- No commit, push, integración a `main` ni despliegue en esta evidencia.
