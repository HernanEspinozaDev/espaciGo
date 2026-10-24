# Inventario de cobertura del rediseño Angular EspaciGo

Fecha de corte inicial: 2026-10-23. Base funcional observada: `0b8548e2dc4ef2ff5d368235087ca0b279296220` (`main`).

Este inventario asigna cada ruta de `frontend/src/app/app.routes.ts` a una sección de producto, sus operaciones visibles y el corte de rediseño que las debe conservar. El inventario de operaciones de API permanece en `matriz_paridad_mock_angular.md`; esta matriz no lo sustituye ni convierte propuestas visuales en capacidad implementada.

## Decisión de diseño que enmarca los cortes

El usuario eligió **A · Editorial funcional** el 2026-10-23 desde `http://127.0.0.1:4173/?direction=A&area=hosting&viewport=desktop&state=content`, sobre la base indicada. La comparación y revisión independiente UX-00/01 está en el worktree local `ux-local-prototypes`; informe SHA-256 `7549d39abd96b965b35cd68cafa94b2e5b92ea77088388decb18b8d34f63ab74`. La dirección A y el conjunto de cuatro contextos fueron aprobados para implementación. B se conserva como referencia histórica de Hosting y Administración global. La elección no representa aceptación final comercial ni cambia contratos, rutas, roles o capacidades.

## Rutas actuales y cobertura por corte

