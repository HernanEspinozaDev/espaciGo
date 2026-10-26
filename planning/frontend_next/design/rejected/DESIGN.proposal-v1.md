---
version: alpha
name: EspaciGo — Terracota + Crema
description: Modern Warm Minimal para espacios flexibles y arriendos residenciales. Especificación de fase 1; todavía no aplicada al runtime Next.
colors:
  primary: "#A44731"
  primary-hover: "#843522"
  primary-active: "#702A1B"
  primary-soft: "#F5E3DB"
  secondary: "#8A6A4D"
  accent: "#D97745"
  canvas: "#FBF7F2"
  surface: "#FFFFFF"
  surface-subtle: "#F5EEE6"
  ink: "#332720"
  muted: "#66574E"
  border: "#DED5CC"
  control-border: "#8A796B"
  focus: "#1D4ED8"
  success: "#15803D"
  success-soft: "#F0FDF4"
  warning: "#B45309"
  warning-soft: "#FFFBEB"
  error: "#B91C1C"
  error-soft: "#FEF2F2"
  info: "#1D4ED8"
  info-soft: "#EFF6FF"
typography:
  heading-xl:
    fontFamily: system-ui
    fontSize: 2rem
    fontWeight: 700
    lineHeight: 1.15
    letterSpacing: "-0.02em"
  heading-md:
    fontFamily: system-ui
    fontSize: 1.25rem
    fontWeight: 650
    lineHeight: 1.3
  body-md:
    fontFamily: system-ui
    fontSize: 1rem
    fontWeight: 400
    lineHeight: 1.55
  label:
    fontFamily: system-ui
    fontSize: 0.875rem
    fontWeight: 600
    lineHeight: 1.4
  caption:
    fontFamily: system-ui
    fontSize: 0.75rem
    fontWeight: 400
    lineHeight: 1.5
rounded:
  sm: 6px
  md: 10px
  lg: 12px
  xl: 16px
spacing:
  xs: 4px
  sm: 8px
  md: 16px
  lg: 24px
  xl: 32px
  section: 48px
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.surface}"
    typography: "{typography.label}"
    rounded: "{rounded.md}"
    height: 44px
    padding: 12px
  button-primary-hover:
    backgroundColor: "{colors.primary-hover}"
    textColor: "{colors.surface}"
    rounded: "{rounded.md}"
  button-primary-active:
    backgroundColor: "{colors.primary-active}"
    textColor: "{colors.surface}"
    rounded: "{rounded.md}"
  button-secondary:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.primary}"
    rounded: "{rounded.md}"
    height: 44px
    padding: 12px
  text-body:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.ink}"
    typography: "{typography.body-md}"
  text-help:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.muted}"
    typography: "{typography.label}"
  form-input:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.md}"
    height: 44px
    padding: 12px
  account-card:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.xl}"
    padding: 24px
  account-secondary:
    backgroundColor: "{colors.surface-subtle}"
    textColor: "{colors.muted}"
    rounded: "{rounded.lg}"
  nav-active:
    backgroundColor: "{colors.primary-soft}"
    textColor: "{colors.primary-active}"
    typography: "{typography.label}"
    rounded: "{rounded.md}"
    height: 44px
  status-success:
    backgroundColor: "{colors.success-soft}"
    textColor: "{colors.success}"
    rounded: "{rounded.sm}"
  status-warning:
    backgroundColor: "{colors.warning-soft}"
    textColor: "{colors.warning}"
    rounded: "{rounded.sm}"
  status-error:
    backgroundColor: "{colors.error-soft}"
    textColor: "{colors.error}"
    rounded: "{rounded.sm}"
  status-info:
    backgroundColor: "{colors.info-soft}"
    textColor: "{colors.info}"
    rounded: "{rounded.sm}"
---

## Overview

EspaciGo conserva la identidad **Terracota + Crema** elegida por el usuario. Dirección: **Modern Warm Minimal**. La superficie principal de cuenta y acceso es **Configure**: facilitar completar datos, entrar y salir con claridad; no convertir el perfil en un hero de marketing ni un dashboard de métricas ficticias.

