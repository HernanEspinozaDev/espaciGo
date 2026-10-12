# Guía de diseño UX/UI y arquitectura SCSS

**Producto:** Marketplace inmobiliario integral de arriendos  
**Estado:** Decisiones de diseño aprobadas; guía inicial para implementación  
**Fecha:** 11 de octubre de 2026  
**Versión del documento:** 1.0  
**Documento complementario:** `guia_frontend_marketplace_inmobiliario.md`

> **Decisión principal:** adoptar **Terracota + Crema** como identidad visual, manteniendo una paleta semántica independiente y accesible para éxito, advertencia, error e información. Implementar **Angular + SCSS + Design Tokens + componentes reutilizables + Angular CDK**. No introducir Tailwind ni Bootstrap en esta línea de diseño.

---

## 1. Propósito, alcance y principios

Esta guía establece un contrato visual y de experiencia de usuario para los módulos públicos y privados del marketplace: búsqueda de inmuebles, fichas, postulaciones, visitas, contratos, arriendos, pagos, garantías, comisiones, administración y soporte.

Los objetivos son:

1. **Confianza:** comunicar operaciones sensibles, propiedad de los datos y estados de transacción sin ambigüedad.
2. **Cercanía:** transmitir la idea de hogar mediante una estética cálida, sobria y contemporánea.
3. **Claridad:** priorizar contenido, precios, disponibilidad, condiciones y próximas acciones.
4. **Accesibilidad:** diseñar para teclado, lectores de pantalla, zoom, contrastes adecuados y mensajes comprensibles.
5. **Consistencia:** permitir que distintos desarrolladores implementen experiencias homogéneas usando tokens y componentes compartidos.
6. **Escalabilidad:** un mismo sistema de diseño debe funcionar para portales públicos y paneles empresariales.

**Dirección de diseño elegida:** *Modern Warm Minimal*: superficies claras, fotografías inmobiliarias protagonistas, tipografía legible, bordes moderadamente redondeados, sombras discretas y acentos terracota reservados para acciones y jerarquía.

**Evitar:** estética excesivamente bancaria, gradientes decorativos en todas las pantallas, glassmorphism para formularios o transacciones, animaciones innecesarias, colores de estado utilizados como decoración, fondos terracota extensos que fatiguen la lectura.

## 2. Paleta oficial

### 2.1 Colores de marca (aprobados)

| Token semántico | HEX | Uso recomendado |
|---|---|---|
| `brand-primary` | `#A44731` | Acción principal, enlaces destacados, elementos de marca |
| `brand-secondary` | `#8A6A4D` | Detalles secundarios, etiquetas y elementos ornamentales discretos |
| `brand-accent` | `#D97745` | Acentos decorativos, resaltados no críticos y superficies, **no texto blanco pequeño** |
| `canvas-cream` | `#FBF7F2` | Fondo cálido principal del portal público |
| `text-primary` | `#332720` | Texto principal y encabezados |

### 2.2 Colores neutrales y de superficie (propuesta normativa)

| Token | HEX | Uso |
|---|---|---|
| `surface-default` | `#FFFFFF` | Tarjetas, menús, formularios, modales |
| `surface-subtle` | `#F5EEE6` | Paneles suaves, secciones diferenciadas |
| `surface-raised` | `#FFFFFF` | Popovers y paneles elevados |
| `border-default` | `#DED5CC` | Bordes y separadores |
| `border-strong` | `#AB9C90` | Bordes de mayor énfasis |
| `text-muted` | `#66574E` | Texto secundario, descripciones y metadatos |
| `brand-primary-hover` | `#843522` | Hover de botones y enlaces principales |
| `brand-primary-active` | `#702A1B` | Estado presionado |
| `brand-primary-soft` | `#F5E3DB` | Fondos para etiquetas o selección no crítica |
| `focus-ring` | `#1D4ED8` | Foco de teclado claramente distinguible del terracota |

Los neutros complementarios son **decisiones propuestas para implementar**; pueden ajustarse tras probarlos con contenido real sin alterar la identidad Terracota + Crema.