| Sección/rutas actuales | Feature y tareas que deben mantenerse | Destino UX y corte | Estado de rediseño |
| --- | --- | --- | --- |
| `/`, `/categorias`, `/como-funciona` | Portada, búsqueda inicial, ocho categorías, explicación y accesos | UX-03 shell/nav; UX-09 consolidación | Shell integrado en PR #541; UX-09 incorpora cobertura explícita de la ruta pública y wildcard. Revisión online queda al usuario. |
| `/explorar`, `/espacios/:id` | Filtros permitidos, cursor/paginación disponible, fichas públicas, cotización/intervalo según capacidades del API | UX-05 catálogo público y detalle; UX-09 consolidación | UX-05 integrado en PR #544 con capturas sintéticas; sin integración Go en UX-09 ni validación online por el agente. |
| `/terminos-y-condiciones`, `/politica-de-privacidad` | Lectura versionada de documentos/consentimiento y estados de carga/error | UX-04 identidad y consentimiento; UX-09 revisión de lectura | UX-04 integrado en PR #543; cobertura unitaria de documento disponible. Revisión de recorrido conjunto en UX-09. |
| `/acceso`, `/registro`, `/verificar-correo`, `/recuperar-acceso` | Login, creación, verificación/reenvío y recuperación; registro no elige rol/capacidades | UX-04 identidad; UX-09 consolidación | UX-04 integrado en PR #543; conserva flujo público y sesión actual. No se emplean cuentas online para la validación UX-09. |
| `/mi-cuenta`, `/mi-cuenta/privacidad` | Perfil propio, privacidad, exportación/solicitud de derechos, baja y confirmaciones existentes | UX-04 cuenta/privacidad; UX-09 revisión cruzada | UX-04 integrado en PR #543; las rutas permanecen detrás de sesión y sin cambios de autorización. |
| `/mi-cuenta/seguridad` | Cambio de contraseña y resultado de sesión, validación/error | UX-04 identidad/seguridad; UX-09 consolidación | UX-04 integrado en PR #543; no exponer tokens ni aflojar guardas. |
| `/mi-cuenta/verificaciones` | Elegibilidad, casos/historial, evidencia autorizada, retiro y reintento según estados reales | UX-04 cuenta/verificaciones; UX-09 consolidación | UX-04 integrado en PR #543; correo verificado sigue distinto de KYC/KYB. |
| `/panel-anfitrion` → `/panel/espacios` | Alias existente; lista de espacios propios, estado del borrador, selección y primera acción comercial autorizada | UX-03 navegación; UX-03P piloto y UX-05 extensión hosting | UX-03P integrado en PR #542 y extensión UX-05 en #544; conserva `allowFirstBusinessAction` y rol Go. |
| `/panel/espacios/:spaceId/editar` | Datos del recurso, validaciones por campo, guardado de borrador y errores del servicio | UX-03P piloto de Hosting; UX-05 datos/atributos/revisión | Implementado dentro del corte #542; E2E local Go/PostgreSQL registró creación y edición de borrador. |
| `/panel/espacios/:spaceId/tarifas` | Importe, unidad CLP hora/día/mes, cotización simulada y guardado permitido | UX-05 Hosting | Ruta y navegación existentes conservadas por PR #544; unidad CLP y reglas del dominio permanecen sin cambios. |
| `/panel/espacios/:spaceId/calendario` | Zona horaria, disponibilidad, conflictos, bloques y confirmación de guardado | UX-05 Hosting | Ruta y navegación existentes conservadas por PR #544; zona IANA y conflictos Go permanecen sin cambios. |
| `/panel/espacios/:spaceId/galeria` | Portadas, generación/subida, progreso/error/reintento/retiro y límites actuales de limpieza | UX-05 Hosting | Ruta y navegación existentes conservadas por PR #544; retirar foto no se presenta como borrado físico. |
| `/reservas`, `/reservas/:id` | Lista/detalle por actor, estado de solicitud/ocupación/contrato/pago separado, historial, conversación, decisiones y reintentos | UX-06 reservas/conversación/contrato; UX-09 recorridos | UX-06 integrado en PR #545; conserva decisiones y estados Go. Las operaciones siguen cubiertas por sus pruebas de feature/E2E previas. |
| `/reservas/:id/disputas` | Evidencia/finanzas de reserva según datos, permisos, reversa/garantía/liquidación y estados actuales | UX-06 reserva y finanzas | UX-06 integrado en PR #545; la lectura se puede reintentar sin repetir una mutación financiera. |
| `/administracion` | Inicio, perfiles operativos y colas de capacidades existentes | UX-07 administración operativa | UX-07 integrado en PR #546; diez secciones existentes, sin soporte/tickets ni KPI inventados. |
| `/administracion/disputas` | Cola, ficha, evidencia y decisiones/motivos cuando las capacidades vigentes lo permiten | UX-07 administración operativa | UX-07 integrado en PR #546; la vista financiera personal permanece sin cambios. |
| `/administracion/global` | Directorios de cuentas/publicaciones, filtros/paginación, ficha, capacidades por operación, indicadores/auditoría existentes | UX-08 administración global | UX-08 integrado en PR #548; Cuentas, Publicaciones, Indicadores y Auditoría respetan capacidades y revocación. |
| `**` → `/` | Ruta no reconocida y retorno seguro | UX-03 shell/nav; pruebas UX-09 | UX-03 shell integrado en PR #541; UX-09 verifica la presencia de wildcard y su retorno explícito a `/`. |

## Cobertura de acciones por área

| Área | Lecturas/navegación | Edición/acciones conservadas | Estados y riesgos cubiertos | Corte |
| --- | --- | --- | --- | --- |
| Pública/consentimiento | Portada, categorías, cómo funciona, catálogo público, detalle, documentos | Buscar/filtrar, cursor, seleccionar fecha/unidad, consentir donde corresponda | Sin resultados, pendiente/error, ubicación sintética, cotización no equivale a retención | UX-03, UX-04, UX-05, UX-09 |
| Identidad/cuenta | Sesión, perfil, privacidad, verificación, historial, documentos | Registro/login, verificar/reemitir, recuperar/cambiar contraseña, exportar/solicitar derechos/cerrar cuenta | Sesión nueva/restringida/vencida, error, consentimiento, KYC distinto de correo | UX-04, UX-09 |
| Anfitrión | Lista/detalle propio, datos, tarifa, calendario, galería | Crear/editar/guardar borrador, corregir, cargar/retirar archivos, revisar/publicar/ocultar si permitido | Vacío/carga/error/guardado/rechazo/conflicto/pendiente de limpieza | UX-03P, UX-05, UX-09 |
| Arrendatario/reservas | Explorar, cotizar, lista/detalle, conversación, contrato, historial de pago/garantía | Solicitar/aceptar/rechazar/cancelar/reintentar/adjuntar evidencia según actor y API | Cotización vencida, sesión, concurrencia, timeout, estados monetarios y de ocupación separados | UX-05, UX-06, UX-09 |
| Operación/admin | Inicio, cola, ficha e historial operativo | Revisar/decidir con capacidad y motivo, moderar/continuar según acción existente | Sin permiso, revocación, concurrencia, acción sensible, carga/error/resultado Go | UX-07, UX-09 |
| Administración global | Cuentas/publicaciones, propietario, visibilidad, recursos relacionados y auditoría | Solo capacidades explícitas existentes; versionado/idempotencia donde corresponda | Sin privilegios por menú/rol genérico, concurrencia, auditoría atómica, lectura minimizada | UX-08, UX-09 |

