# Inventario de cobertura del rediseño Angular EspaciGo

Fecha de corte inicial: 2026-10-23. Base funcional observada: `0b8548e2dc4ef2ff5d368235087ca0b279296220` (`main`).

Este inventario asigna cada ruta de `frontend/src/app/app.routes.ts` a una sección de producto, sus operaciones visibles y el corte de rediseño que las debe conservar. El inventario de operaciones de API permanece en `matriz_paridad_mock_angular.md`; esta matriz no lo sustituye ni convierte propuestas visuales en capacidad implementada.

## Decisión de diseño que enmarca los cortes

El usuario eligió **A · Editorial funcional** el 2026-10-23 desde `http://127.0.0.1:4173/?direction=A&area=hosting&viewport=desktop&state=content`, sobre la base indicada. La comparación y revisión independiente UX-00/01 está en el worktree local `ux-local-prototypes`; informe SHA-256 `7549d39abd96b965b35cd68cafa94b2e5b92ea77088388decb18b8d34f63ab74`. La dirección A y el conjunto de cuatro contextos fueron aprobados para implementación. B se conserva como referencia histórica de Hosting y Administración global. La elección no representa aceptación final comercial ni cambia contratos, rutas, roles o capacidades.

## Rutas actuales y cobertura por corte

| Sección/rutas actuales | Feature y tareas que deben mantenerse | Destino UX y corte | Estado de rediseño |
| --- | --- | --- | --- |
| `/`, `/categorias`, `/como-funciona` | Portada, búsqueda inicial, ocho categorías, explicación y accesos | UX-03 shell/nav; UX-05 contenido público/descubrimiento | Pendiente |
| `/explorar`, `/espacios/:id` | Filtros permitidos, cursor/paginación disponible, fichas públicas, cotización/intervalo según capacidades del API | UX-05 catálogo público y detalle; UX-09 consolidación | Pendiente; sin datos ni disponibilidad inventados |
| `/terminos-y-condiciones`, `/politica-de-privacidad` | Lectura versionada de documentos/consentimiento y estados de carga/error | UX-04 identidad y consentimiento; UX-09 revisión de lectura | UX-04 implementado en PR #543; validación completa de rutas/estados se conserva para UX-09 |
| `/acceso`, `/registro`, `/verificar-correo`, `/recuperar-acceso` | Login, creación, verificación/reenvío y recuperación; registro no elige rol/capacidades | UX-04 identidad | UX-04 implementado en PR #543; mantener flujo público y cookie de sesión actual |
| `/mi-cuenta`, `/mi-cuenta/privacidad` | Perfil propio, privacidad, exportación/solicitud de derechos, baja y confirmaciones existentes | UX-04 cuenta/privacidad; UX-09 revisión cruzada | UX-04 implementado en PR #543; mantener sesión/autorización actuales |
| `/mi-cuenta/seguridad` | Cambio de contraseña y resultado de sesión, validación/error | UX-04 identidad/seguridad | UX-04 implementado en PR #543; no exponer tokens ni aflojar guardas |
| `/mi-cuenta/verificaciones` | Elegibilidad, casos/historial, evidencia autorizada, retiro y reintento según estados reales | UX-04 cuenta/verificaciones | UX-04 implementado en PR #543; no confundir correo verificado con KYC/KYB |
| `/panel-anfitrion` → `/panel/espacios` | Alias existente; lista de espacios propios, estado del borrador, selección y primera acción comercial autorizada | UX-03 navegación; UX-03P piloto y UX-05 extensión hosting | Pendiente; mantener `allowFirstBusinessAction` y rol Go |
| `/panel/espacios/:spaceId/editar` | Datos del recurso, validaciones por campo, guardado de borrador y errores del servicio | UX-03P piloto de Hosting; UX-05 datos/atributos/revisión | Pendiente; no guardar campos no tocados |
| `/panel/espacios/:spaceId/tarifas` | Importe, unidad CLP hora/día/mes, cotización simulada y guardado permitido | UX-05 Hosting | Pendiente; conservar reglas/unidad del dominio |
| `/panel/espacios/:spaceId/calendario` | Zona horaria, disponibilidad, conflictos, bloques y confirmación de guardado | UX-05 Hosting | Pendiente; preservar zona IANA y conflictos Go |
| `/panel/espacios/:spaceId/galeria` | Portadas, generación/subida, progreso/error/reintento/retiro y límites actuales de limpieza | UX-05 Hosting | Pendiente; foto retirada no significa borrado físico |
| `/reservas`, `/reservas/:id` | Lista/detalle por actor, estado de solicitud/ocupación/contrato/pago separado, historial, conversación, decisiones y reintentos | UX-06 reservas/conversación/contrato; UX-09 recorridos | Pendiente; conservar acciones por actor y estados Go |
| `/reservas/:id/disputas` | Evidencia/finanzas de reserva según datos, permisos, reversa/garantía/liquidación y estados actuales | UX-06 reserva y finanzas | Pendiente; no mezclar importes ni afirmar pago por timeout |
| `/administracion` | Inicio, perfiles operativos y colas de capacidades existentes | UX-07 administración operativa | Pendiente; sin soporte/tickets ni KPI inventados |
| `/administracion/disputas` | Cola, ficha, evidencia y decisiones/motivos cuando las capacidades vigentes lo permiten | UX-07 administración operativa | Pendiente; control por rol/capacidad en Go |
| `/administracion/global` | Directorios de cuentas/publicaciones, filtros/paginación, ficha, capacidades por operación, indicadores/auditoría existentes | UX-08 administración global | Pendiente; acciones sensibles dependen de permiso de servidor |
| `**` → `/` | Ruta no reconocida y retorno seguro | UX-03 shell/nav; pruebas UX-09 | Pendiente |

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
| UX-02 | Tokens A, tipografía/espaciado semánticos y primitivas compartidas de estado/acción/foco | Catálogo de tokens, estados, contraste y APIs; regresiones de estilos | Implementado en este PR; pendiente de review técnico. No cambian rutas, permisos ni contratos. |
| UX-03 | Shell, navegación contextual, destino activo, estados de menú, foco/SSR/deep links | Shell reutilizable y capturas pública/privada en escritorio/tablet/móvil | Review técnico/visual; preservar estado auth, SSR, ruta directa y roles/capacidades |
| UX-03P | Piloto funcional de Hosting: lista, selección, abrir/crear/editar borrador, errores y respuesta Go; tarifas/calendario/galería siguen accesibles | Pruebas Go/PostgreSQL de ensayo, E2E Chromium/Firefox/axe/teclado y matriz de estados | **G2**: revisión independiente técnica/visual con puntuación y evidencia; seguir al siguiente corte si pasa. G2 no equivale a aceptación humana final |
| UX-04 | Identidad, consentimientos, cuenta, seguridad, privacidad, verificaciones | E2E por ruta/estado; sin cambios en SMTP/cookies/consentimientos | Review independiente y regresión de sesión/guards |
| UX-05 | Hosting completo y rutas públicas de catálogo/ficha | Recorrido borrador→validación→resultado; acciones preservadas | Review independiente, contrato y captura comparable |
| UX-06 | Reservas, conversación, contratos y finanzas personales | Recorridos por actor, estado y errores/timeout | Review independiente; montos/historial sin regresión |
| UX-07 | Administración operativa/disputas | Colas y decisiones por capacidad/motivo | Review independiente; sin soporte ni rol ampliado |
| UX-08 | Administración global de cuentas/publicaciones/indicadores/auditoría existentes | Pruebas de capacidad por operación, filtros/paginación reales, auditoría | Review independiente; no inventar métrica/operación ni mutar históricos |
| UX-09 | Consolidación de todas las rutas anteriores | Matriz completa, regresiones responsive/SSR/performance/accessibility y capturas con SHA | Aprobación humana final de la experiencia online; merge/despliegue técnico no la sustituye |