### 2.3 Colores de estado — independientes de la marca

| Estado | Texto/icono | Fondo suave | Ejemplos |
|---|---|---|---|
| Éxito | `#15803D` | `#F0FDF4` | Pago confirmado, contrato firmado |
| Advertencia | `#B45309` | `#FFFBEB` | Pago próximo a vencer, revisión pendiente |
| Error | `#B91C1C` | `#FEF2F2` | Transacción fallida, campo inválido |
| Información | `#1D4ED8` | `#EFF6FF` | Solicitud en evaluación, actualización informativa |

**Regla:** nunca depender únicamente del color. Todos los estados deben incorporar un texto explícito y, cuando ayude, un icono. No llamar “pagado” a una transacción hasta recibir confirmación válida del backend Go/proveedor. Diferenciar `procesando`, `pendiente`, `confirmado`, `fallido`, `reembolsado` y `disputado`.

### 2.4 Contrastes verificados para pares seleccionados

Cálculo de contraste relativo bajo WCAG:

| Combinación | Relación aprox. | Interpretación |
|---|---:|---|
| `#A44731` sobre `#FFFFFF` | 5,96:1 | Cumple AA para texto normal |
| `#A44731` sobre `#FBF7F2` | 5,59:1 | Cumple AA para texto normal |
| `#332720` sobre `#FBF7F2` | 13,56:1 | Muy alto contraste |
| `#8A6A4D` sobre `#FFFFFF` | 4,94:1 | Cumple AA para texto normal |
| `#D97745` sobre `#FFFFFF` | 3,15:1 | **No** cumple AA para texto normal |
| `#15803D` sobre `#FFFFFF` | 5,02:1 | Cumple AA para texto normal |
| `#B45309` sobre `#FFFFFF` | 5,02:1 | Cumple AA para texto normal |
| `#B91C1C` sobre `#FFFFFF` | 6,47:1 | Cumple AA para texto normal |
| `#1D4ED8` sobre `#FFFFFF` | 6,70:1 | Cumple AA para texto normal |

Son ejemplos de pares concretos, **no una certificación general de accesibilidad**. Validar todas las combinaciones efectivamente utilizadas, incluidos estados hover/disabled, bordes de controles y foco. Referencia objetivo: WCAG 2.2 nivel AA; normalmente 4,5:1 para texto normal y 3:1 para texto grande y componentes gráficos relevantes.

## 3. Estilo visual y layout

### 3.1 Tipografía

- Fuente propuesta: **Inter** o alternativa sans-serif comparable; confirmar licenciamiento y estrategia de carga local.
- Fuente de respaldo: `system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif`.
- Peso normal: 400; énfasis: 500/600; títulos: 600/700.
- Tamaño base recomendado: `1rem` (16 px con configuración predeterminada del navegador), interlineado de cuerpo cercano a 1,5.
- Escala inicial de tamaños: 12, 14, 16, 18, 20, 24, 30 y 36 px en unidades `rem` equivalentes; evitar tamaños diminutos en información financiera.
- No usar solo peso o color para señalar requerimientos o errores.
- Cantidades monetarias alineadas con números tabulares (`font-variant-numeric: tabular-nums`) en tablas y resúmenes.

### 3.2 Espaciado, radios, sombras y contenedores

- Sistema de espaciado basado en 4 px: 4, 8, 12, 16, 24, 32, 48, 64.
- Radios: pequeño 6 px; normal 10 px; tarjeta 12 px; modal 16 px; pastilla 9999 px.
- Sombras discretas; usar borde antes que sombra cuando baste para separar superficies.
- Portal público: ancho de contenido máximo orientativo 1200–1280 px; mantener márgenes responsivos.
- Panel administrativo: aprovechar anchura para tablas, pero limitar ancho de formularios de lectura secuencial.
- Fotografía inmobiliaria: relación consistente en tarjetas (p. ej. 4:3); permitir galería adaptable en ficha.

### 3.3 Breakpoints de referencia

