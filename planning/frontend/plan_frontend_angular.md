# Plan completo del frontend Angular — FRONTEND-LOCAL-1

Fecha: 2026-10-12. Baseline de planificación: b0f62b976c9586d260cea15ed9acc29ab12257a7, V54. Estado actual: FE-00/FE-01 aceptados en PR #274; FE-02 aprobado y fusionado en PR #275; FE-03 aprobado y fusionado en PR #276; FE-04 aceptado y fusionado en PR #278; FE-05 aprobado y fusionado en PR #279; FE-06 aceptado y fusionado en PR #280; FE-07 aceptado y fusionado en PR #282; FE-08 aprobado y fusionado en PR #283 (merge `503d19359e94c9f187bb955047c150aef8b97507`). FE-09 aprobado y fusionado en PR #284 (merge `0d3bac8a06457899239a49274b54ad6807846f42`); FE-10 aprobado y fusionado en PR #285 (merge `39514662c9002d3c50bdbe7244813c57d13c2b38`); FE-11 aprobado y fusionado en PR #286; FE-12 aprobado y fusionado en PR #287; FE-13 está en revisión en PR #288 y FE-14/15 siguen pendientes.

## 1. Objetivo y condición de cierre

Construir el frontend definitivo local de EspaciGo en Angular, conectado al backend Go, con diseño Terracota + Crema, navegación por capacidades y cobertura de las funciones locales de M01–M11. Al terminar, Angular será la interfaz habitual y se eliminarán código, servidor, targets y dependencias del mock, preservando su historial en Git y las pruebas relevantes mediante su traslado.

**FRONTEND-LOCAL-1 termina cuando hay paridad funcional verificada, recorrido integral local en Docker, accesibilidad revisada y aceptación explícita del usuario.** No significa producto listo para usuarios reales, integraciones externas completas ni despliegue GCP.

## 2. Qué conservamos y qué mejoramos de las propuestas

| Tema | Decisión para este plan |
| --- | --- |
| Angular y colores | Adoptar Angular 22, TypeScript, standalone, Signals y SCSS; conservar los cinco colores elegidos. La guía visual posterior prevalece sobre el ejemplo azul y la marca pendiente del documento arquitectónico anterior. |
| Monorepo | Agregar `frontend/` dentro de `espaciGo/`. Conservar `cmd/`, `internal/`, `db/`, `scripts/` y `planning/` donde están; no mover Go a `backend/`. |
| Arquitectura | Features, fachadas y acceso HTTP separado; puertos y casos de uso solo en flujos complejos. Sin microfrontends ni almacén global por defecto. |
| Producto | Espacios flexibles de ocho categorías; no convertirlo en un portal residencial de dormitorios o arriendos permanentes. |
| API existente | Reutilizar `planning/openapi.yaml` (3.1.0). El inventario actual tiene 118 paths y 144 operaciones declaradas; no recrear el contrato desde cero. Ese conteo no es un porcentaje de avance. |
| Renderizado | Prerender para portada informativa y páginas estáticas; CSR para cuentas, catálogo autenticado y paneles. Preparar soporte híbrido para futuras rutas públicas autorizadas. |
| Portal público indexable | El catálogo y detalle actuales requieren Bearer. No usar cuentas de servicio, tokens incrustados ni SSR para volverlos públicos. Un API público minimizado de catálogo/galería necesitará otro alcance backend autorizado. |
| Finanzas y firma | Consumir estados confirmados Go y fakes actuales. No implementar split real, comisión productiva, validez de firma ni documento fiscal. |
| Visitas/postulaciones/renovaciones | Propuestas futuras sin API local equivalente; no son condiciones para reemplazar el mock y no se agregan como pantallas que aparenten funcionar. |
| Docker | Frontend independiente; reutilizar PostgreSQL 18/PostGIS, Mailpit y migrador existentes. No copiar el Compose ilustrativo de PostgreSQL 17 o credenciales únicas. |
| Ritmo | Entregar recorridos visibles pronto; pruebas enfocadas por cambio y regresión consolidada en hitos. Evitar repetir todas las integraciones Go para cambios exclusivamente visuales. |

LOCAL-1 está aceptado. DOCKER-01 y DOCKER-03 están aceptados; DOCKER-02 conserva pendiente la liquidación tras su plazo real. Se puede iniciar Angular en paralelo, conservando esa condición pendiente para la aceptación final del entorno local.

## 3. Alcance funcional obligatorio

| Módulo | Capacidades a presentar en Angular |
| --- | --- |
| M01 identidad | Términos versionados, preferencia inicial, registro, verificación y reemisión, login, sesión, logout, recuperación/cambio de contraseña; modo restringido. |
| M02 perfil/privacidad | Perfil, foto sintética privada, cuenta de cobro fake, solicitudes de acceso/supresión, exportación JSON y ZIP con artefactos disponibles y estados de retirada. |
| M03 KYC/KYB | Casos sintéticos, consulta, reintento/subsanación, evidencia permitida, historial y elegibilidad efectiva por tipo; revisión y revocación administrativas. |
| M04 oferta | Crear/listar/editar borradores y publicaciones propias; categorías/perfiles/atributos versionados; galería sintética, portada, publicar/ocultar/eliminar; tarifas, modalidades, simulaciones; zona IANA, disponibilidad, bloqueos y horario semanal. |
| M05 descubrimiento | Texto, categoría, atributos tipados, superficie, intervalo, rango de total CLP, proximidad y mapa sintético, paginación, limpiar filtros, detalle seguro, selector de horarios y cotización. |
| M06 reservas/pagos | Bandeja por participante, detalle e historial, solicitud, aprobación/rechazo, vencimientos, pago fake, timeout/reintento/conciliación, cancelación y devolución fake. |
| M07 contratos | Snapshot, historial de firma/rechazo, acciones por participante, PDF privado, invitaciones y estados; identificación permanente del ensayo. |
| M08 operación | Check-in/out, recepción del anfitrión, historial y evidencia sintética privada. |
| M09 comunicación | Hilo paginado, envío idempotente, cursores de lectura y contadores; reseñas elegibles, reportes y moderación; estado de avisos administrativos. |
| M10 disputas/finanzas | Incidencia local y reclamo formal diferenciados, descargo con evidencia, resolución administrativa, garantía/autorización/captura/liberación fake, liquidación, cierre y comprobante de ensayo. |
| M11 administración | Consulta exacta de cuentas por UUID, bloqueo/desbloqueo, reservas/publicaciones minimizadas, auditoría paginada/exportable, reportes agregados ratificados, colas de privacidad/KYC/reportes y recuperación explícita de avisos terminales. |

La [matriz de paridad](matriz_paridad_mock_angular.md) vincula cada operación con su corte. Los endpoints técnicos de callback HMAC no se convierten en controles del navegador; readiness y runners siguen siendo infraestructura. La purga administrativa se conserva como herramienta explícita y acotada en modo de ensayo, con confirmación y permisos; no se expone como acción ordinaria de un titular.

### Límites conservados

- Ocho categorías: oficina, sala/multipropósito, bodega, estacionamiento, local flexible, stand, quincho, parcela/eventos. Los códigos y perfiles proceden de Go, no de clases TypeScript distintas por categoría.
- CLP y unidades hora/día/mes implementadas; no inventar UF, impuestos, comisiones o descuentos.
- PNG sintéticos generados por el servidor; no agregar subida de documentos, fotos o bytes arbitrarios porque un componente `FileUpload` lo permita.
- El mapa actual es un esquema con posiciones normalizadas de ensayo, no una cartografía de direcciones privadas. No incorpora geocodificador ni proveedor de mapas.
- No campañas/promociones/NPS, visitas, postulaciones, proveedores/sandbox reales, GCP ni cambios a ES1.

## 4. Decisiones técnicas

### Toolchain

