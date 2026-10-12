# Diseño UX/UI y SCSS — Angular EspaciGo

Fecha: 2026-10-11. La identidad Terracota + Crema fue elegida por el usuario. Este documento concreta su aplicación al alcance local; no afirma que exista una interfaz implementada o una auditoría de accesibilidad aprobada.

## 1. Dirección visual

Estética cálida, sobria y clara: canvas crema, tarjetas blancas, texto oscuro, terracota para jerarquía y acciones. Orientada a **espacios comerciales flexibles**, no a vivienda: fotos/ilustraciones, superficie, capacidad y características de cada categoría; sin filtros universales de dormitorios, mascotas o baños si el perfil no los define.

Marca visible EspaciGo con logotipo tipográfico provisional. No bloquear desarrollo por un logo final. Fuente inicial `system-ui`; Inter autoalojada es la propuesta posterior si se incorpora con su licencia. Unificar tipografía antes de la aceptación visual, sin servicios externos de fuentes obligatorios.

## 2. Tokens de color

| Token CSS | Valor | Uso |
| --- | --- | --- |
| `--brand-primary` | `#A44731` | CTA, enlaces, selección principal. |
| `--brand-secondary` | `#8A6A4D` | Jerarquía secundaria; uso moderado. |
| `--brand-accent` | `#D97745` | Acentos decorativos; no botón con texto blanco pequeño. |
| `--color-canvas` | `#FBF7F2` | Fondo principal. |
| `--color-text` | `#332720` | Texto y encabezados. |
| `--brand-primary-hover` | `#843522` | Hover de la acción primaria. |
| `--brand-primary-active` | `#702A1B` | Acción presionada. |
| `--brand-primary-soft` | `#F5E3DB` | Superficie de selección. |
| `--color-surface` | `#FFFFFF` | Tarjetas/formularios. |
| `--color-surface-subtle` | `#F5EEE6` | Secciones secundarias. |
| `--color-text-muted` | `#66574E` | Texto secundario. |
| `--color-border` | `#DED5CC` | Separadores decorativos, no único límite de un input. |
| `--color-border-control` | `#8A796B` | Propuesta de borde visible de controles; verificar contraste en cada superficie usada. |
| `--color-focus` | `#1D4ED8` | Foco de teclado, independiente de la marca. |
| `--color-success` / `--color-success-soft` | `#15803D` / `#F0FDF4` | Resultado confirmado. |
| `--color-warning` / `--color-warning-soft` | `#B45309` / `#FFFBEB` | Pendiente/plazo/incertidumbre. |
| `--color-error` / `--color-error-soft` | `#B91C1C` / `#FEF2F2` | Error/rechazo. |
| `--color-info` / `--color-info-soft` | `#1D4ED8` / `#EFF6FF` | Información/contexto. |

La guía original aporta relaciones de contraste para algunos pares. No equivalen a validar todas las combinaciones del producto. Comprobar texto, foco, límites de controles y hover en las pantallas reales. Texto e iconos siempre acompañan los colores de estado; no reutilizar terracota para todos los badges.

## 3. Escala y SCSS

- Espaciado: 4/8/12/16/24/32/48/64 px, expresado en rem.
- Radios: 6/10/12/16 px, sin mezclar radios arbitrarios en features.
- Tipografía: base 1rem, interlineado 1.5; escala 0.875/1/1.125/1.25/1.5/1.875/2.25rem; 0.75rem solo metadatos secundarios no críticos.
- Montos con `font-variant-numeric: tabular-nums`; nunca truncar el monto o unidad necesaria para decidir.
- Contenedor público hasta 80rem; formularios de lectura hasta 42rem; paneles fluidos con tablas propias.
- Breakpoints de referencia: 40/48/64/80rem. Mobile first; grid/flex fluidos y container queries si resuelven un caso real.
- Acciones táctiles preferentemente 44px; foco visible de 3px con offset, movimiento reducido y ancho útil a zoom 200%/400%.
- Tema claro completo. Preparar tokens semánticos; modo oscuro diferido para evitar duplicar revisión de todos los flujos sensibles.

```text
src/styles/
├── abstracts/       # breakpoints, mixins y funciones Sass
├── tokens/          # marca, semántica, espaciado, tipos, radio, elevación
├── base/            # reset mínimo, fuentes y elementos base
├── themes/          # claro; oscuro posterior
└── styles.scss
```

Usar `@use`/`@forward`; includePaths fijado en Angular. Variables CSS para tokens y Sass para herramientas de compilación. Estilos encapsulados por componente. No Tailwind/Bootstrap, `::ng-deep` rutinario, `!important` general, selectores globales de features o anidamiento mayor a tres niveles sin motivo. Los HEX se definen en tokens; Stylelint limita su uso en componentes. CDK aporta comportamiento/foco/overlay, no la identidad visual.

## 4. Pantallas de referencia y navegación

### A. Portada y exploración

Portada informativa con marca, propuesta concreta y las ocho categorías. CTA “Explorar espacios” conduce al acceso si falta sesión; no mostrar un catálogo ficticio como disponibilidad real. Búsqueda en su página dedicada: texto, categoría, fechas/horarios y precio del intervalo; filtros avanzados para superficie, atributos y proximidad. Listado/mapa alternables, estado de filtros, limpiar todo y cursor siguiente/reiniciar.