## Secuencia por corte y gate

| Corte | Cobertura | Entregable verificable | Gate antes del siguiente corte |
| --- | --- | --- | --- |
| UX-00/01 | Inventario UX y selección A; prototipos A/B iniciales y set A de cuatro contextos | Informe local revisado, datos/estados/rutas contrastados, decisión A registrada | Dirección A aprobada; el conjunto no constituye aceptación final de producto |
| UX-02 | Tokens A, tipografía/espaciado semánticos y primitivas compartidas de estado/acción/foco | Catálogo de tokens, estados, contraste y APIs; regresiones de estilos | Integrado en PR #540; no cambian rutas, permisos ni contratos. |
| UX-03 | Shell, navegación contextual, destino activo, estados de menú, foco/SSR/deep links | Shell reutilizable, navegación por rol/capacidad, teclado y pruebas axe | Integrado en PR #541; UX-09 consolida la cobertura de todas las rutas. |
| UX-03P | Piloto funcional de Hosting: lista, selección, abrir/crear/editar borrador, errores y respuesta Go; tarifas/calendario/galería siguen accesibles | Pruebas Go/PostgreSQL de ensayo, E2E Chromium/Firefox/axe/teclado y matriz de estados | G2 aprobado por `HernanEspinozaDev` en PR #542 sobre HEAD `4f8eeebd0185685a116104dcefd48a082e96b34b`; G2 no equivale a aceptación humana final |
| UX-04 | Identidad, consentimientos, cuenta, seguridad, privacidad, verificaciones | E2E por ruta/estado; sin cambios en SMTP/cookies/consentimientos | Integrado y revisado en PR #543; guards y sesión conservados |
| UX-05 | Hosting completo y rutas públicas de catálogo/ficha | Recorrido borrador→validación→resultado; acciones preservadas | Integrado y revisado en PR #544; contrato y capturas registrados |
| UX-06 | Reservas, conversación, contratos y finanzas personales | Recorridos por actor, estado y errores/timeout | Integrado y revisado en PR #545; montos/historial sin regresión reportada |
| UX-07 | Administración operativa/disputas | Colas y decisiones por capacidad/motivo | Integrado y revisado en PR #546; sin soporte ni rol ampliado |
| UX-08 | Administración global de cuentas/publicaciones/indicadores/auditoría existentes | Pruebas de capacidad por operación, filtros/paginación reales, auditoría | Integrado en PR #548; no inventar métrica/operación ni mutar históricos |
| UX-09 | Consolidación de todas las rutas anteriores | Matriz completa, regresiones responsive/SSR/performance/accessibility y capturas con SHA | Aprobación humana final de la experiencia online; merge/despliegue técnico no la sustituye |

Un corte se considera cubierto cuando registra rutas, fuente→destino de acciones, guard/capacidad, casos de carga/vacío/error/éxito/denegación pertinentes, pruebas y captura/hash. Las pruebas automatizadas no sustituyen la autorización Go, una evaluación con participantes o aceptación del usuario. Mantener fuera del alcance módulos de soporte/tickets no existentes y toda capacidad no presente en el código/contrato vigente.

## Regla de validación online para UX-03P–UX-09

