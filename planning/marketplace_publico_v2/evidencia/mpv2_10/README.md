# Evidencia de recorrido MPV2-10

- Código probado: `c0b3a664c1b1fcc0bad6cb32a7078b9522c6d3c9` (`codex/marketplace/fix/mpv2-10-ordering-and-filter-hierarchy`), basado en `ab9039beffa43eb70007e2db64546205cde4c21e`.
- Recorrido servido: FE-14 efímero con Angular, Go y PostgreSQL; Playwright verificó que Nombre A–Z/Z–A cambia resultados reales, que ordenar elimina el cursor y que Atrás/Adelante restaura el orden y el estado. Resultado: 1/1.
- Base de datos de integración: el test Go/PostgreSQL sembró ofertas sintéticas en las ocho categorías, con títulos que empatan por mayúsculas/minúsculas y las tres unidades CLP (hora/día/mes); recorrió varias páginas en ambos sentidos y comprobó ausencia de repeticiones y orden estable. No se agregó migración ni se modificó V1–V56.
- Otras comprobaciones: Go `go test ./...` aprobado (las pruebas que necesitan una base de datos se ejecutaron aparte en PostgreSQL); Angular 243/243; lint; diseño tokens 13/13; arquitectura 16/16; `api:check`; build de producción; `git diff --check` y `bash -n scripts/test-frontend-e2e.sh`.
- Build Angular: correcto con aviso de presupuesto SCSS de `explore-page.scss` excedido por 442 bytes (límite 8.00 kB; resultado 8.44 kB).
- Capturas del catálogo público anónimo con fixture sintético recreable: escritorio 1365×900 y móvil 390×844. Los JSON adyacentes registran dimensiones del documento y el HEAD probado.
- El stack FE-14 desechable y sus datos de ensayo se retiraron al terminar. No se usó la base persistente `espacigo_pgdata`, no se capturaron credenciales ni se prueban fotos reales.
- Alcance de evidencia: no demuestra aceptación final de producto ni aprobación/fusión del PR. MPV2-09 sigue **NO aceptado / cambios solicitados**.

Los hashes de los cuatro artefactos se registran en `SHA256SUMS`.