Tarjeta: ilustración autorizada o placeholder, título, categoría, superficie, atributos disponibles, tarifa/unidad, total estimado si existe intervalo y distancia aproximada cuando procede. No inventar dirección, propietario ni estrellas si no vienen de una fuente autorizada. El mapa local lleva “Ubicación sintética de ensayo”; no requiere mapas externos.

### B. Ficha y cotización

Resumen con título/categoría/superficie; características por perfil, descripción y reglas; tarifa con periodicidad; zona horaria y disponibilidad. En escritorio, resumen de cotización lateral; en móvil, sección clara sin overlay que tape controles o foco. CTA “Elegir horario” y luego “Cotizar”; el CTA de solicitar reserva requiere una cotización vigente para el mismo espacio.

El contrato actual del catálogo no entrega la galería propia. Usar placeholder hasta disponer de acceso autorizado; no publicar fotos privadas de terceros desde Angular. Cambiar fechas/categoría/selección invalida cotización/cursor según el API. Explicar que cotizar no retiene disponibilidad.

### C. Reserva y resumen financiero

Detalle por pestañas: resumen/historial, pago, garantía, contrato, operación, conversación y reclamo. Acciones pendientes primero; cronología desde Go, con fechas y zona explícitas. Separar arriendo, garantía autorizada, capturada/liberada, devolución y liquidación; no sumarlos todos como si fueran un único cobro. La comisión local 0% se muestra solo si el snapshot/política lo indica.

Banner persistente “ENSAYO LOCAL — SIN COBRO REAL”; firma sintética y comprobante no fiscal claramente identificados. Timeout: “Resultado pendiente de confirmar”, clave conservada y acción de consulta/conciliación. Un error HTTP no se traduce automáticamente en pago rechazado.

### D. Panel anfitrión

Mis espacios, edición por pasos y reservas. Edición: datos → características → tarifa/zona/horario → galería/portada → revisión/publicación. Guardados parciales reales, sin autosave que modifique campos no tocados. Publicar/ocultar/eliminar son acciones distintas; conflictos muestran el motivo Go. Eliminar exige confirmación y explica la conservación histórica, sin prometer borrado en cascada.

### E. Cuenta y administración

Cuenta: perfil, verificación, seguridad y privacidad, con propósito de cada dato. Administración separada visualmente: colas, cuentas por UUID, reservas/publicaciones, reclamos/finanzas, auditoría/reportes y avisos. Mostrar solo proyecciones permitidas; no una consola JSON indiscriminada. Acciones privilegiadas solicitan el motivo estructurado y confirmación cuando corresponde.

El usuario puede alternar contexto anfitrión/arrendatario únicamente entre capacidades disponibles. El rol admin no se asigna desde la selección de contexto. La sesión restringida muestra un aviso y permite completar operaciones autorizadas existentes.

## 5. Biblioteca incremental

| Momento | Componentes |
| --- | --- |
| FE-01/02 | Button/Link, FormField, Input/Textarea/Select, Checkbox/Radio, Alert/Badge, Loading/Skeleton/EmptyState y layout. |
| FE-03/06 | SpaceCard, filtros/perfiles dinámicos, Pagination, selector de intervalos, Gallery/Cover y Drawer. |
| FE-07/08 | Dialog de confirmación, Tabs, Timeline, PriceBreakdown, PaymentStatus y visor privado de documentos. |
| FE-09–11 | Thread/ReadCounter, Review, EvidenceViewer, DataTable, periodos/reportes y exportación. |

Implementar solo componentes usados en el corte actual; no construir toda la biblioteca antes del primer recorrido. Semántica nativa primero; CDK para dialog/overlay/gestión de foco cuando corresponda. Cada componente documenta API, variantes y estados; las reglas financieras permanecen fuera de `shared/ui`.

## 6. Accesibilidad y estados

Objetivo WCAG 2.2 AA, sujeto a revisión real: labels persistentes, errores vinculados a controles, navegación por teclado, retorno de foco al cerrar diálogos, estados anunciables cuando procede y skip link. Tablas con encabezados; gráficos/reportes con alternativa textual. No forzar animación ni impedir zoom.

Cada pantalla contempla carga inicial, actualización, vacío, error recuperable, acceso denegado, sesión vencida y éxito confirmado. Preservar datos del formulario cuando se pueda reintentar de forma segura. No mostrar a una nueva sesión el resultado tardío de la anterior.

La validación visual de cierre revisa A/B/C y paneles a 360/768/1280px, además de zoom, teclado y contenido largo. No repetir la revisión visual completa por cada cambio de texto; revisar las pantallas afectadas. Formularios/finanzas necesitan variantes de error y pendiente, no solo captura del estado exitoso.

## 7. Mejoras sobre las fuentes aportadas

1. Mantener colores elegidos y reforzar el borde de controles, sin reemplazar la marca.
2. Sustituir dormitorio/postulación/visita por categoría/perfil, cotización/reserva y operación existentes.
3. Mantener referencias visuales como inspiración de jerarquía; crear componentes propios sin copiar marca/contenido de otra plataforma.
4. Mostrar datos de ensayo como ensayo; separar UI de producto de herramientas técnicas, que van al área de administración/entorno local.
5. No supeditar todo el frontend a SEO público, fuentes externas, tema oscuro, mapas comerciales o Storybook.
6. Adaptar continuamente la guía a endpoints reales. Los proveedores, nuevas reglas comerciales y portal público completo son otro alcance.