| Nombre | Inicio | Orientación |
|---|---:|---|
| `sm` | 640 px | Teléfonos grandes |
| `md` | 768 px | Tablets y dos columnas |
| `lg` | 1024 px | Escritorio, tres columnas si caben |
| `xl` | 1280 px | Contenedores amplios |

**Mobile first**, sin depender únicamente de puntos de ruptura: priorizar flex/grid fluidos y container queries donde aporten valor. Verificar zoom de 200% y 400% y evitar desplazamiento horizontal innecesario.

## 4. Reglas UX por experiencia

### 4.1 Sitio público y búsqueda

- Búsqueda visible desde la portada: ubicación, rango de precios, tipo de inmueble y dormitorios.
- Filtros adicionales progresivos: superficie, baños, mascotas, estacionamiento, disponibilidad y otros datos confirmados por el negocio.
- Mostrar ubicación de forma compatible con privacidad: no exponer direcciones exactas sin autorización; mapa aproximado cuando corresponda.
- Cards: fotografía, ubicación general, precio y periodicidad, atributos clave, disponibilidad y estado de verificación cuando corresponda.
- Precio explícito en CLP; gastos comunes, garantía, comisión y otros cobros informados por separado según el caso.
- Estados de carga, vacío, sin resultados y error con acciones claras para recuperarse.
- Página de propiedad: galería accesible, descripción, condiciones, costos, contacto/postulación y elementos de confianza verificables. No inventar distintivos de verificación.
- SEO: títulos únicos, metadescripciones, URL estable, canonical, sitemap y datos estructurados apropiados. El diseño debe permitir SSR de Angular en páginas indexables.

### 4.2 Postulación y visitas

- Formularios por pasos con indicador de progreso, guardado controlado y resumen final antes del envío.
- Explicar por qué se solicita cada antecedente sensible, finalidad y retención; no pedir documentos innecesarios.
- Distinguir `borrador`, `enviado`, `en revisión`, `aprobado`, `rechazado` y `retirado`.
- Mostrar fecha/hora de visitas con zona horaria y estado de confirmación.

### 4.3 Contratos y firma

- Visualizador con versión del contrato, participantes, fecha y estado de firmas.
- Acciones separadas para revisar, descargar y firmar; no confundir lectura con consentimiento.
- Confirmaciones claras antes de acciones irreversibles.
- Guardar comprobantes y evidencia de firma según el proveedor jurídico/electrónico elegido.

### 4.4 Pagos, garantías y split payments

- El **backend Go**, nunca Angular, determina importes definitivos, beneficiarios, comisiones, cobros, estado de pago y reparto.
- En UI mostrar desglose: renta, garantía, comisión, cargos adicionales, impuestos cuando correspondan y total.
- Identificar quién recibe cada importe y cuándo, según las capacidades reales del proveedor y los acuerdos aplicables.
- Garantía ≠ renta ≠ comisión: pantallas y estados diferenciados.
- Estados explícitos: `pendiente`, `procesando`, `confirmado`, `fallido`, `reembolsado`, `en disputa`.
- Tras redirección del proveedor mostrar “confirmando” hasta validación del backend; nunca confirmar basándose solamente en parámetros URL del navegador.
- Evitar doble envío accidental; explicar reintentos seguros y utilizar identificadores/idempotencia del lado de Go.
- Usar confirmación reforzada para devoluciones, cancelaciones y operaciones de alto impacto.
- Recibos y movimientos: fechas, folios, conceptos y montos claros; no mostrar datos de tarjeta sensibles.

### 4.5 Paneles privados

- Navegación por rol/capacidad: arrendatario, propietario/corredor y administrador; un usuario puede tener varios roles.
- Dashboard con “acciones pendientes” antes que métricas decorativas.
- Tablas con filtros, ordenación, estados legibles, paginación y exportaciones según permisos.
- Accesos y permisos deben verificarse siempre en el backend; ocultar botones no equivale a autorización.

## 5. Biblioteca de componentes del Design System

