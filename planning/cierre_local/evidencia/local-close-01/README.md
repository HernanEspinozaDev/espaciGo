# LOCAL-CLOSE-01 — paridad, contrato y placeholder

Fecha del ensayo: 2026-10-14 (America/Santiago). **HEAD probado:** `726b419824a26a79797561b6b77db6d92af0170e`; rama `codex/marketplace/fix/local-close-01-parity-contract`; autor y committer de los cambios: `HernanMEC`. Base `main` posterior a PR #308: `a26e1dacbe23cbc4b73528c69ee0e6b4daf394ea`.

## Recorrido conectado y aislamiento

Comando ejecutado desde la raíz del repositorio mediante `scripts/test-frontend-e2e.sh`:

```sh
FE14_RUN_ID=726b41991ad6 \
FE14_PROJECT_NAME=espacigo_localclose01_726b41991ad6 \
FE14_STATE_DIR="$PWD/.local/testing/local-close-01-726b41991ad6" \
FE14_PORT_BASE=20750 FE14_PUBLIC_ONLY=1 \
FE14_HEAD=726b419824a26a79797561b6b77db6d92af0170e \
FE14_PUBLIC_TEST_GREP='MPV2-04 host declares a photo public|LOCAL-CLOSE-01' \
FE14_EVIDENCE_OUTPUT_DIR="$PWD/planning/cierre_local/evidencia/local-close-01" \
bash scripts/test-frontend-e2e.sh
```

El runner validó la configuración Compose resuelta antes de arrancar: proyecto `espacigo_localclose01_726b41991ad6`, puertos API/Mailpit/Angular `20750/20751/20752` y volumen nuevo `espacigo_localclose01_726b41991ad6_pgdata`; el puerto SSR fue `20753`. La base vacía recibió sus migraciones. La suite creó cuatro cuentas sintéticas, guardó credenciales en archivo temporal modo 0600 y las retiró con el estado privado al terminar. No usó `espacigo_pgdata` ni volúmenes de demos. Después de finalizar no quedaron contenedores, volumen ni credenciales con este identificador.

Resultado Playwright: **3/3 PASS**.

- `MPV2-04 host declares a photo public; anonymous visitor reads it and withdrawal returns 404`: portada pública y fallback de imagen fallida/retirada en desktop y móvil; API de imagen 200 antes de retirada, 404 después; listado vuelve a placeholder. La petición pública no lleva Bearer.
- `LOCAL-CLOSE-01 deletes only the owner fixture through Angular→Go→PostgreSQL`: fixture KYB sintético, tercero recibe 404 y el propietario retira desde Angular tras confirmar; Go responde 204, la evidencia desaparece de la lista y una repetición devuelve 404. El guard de ruta recibe una sesión real iniciada por el formulario. La UI no persiste tokens en localStorage/sessionStorage.
- `LOCAL-CLOSE-01 missing covers use a solid high-contrast placeholder`: API pública y vista servida, colores computados y capturas en 1365×900 y 390×844.

La cobertura Angular/Go de borrado también incluye cancelar confirmación, errores 401/403/404, aislamiento ante logout/login/cambio de cuenta/caso y respuestas tardías; las pruebas de fachada comprueban revocación de Blob URL. La suite Angular enfocada terminó con 80/80 pruebas. Go `internal/verification/transport/http`, lint/arquitectura/tokens, `api:check` y build Angular SSR/prerender pasaron. Esas comprobaciones se ejecutaron sobre la implementación de producción del mismo corte; los commits posteriores del ensayo solo ajustaron selectores E2E.

## Contraste y capturas

El JSON de contraste conserva estilos computados del navegador: superficie `rgb(245, 238, 230)`, texto/categoría `rgb(51, 39, 32)`, sin degradado y opacidad 1. El cociente WCAG de luminancia relativa calculado para ambos textos es **12,57:1**. Es una medición sobre colores CSS sólidos; no una certificación general de accesibilidad. La suite axe es complementaria y su entorno jsdom advierte que Canvas no está disponible.

Capturas JPEG (desktop 1365×900; mobile 390×844):

- `fe14_local_close_01_placeholder_desktop.jpg`, `fe14_local_close_01_placeholder_mobile.jpg`
- `fe14_local_close_01_placeholder_error_desktop.jpg`, `fe14_local_close_01_placeholder_error_mobile.jpg`
- `fe14_local_close_01_placeholder_retired_desktop.jpg`, `fe14_local_close_01_placeholder_retired_mobile.jpg`

Sus archivos JSON adyacentes registran viewport, dimensiones del documento y SHA. SHA-256 de los JPEG:

| Captura | SHA-256 |
| --- | --- |
| `fe14_local_close_01_placeholder_desktop.jpg` | `6803273c26454dfac08c9efa8dd9904dbbfc3f460abdceba33f1527bb548e841` |
| `fe14_local_close_01_placeholder_mobile.jpg` | `6125d6ffa9d9d424ee80c8b42079d409d71540ad4f943dd7819e88bccc16beec` |
| `fe14_local_close_01_placeholder_error_desktop.jpg` | `66dcad0fdc003539b7ebe36fef313d43a7d777844017f4a5a879a21b2260954e` |
| `fe14_local_close_01_placeholder_error_mobile.jpg` | `ff88fd1caed2884d4bf0c48fe62238c09975eb6dbe311aa204ec47f887e2cced` |
| `fe14_local_close_01_placeholder_retired_desktop.jpg` | `db751c11ee0bee0dbbc4de0ebfd2def9337e4adc3df4ba39e38ce1407aa80afd` |
| `fe14_local_close_01_placeholder_retired_mobile.jpg` | `d4288c9f246b21896255dd6c4befdd34b8536cd26a6928658a93cad80e5d0553` |

Se pueden regenerar con `sha256sum planning/cierre_local/evidencia/local-close-01/*.jpg`.

## Incidencias de la verificación

Los primeros intentos aislados detectaron selectores históricos incompatibles con los nombres accesibles/DOM renderizado y, después, la ruta 503 intencional del fallback seguía activa para una aserción posterior. Se corrigieron los selectores, se recargó el catálogo tras retirar la ruta y se repitió el comando completo; el resultado final anterior pasó. El runner limpió cada proyecto/volumen/archivo de estado de prueba, incluidos los intentos fallidos.

## Límites

No se modificaron migraciones, reglas Go ni OpenAPI: el DELETE propietario ya estaba en el contrato y autorización Go. La retirada borra lógicamente la evidencia del ensayo, no acredita borrado físico o destrucción de respaldos. No se probaron identidad/KYC real, pagos reales, proveedores externos, GCP ni todos los criterios DOCKER-02. El cierre integral y su aceptación humana siguen pendientes.