La comprobación de acceso y experiencia en servicios online la realiza exclusivamente el usuario. El agente no inicia sesión ni inspecciona la interfaz, las sesiones o los datos de cuentas online durante estos cortes. La validación del agente se limita a entornos locales desechables con cuentas sintéticas conforme a la política `LOCAL PRIVADA`; sus credenciales permanecen bajo `.local/testing/` y los recursos del ensayo se retiran al terminar. La evidencia que aporte el usuario no equivale por sí sola a aceptación final del producto.

## Baseline de ejecución

- `ng build`: tres rutas prerenderizadas; bundle inicial 453.64 kB. Baseline supera el warning de `hosting-spaces-page.scss` por 87 bytes (9.09 kB frente a 9.00 kB); no elevar los límites presupuestarios.
- `npm test -- --watch=false`: 341/341 en 42 archivos.
- `npm run lint`: PASS; Stylelint, arquitectura y frontera HTTP.
- El baseline reportaba `zod` 4.6.5 (high, GHSA-mrhx-c7xc-m3w5), dentro del árbol de producción vía Angular Forms. UX-02 lo actualiza a 4.6.6 con override compatible; `npm audit --omit=dev --audit-level=low` queda limpio.

## Evidencia UX-02 (PR #540; integrado)

- `_metrics.scss` agrega alias semánticos de tipografía, control de 44px, movimiento y foco; `_tokens.scss` aporta superficie, elevación y colores de estado, sin cambiar colores de marca. El foco del `h1` mantiene el offset histórico de 0,35rem.
- `UiState` cubre carga/pendiente/vacío/error/éxito/restringido con semántica live apropiada e iconografía decorativa; `UiButton` conserva disabled nativo con contraste legible.
- `npm run lint`: PASS, Stylelint, reglas de tokens/contraste y arquitectura (90 archivos).
- `npm test -- --watch=false`: 342/342 tests en 43 archivos; incluye seis estados con axe.
- `npm run build`: PASS; tres rutas prerenderizadas. Presupuesto existente de `hosting-spaces-page.scss`: 9,09/9,00 kB (+87 bytes), sin ampliarlo; no aparecieron advertencias adicionales.
- `npm audit --omit=dev --audit-level=low`: PASS, cero vulnerabilidades.

El baseline y la cobertura no afirman que el rediseño ya exista en producción. Cada fila cambiará a “implementada” solo en el PR correspondiente y con evidencia sobre su HEAD probado.

## Evidencia UX-08 (PR #548; integrado)

- `/administracion/global` separa Cuentas, Publicaciones, Indicadores y Auditoría en destinos seleccionables y visibles solo con su capacidad vigente. En móvil, cuentas y publicaciones se presentan como tarjetas; las tablas completas permanecen en escritorio. Indicadores y auditoría tienen áreas de trabajo separadas. No se agregan operaciones, métricas ni permisos y se conservan las acciones actuales.
- Playwright local con Chromium 155.0.8059.84, frontend en `127.0.0.1:4387` y API totalmente interceptada con datos sintéticos `.invalid`; se bloquearon destinos externos. Cuatro vistas se capturaron a 1365 px y 390 px. Las ocho vistas no presentan overflow del documento, errores de página ni infracciones axe WCAG 2.0/2.1/2.2 A/AA. La evidencia no representa integración con Go/PostgreSQL ni validación online.
- Capturas y SHA-256: escritorio cuentas `planning/frontend/evidencia/UX08_accounts_desktop_local_sintetica.png` (`109dc9d29b1ac63b355ddfe1baef370b86d6599c51c017069390abe636ba4c4f`), móvil cuentas `UX08_accounts_mobile_local_sintetica.png` (`be4eca633636534b04f110266fd6f5e771d2573a81c515591fd0667ce1b28c6c`), escritorio publicaciones `UX08_publications_desktop_local_sintetica.png` (`b3513126d298f954ca9f363d6170fce842576602cb3c5047a6f286037a414dd6`), móvil publicaciones `UX08_publications_mobile_local_sintetica.png` (`c8c9c92c8ed97e80738f6ba3a7091b80214bc4579abf01ffddef69d7222f7e4d`), escritorio indicadores `UX08_kpis_desktop_local_sintetica.png` (`ba27d43bc931a462cae81115d5199728664f6644f183e8ff893f6b74e75e4b00`), móvil indicadores `UX08_kpis_mobile_local_sintetica.png` (`61eac1a14d3de07483401792fa26094560db116ea8b5889a152f53324971cf1a`), escritorio auditoría `UX08_audit_desktop_local_sintetica.png` (`641c147487231bdb0643021073efe53a2dfc9bdca3eb83cc61393993696c592c`) y móvil auditoría `UX08_audit_mobile_local_sintetica.png` (`71125dd5741ef9c13fd9559b43c42949b607162f9f96527dc4fcbbcc251f1aa3`).
- La revisión detectó la revocación de `accounts.list`; el fix oculta Cuentas, elige una sección autorizada o presenta un estado vacío seguro, y limpia los datos privados. El test cubre perder `accounts.list` preservando `audit.read` y perder todas las capacidades.
- Verificación local final en HEAD `dea9aab7b82a7c507006fc2286d15dd3986d49da`: `npm test -- --watch=false` 361/361 en 45 archivos; suite global 28/28; `npm run lint`, `npm run api:check`, `npm run build` y `git diff --check`: PASS. Build mantiene el warning previo de `hosting-spaces-page.scss` (11,49/9,00 kB).
- PR #548 fue aprobado por `HernanEspinozaDev` y fusionado en `f722047ca7e4effac7e909f86d2d1aff3aa15100`; checks `backend`, `frontend` y `application-images` en SUCCESS. La aceptación visual y comprobación online corresponden exclusivamente al usuario.

