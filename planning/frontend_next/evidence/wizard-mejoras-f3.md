# Mejora del wizard — Fase 3, revisión local

Base/worktree/rama de `wizard-mejoras-f1.md`; código aún sin commit sobre `771134d165046b290b1f5ad3a1074a6357c2327f`.

## Comportamiento revisado

Stepper con botones reales, identificación de paso actual, progreso visitado y bloqueo de pasos no visitados/durante envío. Atrás no valida ni borra campos. Saltar hacia adelante revalida desde el primer requisito hasta el destino; el primer paso inválido queda visible. El envío repite validación de pasos1–4 antes de la mutación. Cambiar categoría limpia atributos incompatibles e invalida progreso visitado. Se conserva validación por perfil/versionado y el feedback inline del enum requerido enlaza el ID real del error.

Corregidos los enlaces y el redirect a las rutas privadas **existentes** `/anfitrion/espacios` y `/anfitrion/espacios/:id/editar`, que se renderizan en el layout Mi Cuenta; no se creó una ruta ficticia `/mi-cuenta/espacios` ni se modificó el shell.

## Ejecución del padre

`pnpm build` PASS para el artefacto F3; `pnpm exec playwright test tests/wizard-improvements.spec.ts --workers=1 --output=<scratch>/espacigo-wizard-evidence/f3-parent-final`: **1 PASS**, Chromium loopback4203.

El caso repite la regresión de JSON/ubicación, vuelve desde confirmación al título y lo edita, salta a confirmación conservando campos, vacía el título y comprueba el bloqueo/feedback sin POST, restaura el título y finalmente crea el borrador simulado. Verifica título actualizado en el POST, ruta del editor real, encabezado de título, tablist de edición y valor del campo del editor. False/0 y omisión opcional siguen cubiertos.

La ejecución anterior falló por una expectativa de categoría inválida que no se había preparado y otra por buscar un encabezado «Editar espacio» inexistente; el padre leyó el DOM real y exigió corregir las pruebas sin cambiar el producto para satisfacer esos nombres inventados. El test corregido no se limita a observar una URL404.

## Límites

Gate técnico local F3 aprobado en este recorrido controlado y revisión de código. No prueba creación persistida ni aceptación humana. El artefacto F3 es anterior a la integración del calendario F4; el build final debe reconstruir y repetir las pruebas sobre todas las fuentes reunidas. Sin commit, push ni despliegue de estas mejoras.
