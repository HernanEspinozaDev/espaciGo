# Corrección fase 1 — paleta sobre las vistas Next reales de main

## Estado y alcance

Implementación cromática local verificada. La maqueta anterior sigue **rechazada** y solo se conserva como historia. La aceptación visual del usuario no se deduce de los tests. Revisión independiente recibida: **sin bloqueantes para modo claro**. Cierre técnico de la fase correctiva; no equivale a aceptación visual del usuario.

Referencia: `main` `8a87e38d1b27c384d054ff4bf463c46bdc49c9d2`. Se mantiene **Tailwind CSS 4**; los archivos CSS son su configuración `@theme` y variables semánticas, no una migración a CSS tradicional.

No se modificaron JSX/componentes, layouts, tipografías, tamaños, pesos, radios, espaciados, sombras, breakpoints, lógica de autenticación, Go, SQL ni infraestructura. Se conserva incluso el logo circular actualmente presente en acceso: sustituirlo por la marca real pertenece a la mejora funcional posterior, no a esta normalización cromática.

## Cambios reales

- `frontend/src/app/brand-palette.css`: primitivas cromáticas y aliases Tailwind para las clases gray/orange existentes. Sin layout, tipografía ni selectores de componentes nuevos.
- `frontend/src/app/globals.css`: importación de esa configuración y mapeo de colores semánticos. Se conservan los radios, fuentes y reglas no cromáticas.
- `frontend/src/app/workspace-template.css`: normalización del scope existente, sin cambiar su fuente/radio ni montar ese scope en sitios donde no estuviera previamente.
- `frontend/tooling/brand-palette.test.mjs`: contrato cromático y comparación AST de propiedades no cromáticas frente al SHA de referencia.
- `frontend/tooling/palette-visual.mjs`: capturas y comparación en Chromium aislado sobre las rutas reales, no una imitación HTML.
- `frontend/tooling/palette-fixture-server.mjs`: fixture HTTP local, solo lectura, sin acceso a Go/SQL ni red exterior.
- `frontend/tests/phase5-identity.spec.ts`: expectativa obsoleta de fallback corregida a `/mi-cuenta`. `AccessScreen` en el SHA de referencia ya usa ese destino; no se cambió su lógica. Se mantiene el rechazo del retorno externo y la comprobación de retorno interno permitido.
- `DESIGN.md`: especificación corregida a colores únicamente. Se eliminaron los tokens de tipografía, espaciado y radios propuestos. La versión rechazada y sus exports se archivaron bajo `planning/frontend_next/design/rejected/`.

## Evidencia antes/después

Servidor Next propio: `127.0.0.1:4328`. Fixture SSR propio: `127.0.0.1:18088`. La suite general usa otro servidor efímero en `127.0.0.1:4329`.

Rutas: `/`, `/explorar`, `/acceso`, `/registro`, `/recuperar-acceso`, `/mi-cuenta`; cada una en **390×844** y **1365×900**.

Resultado final real:

- **12 escenarios** comparados.
- **922 elementos visibles antes y 922 después**.
- **0 diferencias geométricas**, con tolerancia de 0,1 px. Se comparan posición, tamaño, fuente, altura de línea, peso, padding, margin, gap, anchos de borde y radios, además de etiquetas/texto estable.
- **0 violaciones Axe** en los doce escenarios; contraste habilitado, sin desactivar reglas.
- **0 desbordamientos horizontales**; no se enmascaró el layout con `overflow-x:hidden`.
- Primario renderizado `rgb(164, 71, 49)` y canvas `rgb(251, 247, 242)` comprobados en los doce escenarios.
- **0 solicitudes externas y 0 errores de consola inesperados**. Se registran por separado cuatro eventos 404 intencionales de recursos ausentes del fixture de cuenta.

Capturas reales inspeccionadas: acceso desktop y cuenta móvil, antes/después. El montaje `comparacion-acceso-main-paleta.png` contiene recortes iguales de las dos capturas; no añade ni sustituye elementos del producto.

Archivos privados ignorados: `.local/testing/ui-ux-main-palette/baseline.json`, `red-comparison.json`, `after.json`, `comparison.json`, `screenshots/` y el montaje comparativo.

## RED → GREEN y problemas detectados