Angular 22 estable, CLI/core/CDK alineados y lockfile fijado. La tabla oficial consultada el 2026-10-11 indica para 22.0.x Node `^22.22.3 || ^24.15.0 || ^26.0.0`, TypeScript `>=6.0.0 <6.1.0` y RxJS `^6.5.3 || ^7.4.0`. Se propone Node 24 LTS compatible y RxJS 7; FE-00 fija parches y verifica nuevamente la compatibilidad. No reutilizar TypeScript 5.9.3 del mock. [Compatibilidad Angular](https://angular.dev/reference/versions).

Standalone, rutas lazy, Signals/computed para estado visual y Reactive Forms tipados para formularios y perfiles dinámicos. HttpClient/RxJS coordinan HTTP y cancelación. No iniciar con Signal Forms experimental ni NgRx Store. SignalStore se incorpora únicamente si una feature demuestra complejidad compartida que la fachada local no resuelve. [Reactive Forms](https://angular.dev/guide/forms/reactive-forms).

Cliente propuesto: OpenAPI Generator `typescript-angular`, versión/configuración fijadas, `ngVersion` compatible y tipos generados sin edición manual. FE-00 debe probar sobre el OpenAPI real: null/uniones/const, errores, cursor, parámetros JSON, PDF/PNG/ZIP y encabezados. Si el generador no representa correctamente 3.1, documentar el resultado y usar tipos `openapi-typescript` con adaptadores HttpClient tipados; conservar el contrato canónico, no degradarlo silenciosamente. [Generador Angular](https://openapi-generator.tech/docs/generators/typescript-angular/), [tipos OpenAPI](https://openapi-ts.dev/introduction).

Playwright para E2E y comprobación de descargas; herramienta de pruebas Angular configurada por CLI y fijada en FE-00; ESLint/Stylelint. Catálogo de componentes ligero dentro del entorno de desarrollo; Storybook opcional cuando haya biblioteca suficiente, no prerrequisito para el primer recorrido.

### Estructura propuesta

```text
espaciGo/
├── cmd/ internal/ db/             # Go actual, conservado
├── mock/                         # Temporal hasta FE-15
├── frontend/
│   ├── src/app/
│   │   ├── core/                 # config, session, http, shell
│   │   ├── shared/ui/            # componentes accesibles sin reglas financieras
│   │   └── features/
│   │       ├── auth/ account/ privacy/ verification/
│   │       ├── spaces/ catalog/ bookings/ payments/
│   │       ├── contracts/ operations/ communication/
│   │       ├── disputes/ finance/ administration/
│   ├── src/generated/api/        # generado, no editar
│   ├── src/styles/               # tokens, Sass modules, globales mínimos
│   ├── e2e/ public/ Dockerfile
│   └── angular.json package.json package-lock.json
├── compose.yaml                  # servicios vigentes
├── compose.frontend.yaml         # integración Angular durante transición
└── planning/frontend/
```

Features simples: `pages/`, `components/`, `data-access/`, `state/`, `*.routes.ts`. Pagos, sesión y reservas pueden añadir `domain/` y `application/` si necesitan puertos/coordinar varias operaciones. No obligar a todas las features a usar seis capas. Los componentes UI no importan clientes de API; los adaptadores no importan páginas. `core` no se convierte en depósito de lógica de todos los módulos.

### Contrato de sesión, concurrencia y dinero

1. Conservar Bearer y el comportamiento actual; token inicialmente en memoria, no en localStorage, URL o logs. La recarga completa requiere login salvo que se autorice después otro contrato de sesión. No inventar refresh tokens ni cookies HttpOnly/BFF sin API y decisión explícita.
2. `/auth/session` devuelve roles y `restricted_mode`. La preferencia ofrecer/arrendar no concede permisos. Cambiar el contexto de navegación tampoco concede un rol.
3. 401 de la sesión vigente limpia sesión y estado privado incluso si cambió el filtro. 403 administrativo limpia las vistas administrativas y refresca capacidades; un 403 de otro recurso no implica logout global. Capturar cuenta/token/generación y descartar respuestas de sesiones anteriores.
4. Cada feature con selección o búsqueda captura recurso/revisión/filtros; `switchMap` cancela lecturas reemplazadas y los identificadores de generación protegen también blobs/descargas y promesas no cancelables. No aplicar `switchMap` a escrituras suponiendo que cancelar HTTP deshace el servidor.
5. Reintentos de escrituras conservan clave idempotente y payload de la misma operación. Con resultado incierto consultar/conciliar; no generar otra clave para volver a cobrar. No guardar tokens en almacenamiento de intentos. Recuperar estado durable desde Go tras recargar.
6. Una cuenta restringida puede acceder solo a las acciones que Go permite sobre reservas existentes; no se bloquea todo el panel ni se habilitan publicaciones/reservas nuevas por decisión visual.
7. JavaScript no representa todos los enteros int64 de Go. FE-00 debe definir decodificación/serialización sin pérdida para importes: respuestas monetarias como texto crudo con codec de números exactos y modelos de dominio decimal/BigInt donde haga falta; no hacer `JSON.parse`/`Number` antes de detectar el desbordamiento. Los DTO generados no resuelven esto por sí solos. Mantener el formato de wire del API; no recortar máximos del backend a escondidas ni redondear importes. Probar límites, cuerpos parciales y el precio cambiado por otra sesión.
8. Timestamps con offset y zona IANA del espacio. El navegador no decide vencimientos ni computa cobros multiplicando horas de días de calendario. Cotizar y confirmar contra Go; probar DST/Santiago y unidades día/mes.
9. PDF/PNG/ZIP privados mediante petición autenticada y Blob URL temporal, revocada al cambiar recurso/logout/destruir vista. No poner Bearer en URLs ni volver públicos endpoints owner-only. Los identificadores retirados son `null`, sin reconstruirlos.
10. Guards mejoran navegación; la autorización sigue en Go. Ningún secreto DB, HMAC, SMTP, clave de cifrado o proveedor puede llegar a Angular. [Guards Angular](https://angular.dev/guide/routing/route-guards).

## 5. Rutas y experiencia

| Área | Rutas propuestas | Renderizado / acceso |
| --- | --- | --- |
| Presentación | `/`, `/como-funciona`, `/categorias` | Prerender informativo, contenido estático de las ocho categorías; sin datos privados ni métricas inventadas. |
| Identidad | `/acceso`, `/registro`, `/verificar-correo`, `/recuperar-acceso` | CSR, noindex; formularios del API actual. |
| Descubrimiento | `/explorar`, `/espacios/:id` | CSR con sesión, noindex mientras el catálogo sea autenticado. Filtros públicos no sensibles en URL; cursor y cotización no se comparten en URL. |
| Cuenta | `/mi-cuenta`, `/mi-cuenta/verificacion`, `/mi-cuenta/privacidad` | CSR con sesión/capacidades. |
| Anfitrión | `/panel/espacios`, `/panel/espacios/:id/editar`, `/panel/espacios/:id/calendario` | CSR por titularidad y permisos, navegación progresiva. |
| Reservas | `/panel/reservas`, `/panel/reservas/:id` | CSR para participante; pestañas resumen, pagos/garantía, contrato, operación, conversación y reclamo. |
| Administración | `/administracion/...` | CSR con rol verificado; contexto separado, proyecciones mínimas. |

SSR híbrido disponible desde el setup, limitado inicialmente a contenido no privado. No transfer cache para respuestas autenticadas ni POST de operaciones, no datos de cuenta en HTML de prerender y no estado de sesión compartido entre solicitudes SSR. La indexación de fichas reales queda diferida hasta existir acceso público autorizado. [Renderizado híbrido](https://angular.dev/guide/ssr).

La ficha sigue la jerarquía útil de una referencia como Realmo: resumen, categoría, características, superficie, tarifa/unidad, condiciones, disponibilidad y CTA de cotización. En el contrato actual no hay imágenes de galería para otro participante: mostrar una ilustración local neutral, y únicamente usar fotos de ficha si hay endpoint autorizado. La galería propia sí se implementa en gestión del anfitrión. No reutilizar el endpoint privado del dueño para simular una galería pública.

## 6. Cortes funcionales y orden

Cada fila especifica dependencias, entregable tangible y aceptación. Los PR pueden dividirse si una fila resulta grande, conservando un solo resultado de usuario y evidencia por subcorte. No crear 144 Issues, una por endpoint.

| ID | Dependencias | Entrega | Aceptación y evidencia mínima |
| --- | --- | --- | --- |
| FE-00 contrato y setup | Plan actual | Inventario/cliente piloto, parches fijados, DTO/null/blob/dinero/errores, estrategia de sesión y configuración | Generación repetible; clientes de login, perfil, catálogo y ZIP compilables; no cambios de políticas Go. Matriz con operación/rol/pantalla/prueba. |
| FE-01 base visible | FE-00 | Aplicación `frontend/`, shell, portada y categoría, tokens/componentes iniciales, config pública/readiness, imagen Docker propia | Portada responsive Terracota + Crema; builds Angular/Go independientes; Angular sin secretos DB. Prerender solo contenido informativo. |
| FE-02 identidad | FE-01 | Registro, términos, verificación, login/logout, recuperación/cambio, sesión y navegación restringida | Recorrido con cuentas sintéticas; errores/tokens y respuestas tardías; roles no derivados de preferencias. |
| FE-03 explorar y cotizar | FE-02 | Búsqueda, categorías/perfiles, filtros, mapa sintético, paginado, detalle, selector y cotización | Login → buscar → detalle → horario → cotizar; cambio de espacio invalida cotización; limpiar filtros reinicia cursor; DST e importes exactos. Primera entrega de producto navegable. |
| FE-04 cuenta y privacidad | FE-02 | Perfil/foto fake, cuenta de cobro, solicitudes, JSON/ZIP, avisos de alcance | Dos cuentas aisladas; descarga real inspeccionable; estado vacío/retirado; logout no expone blobs anteriores. |
| FE-05 KYC sintético | FE-02 | Casos, reintento, evidencia, historial y elegibilidad por tipo | Aprobado/rechazado/pendiente/revocado desde Go; no subir documentos reales ni mostrar verificación comercial. |
| FE-06 oferta y calendario | FE-02/05 | Borradores/perfiles, publicación/edición/ocultar/eliminar, galería/portada, tarifas/simulaciones, IANA/bloqueos/horario semanal | Propietario completa configuración/publicación; terceros rechazados; cambios parciales y snapshots; conflicto de calendario al cambiar modalidad; limpieza recuperable observada. |
| FE-07 reserva y pago | FE-03/05 | Solicitud, bandejas por participante, aprobación/rechazo, fake éxito/rechazo/timeout, cancelación y devolución | Arrendatario solicita/paga, anfitrión decide; reintentos conservan clave, errores muestran estado confirmado; vencimientos/solapes; no aviso falso de cobro. |
| FE-08 contrato y operación | FE-07 | Generar/consultar/firmar/rechazar/PDF, check-in/out, recepción/evidencia | Dos firmas → operación → finalización; rechazo/plazo/cancelación respetan Go; evidencia privada para autorizados. |
| FE-09 conversación y reputación | FE-07/08 | Hilo, paginación, lectura/contadores, reseñas y reportes | Ambos participantes; tercero rechazado; terminal solo lectura; reseña de finalizada/cerrada según Go; HTML como texto. |
| FE-10 disputas y finanzas | FE-07/08 | Incidencias/reclamos/descargo/evidencia y paneles de garantía, deducción, liquidación, cierre/comprobante | Titularidad y admin, timeout/conciliación, importes separados, aviso de ensayo; sin recrear políticas ni falsear fechas. |
| FE-11 administración completa | FE-04/05/06/09/10 | Colas KYC/privacidad/reportes, consultas/cuentas/roles bloqueados, auditoría/exportación y reportes agregados, avisos/reapertura | Admin/no-admin, 401/403 y sesiones tardías; mínimo dato; páginas/filtros/cursor; acciones con motivo e idempotencia. |
| FE-12 endurecimiento visual | FE-03–11 | Biblioteca consolidada, responsive, teclado/foco, contenido extenso y accesibilidad, estados de error/empty/offline | Tres pantallas de referencia y todos los flujos críticos en móvil/escritorio; revisión AA objetivo, sin afirmar certificación. Catálogo interno; Storybook solo si aporta. |
| FE-13 Docker y paridad | FE-01–12 | Stack Angular local reproducible, URLs/CORS/redes, runner Angular integrado, baseline nuevo sin depender del mock | Fresh install, reinicios/API caída/sesión y restauración; frontend sin red/credenciales DB; coexistencia mock documentada. LOCAL-DOCKER aceptado para cierre de entorno. |
| FE-14 aceptación Angular | FE-13 | Regresión consolidada + recorrido visual integral y matriz de paridad completada | Toda operación de UI implementada o exclusión técnica justificada; E2E/descargas/roles; evidencia exacta del commit/imagen; usuario acepta reemplazo. |
| FE-15 retirar mock | FE-14 aceptado | Eliminar mock/servidor/targets/scripts exclusivamente dependientes; migrar tests y referencias; Angular predeterminado | Checkout sin dependencias activas del mock, QA backend sigue ejecutable sin Node del mock, Compose/defaults documentados; regresión de integración sin mock. Aceptación FRONTEND-LOCAL-1. |

### Registro de avance FE-02 (2026-10-11)

**Estado: aprobado y fusionado en PR [#275](https://github.com/HernanEspinozaDev/espaciGo/pull/275).** Se entregó identidad CSR/noindex usando el SDK OpenAPI generado, con acceso HTTP separado del estado de sesión y de componentes. El recorrido usa términos versionados de Go; registro/preferencia inicial; verificación y reemisión; login, sesión y logout; recuperación y cambio de contraseña. El Bearer vive solo en memoria, los estados privados se limpian al salir y las respuestas anteriores a una sesión nueva se descartan. Roles y `restricted_mode` provienen de la respuesta de Go.

**Evidencia y validación:** 26 pruebas Angular tras responder a revisión; build y prerender; `api:check`; recorrido M01 con cuenta sintética y Mailpit dentro de un stack temporal aislado; errores, repetición de tokens, revocación, aislamiento y escaneo de logs. Las regresiones cubren recuperación con sesión activa, limpieza de contraseñas aun si logout no llega a Go, respuesta tardía de logout A tras login B y respuesta tardía de recuperación después de limpiar sesión y navegación fuera de la cuenta cuando Go no confirma logout; login válido desde el componente con IdentitySession real; recuperación A tardía después de logout y login B, con revalidación explícita de B; recuperación anónima aceptada que limpia tokenId, token y contraseñas del formulario vigente. Evidencia visual en [`evidencia/FE02_sesion_escritorio.jpg`](evidencia/FE02_sesion_escritorio.jpg) y [`evidencia/FE02_sesion_movil.jpg`](evidencia/FE02_sesion_movil.jpg). API/rutas Go permanecen intactas; el mock continúa en Compose. El PR fue aprobado y fusionado; FE-03 puede avanzar.

**Límites:** los términos locales exponen códigos/versiones/hash y no constituyen texto legal; se informa este alcance en la UI. Los destinos de catálogo, anfitrión y administración siguen en estado futuro y no implementan sus capacidades. No hay proveedores externos ni despliegue GCP.

### Registro de avance FE-03 (2026-10-11)

**Estado: aprobado y fusionado, [PR #276](https://github.com/HernanEspinozaDev/espaciGo/pull/276).** `/explorar` y `/espacios/:id` usan los roles confirmados por Go y permanecen en CSR/noindex. El recorrido incluye búsqueda, categorías/perfiles versionados, filtros, paginación opaca, mapa `x/y` sintético, detalle sin datos privados, horarios de disponibilidad y cotización Go. El Bearer sigue en memoria; respuestas y cursores se invalidan al cambiar cuenta/sesión. Cambios de espacio, fecha, duración o intervalo descartan una cotización anterior; limpiar filtros reinicia resultados y cursor.

**Importes y tiempo:** las respuestas del catálogo/cotización llegan como texto y se convierten en cadenas CLP antes de que el parser JSON vea los literales `int64`. La UI usa `BigInt` para validar/formatear importes, sin redondear mediante `Number`. El selector toma la fecha en la zona IANA de cada espacio, consume intervalos UTC de Go, y presenta zona/offset para distinguir las horas repetidas DST; horarios inexistentes dependen del resultado de Go.

**Correcciones de revisión:** cambios de filtros y atributos invalidan las búsquedas en curso y reinician el cursor; un 401 limpia la sesión y el estado privado solo si coincide con el Bearer vigente; categorías/perfiles conservan el status HTTP; cambios de espacio/modalidad restablecen una duración admitida; la opción vacía del horario anula la selección y la cotización.

**Evidencia y validación:** 41 pruebas Angular, `api:check`, build/prerender, prueba Go de catálogo/disponibilidad/cotización en una base PostgreSQL temporal `tmpfs`, cuenta sintética y evidencia visual escritorio/móvil en la matriz de paridad. Errores/permisos y aislamiento A→B cubiertos por pruebas de componente/servicio. El mock se conserva; no se cambia Go ni se inician proveedores/GCP. FE-03 fue aceptado y fusionado antes de iniciar FE-04.

### Registro de avance FE-04 (2026-10-11)

**Estado: implementado y en revisión en [PR #280](https://github.com/HernanEspinozaDev/espaciGo/pull/280), commit `8aa32e6`.** `/mi-cuenta` y `/mi-cuenta/privacidad` son rutas privadas CSR/noindex. El corte integra perfil, foto sintética privada, cuenta de cobro fake, solicitudes de acceso/supresión y exportaciones JSON/ZIP mediante el cliente generado y operaciones Go existentes. El Bearer se conserva en memoria; los 401 vigentes limpian el estado privado, las respuestas tardías no afectan otra sesión y las URLs Blob privadas se revocan al cambiar recurso, cerrar sesión o destruir la vista. Los vacíos/artefactos retirados reflejan la respuesta del servidor sin reconstrucción local.

**Validación y evidencia:** 55 pruebas Angular, `api:check`, build/prerender y pruebas Go enfocadas con PostgreSQL temporal/Mailpit desechable para titular/tercero, aislamiento entre dos cuentas, foto y cuenta fake, exportaciones y exclusión de credenciales/datos ajenos. Las pruebas de componente con IdentitySession real incluyen la creación del primer perfil tras GET 404, guardado concurrente a una solicitud, protección contra GET de perfil obsoleto posterior al PUT confirmado y avisos de foto después de recargas, cambio de sesión y destrucción de vista. La matriz de paridad registra por operación los recorridos y las capturas [escritorio](evidencia/FE04_cuenta_escritorio.png) y [móvil](evidencia/FE04_cuenta_movil.png). No se automatizó el diálogo del sistema operativo para verificar la escritura física de descargas; se inspeccionan los Blob y el contenido ZIP generado en pruebas. El mock se mantiene, Go no se modifica y no se inician GCP ni proveedores reales. FE-04 fue aceptado y fusionado en PR #278 antes de comenzar FE-05.

### Registro de avance FE-05 (2026-10-12)

**Estado: aprobado y fusionado en [PR #279](https://github.com/HernanEspinozaDev/espaciGo/pull/279), commit `3405f05`.** `/mi-cuenta/verificaciones` es privada, CSR/noindex y consume el SDK OpenAPI generado. Presenta casos propios KYC/KYB, estados y elegibilidad confirmados por Go, historial, subsanación del rechazo y generación de evidencia exclusivamente mediante `synthetic-png-v1`. El cliente no acepta archivos ni bytes arbitrarios; los contenidos se descargan con Bearer y las Blob URLs temporales se revocan en cambio de caso, cierre/cambio de sesión y destrucción de vista. No asigna roles ni concede elegibilidad.

**Validación y evidencia:** 7 pruebas KYC enfocadas; regresión Angular consolidada de 62 pruebas en 14 archivos; `npm run api:check`; build/prerender; pruebas Go del handler y de repositorio PostgreSQL temporal (`tmpfs`) para autorización titular/tercero, estados, reintento y evidencia. Las regresiones Angular cubren 401 vigente/tardío, cambio A→B, PNG fuera de orden, Blob URL en cambio de recurso/logout/destrucción, y evidencia retirada. Capturas de escritorio/móvil usan fixtures sintéticos locales y se enlazan en la matriz. FE-04 y FE-05 fueron aceptados y fusionados en PR #278 y #279 antes de iniciar FE-06.

### Registro de avance FE-06 (2026-10-12)

**Estado: implementado y en revisión en [PR #280](https://github.com/HernanEspinozaDev/espaciGo/pull/280).** El panel de anfitrión `/panel/espacios` y las rutas de edición, tarifas, calendario y galería son privadas, CSR/noindex y requieren el rol de arrendador confirmado por Go. FE-05 se registra como aceptado y fusionado en PR #279. El corte cubre borradores y contenido publicado, perfiles de atributos versionados, tarifa exacta en CLP y versiones/simulaciones, publicación/ocultamiento/eliminación diferenciados, galería sintética/portada con contenido privado y Blob URL revocable, zona IANA, disponibilidad, bloqueos y horario semanal. Los borradores usan un GET fresco y fusionan solo campos modificados antes de enviar el PUT requerido por contrato; el API de contenido publicado usa PATCH lógico por campos. Claves idempotentes y payload de fixture se conservan para una creación incierta de imagen sintética. Go decide propiedad, elegibilidad, estado y conflictos. El mock y Go se mantienen sin cambios de ruta; no se iniciaron GCP ni proveedores reales.

**Validación de revisión:** 22 pruebas FE-06 enfocadas y 95 pruebas Angular consolidadas en 17 archivos; `npm run api:check`; `npm run build` y prerender de 3 rutas completados. Las pruebas Go de atributos/PostGIS efímero siguen pasando. Se añadieron dos regresiones para 401 de configuración paralela: cambio A→B con Bearer vigente expira sesión/estado privado; logout/login preserva el Bearer nuevo ante 401 del viejo. En un proyecto Docker y volumen exclusivos `espacigo_fe06`, Go confirmó 10 cuentas anfitrión con roles explícitos, KYC sintético aprobado, publicaciones, tarifa, galería/portada. Como el registro no concede el rol de anfitrión, los roles se prepararon como fixtures explícitos únicamente en la base desechable; KYC se completó por el flujo administrativo local de Go; y 3 arrendatarias verificaron búsqueda/detalle/cotización, rechazo de acceso privado y solicitudes aceptadas o conflictos HTTP 409 confirmados por Go. La cuenta y los pasos de recreación están en `.local/testing/cuentas.md` (modo 0600, ignorado localmente, nunca versionado). Se capturó el panel Angular conectado a Go en [escritorio](evidencia/FE06_oferta_escritorio.jpg) y [móvil](evidencia/FE06_oferta_movil.jpg); la vista móvil se comprobó a 390×844 CSS px, aunque la superficie de captura del navegador integrado la limita a 375×260 px. No se usó el volumen persistente, no se alteraron rutas Go y se conserva el mock.

### Registro de avance FE-07 (2026-10-12)

**Estado:** aceptado y fusionado en el PR #282 (merge `978a27e4bc628c48b4ccffa55e784b369bbcd6d5`; commit de trazabilidad `08b3d7abe6a0941db7b42b3db9e535635382da90`). Autor del PR: HernanMEC. FE-06 y FE-07 quedan aceptados/fusionados; FE-08 comienza en esta rama desde `main` actualizado.

**Implementación:** bandejas autenticadas por participante, detalle e historial confirmado por Go, solicitud desde cotización, decisión del anfitrión, pago fake (éxito/rechazo/timeout), política y cancelación, devolución fake. Los módulos de API/estado/componentes están separados. Los intentos inciertos conservan clave y payload en memoria; importes int64 se parsean desde texto JSON a decimal exacto; sesión Bearer solo en memoria. Go decide acceso y transición; roles no se infieren de preferencias. Go mantiene sus rutas y el mock permanece disponible.

**Validación:** 12 pruebas de componente FE-07 con IdentitySession real y una prueba de store; suite Angular consolidada: 112 pruebas en 19 archivos; `npm run api:check`; `npm run build` con prerender de 3 rutas. Las regresiones cubren 4xx de un Bearer previo tras logout/login de la misma cuenta, devolución/cancelación con POST pendiente y GET posterior completada, lectura antigua de devolución fallida posterior a una devolución exitosa, respuestas de cancelación fuera de orden, recarga fallida o sin campos de devolución y protección monotónica del snapshot confirmado. El proyecto/volumen Docker aislado `espacigo_fe07`, Mailpit y Angular en puerto 18420 verificaron cinco cuentas sintéticas (administración de fixtures, anfitrión y tres arrendatarias), registro/confirmación por Mailpit y alta de roles/KYC/KYB mediante las rutas locales admitidas, con rol anfitrión fixture explícito solamente en la DB desechable. Go confirmó solicitud duplicada con la misma clave, rechazo de tercero, pago exitoso y autorizado, aprobación del anfitrión, cancelación con devolución fake, rechazo de pago, timeout 504 con reintento incierto y rechazo del anfitrión. La UI Angular conectada completó login, búsqueda/cotización, solicitud, pago fake, aprobación, cancelación y devolución pendiente → completada confirmadas por Go. La respuesta autoritativa de cancelación aporta la operación y el importe de devolución que Go no repetía en el GET de detalle; se conserva como snapshot en memoria y se presenta solo para la misma cuenta/reserva. La respuesta del POST se aplica antes de recargar; un GET posterior reemplaza el estado y snapshot si Go devuelve campos confirmados, mientras que GET sin esos campos reutiliza la confirmación guardada. Una recarga fallida mantiene la confirmación del POST. Las respuestas fuera de orden no degradan la vista ni el snapshot. Las credenciales locales están en `.local/testing/fe07-cuentas.md` (modo 0600, excluidas localmente; no versionadas ni incluidas en capturas/logs). Evidencia: [devolución completada escritorio](evidencia/FE07_devolucion_completada_escritorio.jpg), [devolución completada móvil](evidencia/FE07_devolucion_completada_movil.jpg), [bandeja escritorio](evidencia/FE07_reservas_escritorio.jpg), [bandeja móvil](evidencia/FE07_reservas_movil.jpg), [detalle escritorio](evidencia/FE07_reserva_detalle_escritorio.jpg), [decisión anfitrión móvil](evidencia/FE07_decision_anfitrion_movil.jpg). La comprobación móvil fue a 390×844 CSS px; las capturas reflejan la superficie disponible del navegador integrado. No se tocó el volumen persistente ni se modificó Go o el mock. Los resultados son locales y sintéticos: no prueban proveedores, cobros, devoluciones ni entrega real.

### Hitos para revisar avance

- **H1 visible:** FE-00–03. Portada y primer recorrido real de búsqueda/cotización.
- **H2 anfitrión/arrendatario:** FE-04–08. Publicación → reserva → pago → contrato → operación.
- **H3 cobertura completa:** FE-09–11. Comunicación, disputas, finanzas y administración.
- **H4 frontend reemplaza mock:** FE-12–15. Diseño, calidad, Docker, aceptación y retirada.

## 7. Docker y coexistencia

Desarrollo rápido: Angular en localhost:4200 con API local o proxy de desarrollo; imagen de ejecución separada para comprobar prerender/SSR y CSR. Reservar el puerto, verificar que esté libre y permitir override; en stacks aislados usar otro puerto propio. No cambiar silenciosamente puertos 8080/8081/18080/18081 existentes.

`frontend/Dockerfile` usa contexto `frontend/`; Go excluye frontend/node_modules/dist del contexto correspondiente. `compose.frontend.yaml` añade servicio Angular a la topología existente; el servicio frontend solo participa en la red API. Conserva los secretos y volúmenes y las credenciales runtime/admin separadas. CI/build por directorio; cambios OpenAPI ejecutan verificación del cliente y pruebas backend pertinentes.

Dos URLs públicas/configurables: `API_PUBLIC_BASE_URL` para navegador y `API_INTERNAL_BASE_URL` para SSR autorizado no privado (`http://backend:8080/api/v1` solo dentro de Docker). Config pública en ejecución, sin rebuild para cambiar el host y sin secretos. CORS del Backend permite los orígenes del mock y Angular durante transición. No declarar que un servicio Docker es accesible por su nombre desde el navegador. Healthcheck frontend verifica su propio servicio; caída API genera una vista recuperable, no un éxito ficticio.

La configuración SSR futura no modifica la sesión Bearer elegida ni crea un BFF financiero. El servicio Angular escucha en `0.0.0.0` y acepta puerto configurable; esto prepara el contenedor, sin desplegar ni certificar Cloud Run.

**Criterio de cierre FE-13:** FE-12 está aceptado/fusionado en #287. La validación se implementa con `scripts/local-docker-stack.sh up-frontend|verify-frontend|down-frontend` para un proyecto y volumen explícitos; `scripts/test-frontend-docker.sh` selecciona puertos/proyecto/volumen/secretos propios y elimina únicamente los recursos que acaba de crear. El runner integrado no recibe credenciales y verifica shell Angular, `/config.json`, readiness de Go, origen CORS y preflight de login. La URL pública se configura para que el navegador del host alcance la API publicada; el runner usa `http://backend:8080` solo para tráfico interno. El corte conserva mock y topología Go, sin acceso a la red de datos ni credenciales DB desde Angular/runner. Registrar evidencia y resultados reales por commit en la trazabilidad FE-13 y la matriz. FE-14 permanece pendiente de aprobación y fusión de FE-13.

## 8. Validación sin ciclos redundantes

El documento propone pruebas para ejecutar durante implementación; **esta planificación no ejecuta suites ni crea tests**.

| Cambio | Validación mínima propuesta |
| --- | --- |
| Tokens/layout/texto | Build/lint frontend y revisión de pantallas afectadas; sin suite Go completa. |
| Estado/formulario/HTTP | Tests de componente/servicio, respuestas tardías y errores pertinentes, contrato generado y E2E enfocado. |
| API/Go/schema | Pruebas backend correspondientes y cliente/frontend afectado; DB siempre desechable. |
| Hito H2/H3 | Recorrido conectado y regresión de features entregadas; reutilizar evidencia inalterada identificada por commit. |
| FE-14/15 | Gate integral frontend + backend y recorrido Docker, sin omisiones ocultas; repetir si cambia código relevante después. |

Escenarios obligatorios: visitante, arrendatario, anfitrión, admin y tercero; roles coexistentes; cuenta sin verificar/sin KYC/bloqueada; 401/403/404/409/422; network timeout/API caída; doble clic/retry; logout/login misma cuenta; respuestas fuera de orden; cambio de filtro/selección; zonas horarias/DST; montos fuera de precisión JS; históricos minimizados; PDF/ZIP/PNG vacío/retirado; operaciones financieras inciertas.

No usar como aceptación solo el número de tests. La evidencia registra operación, usuario, respuesta/estado persistido, commit/imagen, comandos y limitaciones. Para descargas E2E capturar el evento de descarga y abrir el archivo; si una herramienta no proporciona ese evento, declarar el límite y conservar prueba HTTP sin afirmar descarga física.

Pruebas de liquidación con reloj sustituible en recursos desechables; la comprobación real de 24 horas continúa independiente. No falsear timestamps de la base activa para acelerar una demo. Las cuentas de ensayo pueden regenerarse/restablecerse de forma reproducible, con credenciales fuera de Git, sin bloquear el avance por una contraseña desconocida.

## 9. Retirada completa del mock

Retirar solo tras FE-14 aceptado; no al completar home/login. Inventariar antes todos los consumidores de `mock/`, `cmd/mock`, `internal/mockserver`, targets del Dockerfile, Compose, npm y runners.

1. Trasladar los escenarios de carreras/idempotencia del mock a tests Angular; no descartar la protección al borrar sus archivos.
2. Adaptar `test-local-qa.sh`, `verify-http.py`, `verify-m01.py`, scripts Docker y documentación para que acepten el origen/recursos Angular sin buscar `app.js` del mock o su `config.json` antiguo.
3. Dejar Angular como servicio por defecto y retirar el mock del arranque habitual; comprobar el recorrido desde cero.
4. Eliminar código/targets/dependencias exclusivas del mock en un PR enfocado. Conservar tests de dominio/API Go y su historial Git; no mantener otra copia congelada del código en el checkout.
5. Confirmar que un build API no compila Angular/Node y que el build Angular no necesita migraciones/credenciales DB. Consolidar la aceptación local sin cerrar padres productivos.

## 10. Estimación y seguimiento

Estimación orientativa de trabajo enfocado para un desarrollador; no fechas prometidas ni medición de velocidad de un agente. Validar después de FE-03.

| Hito | Jornadas estimadas |
| --- | ---: |
| H1 — base, identidad y explorar/cotizar | 8–13 |
| H2 — cuenta/KYC/oferta/reserva/contrato/operación | 12–20 |
| H3 — comunicación/disputas/finanzas/admin | 9–15 |
| H4 — diseño final, Docker, aceptación y retirada | 7–12 |
| Total base | 36–60 |

Añadir margen de aproximadamente 20% si aparecen incompatibilidades de generación, dinero exacto, bibliotecas o endpoints. Rangos no incluyen proveedores reales, GCP ni nuevo portal público. Calibrar con entregas reales, no repartir un porcentaje ficticio por módulo.

Estado vigente: FE-09 aprobado y fusionado en [PR #284](https://github.com/HernanEspinozaDev/espaciGo/pull/284), merge `0d3bac8a06457899239a49274b54ad6807846f42`; FE-10 aprobado y fusionado en [PR #285](https://github.com/HernanEspinozaDev/espaciGo/pull/285), merge `39514662c9002d3c50bdbe7244813c57d13c2b38`, rama `codex/frontend-local-1-fe10-disputes-finance`. FE-11 se trabaja desde `main` actualizado. El backend y Docker aportan la base reutilizable; no cuentan como pantallas Angular terminadas. Cada FE mantiene estado, PR/commit y evidencia. Si un contrato falla, registrar brecha concreta y ajuste mínimo; no reabrir todo LOCAL-1.

### FE-08 — Contrato y operación (rama `codex/frontend-local-1-fe08-contract-operations`)

**Estado:** aprobado y fusionado en [PR #283](https://github.com/HernanEspinozaDev/espaciGo/pull/283), merge `503d19359e94c9f187bb955047c150aef8b97507`; HEAD `e85e8cc493589badb34dd305dcf128c57982667c`. FE-07 se aceptó y fusionó en PR #282, merge `978a27e4bc628c48b4ccffa55e784b369bbcd6d5`.

El detalle de la reserva integra generación/consulta de snapshot, firma/rechazo, descarga de PDF privado, check-in/out, recepción e historial/evidencia PNG sintética privada. La UI condiciona las acciones a participación y estado que informó Go; el backend mantiene la autoridad. Se reutiliza el cliente generado y los accesos HTTP están separados del panel. No se admiten documentos reales ni bytes arbitrarios.

Bearer permanece en memoria. Respuestas tardías se descartan por identidad, token, generación y reserva; intentos inciertos conservan idempotency-key y payload en memoria por cuenta/reserva/operación, sin Bearer. Los Blob URLs de PDF/PNG se revocan en logout, cambio de recurso y destrucción del panel.

**Validación integrada:** pruebas de componente con `BookingDetailPage`, `BookingLifecyclePanel` e `IdentitySession` real cubren las dos firmas, actualización del padre desde Go, check-in, check-out y recepción, rechazo, intento incierto con payload no vacío y misma clave al reautenticar, respuesta 4xx antigua que no borra un intento nuevo, aislamiento de formulario y respuestas tardías. En `espacigo_fe08` (PostgreSQL/Mailpit desechables), el navegador Angular operó una reserva Go recién aprobada: ambas firmas, descarga privada solicitada, check-in/out con comentarios sintéticos y recepción del anfitrión; la UI mostró estados e historial devueltos por Go. Go además rechazó a terceros en reserva, contrato, PDF, operaciones y evidencias, y confirmó el rechazo de versión. No se usaron proveedores reales.

**Pruebas de corte:** `npm test -- --watch=false` — 125 pruebas en 22 archivos; `npm run api:check`; `npm run build` — prerender de 3 rutas. El cliente FE-08 ahora omite `Cache-Control` en solicitudes CORS; la preflight Go no admitía ese encabezado y bloqueaba las acciones en navegador.

**Evidencia visual:** [recorrido escritorio](evidencia/FE08_flujo_angular_go_escritorio.png) y [recorrido compacto](evidencia/FE08_flujo_angular_go_movil.png), capturas de página completa con detalle y estados confirmados por Go. El viewport compacto efectivo fue 500×758 CSS px (488 px de contenido, sin desbordamiento horizontal); el Firefox Snap del host impone 500 px como ancho mínimo, por lo que no se pudo medir 390 px. La sesión de las capturas se obtuvo de Go y se inyectó solo en memoria en el navegador de prueba; el formulario de login visual no forma parte de esta evidencia. La descarga privada se comprobó en la respuesta Go y en la acción Angular; el navegador no expone una aserción automatizada del archivo guardado por el sistema operativo.

FE-08 queda aceptado. FE-09 aprobado y fusionado en PR #284 (`0d3bac8a06457899239a49274b54ad6807846f42`); FE-10 aprobado y fusionado en PR #285; FE-11 aprobado y fusionado en PR #286.

## FE-09 — Conversación, lectura, reseñas y reportes (aceptado y fusionado)

**Estado:** aprobado y fusionado en [PR #284](https://github.com/HernanEspinozaDev/espaciGo/pull/284), merge `0d3bac8a06457899239a49274b54ad6807846f42`, rama `codex/frontend-local-1-fe09-conversation`. FE-10 aprobado y fusionado en PR #285; FE-11 aprobado y fusionado en PR #286.

El detalle de reserva integra `BookingConversationPanel` y `BookingCommunicationApi`. Usa operaciones Go para páginas de mensajes, envío, cursor de lectura, contador de no leídos, reseñas de reserva, reporte de reseñas, reseñas públicas de espacios y reputación propia. El mensaje se renderiza como texto. Solo los estados editables autorizados por Go permiten enviar; los terminales quedan en modo de lectura. La UI no concede permisos ni altera estados.

Bearer permanece en memoria. Intentos inciertos guardan clave y payload originales, aislados por cuenta/reserva/acción y sin Bearer. Lecturas/escrituras y efectos de 401 se validan contra sesión, token, reserva y generación; respuestas viejas no sobrescriben otra página, sesión o contador. La paginación de mensajes deduplica y ordena elementos; el cursor de lectura solo avanza cuando el mensaje final ya está renderizado. El número no leído se consulta desde la bandeja Go, cuya respuesta incluye el contador autoritativo.

**Validación Go desechable:** `espacigo_fe09_20261012` con volumen nuevo `espacigo_fe09_pgdata`, sin tocar volúmenes persistentes. El recorrido verificó 5 cuentas sintéticas, 4 sesiones distintas, paginación de 38 mensajes (30 + anteriores), incremento y limpieza del contador, acceso titular/tercero, lectura terminal y rechazo de escritura, reseña por anfitrión/arrendatario, reporte del anfitrión y reputación. Se activó una publicación propia del anfitrión mediante las operaciones Go y se comprobó `GET /spaces/{spaceID}/reviews` con HTTP 200 y 1 reseña.

**Regresión:** `npm test -- --watch=false` — 141 pruebas en 24 archivos; incluye sesión real, cambio de sesión y respuestas tardías/401, paginación con páginas solapadas y refresh concurrente (éxito/error viejo), terminal de solo lectura, texto seguro, avance del cursor tras render, actualización de no leídos y clic DOM real de reintento con formulario limpio, misma clave/payload e intento aislado de otra cuenta. Crear reseña vuelve a leer reseñas de reserva y resumen público; una consulta pública vieja no sobrescribe el resultado nuevo. `npm run api:check` valida generación OpenAPI y tipos; `npm run build` prerenderiza 3 rutas.

**Revisión visual autenticada (12-10-2026):** Angular en `127.0.0.1:4200` respondió HTTP 200 y Go `GET /health/ready` HTTP 200 (`ready`) con `apiBaseUrl=http://127.0.0.1:18180/api/v1`. El navegador integrado abrió la app; no reapareció `ERR_BLOCKED_BY_CLIENT`, así que no hubo permiso del navegador que revisar. Tras detectar que un ID de reserva guardado en un fixture antiguo ya no correspondía a la cuenta, se consultó la lista actual de Go y se usó la reserva participante confirmada. Con `fe09-renter-04`, Go confirmó `aprobada_host`, 40 mensajes, carga anterior de página desde la interfaz, reseña pública promedio 5/5 (1), y un mensaje sintético que, tras detener brevemente el backend desechable, mantuvo el control «Reintentar mensaje original» y se guardó una sola vez (#40) al volver Go. Con `fe09-host-02`, Go confirmó reserva finalizada en solo lectura, reseña del arrendatario reportada, reseña del anfitrión publicada y resumen público actualizado. Capturas verificadas: conversación desktop [1413×1004](evidencia/FE09_conversacion_escritorio.jpg), móvil [473×824](evidencia/FE09_conversacion_movil.jpg); reintento incierto desktop [1413×1004](evidencia/FE09_reintento_escritorio.jpg); reseñas/reportes desktop [1413×989](evidencia/FE09_reportes_escritorio.jpg), móvil [473×824](evidencia/FE09_reportes_movil.jpg). El navegador dejó tokens en memoria; no se publicaron credenciales ni capturas contienen secretos. El stack desechable fue reiniciado y readiness quedó en HTTP 200.


## FE-10 — Disputas y finanzas (aceptado y fusionado)

**Estado:** aprobado y fusionado en [PR #285](https://github.com/HernanEspinozaDev/espaciGo/pull/285), merge `39514662c9002d3c50bdbe7244813c57d13c2b38`. Implementación `868116cbe4508b620bb7edc69482cfffcd6cc90c`; correcciones de revisión `b2da3243becb9db0fa10828baf5cae9782af474f`. FE-11 fue aprobado y fusionado en PR #286.

La ruta de reserva integra incidencias, historial de reclamos/descargos, evidencia sintética privada, garantía fake, decisión administrativa de reclamos, liquidación fake, conciliación y comprobante de ensayo con las operaciones M10 de Go. La vista de administración requiere el rol que devuelve Go. La UI distingue cierre de incidencia, resolución del reclamo y decisión financiera; solo habilita registrar una decisión económica después de que Go confirme la resolución, y refleja su respuesta. Los artefactos privados se descargan con Bearer y Blob URLs temporales.

Los enteros CLP se leen como cadenas decimales desde JSON textual. Los importes ingresados se limitan a rangos locales pequeños antes de convertirse a entero seguro; el servidor sigue validando autorización e importe. Los POST conservan clave y payload solo ante resultado incierto y no almacenan Bearer. Las lecturas y efectos de 401 se aíslan por sesión, token, reserva, generación y ciclo de vida.

**Validación desechable:** stack independiente `espacigo_fe10_20261012`, PostgreSQL nuevo, API 18280, Mock 18281 y Mailpit 18225; volumen prefijado `espacigo_fe10_20261012_pgdata`, sin tocar volúmenes persistentes. Cuentas admin, anfitrión, arrendatarios participantes y tercero sintético. Go confirmó acceso participante y rechazo 404 a terceros; el anfitrión abrió incidencia y reclamo; el arrendatario participante presentó descargo y el tercero no pudo consultar; administrador cerró la incidencia y resolvió el reclamo. Se generó/consultó evidencia PNG privada con descarga autenticada; logout revocó el Blob URL y eliminó el `<img>` privado. La interfaz completó la resolución primero y el registro de una deducción fake de CLP 1.000 ligada a evidencia sintética. La consulta administrativa desde Angular mostró a Go confirmar `acogido`, `dano_acreditado`, evidencia PNG, `deduction_clp=1000` y estado `pendiente` (no aplicada mientras no hay captura confirmada).

En el recorrido de navegador de este fixture, la reserva seguía `en_disputa` con decisión financiera pendiente y el check-out no había alcanzado las 24 h: Go devolvió 404 al POST de liquidación y al comprobante aún no disponible. No se alteraron fechas ni estados. Para comprobar el camino exitoso sin cambiar ese fixture, se ejecutó `TestLocalFakeSettlementBoundaryTimeoutRestartAndConcurrentReplay` sobre PostgreSQL temporal con reloj inyectado: avanzó el reloj hasta el límite exacto de check-out + 24 h, confirmó la liquidación fake, transición a `cerrada`, un único comprobante y replay idempotente; además ejercitó el timeout y conciliación administrativa exitosa. `TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry` comprobó con reloj sustituible autorización, captura y liberación exitosas, replay y snapshot de garantía. Ambos tests crean bases descartables y las eliminan al finalizar; no usan ni modifican el volumen FE-10.

**Regresión y build:** Angular — 149 pruebas en 26 archivos; se añadieron cambios de reserva con GET fuera de orden, cambios de cuenta con intento incierto y protección de una nueva instancia frente a respuestas viejas exitosas/4xx. Los tests de componente usan `IdentitySession` real. `go test ./internal/adapters/postgres/booking -count=1` pasó sus tres integraciones en un PostgreSQL efímero con PostGIS; cubre reloj sustituible, autorización/captura/liberación, liquidación, conciliación, replay y comprobante. `npm run api:check` pasó con `ng serve` detenido; `npm run build` compiló y prerenderizó 3 rutas. Después, `ng serve` quedó reactivado y Angular volvió a responder HTTP 200; Go readiness respondió HTTP 200.

**Correcciones de revisión PR #285:** se invalida y limpia el contexto financiero administrativo al cambiar la reserva, las claves de intentos incluyen cuenta + reserva + operación (sin Bearer) y `FinanceApi` elimina únicamente la misma instancia de caché que originó el resultado. Las regresiones validan cambio de contexto mientras hay solicitudes y dos respuestas fuera de orden sobre una operación incierta. FE-10 aceptado y fusionado en PR #285; FE-11 fue aceptado y fusionado en PR #286; FE-12 comienza ahora.

**Evidencia visual:** [participante escritorio 1425×2787](evidencia/FE10_disputas_escritorio.jpg), [participante móvil 375×4050](evidencia/FE10_disputas_movil.jpg), [cola admin escritorio 1425×2289](evidencia/FE10_admin_escritorio.jpg), [cola admin móvil 375×2703](evidencia/FE10_admin_movil.jpg), y [panel admin conectado actual escritorio 1265×712](evidencia/FE10_admin_actual_escritorio.jpg) / [móvil 375×812](evidencia/FE10_admin_actual_movil.jpg). Las capturas del panel actual son viewport del navegador integrado; las capturas anteriores son completas.


## 11. Gobierno y definición de terminado

La implementación se publica por cortes con HernanMEC y solicita revisión a HernanEspinozaDev según el flujo existente. Los cambios documentales ajenos se conservan sin incorporarlos al corte. FE-07 fue fusionado en #282; FE-08 fue aprobado y fusionado en #283; FE-09 fue aprobado y fusionado en #284 antes de iniciar FE-10.

FRONTEND-LOCAL-1 se acepta cuando:

- [ ] Paleta/SCSS y navegación coherentes; Angular standalone/strict, clientes generados y límites de imports.
- [ ] Funciones de UI M01–M11 locales cubiertas en la matriz, con acceso por rol/titularidad y estados reales Go.
- [ ] Primer recorrido y variantes relevantes comprobados en Angular, no reutilizados como si fueran capturas del mock.
- [ ] Montos exactos, fechas IANA, idempotencia y aislamiento de sesión/selección preservados.
- [ ] Archivos y descargas privadas, privacidad, minimización y retención reflejan las políticas actuales.
- [ ] Frontend/API/DB/worker/migrador independientes en Docker; reinicio y configuración reproducibles.
- [ ] Accesibilidad y responsive revisados; problemas críticos pendientes resueltos y límites menores registrados.
- [ ] Gate consolidado y recorrido integral pasan en el árbol final; aceptación explícita de paridad y retirada del mock.
- [ ] Mock eliminado del código activo y runners/documentación actualizados, backend todavía verificable.
- [ ] LOCAL-DOCKER aceptado; sin afirmaciones de despliegue, fondos reales o cumplimiento jurídico probado.

## 12. Fuentes

Documentos aportados por el usuario y conversación en el checkout local. Implementación: `../openapi.yaml`, `../matriz_cierre_backend_local.md`, `../local_docker.md`, `../../internal/booking/transport/http/handler.go` y mock vigente. Angular, generación y renderizado: enlaces oficiales de las secciones técnicas, consultados el 2026-10-11. Las propuestas técnicas se revalidan al fijar versiones; no se atribuye aprobación del usuario a herramientas todavía no implementadas.


## FE-11 — Administración completa (aprobado y fusionado)

**Estado:** aprobado y fusionado en [PR #286](https://github.com/HernanEspinozaDev/espaciGo/pull/286), autor HernanMEC y revisión de HernanEspinozaDev. FE-12 comienza sobre `main` actualizado.

La ruta `/administracion` mantiene el guardado por rol confirmado en la sesión Go y el servidor decide los permisos efectivos. El panel coordina colas de verificaciones/evidencias, privacidad y supresión, reportes de reseñas, avisos terminales y reapertura, cuentas/bloqueo, publicaciones, reservas, auditoría/exportación y reportes agregados. Las acciones administrativas solicitan motivo y confirman acciones destructivas; las operaciones repetibles conservan clave/payload inciertos en memoria y por cuenta/ámbito, sin Bearer. Las lecturas se aíslan por token/cuenta/revisión; un 401 expira únicamente el token que lo originó y un 403 deja la identidad en manos de Go. Finanzas conserva importes CLP decimales exactos. Evidencia KYC se descarga con Bearer y Blob URL temporal, con revocación al cambiar selección, sesión o vista.

La matriz registra las 29 operaciones OpenAPI del corte y su cobertura UI. Regresión Angular actual: `npm test -- --watch=false` — 163 pruebas en 28 archivos; incluye replay del payload completo incierto con `correlation_id`, 403 vigente que revoca evidencia/controles y revalida capacidades Go sin borrar Bearer, 403 antiguo aislado y período de auditoría estable entre paginación/exportación. El panel se recorrió en Angular con sesión administradora confirmada por Go y siete cuentas sintéticas; se ejercieron aprobación KYC con evidencia privada, revisión y ejecución de supresión, moderación de reseña, bloqueo/desbloqueo de cuenta y exportación JSON de auditoría. Go confirmó cuenta activa tras desbloqueo, cola de reportes vacía después de moderar y 14 eventos en el JSON descargado. La solicitud KYC y la cuenta suprimida ya no aparecen en sus consultas posteriores (404).

El stack desechable usa `127.0.0.1:18380`, readiness `GET /health/ready` HTTP 200; el frontend respondió HTTP 200 en `127.0.0.1:4200`. Para poder validar la operación existente de reportes de reseñas se montó su handler Go, que estaba implementado pero no registrado en el router; prueba `TestReputationRoutesDoNotInterceptBookingOperations` pasa. Se usó un proyecto/volumen FE-11 desechable; no se eliminaron ni alteraron volúmenes persistentes. Capturas JPEG reales de Angular autenticado y sin credenciales: [escritorio 1440×914](evidencia/FE11_admin_escritorio.jpg) muestra KYC/evidencia privada, [móvil 500×758](evidencia/FE11_admin_movil.jpg) muestra auditoría; ambos viewports tuvieron `scrollWidth <= innerWidth`. Firefox headless impone mínimo 500 px de ancho, así que el ancho móvil es 500 px, no 390 px.

Límites: la paginación de auditoría está cubierta por regresiones, pero no se pudo avanzar una página en vivo porque la exportación devolvió 14 eventos (el límite por página es 25). No se generó terminal de avisos para ejercitar reapertura; no se atribuye esa mutación al recorrido vivo. `api:check` se ejecuta con `ng serve` detenido; el build/prerender debe completar tres rutas. `frontend/public/config.json` apunta localmente a `http://127.0.0.1:18380/api/v1` y se conserva fuera del PR. Credenciales y tokens permanecen privados/locales; Bearer solo en memoria.


## FE-12 — Consolidación visual y accesibilidad (aceptado y fusionado)

**Estado:** aprobado y fusionado en PR #287; merge `106942ff903a25b89628a383f1f7482805c46842`. FE-13 está en ejecución desde `origin/main` actualizado.

Este corte consolida la navegación y las reglas globales de foco/objetivos táctiles, atiende el menú móvil con Escape y retorno del foco, mueve el foco al título tras cambiar de ruta, anuncia el estado offline y vuelve a consultar readiness al recuperar conexión. Los roles, Bearer en memoria, claves idempotentes y payloads de operaciones inciertas no cambian. El estado sin conexión explica cómo comprobar si una escritura fue confirmada antes de reintentar. Se eliminó un ID `contenido` duplicado en identidad.

**Validación visual HEAD (12-10-2026):** Angular `127.0.0.1:4200` y readiness Go `127.0.0.1:18380/health/ready` respondieron HTTP 200. Con cuentas sintéticas de anfitrión y administrador, y acceso confirmado por Go, se recorrieron cuenta privada y bandeja de reservas, catálogo autenticado y ficha sintética. Se revisaron el estado vacío de perfil/foto/cobro, ficha sin horarios disponibles, y ocho reservas en estados terminales/expirados con textos largos. Capturas A/B/C en `matriz_paridad_mock_angular.md`; cada referencia tiene viewport móvil 390×844 CSS px y escritorio 1365×900 CSS px. La captura móvil mide 375×812 (scrollbar reservado de 15 px); `clientWidth=scrollWidth=375`, sin overflow. Escritorio mide 1350×890 y no presenta overflow. El panel de cuenta tiene una captura adicional en ambos tamaños.

**Teclado y contraste:** el Router real mantiene el foco en el encabezado al cambiar parámetros de una ruta reutilizada; solo una ruta anidada tiene `aria-current="page"`. Escape cierra el menú y devuelve el foco al botón. El contorno azul de 3 px contrasta 6.28:1 con el fondo Crema; el muestreo de texto visible de `/mi-cuenta` no encontró fallos bajo 4.5:1/3:1. axe-core no reportó infracciones A/AA en el shell compartido. Esto no es certificación WCAG ni cubre zoom 400% o lector de pantalla.

**Revisión visual complementaria del tercer hilo de #287:** revisé `/panel/espacios` y `/panel/espacios/:spaceId/calendario` con anfitrión, `/mi-cuenta/verificaciones` con KYC/KYB elegibles y aprobados, `/reservas/:id` con contrato firmado, operación completa y conversación de solo lectura, además del fixture terminal `Cancelada por firma`; y `/administracion` + `/administracion/disputas` como administrador. Go confirmó los estados. Las vistas presentan borrador con descripción larga, horario `America/Santiago` 09:00–17:00, PNG sintético privado, historial largo, cola KYC vacía y autorización sintética de CLP 50.000 sin captura/liberación/settlement. No ejecuté mutaciones de negocio. En las vistas revisadas a 390×844 y 1365×900 CSS px no hubo overflow horizontal; la banda de pestañas administrativas se desplaza dentro de su propio carril. Muestreo de contraste por página: cero fallos a 4.5:1/3:1, incluyendo ambos extremos del degradado Terracota; Tab mostró anillo visible azul de 3 px en controles/enlaces. Los JPEG representativos y las medidas están en la sección FE-12 de la matriz.

**Error/offline:** la reserva sintética de `.local/testing/fe11-angular-rejection-reservation.txt` ofrece un estado terminal real de Go (`Cancelada por firma`). La desconexión/offline se conserva como la regresión existente de `frontend/src/app/app.spec.ts`; readiness siguió saludable y no se deshabilitó la red ni se cortó Go para crear una captura falsa.

**Regresión final del HEAD:** 168 pruebas Angular en 28 archivos, `api:check` con `ng serve` detenido y build/prerender de 3 rutas. La prueba manual mantuvo readiness online; error de red y offline se cubren con regresiones del aviso/refresh, pero no se capturó una sesión visual sin red ni se interrumpió el servicio durante la revisión.

## Trazabilidad FE-13 — Stack Docker y paridad sin mock (en revisión)

**Estado:** en revisión en [PR #288](https://github.com/HernanEspinozaDev/espaciGo/pull/288), autor verificado `HernanMEC`, revisión solicitada a `HernanEspinozaDev`. Rama `codex/frontend-local-1-fe13-docker`, creada desde `origin/main` actualizado al merge #287 (`106942ff`). FE-12 aceptado/fusionado; FE-14 queda pendiente de la aprobación y fusión de este corte.

El cambio integra el servicio Angular y un runner técnico al Compose aislado. La configuración pública de navegador apunta a `127.0.0.1:<puerto API>`; los nombres `backend` y `angular-frontend` se usan solo en la red Compose. CORS incorpora la misma combinación de puerto local para `localhost` y `127.0.0.1`. El runner valida readiness Go y preflight de login; no recibe credenciales, no tiene acceso a la red de datos y no depende del mock. La imagen Angular se construye con `npm ci` y lockfile fijado; el mock permanece disponible.

**Validación (código `c6b46db9bb62ed7ba0ab943e1398e32b7645c450`):** `npm ci` agregó 403 paquetes y auditó 0 vulnerabilidades; Angular pasó 169 pruebas/28 archivos; `npm run api:check` y build/prerender (3 rutas) pasaron. La imagen Angular (Node 24.21.0 Alpine por digest, npm 12.1.0) se instaló desde cero. El runner confirmó shell/config/readiness Go HTTP 200, CORS/preflight HTTP 204 y URL pública del host. Angular quedó solo en la red `api`, sin credenciales DB; no se inició el mock. Se detuvo el backend: readiness dejó de conectar, Angular SSR y healthz permanecieron HTTP 200; al levantar otra vez Go, readiness y runner pasaron. La regresión Angular con `IdentitySession` real confirmó que error de red en readiness conserva sesión/token en memoria y la recuperación no los altera. Evidencia detallada: `evidencia/FE13_docker_stack.log`; imagen Angular `sha256:2b0b71689b4be10667e485afc6fd550fa7bb1fc3da5aa3caabcae1059eac72b6`, runner `sha256:83812f89384f93e01a5520122c4b87bd3bb97964a569476a7207c72f129308e8`. Proyecto, redes y volumen descartables fueron retirados; no se usó volumen persistente. No se ejecutó login visual durante la interrupción del backend. No se retira el mock: esa decisión queda para FE-15 tras aceptación de paridad. FE-14 sigue pendiente.
