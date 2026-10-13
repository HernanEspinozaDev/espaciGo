# Auditoría y adecuación arquitectónica del frontend local

Fecha: 2026-10-12. Baseline auditado: merge FE-14 `2ebee8d10c32113bf5ebe113be48c85dfda3b5ad`. La aprobación técnica de #289 acreditó regresiones y compatibilidad; no acreditó cumplimiento completo de las decisiones arquitectónicas originales. La aceptación del usuario y FE-15 permanecen bloqueados hasta revisar esta adecuación.

## Fuentes y precedencia

- [Decisiones originales aportadas por el usuario](referencias/decisiones_frontend_original.md), sección 5 y registro de decisiones.
- [Guía original de diseño y SCSS](referencias/guia_diseno_original.md), secciones 5/6/11.
- [Diseño aplicado al alcance local](diseno_scss_angular.md).
- [Plan local](plan_frontend_angular.md), decisiones técnicas y contrato de sesión/concurrencia/dinero.
- `frontend/AGENTS.md` y versiones instaladas Angular 22.2.2/TypeScript 6.0.3.

Los IDs FE-01…FE-16 del ADR original identifican **decisiones**, no los cortes FE-00…FE-15 del plan de entrega. En particular, la decisión original FE-14 trata del backend modular; el corte FE-14 es la aceptación Angular.

## Diagnóstico anterior

No era una copia estática del mock: existían Angular standalone, rutas lazy, Signals, contrato generado, adaptadores HTTP, operaciones reales Go, protección de sesión y pruebas. Sin embargo, no cumplía completamente la arquitectura acordada:

| Decisión | Evidencia del baseline | Brecha y adecuación |
| --- | --- | --- |
| Feature-Based | Features por catálogo, oferta, reservas, cuenta, verificación, finanzas y administración | Se conserva; no se crean módulos sin API local. |
| Hexagonal ligera selectiva | Páginas dependían de clases `*Api`; coordinación de reintentos/selección estaba en vistas, incluida una página de oferta de 1394 líneas | Puertos propios, fachadas de aplicación por vista, adaptadores en infraestructura y composición DI explícita. Vistas delegan; no se añaden capas a páginas informativas. |
| Núcleo independiente | UI y coordinación importaban tipos del SDK; algunas fachadas conocían `HttpErrorResponse` | Proyecciones puras, errores/puertos sin Angular ni transporte y comprobación estructural contra el SDK en infraestructura. |
| Sesión/reservas complejas | Existían IdentitySession y registro de intentos con garantías verificadas, pero no una frontera explícita con el transporte | Sesión en application; IdentityPort y SessionBoundary; registro de intentos puro con fábrica de claves suministrada por Angular. |
| Administración/finanzas | La página construía rutas REST y usaba llamadas genéricas HTTP | Recursos/comandos semánticos; gateways traducen a URL/verbos/query. La capa de aplicación no conoce rutas Go. |
| Angular y formularios | Standalone/Signals y Signal Forms de identidad; oferta/admin usaban FormsModule/ngModel/CommonModule; tsconfig no habilitaba strict/strictTemplates | Estrictos habilitados; Reactive Forms tipados para oferta/admin, FormRecord para atributos dinámicos y DatePipe específico. Signal Forms de identidad se mantienen conforme a la versión instalada/AGENTS. |
| Design System/CDK/SCSS | Tokens de color y SCSS encapsulado, pero colores HEX duplicados por feature; sin UI compartida ni CDK | UiButton/UiState usados, catálogo interno documentado, tokens semánticos/espaciado/radio; FocusMonitor CDK en shell. Sin Tailwind/Bootstrap/Material. |
| Calidad verificable | Unit/E2E/build/contrato; no reglas de imports ni lint arquitectónico/SCSS/CI frontend | Angular ESLint, Stylelint, chequeo AST de capas/imports/URLs y workflow CI reproducible. |
| Go como autoridad | Contrato Go, int64 exactos en transportes financieros, estados/roles confirmados | Se conserva. No se introducen reglas económicas, fechas falsas ni permisos frontend. |
| SSR/CSR/Docker | Prerender de 3 páginas públicas; CSR privado; contenedores independientes | Se conserva. Catálogo autenticado continúa privado; Cloud Run/proveedores reales no entran en este alcance. |

## Arquitectura resultante

```text
Vista Angular (template, input/output, formularios y foco)
  -> Fachada de aplicación (casos de uso/estado de esa vista)
      -> Puerto abstracto (contrato propio)
          <- Adaptador HTTP / gateway
              -> SDK OpenAPI o HttpClient con codec CLP exacto -> Go

Dominio / contratos puros: proyecciones, recursos semánticos y registro de intentos
Composición: frontend.providers.ts; elección de implementaciones fuera de UI/application
Shared UI/forms: no imports de features ni reglas financieras
```

El registro de intentos puede probarse con una fábrica de claves determinista y sin Angular. Las fachadas aceptan implementaciones de puertos sin SDK/HttpClient. Sus Signals y su lifecycle pertenecen a la aplicación Angular; el dominio no depende de ellos. Los adaptadores solo notifican revocaciones/revalidación mediante SessionBoundary; no importan la implementación de sesión.

Las proyecciones mantienen nombres y estados del contrato Go para evitar duplicar políticas. Son tipos propios, independientes del código generado; `contract-compatibility.compile.ts` comprueba correspondencia estructural al regenerar SDK. Una modificación del contrato debe revisarse y mapearse en infraestructura, no copiarse silenciosamente a vistas.

