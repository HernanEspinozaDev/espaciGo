# Evidencia — UI/UX fase 1: marca y especificación visual

## Estado

**Corrección posterior del usuario: entrega visual rechazada.** La maqueta no representa la dirección solicitada. Los resultados técnicos que siguen se conservan como evidencia histórica, no como aceptación de diseño ni como cierre válido de la fase 1. Mantener el diseño Next actual de `main` y normalizar únicamente su paleta; no trasladar composición, radios, espaciado o tipografía desde la maqueta o el Angular anterior. La aprobación de persistencia se mantiene; fase 2 no iniciada.

### Registro técnico anterior, no aceptación visual

Fase 1 completada **localmente**. La aprobación de persistencia HttpOnly/Secure quedó registrada en `planning/frontend_next/adr_ui_auth_001_persistencia_bff.md`; la persistencia real **no está implementada** en este corte y corresponde a fase 2.

Base de aplicación: `main` `8a87e38d1b27c384d054ff4bf463c46bdc49c9d2`. No se cambió código productivo `frontend/`, Go, SQL, IAM, Terraform, Cloud Build, IAP ni `AGENTS.md`. No se hizo commit/push o despliegue de estos artefactos durante la fase visual.

## Entregables reales

- `DESIGN.md`: especificación Terracota + Crema, tipografía local, espaciado, radios, componentes, estados y límites.
- `planning/frontend_next/design/tokens.dtcg.json`, `tokens.css`, `theme.css`: exports de `@google/design.md@0.4.0`.
- `planning/frontend_next/design/next-semantic-map.css`: referencia explícita hacia las variables Shadcn/Next existentes, **sin importarla en la app**.
- `planning/frontend_next/design/fase1-prototipo.html`: maqueta autocontenida con SVG real de EspaciGo; acceso, cuenta, navegación, identidad nombre/ID y estados sintéticos.
- `planning/frontend_next/design/verify-prototype.cjs`: comprobación reproducible local con Playwright y Axe, sin reglas de contraste deshabilitadas.
- `planning/frontend_next/design/README.md`: instrucciones de revisión, ejecución y límites.

## Pruebas ejecutadas por el agente principal

### Especificación y exports

CLI real: `npm exec --yes --package=@google/design.md@0.4.0 -- designmd lint DESIGN.md`.

- 0 errores de estructura/referencias.
- 0 avisos de contraste.
- 4 avisos `orphaned-tokens`: accent, border, control-border y focus. Son tokens de decoración/borde/foco descritos en el cuerpo; no se inventaron pares de texto/fondo para silenciar el linter.
- Inventario: 22 colores y 14 componentes.
- Comparación programática: los 22 colores DTCG y CSS prefijado coinciden con `DESIGN.md`.
- Huella de especificación de `fase1-tokens-validation.json` comprobada contra el archivo actual.
- Sintaxis JS del verificador y formato Prettier de los artefactos comprobados.

El export Tailwind genérico no se importa directamente: su primitiva `muted` es color de texto, mientras el tema actual usa `muted` como fondo. El adaptador evita cambiar inadvertidamente semántica de superficies al adoptar la paleta.

### Prototipo

Servidor propio usado: `http://127.0.0.1:4327/fase1-prototipo.html`, sirviendo únicamente la carpeta de diseño. Chromium con perfil de prueba aislado; no navegador/pestañas del usuario.

Comando final:

```bash
node planning/frontend_next/design/verify-prototype.cjs
```

La última repetición también comprobó el modo autónomo: servidor temporal creado en `127.0.0.1`, puerto libre asignado por el sistema y cierre del servidor por el propio script.

Resultado real: **80 PASS, 0 FAIL, 0 SKIP**. Viewports **1365×900** y **390×844**. Incluye **16 auditorías Axe con contraste habilitado**, sin violaciones, sobre ocho estados en ambos tamaños.

Cobertura comprobada:

- Logo real y formulario simulado que no solicita contraseña.
- Invitado → acceso → cuenta simulada.
- ID ficticio con formato UUID; nombre de muestra actualizado sin reload.
- Logo → inicio conservando estado simulado → Mi cuenta directo.
- Workspace sin enlace Acceso redundante y con cierre de sesión.
- Logout purga también el nombre retenido en formularios ocultos.
- Comprobando, vencimiento y error 503 explícitos.
- Sin desbordamiento horizontal, **sin ocultarlo con `overflow-x:hidden`**.
- Objetivos táctiles de al menos 44 px en cada estado auditado.
- Cero errores de consola a lo largo del flujo, cero cookies/Web Storage, cero intentos de API o red externa.

Capturas y reporte completos: `.local/testing/ui-ux-fase1/`, excluido de Git. Se inspeccionaron visualmente acceso desktop, cuenta con nombre desktop y cuenta sin nombre móvil. Se corrigió el tamaño del ID en el saludo móvil para mantenerlo secundario y legible.

## Correcciones tras revisar la delegación

La primera entrega daba 32 comprobaciones y deshabilitaba `color-contrast`; no se aceptó eso como auditoría completa. Se reactivó la regla, se ampliaron estados y comprobaciones y se volvió a ejecutar todo. Las nuevas pruebas detectaron identidad retenida tras logout y un cierre de aviso menor de 44 px; ambos se corrigieron antes del resultado final.

La cuenta de muestra y su UUID son fixtures explícitos; no son el usuario real ni un ID obtenido de Go.

## Autoauditoría visual

Superficie principal **Configure**: acceso y edición de cuenta; el inicio representa únicamente la navegación de regreso, no un catálogo ni una landing comercial nueva. Se detectó una cuadrícula genérica de tres tarjetas con afirmaciones de producto no comprobadas; se retiraron esas tarjetas y el CTA sin destino. Evaluación cualitativa de los diez indicadores de la skill de diseño: **1/10 antes, 0/10 tras el ajuste**. No hay composición hero/tres tarjetas en el flujo de configuración, métricas ficticias, vidrio o iconos ornamentales. La tipografía de sistema se conserva deliberadamente por continuidad local y ausencia de fuentes remotas; el indicador lateral del tab es estado activo, no una franja decorativa de tarjeta. Esta autoauditoría no equivale a aprobación visual del usuario.

## Límites y siguiente fase

- Esto no prueba login real, cookie HttpOnly, continuidad tras recarga o sesión Go entre instancias. La maqueta conserva únicamente estado simulado en memoria.
- Seguridad, Privacidad y Verificaciones están identificadas como secciones no implementadas en la maqueta; no se eliminaron de la aplicación real.
- No se recopilaron credenciales ni datos reales.
- La revisión propia no sustituye la aceptación visual del usuario ni una auditoría exhaustiva de accesibilidad del producto.
- Fase 2 ya está habilitada por la aprobación del usuario: implementar BFF/cookie, migración de transporte y pruebas de sesión sin ampliar `expires_at`. No volver a solicitar la misma aprobación de política.
