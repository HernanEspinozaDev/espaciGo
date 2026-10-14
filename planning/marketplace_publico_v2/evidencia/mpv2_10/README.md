# Evidencia de recorrido MPV2-10

- Código probado: `ab24dae` (`codex/marketplace/fix/mpv2-10-ordering-and-filter-hierarchy`), basado en `ab9039beffa43eb70007e2db64546205cde4c21e`.
- Recorrido servido: FE-14 efímero con Angular, Go y PostgreSQL; Playwright verificó que Nombre A–Z/Z–A cambia resultados reales, que ordenar elimina el cursor y que Atrás/Adelante restaura el orden y el estado. También reprodujo el requisito de modalidad de precio, completó hora, pulsó «Aplicar y ordenar» y comprobó contra Go más de un resultado, precios diferentes y orden ascendente y descendente. Resultado: 2/2.
- La búsqueda conserva el orden pendiente: valida sus requisitos antes de navegar, lo incorpora a la consulta al enviar Buscar y lo mantiene pendiente si falta un requisito o falla otra validación. El botón indica «Aplicar y ordenar» cuando consumirá ese orden. Las acciones Aplicar/Descartar/Cancelar del selector se conservan.
- El foco desde las ayudas desktop y móvil apunta a los controles existentes `#rate-unit` y `#start-at`; la prueba DOM confirma que Desde recibe foco dentro de la interfaz pertinente.
- Base de datos de integración: el test Go/PostgreSQL sembró ofertas sintéticas en las ocho categorías, con títulos que empatan por mayúsculas/minúsculas y las tres unidades CLP (hora/día/mes); recorrió varias páginas en ambos sentidos y comprobó ausencia de repeticiones y orden estable. No se agregó migración ni se modificó V1–V56.
- Otras comprobaciones: Go `go test ./...` aprobado (las pruebas que necesitan una base de datos se ejecutaron aparte en PostgreSQL); Angular 248/248; lint; diseño tokens 13/13; arquitectura 16/16; `api:check`; build de producción; `git diff --check` y `bash -n scripts/test-frontend-e2e.sh`.
- Build Angular: correcto sin exceder el presupuesto SCSS. Se retiraron selectores sin uso (`.safety`, `.price-filter`, `.proximity-filter`, mapa sintético y `.text-button`) sin elevar el límite.
- Capturas del catálogo público anónimo con fixture sintético recreable: escritorio 1365×900 y móvil 390×844. Los JSON adyacentes registran dimensiones del documento y el HEAD probado.
- El stack FE-14 desechable y sus datos de ensayo se retiraron al terminar. No se usó la base persistente `espacigo_pgdata`, no se capturaron credenciales ni se prueban fotos reales.
- Alcance de evidencia: no demuestra aceptación final de producto ni aprobación/fusión del PR. MPV2-09 sigue **NO aceptado / cambios solicitados**.

Los sidecars registran dimensiones y el HEAD probado; los hashes de las cuatro capturas se registran en `SHA256SUMS`.