Se conserva el contrato Promise de los puertos del SDK y las guardas de generación/revisión de lecturas no cancelables. HttpClient/RxJS permanece en adaptadores. No se convierte la cancelación de una escritura en una supuesta reversión de Go. No se incorpora NgRx ni un store global adicional.

## Gate y límites

- `npm run lint`: ESLint Angular/TypeScript/templates, Stylelint y arquitectura.
- `npm run api:check`: generación SDK y TypeScript estricto/compatibilidad de proyecciones.
- `npm test -- --watch=false`: regresiones existentes y pruebas nuevas de puertos, registro puro, mapper y formularios.
- `npm run build`: compilación Angular con templates estrictos y 3 rutas prerenderizadas.
- Docker/E2E: repetidos sobre el árbol adecuado; aislamiento del runner, borrador desde formularios reales, roles/descargas y caída/restauración. Evidencia abajo.
- La integración conserva datos sintéticos y no afirma certificación WCAG ni readiness productiva.
- Los lints se ejecutan con patrones locales fijos. La dependencia de desarrollo Stylelint arrastra el aviso sin versión corregida de `braces` (GHSA-vfj7-8cjw-p6xm); `npm audit --omit=dev` no reporta vulnerabilidades. El tooling no recibe patrones externos. Registrar/actualizar el aviso cuando exista corrección; no ocultarlo en la aceptación.

## Siguiente paso

Revisar el PR de esta adecuación y su evidencia. Después preparar la demo del commit aprobado con cuentas sintéticas y contenido representativo. Solo tras aceptación explícita del usuario se inicia FE-15. Los documentos locales de aceptación en `2492661` y los cambios ajenos del checkout original no se modifican ni se incorporan a este corte.

## Resultado de la validación de la adecuación

PR: [#290](https://github.com/HernanEspinozaDev/espaciGo/pull/290), autor **HernanMEC**. Código de la repetición final: `70a97c42db810705360142b3907fe35c016744c2`. La actualización posterior de evidencia no cambia ese código de aplicación.

| Gate | Resultado |
| --- | --- |
| ESLint + Stylelint + frontera AST | PASS; 11 puertos, 12 fachadas, 18 componentes |
| Angular | **178/178**, 32 archivos; conserva las 169 regresiones del baseline |
| OpenAPI y proyecciones propias | `api:check` PASS con TypeScript estricto |
| Build | PASS, 3 rutas prerenderizadas; bundle inicial 422,62 kB, sin advertencias de presupuesto |
| Docker FE-13 | PASS en proyecto desechable `espacigo_fe13_20261013000741_2603346`; redes, runtime, CORS, preflight, caída y recuperación |
| E2E autenticado | **4/4** en `espacigo_fe14_833978ce068c`: aislamiento, creación/edición de borrador con POST 201 y PUT 200, roles/JSON/ZIP, misma sesión tras caída/restauración |
| CI GitHub | [Run 38325464841](https://github.com/HernanEspinozaDev/espaciGo/actions/runs/38325464841) PASS sobre el commit de código probado |
| Auditoría npm de producción | 0 vulnerabilidades; aviso de tooling de desarrollo documentado arriba |

La primera ejecución de la nueva prueba de borrador falló por el localizador exacto de la categoría; la traza señaló `selectOption`. Se corrigió a combobox por nombre accesible y se repitió el stack completo: 4/4. No se cuenta la corrida fallida como evidencia aprobada.

[Manifiesto](evidencia/arquitectura/manifest.json) con SHA-256 de imágenes, viewport/documento y IDs Docker. [Log Angular](evidencia/arquitectura/angular-tests.log), [Docker](evidencia/arquitectura/docker.log) y [E2E](evidencia/arquitectura/e2e.log). Capturas nuevas separadas de las históricas: [oferta móvil](evidencia/arquitectura/host_mobile.jpg) y [oferta escritorio](evidencia/arquitectura/host_desktop.jpg). Las capturas `architecture_host_*` muestran el formulario desplazado después de guardar; las `host_*` muestran el encabezado y el borrador de Go.

Los viewports fueron **390×844** y **1365×900**, sin desbordamiento horizontal en estas muestras. El anfitrión del E2E no tiene KYC: Go conserva restricciones/409 en recursos de elegibilidad; el borrador sigue en ese estado y no se presenta como publicación activa. Administración usa colas sintéticas vacías. Este ensayo no sustituye la demo de todos los estados de contratos, finanzas y administración.

Los contenedores, redes y volúmenes propios de ambas repeticiones se retiraron. No se modificó la configuración privada del checkout original, sus cambios ajenos ni el registro local `2492661`.

La biblioteca UI entregada es una base usada por la aplicación (controles nativos, estado, formularios y foco), no una biblioteca exhaustiva de todos los widgets de la guía. Se conservan las medidas de layout existentes por feature para no alterar su geometría durante esta adecuación; los controles compartidos y la paleta sí consumen tokens. La página pública indexable de propiedad, staging y Cloud Run pertenecen al alcance objetivo y no se declaran implementados por este corte local.

El [recorrido de aceptación](demo_aceptacion_arquitectura.md) está preparado. **La demo acompañada, la aceptación explícita del usuario de FE-14 y FE-15 permanecen pendientes** hasta revisar y aprobar esta adecuación.