Esta especificación corresponde a la fase 1 del plan `planning/frontend_next/plan_ui_ux_marca_sesion_cuenta.md`. La maqueta es una simulación local, no un login nuevo. No se han importado estos tokens en `globals.css` ni se ha modificado la autenticación productiva por crear este archivo.

La aprobación de persistencia está registrada en `planning/frontend_next/adr_ui_auth_001_persistencia_bff.md`. Go conserva la autoridad; la implementación BFF/cookie corresponde a la fase 2. No reiniciar esa decisión al cambiar de modelo.

Fuentes de marca: `planning/frontend/referencias/guia_diseno_original.md`, `planning/frontend/diseno_scss_angular.md` y los valores históricos de `_tokens.scss`. Recuperar valores y reglas visuales, nunca Angular, sus reglas técnicas antiguas, servicios externos ni datos privados.

## Colors

- Primario terracota para una acción de mayor énfasis por contexto; blanco sobre `primary`, no blanco sobre `accent` para texto normal.
- Canvas crema para continuidad entre marketplace, acceso y cuenta; tarjetas blancas para agrupación funcional.
- `ink` y `muted` sostienen la jerarquía sin usar opacidad que reduzca contraste accidentalmente.
- `secondary` y `accent` son detalles discretos; el logo existente conserva sus colores/gradiente originales. No recolorear el SVG para hacerlo coincidir exactamente con un único token.
- Bordes decorativos usan `border`; límites de controles usan `control-border` cuando su identificación dependa del borde. Un separador tenue no se usa como única señal de un input.
- El foco `focus` azul está separado de la selección de marca. Anillo de 3 px con offset de 2 px; no eliminar outline sin sustitución equivalente.
- Exportaciones en `planning/frontend_next/design/`: DTCG JSON, CSS con prefijo `espacigo`, Tailwind v4 genérico y `next-semantic-map.css`. **No importar `theme.css` directamente en el runtime actual:** sus primitivas `muted`, `secondary` y `accent` no significan lo mismo que las superficies semánticas Shadcn existentes. El adaptador separa, por ejemplo, el texto muted de su fondo y el marrón de marca del fondo secondary. Ningún export se importa durante fase 1.
- El linter puede advertir que accent/border/control-border/focus no están referenciados por componentes YAML: son colores de decoración/borde/foco documentados aquí, no pares de texto/fondo ficticios para silenciar avisos.
- Éxito, advertencia, error e información mantienen semántica propia con texto e icono cuando ayude; terracota no significa «pagado», «verificado» o «sesión válida».
- Contraste sRGB recalculado: blanco/primary 5,96:1; blanco/hover 8,36:1; ink/canvas 13,56:1; muted/canvas 6,48:1. Blanco/accent 3,15:1: combinación excluida de texto normal.

## Typography

Se conserva deliberadamente la familia local `system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif` para legibilidad, ausencia de descargas y coherencia con los fallbacks existentes. No agregar fuentes CDN como requisito visual.

Títulos de página 32 px; secciones 20 px; cuerpo 16 px; labels 14 px; ayudas complementarias 12 px con contraste alto. El texto funcional principal no baja a caption por falta de espacio. En móvil reducir la escala del título cuando sea necesario, no el tamaño de controles.

ID de cuenta en monospace solo donde mejore lectura; precios/tablas con cifras tabulares. No convertir todo el panel en monospace ni usar mayúsculas espaciadas para párrafos.

## Layout