**Base UI (compartidos):** Button, IconButton, Link, Input, Textarea, Select, Checkbox, Radio, DatePicker, FormField, ErrorMessage, Badge, Alert, Toast, Dialog, Drawer, Tabs, Breadcrumbs, Pagination, Skeleton, EmptyState, Table, FileUpload, Tooltip, Stepper y CurrencyDisplay.

**Componentes de negocio:** PropertyCard, PropertyGallery, SearchFilters, PropertyPriceBreakdown, RentalApplicationStepper, VisitCalendar, ContractStatus, PaymentStatus, PaymentSummary, DepositTimeline, CommissionBreakdown y AuditEvent.

Cada componente debe documentar:

1. API pública (`@Input`, `@Output` o equivalentes Angular modernos) y variantes admitidas.
2. Tokens usados; estados default, hover, focus-visible, active, disabled, loading y error.
3. Comportamiento responsive y accesibilidad.
4. Ejemplos de uso y casos límite.
5. Pruebas unitarias y visuales cuando corresponda.

**Angular CDK:** utilizar selectivamente para overlays, focus management, accesibilidad y otras primitivas. No asumir que CDK aporta el estilo final. No introducir Angular Material completo sin una decisión explícita.

## 6. Arquitectura SCSS aprobada

### 6.1 Enfoque

- **SCSS y CSS nativo**: módulos Sass (`@use`, `@forward`) para mixins, funciones y configuración en compilación.
- **CSS custom properties** para tokens temáticos que deban existir en tiempo de ejecución.
- **Estilos encapsulados por componente Angular** para presentación específica.
- **Globales mínimos**: reset, tipografía base, tokens, estilos de elementos raíz y utilidades accesibles esenciales.
- **Sin Tailwind ni Bootstrap** en el stack base.
- **Hexagonal para lógica**, no para hojas de estilo; los estilos se rigen por Design System y fronteras de funcionalidades.

### 6.2 Estructura sugerida

```text
frontend/src/
├── styles/
│   ├── abstracts/
│   │   ├── _breakpoints.scss
│   │   ├── _mixins.scss
│   │   └── _functions.scss
│   ├── tokens/
│   │   ├── _brand.scss
│   │   ├── _semantic.scss
│   │   ├── _spacing.scss
│   │   ├── _typography.scss
│   │   └── _radius.scss
│   ├── themes/
│   │   ├── _light.scss
│   │   └── _dark.scss
│   ├── base/
│   │   ├── _reset.scss
│   │   └── _typography.scss
│   └── styles.scss
└── app/
    ├── shared/ui/
    │   ├── button/
    │   │   ├── button.component.ts
    │   │   ├── button.component.html
    │   │   └── button.component.scss
    │   └── ...
    └── features/
        ├── properties/presentation/components/property-card/
        │   ├── property-card.component.ts
        │   ├── property-card.component.html
        │   └── property-card.component.scss
        ├── contracts/
        └── payments/
```

### 6.3 Ejemplo de tokens CSS

```scss
/* styles/tokens/_brand.scss */
:root {
  --brand-primary: #A44731;
  --brand-primary-hover: #843522;
  --brand-primary-active: #702A1B;
  --brand-secondary: #8A6A4D;
  --brand-accent: #D97745;
  --brand-primary-soft: #F5E3DB;

  --color-canvas: #FBF7F2;
  --color-surface: #FFFFFF;
  --color-surface-subtle: #F5EEE6;
  --color-text: #332720;
  --color-text-muted: #66574E;
  --color-border: #DED5CC;
  --color-focus: #1D4ED8;
}
```

```scss
/* styles/tokens/_semantic.scss */
:root {
  --color-success: #15803D;
  --color-success-soft: #F0FDF4;
  --color-warning: #B45309;
  --color-warning-soft: #FFFBEB;
  --color-error: #B91C1C;
  --color-error-soft: #FEF2F2;
  --color-info: #1D4ED8;
  --color-info-soft: #EFF6FF;
}
```

```scss
/* styles/tokens/_spacing.scss */
:root {
  --space-1: 0.25rem;
  --space-2: 0.5rem;
  --space-3: 0.75rem;
  --space-4: 1rem;
  --space-6: 1.5rem;
  --space-8: 2rem;
  --space-12: 3rem;
  --space-16: 4rem;
  --radius-sm: 0.375rem;
  --radius-md: 0.625rem;
  --radius-card: 0.75rem;
  --radius-dialog: 1rem;
}
```

