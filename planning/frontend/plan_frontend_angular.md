# Plan completo del frontend Angular — FRONTEND-LOCAL-1

Fecha: 2026-10-11. Baseline de planificación: `b0f62b976c9586d260cea15ed9acc29ab12257a7`, V54. Estado actual: FE-00/FE-01 aceptados en PR #274; FE-02 aprobado y fusionado en PR #275; FE-03 aprobado y fusionado en PR #276; FE-04 aceptado y fusionado en PR #278; FE-05 implementado y en revisión. FE-06–FE-15 siguen pendientes, no son capacidades implementadas.

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

**Estado: implementado y preparado para revisión.** `/mi-cuenta` y `/mi-cuenta/privacidad` son rutas privadas CSR/noindex. El corte integra perfil, foto sintética privada, cuenta de cobro fake, solicitudes de acceso/supresión y exportaciones JSON/ZIP mediante el cliente generado y operaciones Go existentes. El Bearer se conserva en memoria; los 401 vigentes limpian el estado privado, las respuestas tardías no afectan otra sesión y las URLs Blob privadas se revocan al cambiar recurso, cerrar sesión o destruir la vista. Los vacíos/artefactos retirados reflejan la respuesta del servidor sin reconstrucción local.

**Validación y evidencia:** 55 pruebas Angular, `api:check`, build/prerender y pruebas Go enfocadas con PostgreSQL temporal/Mailpit desechable para titular/tercero, aislamiento entre dos cuentas, foto y cuenta fake, exportaciones y exclusión de credenciales/datos ajenos. Las pruebas de componente con IdentitySession real incluyen la creación del primer perfil tras GET 404, guardado concurrente a una solicitud, protección contra GET de perfil obsoleto posterior al PUT confirmado y avisos de foto después de recargas, cambio de sesión y destrucción de vista. La matriz de paridad registra por operación los recorridos y las capturas [escritorio](evidencia/FE04_cuenta_escritorio.png) y [móvil](evidencia/FE04_cuenta_movil.png). No se automatizó el diálogo del sistema operativo para verificar la escritura física de descargas; se inspeccionan los Blob y el contenido ZIP generado en pruebas. El mock se mantiene, Go no se modifica y no se inician GCP ni proveedores reales. FE-04 fue aceptado y fusionado en PR #278 antes de comenzar FE-05.

### Registro de avance FE-05 (2026-10-12)

**Estado: implementado y en revisión en [PR #279](https://github.com/HernanEspinozaDev/espaciGo/pull/279), commit `3405f05`.** `/mi-cuenta/verificaciones` es privada, CSR/noindex y consume el SDK OpenAPI generado. Presenta casos propios KYC/KYB, estados y elegibilidad confirmados por Go, historial, subsanación del rechazo y generación de evidencia exclusivamente mediante `synthetic-png-v1`. El cliente no acepta archivos ni bytes arbitrarios; los contenidos se descargan con Bearer y las Blob URLs temporales se revocan en cambio de caso, cierre/cambio de sesión y destrucción de vista. No asigna roles ni concede elegibilidad.

**Validación y evidencia:** 7 pruebas KYC enfocadas; regresión Angular consolidada de 62 pruebas en 14 archivos; `npm run api:check`; build/prerender; pruebas Go del handler y de repositorio PostgreSQL temporal (`tmpfs`) para autorización titular/tercero, estados, reintento y evidencia. Las regresiones Angular cubren 401 vigente/tardío, cambio A→B, PNG fuera de orden, Blob URL en cambio de recurso/logout/destrucción, y evidencia retirada. Capturas de escritorio/móvil usan fixtures sintéticos locales y se enlazan en la matriz. FE-04 fue aceptado y fusionado en PR #278; FE-05 queda en revisión y FE-06 pendiente hasta su aprobación/fusión.

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

A la fecha de este registro, FE-00/FE-01 están aceptados (PR #274), FE-02 está aprobado y fusionado ([PR #275](https://github.com/HernanEspinozaDev/espaciGo/pull/275)), FE-03 está aprobado y fusionado ([PR #276](https://github.com/HernanEspinozaDev/espaciGo/pull/276)), FE-04 fue aceptado/fusionado (PR #278) y FE-05 está en revisión. El backend y Docker aportan la base reutilizable; no cuentan como pantallas Angular terminadas. Cada FE mantiene estado, PR/commit y evidencia. Si un contrato falla, registrar brecha concreta y ajuste mínimo; no reabrir todo LOCAL-1.

## 11. Gobierno y definición de terminado

La implementación se publica por cortes con HernanMEC y solicita revisión a HernanEspinozaDev según el flujo existente. Los cambios documentales ajenos se conservan sin incorporarlos al corte. La aprobación humana de FE-05 precede al siguiente corte FE-06.

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
