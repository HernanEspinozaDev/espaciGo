# Catálogo interno UI Angular

| Primitiva usada | API | Estados/semántica | Tokens y consumidores |
| --- | --- | --- | --- |
| `UiButton` | `button[appButton]` o `a[appButton]`; variante primary/secondary/text | Elemento nativo; disabled conserva semántica; foco compartido; min-height 44px. No asigna permisos ni hace HTTP. | Colores semánticos y espaciado/radio compartidos. Shell y features. |
| `UiState` | `<app-state kind="loading|empty|error|success">…</app-state>` | Error usa role=alert/aria-live=assertive; otros estados role=status/polite. Contenido proyectado, sin reglas del dominio. | Tokens de superficie/error/success. Usado en la carga KYC; variantes pueden reutilizarse en vistas futuras. |
| `ModelControls` | forProperty/forSignal devuelve FormControl tipado; por instancia de vista | Sincronización de modelo/señal y reset sin reescritura; handlers de transición de la feature conservan invalidación. Subs se destruyen con la vista. | Oferta/admin; FormGroup/FormRecord tipados, sin ngModel. |
| `FocusMonitor` (CDK) | focusVia en el shell | Escape devuelve foco al menú; NavigationEnd/render enfoca el h1; SSR no toca DOM. | Anillo azul compartido de 3px. |

Las variantes de negocio (precios, contratos, elegibilidad, evidencia autorizada) se coordinan en su feature, no en shared/ui. Las páginas informativas conservan componentes standalone simples. El catálogo es interno al repositorio; no añade navegación técnica al producto.

## Métricas y responsive revisados

`frontend/src/styles/_metrics.scss` centraliza la escala de espaciado y tipografía del baseline visual. Los nombres numéricos conservan puntos ópticos existentes, no autorizan nuevas medidas arbitrarias por feature. La escala queda explícita para revisión y posterior consolidación visual deliberada. Sass compila esos tokens sin duplicar CSS ni alterar la geometría. Los colores/tema permanecen en variables CSS de `_tokens.scss`.

`_breakpoints.scss` conserva los once umbrales de composición existentes en un mapa y expone `bp.down(nombre)`. Una clave desconocida falla al compilar. Agregar/cambiar un token o umbral exige justificar su consumidor y revisar su efecto visual.

Stylelint valida padding/margin/gaps y tipografía; rechaza literales, tokens desconocidos, variables locales y fallbacks arbitrarios. Las queries de viewport se declaran solo en el módulo común, incluidos los rangos de media queries. El chequeo AST prohíbe CSS inline de componentes; ESLint revisa templates externos e inline y prohíbe estilos literales. Se admiten bindings dinámicos de datos, como las coordenadas del mapa sintético. No hay excepciones generales por feature.

La migración puede reproducirse con `node scripts/check-frontend-style-compatibility.mjs f461c2eab3cdf5f2bd6f94e2981c72b20d0c400e`, con el baseline disponible localmente y las dependencias instaladas. No congela cambios futuros de diseño; acredita que esta adecuación conserva exactamente el CSS compilado de los 20 estilos afectados.