### 6.4 Sass modules y responsive

```scss
/* styles/abstracts/_breakpoints.scss */
@use "sass:map";

$breakpoints: (
  sm: 40rem,
  md: 48rem,
  lg: 64rem,
  xl: 80rem
);

@mixin up($name) {
  $value: map.get($breakpoints, $name);
  @if $value == null {
    @error "Breakpoint no definido: #{$name}";
  }
  @media (min-width: $value) {
    @content;
  }
}
```

```scss
/* components/property-grid.component.scss */
@use "abstracts/breakpoints" as bp;

.property-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: var(--space-4);

  @include bp.up(md) {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  @include bp.up(lg) {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
```

**Nota de implementación:** configurar `stylePreprocessorOptions.includePaths` en `angular.json` (p. ej. `src/styles`) si se desean importaciones `@use "abstracts/breakpoints"`. Mantener `@use`/`@forward`, no el antiguo `@import` de Sass.

### 6.5 Botón principal — patrón accesible de referencia

```scss
:host {
  display: inline-block;
}

.button {
  min-height: 2.75rem;
  border: 1px solid transparent;
  border-radius: var(--radius-md);
  padding: 0.625rem var(--space-4);
  font: inherit;
  font-weight: 600;
  cursor: pointer;
}

.button--primary {
  background: var(--brand-primary);
  color: #fff;
}

.button--primary:hover:not(:disabled) {
  background: var(--brand-primary-hover);
}

.button:focus-visible {
  outline: 3px solid var(--color-focus);
  outline-offset: 3px;
}

.button:disabled {
  cursor: not-allowed;
  opacity: 0.65;
}
```

No usar `brand-accent` (`#D97745`) como fondo con texto blanco normal, porque no alcanza contraste AA para texto de tamaño normal.

### 6.6 Normas de autoría SCSS

1. No introducir valores HEX literales en componentes de funcionalidad: consumir tokens, salvo excepciones justificadas en el Design System.
2. No introducir `!important` para resolver conflictos ordinarios de especificidad.
3. Evitar selectores globales profundos y `::ng-deep`; preferir APIs públicas y custom properties para personalización.
4. Limitar anidamiento Sass a ~3 niveles; evitar selectores sobreacoplados a la estructura HTML.
5. No crear mixins para cada propiedad CSS: abstraer patrones de verdad repetidos.
6. Mantener variantes explícitas para componentes compartidos (primary/secondary/danger, tamaños y estados).
7. Usar BEM **opcionalmente** en componentes grandes, no como obligación universal; priorizar nombres claros.
8. Mantener layout y estilos de negocios dentro de sus features; los tokens y componentes genéricos van en `shared`.
9. Documentar cualquier token nuevo y revisar su necesidad antes de incorporarlo.
10. Comprobar contraste, foco, movimiento reducido y contenido que se desborda antes de aprobar PRs.

## 7. Modo oscuro

**No es alcance obligatorio del MVP.** Preparar el sistema con tokens CSS semánticos permite incorporarlo posteriormente, pero no introducir un tema oscuro incompleto en pantallas contractuales y financieras. Si se implementa, validar todos los estados y contrastes en ambos modos; respetar preferencia del sistema y guardar elección explícita del usuario según política de preferencias.

## 8. Accesibilidad y lenguaje

Objetivo: **WCAG 2.2 nivel AA**, sujeto a auditoría y pruebas reales.

- HTML semántico, encabezados jerárquicos, labels visibles y mensajes de error asociados a controles.
- Navegación íntegra por teclado, foco claramente visible, trampas de foco solo donde corresponda a diálogos modales.
- Mensajes de estado anunciables para tecnologías de asistencia cuando sea pertinente.
- Objetivos táctiles cómodos y separación adecuada entre controles.
- Respetar `prefers-reduced-motion` y evitar animaciones obligatorias.
- Usar español claro apropiado para Chile, fechas comprensibles, montos en CLP y UF cuando aplique; explicitar periodicidad y unidad.
- Evitar mensajes ambiguos como “hubo un problema”; indicar qué ocurrió, si se cobró o no, y qué puede hacer el usuario.
- No depender de imágenes para información contractual o financiera esencial.