Un corte se considera cubierto cuando registra rutas, fuente→destino de acciones, guard/capacidad, casos de carga/vacío/error/éxito/denegación pertinentes, pruebas y captura/hash. Las pruebas automatizadas no sustituyen la autorización Go, una evaluación con participantes o aceptación del usuario. Mantener fuera del alcance módulos de soporte/tickets no existentes y toda capacidad no presente en el código/contrato vigente.

## Regla de validación online para UX-03P–UX-09

La comprobación de acceso y experiencia en servicios online la realiza exclusivamente el usuario. El agente no inicia sesión ni inspecciona la interfaz, las sesiones o los datos de cuentas online durante estos cortes. La validación del agente se limita a entornos locales desechables con cuentas sintéticas conforme a la política `LOCAL PRIVADA`; sus credenciales permanecen bajo `.local/testing/` y los recursos del ensayo se retiran al terminar. La evidencia que aporte el usuario no equivale por sí sola a aceptación final del producto.

## Baseline de ejecución

- `ng build`: tres rutas prerenderizadas; bundle inicial 453.64 kB. Baseline supera el warning de `hosting-spaces-page.scss` por 87 bytes (9.09 kB frente a 9.00 kB); no elevar los límites presupuestarios.
- `npm test -- --watch=false`: 341/341 en 42 archivos.
- `npm run lint`: PASS; Stylelint, arquitectura y frontera HTTP.
- El baseline reportaba `zod` 4.6.5 (high, GHSA-mrhx-c7xc-m3w5), dentro del árbol de producción vía Angular Forms. UX-02 lo actualiza a 4.6.6 con override compatible; `npm audit --omit=dev --audit-level=low` queda limpio.