## Evidencia UX-09 (consolidación local; PR y revisión pendientes)

- La rama de consolidación parte de `main` en merge UX-08 `f722047ca7e4effac7e909f86d2d1aff3aa15100`. La matriz de rutas quedó sincronizada con PR integrados #541–#546 y #548 y conserva por separado los límites de integración Go/PostgreSQL y validación online de cada corte; #547 permanece pausado y fuera de UX-09.
- `frontend/src/app/app.routes.spec.ts` agrega un inventario ejecutable de las 28 declaraciones de ruta: componentes lazy con título, alias y wildcard explícitos, rutas públicas y guards de identidad, rol o capacidad. Así, agregar/quitar una ruta requiere actualizar la cobertura en la misma prueba.
- `frontend/e2e/ux09-public-routes.spec.cjs` recorre las 10 rutas públicas (`/`, categorías, cómo funciona, catálogo, documentos, acceso, registro, verificación y recuperación) en 1365×900 y 390×844. Health es una respuesta sintética 200; todas las llamadas `/api/v1/*` reciben 503 sintético para comprobar el estado recuperable; se bloquean hosts externos. Las 20 comprobaciones pasan: encabezado presente, documento sin overflow, cero errores de página y cero infracciones axe WCAG 2.0/2.1/2.2 A/AA. Esto no es integración de API ni indica disponibilidad de servicio.
- Las capturas de los 10 estados en ambos viewports y su SHA-256 se conservan en `planning/frontend/evidencia/UX09_public_routes_local_sinteticas.json`; archivos `UX09_*_local_sintetica.png`. Se capturaron con Chromium 155.0.8059.84 sobre el frontend local, sin cuentas, datos personales, login online ni solicitudes externas.
- Las rutas privadas y mutaciones permanecen cubiertas por pruebas de guard, suites de feature y E2E ya registrados en UX-03P/04/05/06/07/08; el nuevo barrido browser no simula sesión privada ni repite mutaciones. Se conservan los artefactos previos por corte, sin presentar un mock como recorrido Go/PostgreSQL.
- Verificación local sobre esta consolidación: `npm test -- --watch=false` 362/362 en 45 archivos; `npm run lint`, `npm run api:check`, build/prerender de 3 rutas, gateway (11), integración SSR del gateway (1), IAP assertions (8), E2E sintético UX-09 (10 rutas × 2 viewports) y `npm audit --omit=dev --audit-level=low` (0 vulnerabilidades de producción): PASS. Build: bundle inicial 462,96 kB raw / 115,34 kB estimado transferido; mantiene la advertencia conocida de `hosting-spaces-page.scss` (11,49/9,00 kB, debajo del límite estricto de 12 kB).
- Una instalación limpia reporta además cuatro vulnerabilidades solo en el árbol de desarrollo (`@angular/build` low, `fast-uri` moderate, `undici` y `vite` high); la auditoría del árbol de producción está limpia. UX-09 no modifica `package.json` ni lockfile.
- La revisión técnica del PR UX-09 sigue pendiente; PR/merge/despliegue no constituyen aceptación humana final de UX-09 ni verificación online.
- El plan registra UX-03P G2 aprobado en PR #542, y UX-02 a UX-08 integrados en PR #540–#546 y #548. El PR #547 sigue pausado y se mantiene fuera de este corte. Esto no completa aceptación humana final: la experiencia online, incluida la aprobación final de UX-09, corresponde al usuario.

