---
version: alpha
name: EspaciGo — paleta de marca sobre Next existente
description: Normalización cromática sobre el diseño Next existente en main. No sustituye layouts, tipografía, espaciado ni componentes.
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
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.surface}"
  button-primary-hover:
    backgroundColor: "{colors.primary-hover}"
    textColor: "{colors.surface}"
  button-primary-active:
    backgroundColor: "{colors.primary-active}"
    textColor: "{colors.surface}"
  button-secondary:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.primary}"
  text-body:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.ink}"
  text-help:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.muted}"
  form-input:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
  account-card:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
  account-secondary:
    backgroundColor: "{colors.surface-subtle}"
    textColor: "{colors.muted}"
  nav-active:
    backgroundColor: "{colors.primary-soft}"
    textColor: "{colors.primary-active}"
  status-success:
    backgroundColor: "{colors.success-soft}"
    textColor: "{colors.success}"
  status-warning:
    backgroundColor: "{colors.warning-soft}"
    textColor: "{colors.warning}"
  status-error:
    backgroundColor: "{colors.error-soft}"
    textColor: "{colors.error}"
  status-info:
    backgroundColor: "{colors.info-soft}"
    textColor: "{colors.info}"
---

## Overview

**La referencia visual es la aplicación Next real de `main`.** El usuario rechazó la maqueta `fase1-prototipo.html`: no usarla ni usar Angular como referencia de composición. Este archivo especifica **colores**, no un nuevo diseño.

Mantener layouts, componentes, tipografías, tamaños, pesos, alturas de línea, espaciados, radios, sombras, densidad, orden del DOM y breakpoints actuales. No exportar valores geométricos nuevos ni sustituir las vistas por una maqueta. El archivo anterior se conserva únicamente como propuesta rechazada en `planning/frontend_next/design/rejected/DESIGN.proposal-v1.md`.

La aprobación de sesión persistente sigue registrada en `planning/frontend_next/adr_ui_auth_001_persistencia_bff.md`. No implementarla como efecto lateral de normalizar colores. Logo, nombre/ID, cierre de sesión y navegación se mantienen como mejoras funcionales separadas.

## Colors

- Terracota `primary` para acciones principales; `primary-hover` y `primary-active` para estados correspondientes.
- Crema `canvas` para el fondo; blanco `surface` para superficies. `surface-subtle` y `primary-soft` son fondos suaves, no colores de texto.
- `secondary` marrón y `accent` naranja son primitivas de marca, **no** equivalentes automáticos de las superficies Shadcn `secondary`/`accent`.
- `ink` y `muted` son texto. No asignar `muted` a una superficie de fondo.
- Blanco sobre primary da 5,96:1; sobre accent solo 3,15:1. No usar accent con texto blanco normal.
- Bordes decorativos y bordes que identifican controles son roles distintos. Mantener los anchos existentes; corregir color de borde sin alterar geometría.
- Conservar la semántica actual de éxito, advertencia, error e información; no convertir todos los estados en terracota. Los tokens semánticos son referencia de contraste, no autorización de una sustitución indiscriminada de escalas.
- Mantener el color del foco distinto de selección, con el grosor y offset existentes en Next. No importar el foco de 3 px de la maqueta rechazada.
- El SVG real conserva su gradiente y dimensiones actuales; normalizar paleta no autoriza redibujarlo.

Fuentes: `planning/frontend/referencias/guia_diseno_original.md` y valores históricos saneados de marca. Se recuperan colores, no reglas Angular.

## Typography

No se exporta una escala nueva. Las familias y propiedades tipográficas existentes en el código Next son la autoridad, incluyendo diferencias entre marketplace y workspace. No añadir fuentes remotas.

## Layout

Conservar las vistas integradas desde Your Next Store y Next.js SaaS Starter tal como están en `main`. Validar en 390×844 y 1365×900 comparando el mismo contenido y estado antes/después del cambio de colores. La normalización no debe modificar la geometría de los elementos visibles.

## Elevation & Depth

Conservar las sombras, capas y composición actuales. No introducir sombras cálidas, vidrio ni efectos provenientes de la maqueta.

## Shapes

Conservar los radios y tamaños actuales; no sustituir los `rounded-full` de las plantillas por los radios propuestos anteriormente. Cualquier defecto de accesibilidad ajeno a color se documenta y acota por separado.

## Components

Los pares de color del YAML documentan roles y contraste, no componentes nuevos. La implementación usa los componentes actuales. Un CSS de intercambio Tailwind no debe importarse directamente: hay que adaptar roles a las variables semánticas reales y revisar los overrides `.saas-application`.

Las capturas controladas y sus datos sintéticos no prueban integración Go ni persistencia real. El antes/después se obtiene de la aplicación Next, nunca de HTML que imite sus vistas.

## Do's and Don'ts

- Sí: modificar solo valores cromáticos y comprobar contraste, estados y geometría.
- Sí: documentar por separado defectos preexistentes y cualquier regresión nueva.
- No: presentar pruebas de la maqueta rechazada como aceptación de este ajuste.
- No: reemplazar vistas, fuentes, espaciados, radios, sombras o interacciones por la propuesta rechazada.
- No: añadir reglas Angular, dependencias, tracking, nuevas ofertas o capacidades ficticias.
- No: modificar Go, SQL, IAM, Terraform, IAP, Cloud Build ni datos persistentes por este alcance.