## 9. Gobernanza y herramientas

- **Storybook**: catálogo de componentes y variantes cuando se consolide la biblioteca compartida.
- **Stylelint** con soporte SCSS: convenciones, propiedades duplicadas y restricciones de selectores.
- **ESLint y TypeScript estricto**: componentes Angular y APIs de la biblioteca.
- **Pruebas de componentes**: estados, teclado, foco, validación y errores.
- **Pruebas visuales**: cambios de apariencia en pantallas representativas y breakpoints.
- **Auditoría de accesibilidad**: automatizada y manual; pruebas con teclado y lector de pantalla.
- **Control de cambios**: actualizar esta guía si cambian tokens, variantes o patrones UX aprobados.

## 10. Ruta de implementación

| Fase | Entregable | Criterio de aceptación |
|---|---|---|
| 1. Fundamentos | Tokens, reset, tipografía, breakpoints y estilos base | Compila Angular; contraste de pares principales revisado |
| 2. Biblioteca base | Buttons, Inputs, Badges, Alerts, Dialogs, Table | Variantes, foco, teclado y estados documentados |
| 3. Portal público | Home, búsqueda, resultados, ficha | Responsive, SSR y SEO compatibles; imágenes optimizadas |
| 4. Procesos del usuario | Registro, postulación, visitas | Formularios accesibles, progreso y errores claros |
| 5. Gestión privada | Paneles de arrendatario y propietario | Navegación por capacidades, estados consistentes |
| 6. Flujos sensibles | Contratos, pagos, garantías y comisiones | Confirmación de estado vía Go; desglose transparente |
| 7. Calidad | Storybook, Stylelint, pruebas visuales y accesibilidad | Sin problemas críticos pendientes antes de producción |

## 11. Decisiones cerradas y pendientes

### Decisiones cerradas

- Marca: **Terracota + Crema** (`#A44731`, `#8A6A4D`, `#D97745`, `#FBF7F2`, `#332720`).
- Alertas independientes: verde, ámbar, rojo y azul.
- Estilo: *Modern Warm Minimal*, profesional y accesible.
- CSS: **SCSS + variables CSS semánticas + Sass Modules**.
- UI: componentes compartidos Angular, estilos por feature y Angular CDK según necesidad.
- No usar Tailwind ni Bootstrap como base.
- Frontend: Angular, componentes standalone, Signals; NgRx SignalStore solo cuando aporte valor.
- Arquitectura de aplicación: Feature-Based + hexagonal ligera.
- Backend Go es autoridad de negocio y de split payments.

### Decisiones aún por validar en prototipos

- Nombre y logotipo definitivos de la marca.
- Familia tipográfica exacta y autoalojamiento.
- Ajustes finos de neutrales y radios después de probar pantallas reales.
- Iconografía definitiva y biblioteca concreta.
- Componentes que requerirán Angular CDK o implementación propia.
- Necesidad y alcance de un tema oscuro.
- Microcopy, reglas comerciales, comisiones visibles y detalles legales de contratos/garantías conforme al modelo final.

## 12. Criterio de finalización del diseño

Antes de considerar cerrado el Design System inicial, diseñar y validar **tres pantallas de referencia**: (1) portada + búsqueda, (2) ficha de inmueble + postulación, y (3) panel privado + resumen de pago/desglose de garantía/comisión. Estas tres pantallas deben compartir los mismos tokens, componentes y estados, funcionar en móvil y escritorio, y superar una revisión de accesibilidad.

---

**Regla rectora:** el color terracota identifica la marca y las acciones; los colores semánticos comunican estados; el backend Go determina la verdad de las operaciones. El sistema de diseño existe para proporcionar coherencia, no para imponer complejidad innecesaria.