## Evidencia UX-02 (HEAD de este PR)

- `_metrics.scss` agrega alias semánticos de tipografía, control de 44px, movimiento y foco; `_tokens.scss` aporta superficie, elevación y colores de estado, sin cambiar colores de marca. El foco del `h1` mantiene el offset histórico de 0,35rem.
- `UiState` cubre carga/pendiente/vacío/error/éxito/restringido con semántica live apropiada e iconografía decorativa; `UiButton` conserva disabled nativo con contraste legible.
- `npm run lint`: PASS, Stylelint, reglas de tokens/contraste y arquitectura (90 archivos).
- `npm test -- --watch=false`: 342/342 tests en 43 archivos; incluye seis estados con axe.
- `npm run build`: PASS; tres rutas prerenderizadas. Presupuesto existente de `hosting-spaces-page.scss`: 9,09/9,00 kB (+87 bytes), sin ampliarlo; no aparecieron advertencias adicionales.
- `npm audit --omit=dev --audit-level=low`: PASS, cero vulnerabilidades.

El baseline y la cobertura no afirman que el rediseño ya exista en producción. Cada fila cambiará a “implementada” solo en el PR correspondiente y con evidencia sobre su HEAD probado.

## Evidencia UX-04 (PR #543)

- Merge `3cc27ca57a3f30574f35b3167ea174db065445b6`; HEAD revisado `72b4f63ec982a607c10dfe803478ec54cc515534`; fusionó `HernanEspinozaDev`.
- 350 pruebas Angular, lint/arquitectura, `api:check`, build, auditoría de producción, gateway SSR e IAP aprobados en local/CI.
- Inspección local de acceso/registro con backend apagado; consentimiento quedó en estado no disponible y no se probó una cuenta. Las rutas privadas con Go y toda validación online siguen fuera de esa evidencia; online lo comprueba exclusivamente el usuario.
- El lockfile actualizó `body-parser` 2.3.0→2.4.0 y `qs` 6.16.1 para resolver la auditoría HIGH de producción; `package.json` no cambió.
