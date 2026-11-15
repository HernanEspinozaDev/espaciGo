# N08-C — accesibilidad nativa del formulario (2026-11-09)

## Baseline y límites

- Checkout `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8` más árbol dirty preservado.
- Probe versionado `frontend/tooling/kyc-native-accessibility.mjs`, SHA-256 `d6693c55952bcceb1fac82ae4b1900bc6661bd83bbd44019f36a1a2b4e0ef139`.
- UI nativa `/mi-cuenta/verificaciones` en HTTPS4219, build aislado N07 servido por Next4201 y API harness18090 contra `espacigo_dms_stage_20261020`/OID16388/motor180006.
- Cuenta sintética existente de la política; sin nueva cuenta ni credenciales reales. Formulario poblado con valores sintéticos del código pero no enviado. Ninguna captura de login, correo/password de política, perfil ni navegación privada.
- Avanza C11 solo para esta ruta/estado y tamaños. No es aprobación visual humana ni auditoría de todos los estados futuros del resultado/retorno, ni medición de todos los objetivos táctiles.

## Revisión y preparación

El ejecutor `deleg_783344e2` entregó solo fuente/checks. Padre revisó el archivo y corrigió antes de operar: ausencia legítima del checkbox Didit mientras no hay datos guardados, comprobación de texto además de inputs antes de capturar, espera de fuentes y detección de acrónimos KYC/KYB en encabezados. No se cambió producto ni se suprimieron reglas Axe.

`node --check`, Prettier y ESLint focalizados PASS; ausencia de opt-in rechazada offline con exit2 antes de abrir archivos privados. El probe valida tipo/no-symlink/owner/mode del receipt0600 y parent0700, y solo accede tras preflight Go que comprueba la política.

Preflight nativo READ ONLY PASS. Backup fresco `.local/testing/kyc-native/pre-n08-accessibility-20261109.dump`, 0600, 8,322,217bytes, SHA-256 `08deed47bfcf151640d8956687488f803a0534933c4808d8c0f981e23c924734`. `pg_restore --list` y consumo completo PASS; no restore.

Prepare/finalize versionados por sudo directo parental; el harness mantiene su preflight de cleanup aun cuando no haya guardado. Ejecución `run_kyc_n07_bff.py accessibility` con entorno allowlist. Probe equivalente: `KYC_NATIVE_ACCESSIBILITY=1 node frontend/tooling/kyc-native-accessibility.mjs`, solo después del preflight/harness autorizados. El finalizador directo se ejecutó después de cerrar el harness, sin depender de sudo background.

## Resultados reales

| Vista | Axe completo por defecto | Ancho documento/viewport | Overflow | Copy observado | Mutaciones / tráfico exterior |
| --- | --- | --- | --- | --- | --- |
| 1365×900 | 0violaciones, sin reglas desactivadas | 1365/1365 | No | Sin claims prohibidos ni acrónimos de usuario en encabezados | 0/0 |
| 390×844 | 0violaciones, sin reglas desactivadas | 390/390 | No | Sin claims prohibidos ni acrónimos de usuario en encabezados | 0/0 |

Login200/sesión200. GET declaración404 real antes de llenar cada vista. Consentimiento de declaración no marcado; el de Didit no existe aún en este estado, coherente con los dos pasos. No submit ni llamada de proveedor.

Logout final204, sesión401 y cookies de sesión ausentes. **ok=true, probe_exit=0, harness_exit=0**. `TestNativeKYCAPIWithBFF` PASS6.35s (paquete7.426s con `-race`). Finalizador COMMIT.

## Capturas sintéticas privadas

Guardadas create-only, sin sobrescribir, en directorio propio0700 y archivos0600, solo el `<form>` que contiene los campos de identidad; verificaciones de exclusión de email/password antes de capturar. No son capturas de la pantalla completa ni una prueba del chrome de cuenta.

| Archivo privado | SHA-256 |
| --- | --- |
| `.local/testing/kyc-native/visual/desktop-form.png` | `2dc8c6ac5fecf75e82924d4b0571c99228f5075f68d2de1894674c39cfb9d3b5` |
| `.local/testing/kyc-native/visual/mobile-form.png` | `8f900be491b4c524755469c36278baae7feb6aabe4d62957f6e3156018dec985` |

Inspección visual parental de ambas imágenes: labels/campos/ayudas/consentimiento/CTA legibles; composición de dos columnas en escritorio y apilada en móvil, sin recortes horizontales visibles; sin correo, contraseña ni navegación ajena dentro de la captura. No se transcriben valores de identidad ni se versionan las imágenes privadas. No se inventa aceptación humana.

## Readback independiente y estado

READ ONLY posterior: audit/outbox DELETE=false, owner CREATE=false, audit SELECT=false, ledger90; outbox50/pendientes0, sin crecimiento durante este slice. Preflight fixture posterior PASS, journal de declaración ausente, markers retirados tras validar metadata y API18090 detenida. Ambas cuentas conservadas. Next4201/proxy4219/fakeprovider19091 siguen retenidos solo para N08.

**N08-C completa en este alcance de formulario nativo desktop/móvil. N08 global sigue parcial.** La contención de descubrimiento implícito de credenciales en tests de adaptador continúa separada y bloquea reejecutar esa suite hasta revisión; no se corrió para este slice. Permanecen pendientes los demás criterios de la matriz, incluidos webhook/decisión y estados técnicos. No proveedor externo, worker general, datos reales, publicación, main ni despliegue.
