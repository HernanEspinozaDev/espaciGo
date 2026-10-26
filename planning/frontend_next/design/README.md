# Fase 1 — especificación y maqueta visual

> **Maqueta rechazada por el usuario.** Conservar solo como historial de la propuesta descartada. La referencia es el diseño Next actual de `main`, normalizando su paleta sin alterar layout, componentes, tipografía, espaciado, radios o densidad. `DESIGN.md` y sus exports no autorizan trasladar valores no cromáticos a la aplicación. Las pruebas de esta maqueta no constituyen aceptación visual. Ver el plan UI/UX actualizado antes de iniciar fase 2.

Estos archivos **no son rutas de producción** y no están importados por el frontend Next. La maqueta no llama a Go, no crea cookies, no persiste credenciales y no implementa todavía la ADR BFF aprobada.

## Implementación corregida en la aplicación real

Se mantiene **Tailwind CSS 4**. La paleta de la aplicación se configura en `frontend/src/app/brand-palette.css` mediante `@theme` y variables consumidas por las mismas clases existentes; no se reemplazó Tailwind ni se trasladaron los estilos de esta maqueta. El antes/después real se verifica con `frontend/tooling/palette-visual.mjs` y se documenta en `../evidence/ui-ux-fase1-correccion-paleta-main.md`.

`DESIGN.md` y los exports actuales contienen solo roles cromáticos, sin imponer fuentes, radios o espaciados. Los originales de la propuesta rechazada se conservan en `rejected/`. La validación de tokens actual corresponde al nuevo archivo; la evidencia anterior se conserva como historia, no como aprobación del rediseño.

## Artefactos

- `../../../DESIGN.md`: contrato visual legible y tokens normativos de esta fase.
- `tokens.dtcg.json`: exportación DTCG de los tokens.
- `tokens.css`: primitivas CSS con prefijo `--espacigo-*`, sin colisión accidental con el tema actual.
- `theme.css`: exportación de intercambio Tailwind v4; **no importar directamente en la app**. Sus primitivas `muted`, `accent` y `secondary` difieren de las superficies semánticas Shadcn actuales.
- `next-semantic-map.css`: referencia de adaptación explícita hacia las variables de la app; todavía no importada.
- `fase1-prototipo.html`: maqueta autocontenida con el logo real, estados sintéticos y navegación entre vistas. Abrir como archivo local no requiere servidor ni dependencias externas.
- `verify-prototype.cjs`: pruebas locales con Playwright y Axe instalados en `frontend/`. No usa el perfil del navegador del usuario.
- `fase1-tokens-validation.json`: resultado del linter y huella de la especificación.

## Interacciones de la maqueta

1. Como invitado, usar **Ingresar a EspaciGo** e **Iniciar sesión simulada**.
2. En Mi cuenta, observar el ID ficticio completo y **Cerrar sesión**, sin Acceso redundante.
3. Guardar un nombre de muestra: cabecera y saludo cambian sin recargar.
4. Volver al inicio con el logo: **Mi cuenta** vuelve directamente a la vista de cuenta.
5. Probar invitado, con/sin nombre, comprobando, sesión expirada y error 503 desde la banda de escenarios.
6. Salir: se limpia también la identidad que permanecía en formularios ocultos.

La continuidad de la maqueta solo existe en memoria mientras está abierta. No es una prueba de persistencia HttpOnly ni de autorización real.

## Verificación reproducible

Desde la raíz del repositorio, con Node/pnpm del proyecto disponibles:

```bash
node planning/frontend_next/design/verify-prototype.cjs
```

Sin URL, el script inicia y cierra su propio servidor aislado en `127.0.0.1` con puerto libre. Si ya existe un servidor propio y verificado que sirva esta carpeta:

```bash
PROTOTYPE_URL=http://127.0.0.1:4327/fase1-prototipo.html \
  node planning/frontend_next/design/verify-prototype.cjs
```

Las capturas y `report.json` se escriben en `.local/testing/ui-ux-fase1/`, excluido de Git. No se envían peticiones externas ni API; las pruebas bloquean y reportan esos intentos. Axe se ejecuta **con contraste habilitado** en inicio, acceso, cuenta ID/nombre, inicio autenticado, comprobando, vencimiento y error, para móvil y escritorio. No usar `overflow-x:hidden` como sustituto de corregir el layout.

El script falla si no dispone de Playwright/Axe; no presenta omisiones como una auditoría completa.

Para verificar/exportar la especificación se utilizó `@google/design.md@0.4.0`:

```bash
npm exec --yes --package=@google/design.md@0.4.0 -- designmd lint DESIGN.md
npm exec --yes --package=@google/design.md@0.4.0 -- designmd export --format dtcg DESIGN.md
npm exec --yes --package=@google/design.md@0.4.0 -- designmd export --format css-vars --prefix espacigo DESIGN.md
npm exec --yes --package=@google/design.md@0.4.0 -- designmd export --format css-tailwind DESIGN.md
```

Los exports se guardan como los archivos anteriores; los comandos no requieren instalación global. No modificar dependencias de la app para ejecutar este CLI de diseño.

## Límite de fase

La persistencia fue aprobada en `../adr_ui_auth_001_persistencia_bff.md`. La implementación real es **fase 2**: respetar `expires_at` de Go, HttpOnly/Secure, CSRF, no caché compartida y cero token en JavaScript/Web Storage. No deducir que está resuelta a partir de los estados simulados de este HTML.