1. Baseline capturado con fuente productiva intacta.
2. La comparación se ejecutó antes de introducir la paleta: falló en los doce escenarios **solo por colores objetivo ausentes**, con cero diferencias de geometría y cero problemas nuevos de Axe. Registro conservado en `red-comparison.json`.
3. Unitarias nuevas antes del cambio: dos fallos esperados por paleta ausente y una comprobación de conservación del diseño aprobada.
4. Tras normalizar la paleta, los doce escenarios pasan sin diferencias de geometría.
5. La suite amplia detectó que el rojo original no alcanzaba contraste sobre crema y que el verde de algunos botones no admitía texto blanco normal. Se usaron los roles semánticos rojos/verdes ya documentados (`#B91C1C` y `#15803D`), sin convertirlos en terracota ni cambiar su significado.
6. Una expectativa antigua de retorno a `/` contradecía el fallback real ya implementado en `main`; se corrigió la prueba, no la autenticación.

## Gates ejecutados

- `pnpm api:check`: tipos, allowlist e inventario de **179 operaciones** actuales.
- `pnpm typecheck`: PASS.
- `pnpm lint`: PASS.
- `pnpm format:check`: PASS.
- `pnpm test`: **59 PASS, 0 FAIL, 0 SKIP**. Persisten avisos Node `MODULE_TYPELESS_PACKAGE_JSON` de módulos TS existentes; no se cambió `package.json` para silenciarlos.
- `pnpm build`: PASS, Next standalone compilado tras los cambios finales.
- `pnpm test:e2e`: **64 PASS, 0 FAIL, 45 SKIP**. Las omisiones corresponden a pruebas de integración nativa Go deliberadamente deshabilitadas; no se presentan como integración aprobada.
- `palette-visual.mjs --stage after`: PASS en los doce escenarios finales.
- `git diff --check`: PASS; sin cambios en componentes/features, `AGENTS.md` o `next-env.d.ts`.

La herramienta MCP de Playwright se intentó usar, pero su distribución Chrome no está instalada en `/opt/google/chrome/chrome`. Se utilizó Chromium instalado por Playwright del proyecto, en un perfil aislado; no se instaló Chrome ni se utilizó el navegador del usuario.

## Reproducción

Desde `frontend/`, con Node/pnpm del proyecto:

```bash
PORT=18088 node tooling/palette-fixture-server.mjs
# En otra terminal propia:
API_BACKEND_BASE_URL=http://127.0.0.1:18088/api/v1 pnpm build
API_BACKEND_BASE_URL=http://127.0.0.1:18088/api/v1 PORT=4328 pnpm start:local
# Baseline se captura ANTES del cambio cromático:
node tooling/palette-visual.mjs --stage before --base-url http://127.0.0.1:4328
# Tras aplicar colores y reconstruir/reiniciar standalone:
node tooling/palette-visual.mjs --stage after --base-url http://127.0.0.1:4328
```

No sobrescribir `baseline.json` con un estado posterior y presentarlo como antes/después. Mantener referencia SHA y mismas respuestas sintéticas.

## Revisión independiente y comprobación final

El revisor confirmó el uso de Tailwind 4, el mapeo correcto entre superficies claras y texto oscuro y la conservación de fuentes/radios/composición. Señaló como riesgo la apariencia de las utilidades gray/orange bajo `.dark`: modo oscuro no está certificado por este corte y no se afirma ausencia de regresiones allí.

Se mantiene deliberadamente el foco azul separado de la selección terracota; no se cambió por una preferencia estética del revisor. El contraste blanco/primario previamente calculado es 5,96:1; no se adoptan estimaciones distintas de la revisión. El revisor citó una utilidad de un estado intermedio: el archivo final verificado contiene `:root` y `@theme inline`, **sin `@utility` adicional**.

El agente principal verificó la versión final y los gates, no solo el resumen delegado. No se requieren cambios adicionales para cerrar este alcance de modo claro.

## Limpieza verificada

Los servidores propios de Next y del fixture se detuvieron por sus identificadores de proceso. Se comprobó que `4328`, `4329` y `18088` quedaron sin listeners. No se tocaron otros procesos, servicios, datos ni volúmenes.

El guard AST usa huellas SHA-256 calculadas y contrastadas contra el SHA de referencia, no `git show` durante los tests: funciona también con checkout shallow o sin `.git` en CI/contenedores.

## Límites

- Catálogo sintético vacío; no se verifica aquí la geometría de tarjetas con inventario/imágenes reales ni la integración de filtros contra Go.
- Modo claro auditado. No se afirma una auditoría completa de modo oscuro, todos los estados interactivos, lector de pantalla o todo WCAG a partir de Axe.
- No se crearon cuentas, consumieron contraseñas/tokens reales, tocaron datos persistentes o hicieron cambios cloud.
- La persistencia HttpOnly/Secure **sigue pendiente de fase 2**; el token legacy en `sessionStorage` no se modificó en esta fase de colores. La aprobación previa del BFF sigue vigente.
- Cambios locales sin commit/push/despliegue en este corte; no confundir capturas locales con una nueva versión publicada.