## Evidencia UX-04 (PR #543)

- Merge `3cc27ca57a3f30574f35b3167ea174db065445b6`; HEAD revisado `72b4f63ec982a607c10dfe803478ec54cc515534`; fusionó `HernanEspinozaDev`.
- 350 pruebas Angular, lint/arquitectura, `api:check`, build, auditoría de producción, gateway SSR e IAP aprobados en local/CI.
- Inspección local de acceso/registro con backend apagado; consentimiento quedó en estado no disponible y no se probó una cuenta. Las rutas privadas con Go y toda validación online siguen fuera de esa evidencia; online lo comprueba exclusivamente el usuario.
- El lockfile actualizó `body-parser` 2.3.0→2.4.0 y `qs` 6.16.1 para resolver la auditoría HIGH de producción; `package.json` no cambió.

## Evidencia UX-05 (PR #544; integrado)

- Explore corrige la proporción de portada y no muestra el orden predeterminado como filtro aplicado; detalle ofrece galería pública seleccionable, ubicación/datos declarados y reintento ante error de carga. Los flujos de disponibilidad, cotización y sesión conservan el contrato actual.
- Capturas Playwright local con Chromium 155, frontend estático local y API de vista interceptada con datos sintéticos; las fotos son SVG sintéticos y el estado de salud fue simulado solo para presentar la vista. No hubo solicitudes a hosts externos. No representa integración Go/PostgreSQL ni datos reales: `planning/frontend/evidencia/UX05_catalogo_publico_local_sintetico.png` (SHA-256 `6b281821800d2ba6d5699364b8aa9531b69d4fa9d35ebec00ade9016744c88e6`) y `UX05_ficha_publica_local_sintetica.png` (SHA-256 `c451b08c283b66e6e710ee840b7e84a9af8b4ef58c14953136d54d8bcce921cf`).
- Revisión visual local comprobó portada 304.7×228.5 px y presencia de resumen; no aparecen filtro de orden predeterminado ni error de salud simulado. Revisión de la ficha comprobó resumen, reglas, fotos públicas y acceso condicionado por sesión.
- `CI=true npm test -- --watch=false`: 353 pruebas/44 archivos; `npm run lint`, `npm run api:check`, build, `npm audit --omit=dev` y `git diff --check` pasan. Build mantiene el warning anterior de Hosting (11.49/9.00 kB); Explore queda bajo el presupuesto.
- PR #544 fue aprobado y fusionado en `6081502872b564f39bd88ac73f933df6f8f61bf8` desde el HEAD revisado `cb714276ee3123a89346420635b0548524ec0c85`. Sin recorrido con Go/PostgreSQL, flujo completo de Hosting en navegador ni prueba de la aplicación online; online lo valida exclusivamente el usuario.

## Evidencia UX-06 (PR #545; integrado)

