# Evidencia — VI-04, accesibilidad del formulario de captura sintética

- **Fecha:** 2026-11-11
- **Estado:** subcorte Playwright/Axe local completado y aprobado en follow-up independiente read-only `deleg_5726a0cd` el 2026-11-11; no cierra VI-04.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron cambios ajenos.

## Alcance

- Se añadió `@phase6 @a11y identity capture form is accessible at desktop and mobile` a `frontend/tests/phase6-account.spec.ts`.
- El test autentica con una sesión sintética, crea un caso de identidad mediante un POST interceptado, selecciona `identity_back` y adjunta un PNG con contenido sintético. Un listener registra todos los POST del browser posteriores al login: afirma que el único es `/api/backend/document-verification/cases`; el mock además verifica que no haya POST de upload `/files` ni submit `/submit`.
- Ejecuta Axe sobre el formulario con el archivo seleccionado en 1280×900 y 390×844, y afirma que no hay violaciones ni overflow horizontal.

## Verificación local

- `./node_modules/.bin/prettier --write tests/phase6-account.spec.ts tests/identity-upload-recovery.spec.ts` — exit 0; archivos formateados.
- `./node_modules/.bin/eslint tests/phase6-account.spec.ts tests/identity-upload-recovery.spec.ts` — PASS, sin warnings.
- `./node_modules/.bin/tsc --noEmit` — PASS.
- `./node_modules/.bin/playwright test tests/identity-upload-recovery.spec.ts --workers=1` — **2/2 PASS**; el 409 no reintenta al refrescar, conserva el archivo si el caso sigue editable y solo un clic expreso hace la carga posterior.
- `./node_modules/.bin/playwright test tests/phase6-account.spec.ts -g '@a11y' --workers=1` — **2/2 PASS**; incluye Axe/overflow del formulario en ambos viewports y la regresión de rutas de Mi cuenta.
- Chromium utilizó el Next standalone local en `127.0.0.1:4203`. No se reconstruyó porque este subcorte solo agregó/corrigió pruebas; no cambió código de producción.

## Límites

- El test usa respuestas sintéticas interceptadas en el navegador. El único POST observado crea el caso por mock; la carga del PNG se limita al control local. No demuestra accesibilidad de BFF/Go, persistencia, inspección de archivos ni el recorrido real de carga/envío.
- No se usaron documentos/datos reales. No hubo SQL/DDL, conexión a base, migraciones, Cloud, commit, push, integración a `main` ni despliegue.
- El resultado acredita únicamente la accesibilidad y contención visual de este estado de formulario en las dos dimensiones probadas; la fase VI-04 permanece parcial.

## Revisión independiente

- La revisión read-only `deleg_d2236761` señaló correctamente que la versión inicial no contaba solicitudes y que su redacción no distinguía el POST mockeado de creación del upload/submit. Se añadieron aserciones sobre todos los POST posteriores al login y sobre las mutaciones del mock; la única ruta registrada es la creación. El follow-up `deleg_5726a0cd` aprobó las aserciones, el escenario de refresh editable y los límites documentados; no reportó defectos ni sugerencias.