- Contenedor público máximo orientativo 1280 px; workspace existente se mantiene, con sidebar de 240–256 px y contenido de anchura flexible.
- Cabecera pública: marca + navegación + CTA de sesión. Invitado: Ingresar a EspaciGo. Autenticado confirmado: Mi cuenta directo a `/mi-cuenta`.
- Cabecera de workspace: marca → inicio; nombre o ID confirmado y Cerrar sesión. No Acceso redundante en sesión autenticada.
- Acceso: marca real, título, explicación breve y formulario de ancho 400–440 px. Una columna en móvil; zona de contexto sobria en escritorio si cabe sin desplazar el formulario.
- Cuenta: título/saludo, navegación de secciones y formulario. Priorizar datos y guardado, no tarjetas vacías ni estadísticas inventadas.
- Grid espacial 4/8 px; separaciones funcionales 16/24 px y separación de secciones 32/48 px.
- Sidebar a partir de 768 px; en móvil navegación compacta accesible. Ningún elemento debe provocar overflow horizontal del viewport; IDs largos ajustan línea o conservan alternativa accesible completa.
- Viewports de aceptación: 390×844 y 1365×900; también zoom y foco de teclado. Los tamaños no son supuestos sobre el dispositivo del usuario.

## Elevation & Depth

Sombra cálida de baja intensidad basada en `rgb(51 39 32 / 12%)`, solo donde aclare agrupación o elevación. Bordes y espaciado antes que sombras. Sin glassmorphism ni fondos degradados decorativos en formularios. El gradiente del logo real no se elimina.

## Shapes

Radios: 6 px para detalles, 10 px controles, 12 px agrupaciones y 16 px tarjetas destacadas. Pastillas solo para etiquetas breves o elementos que lo justifiquen; no usar `rounded-full` como forma universal de formulario y tarjetas.

Objetivos interactivos de 44 px como mínimo. Icons auxiliares no sustituyen la etiqueta de una acción sensible.

## Components

### Marca

Usar `frontend/public/espacigo-mark.svg` sin redibujarlo y con dimensiones explícitas. Si el logotipo incluye texto EspaciGo contiguo, el SVG puede ser decorativo para no duplicar el nombre accesible. El enlace completo describe el destino al inicio.

### Identidad y sesión

Estado checking con placeholder estable y anuncio discreto. Solo la sesión confirmada presenta Mi cuenta. Nombre del perfil normalizado por Go; si no existe perfil o nombre, ID de cuenta confirmado. No usar email como fallback.

Guardar nombre actualiza cabecera y saludo sin reload. Las respuestas tardías de otra generación de sesión se descartan. Logout limpia identidad y datos privados; en la maqueta se muestra únicamente una simulación sin credenciales.

### Inputs y acciones

Etiquetas visibles y asociadas, ayuda de campo solo cuando aporta contexto, foco de error explícito. Una acción primaria por formulario. Estado guardando impide doble envío y conserva dimensiones. Confirmación mediante `role=status`/live region apropiada; error mediante mensaje accesible, nunca solo rojo.

### Navegación

General, Seguridad, Privacidad y Verificaciones conservan sus rutas reales; estados activos con `aria-current=page`. No inventar funcionalidades en secciones no cubiertas por la maqueta. Acceso administrativo sujeto a capacidades Go, no al nombre del usuario ni a un badge local.

### Maqueta de fase 1

`planning/frontend_next/design/fase1-prototipo.html` contiene escenarios locales sintéticos de acceso, cabeceras, cuenta, nombre/ID, checking, error y salida. No conecta API, almacena credenciales ni establece cookie; no es una ruta de la aplicación productiva.

## Do's and Don'ts

- Sí: jerarquía, foco visible, label persistente, estado de carga, movimiento reducido y mensajes concretos.
- Sí: continuidad de marca entre plantillas Next ya integradas, sin perder su composición.
- Sí: separar diseño visual, simulación, contrato y prueba nativa real.
- No: importar la antigua prohibición de Tailwind desde documentos Angular. Next/Tailwind siguen vigentes.
- No: métricas, fotografías personales, pagos, verificaciones o testimonios ficticios presentados como reales.
- No: contraseñas o tokens en localStorage/sessionStorage/HTML/URLs/logs ni campos reales en una maqueta.
- No: modificar SQL, Go, IAM, Terraform, IAP, Cloud Build o datos persistentes por esta fase visual.
- No: decir que la persistencia ya funciona porque una maqueta conserva estado mientras está abierta.