- Las reservas ofrecen reintento de lectura sin repetir una mutación; el detalle y finanzas distinguen la consulta confirmada de un reintento de pago. El panel de conversación mantiene la actualización de solo lectura en el encabezado y ofrece reintento tras error de reseñas/calificaciones. El detalle agrega navegación a sus secciones sin alterar contratos, estados ni permisos.
- Capturas Playwright local con frontend en `127.0.0.1:4317`, respuestas de API sintéticas interceptadas en `localhost:8080` y Chromium headless; se bloquearon otros orígenes y no se observaron errores de página. La captura de error/reintento muestra una respuesta 503 simulada y la acción de repetir una lectura; la prueba verifica que la segunda lectura confirma la lista. No son integración Go/PostgreSQL ni datos reales: `planning/frontend/evidencia/UX06_reservas_local_sinteticas.png` (SHA-256 `db4201dc88cd8bd8c5de88ffc146ed73a28a59c6d426d8995a5a0cc16a990096`), `UX06_reserva_detalle_local_sintetica.png` (SHA-256 `1a3d010f9556de75ce5ca8ca530aa6527aea2b954e73a0f21e253047ceffe754`), `UX06_finanzas_reserva_local_sintetica.png` (SHA-256 `a449a5d6103510ea07ea17b747aa106bd649909fa1039fbe08e19ff88aa49cac`) y `UX06_error_reintento_reservas_local_sintetica.png` (SHA-256 `7a5c388ac965b4a644d3c4556f71a1da6f7fffb919141e9ad5aad50aafa58ab1`).
- `CI=true npm test -- --watch=false`: 356 pruebas en 45 archivos; suites focalizadas: 31/31. `npm run lint`, `npm run api:check`, `npm run build`, `npm audit --omit=dev` y `git diff --check`: PASS. El build conserva el warning previo de `hosting-spaces-page.scss` (11,49/9,00 kB).
- Sin recorridos con Go/PostgreSQL, operaciones financieras reales ni verificación de la aplicación online. La prueba online corresponde exclusivamente al usuario; el agente no debe inspeccionar la aplicación ni sesiones online.

## Evidencia UX-07 (PR #546; integrado)

- Código y capturas ejecutados/probados sobre `d38bfa1c73599ac08c0c821f2402a835bd61e56f`; documentación de cobertura registrada en este PR.
- `/administracion` organiza las diez secciones existentes en controles seleccionables, con `aria-pressed`, foco por teclado y ajuste responsive; conserva filtros, consultas, decisiones, motivos y acciones que ya confirmaba Go. La ruta de disputas separa visualmente la conciliación administrativa y sus consultas/acciones actuales. La vista de finanzas de la reserva personal no cambia.
- Se corrigió la creación de efectos de sincronización de `ModelControls.forSignal`: al renderizar secciones condicionales que enlazan filtros, Angular podía lanzar `NG0602` dentro del contexto reactivo. La regresión real de seleccionar “Reservas” está cubierta por la prueba de `AdminPage`.
- Capturas Playwright local con Chromium headless, frontend en `127.0.0.1:4319` y respuestas sintéticas interceptadas en `localhost:8080`; cuenta `.invalid` y datos fixture, solo hosts locales permitidos. No hubo solicitudes externas, errores de página ni desbordamiento a 390 px. axe-core encontró cero violaciones en Administración escritorio/móvil y Disputas administración escritorio: `planning/frontend/evidencia/UX07_administracion_local_sintetica.png` (SHA-256 `fbc51ec8bb6fb25f0281daee518e9548bd3328ef0996aaf5f4559293d60cd8f9`), `UX07_administracion_mobile_local_sintetica.png` (SHA-256 `4f9562513b35e346b54d5f6cb7a4f358e2028d0d5455a1578412bf2fb8564ca5`) y `UX07_disputas_admin_local_sintetica.png` (SHA-256 `9463d1eeea5893e46dc35159f382de05b4d5ab4ab0dd0c681aac888905f6b486`).
- `CI=true npm test -- --watch=false`: 357/357 pruebas en 45 archivos; suites de administración y disputas: 11/11. `npm run lint`, `npm run api:check`, `npm run build`, `npm audit --omit=dev --audit-level=low` y `git diff --check`: PASS. El build conserva el warning previo de `hosting-spaces-page.scss` (11,49/9,00 kB).
- Sin integración Go/PostgreSQL ni validación online. Las capacidades, autorización y decisiones siguen siendo responsabilidad de Go; no se agregan roles, endpoints ni migraciones. La validación online corresponde exclusivamente al usuario.
