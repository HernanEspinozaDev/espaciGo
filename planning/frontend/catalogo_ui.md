# Catálogo interno UI Angular

| Primitiva usada | API | Estados/semántica | Tokens y consumidores |
| --- | --- | --- | --- |
| `UiButton` | `button[appButton]` o `a[appButton]`; variante primary/secondary/text | Elemento nativo; disabled conserva semántica; foco compartido; min-height 44px. No asigna permisos ni hace HTTP. | Colores semánticos y espaciado/radio compartidos. Shell y features. |
| `UiState` | `<app-state kind="loading|empty|error|success">…</app-state>` | Error usa role=alert/aria-live=assertive; otros estados role=status/polite. Contenido proyectado, sin reglas del dominio. | Tokens de superficie/error/success. Usado en la carga KYC; variantes pueden reutilizarse en vistas futuras. |
| `ModelControls` | forProperty/forSignal devuelve FormControl tipado; por instancia de vista | Sincronización de modelo/señal y reset sin reescritura; handlers de transición de la feature conservan invalidación. Subs se destruyen con la vista. | Oferta/admin; FormGroup/FormRecord tipados, sin ngModel. |
| `FocusMonitor` (CDK) | focusVia en el shell | Escape devuelve foco al menú; NavigationEnd/render enfoca el h1; SSR no toca DOM. | Anillo azul compartido de 3px. |

Las variantes de negocio (precios, contratos, elegibilidad, evidencia autorizada) se coordinan en su feature, no en shared/ui. Las páginas informativas conservan componentes standalone simples. El catálogo es interno al repositorio; no añade navegación técnica al producto.
