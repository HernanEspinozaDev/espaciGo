# Evidencia MPV2-05 · Portada, categorías y catálogo público

**Fecha:** 2026-10-13. **Rama:** codex/marketplace/feature/mpv2-05-public-listing-ux. **Base:** merge de MPV2-04 #298, c3e5cd297a32863446df33d271931aacb6fc2a8b. El HEAD probado se registra en la matriz al publicar el PR.

## Recorrido conectado FE-14

El ensayo usó el proyecto Compose aislado espacigo_fe14_65cfff1be26d, con API/Angular/Mailpit en puertos 18780/18782/18781 y volumen PostgreSQL propio. La configuración resuelta confirmó que no montaba espacigo_pgdata. El script retiró contenedores, red, volumen y estado temporal al terminar; eliminó las credenciales sintéticas del ensayo.

Resultado: **2/2 pruebas FE-14 aprobadas**. La prueba anónima abrió la búsqueda de portada, navegó desde categorías a Oficina, buscó y filtró en Go, verificó atributos, ubicación, total estimado y ausencia de Authorization. En móvil abrió el panel, comprobó Escape y retorno del foco al disparador. También cargó una página adicional mediante cursor real y verificó Atrás/Adelante con Router. Luego consultó el detalle público. Una segunda prueba de regresión verificó publicación/retiro de fotos desde el flujo de anfitrión y lectura anónima. Las publicaciones y cuentas fueron sintéticas y recreables.

## Verificaciones

- Angular: 204/204 unitarias.
- npm run lint: ESLint, estilos por tokens y arquitectura aprobados.
- npm run api:check: OpenAPI/SDK regenerado y TypeScript aprobado; MPV2-05 no cambia el contrato.
- npm run build: compilación y prerender aprobados.
- FE-14: FE14_PUBLIC_ONLY=1 FE14_EVIDENCE_OUTPUT_DIR=planning/marketplace_publico_v2/evidencia/mpv2_05 bash scripts/test-frontend-e2e.sh — 2/2 aprobadas contra Angular, Go y PostgreSQL desechable.
- No se cambiaron Go, OpenAPI ni migraciones.

## Capturas

Los JSON adjuntos registran viewport CSS y tamaño del documento. Capturas de página completa:

| Captura | Viewport CSS | JPEG | SHA-256 |
| --- | ---: | ---: | --- |
| Catálogo con filtros aplicados, escritorio | 1365×900 | 1365×1917 | 4c374621494f8156dd7c4d83415bec0877fd803fb85baa46a8a6bd36cf4ba17f |
| Panel de filtros móvil abierto | 390×844 | 390×844 | 006f1df6d54742ce30d34f00847aee6dd608fdd29bed4c48ef4db6885c1fbef3 |

## Límites

Las tarjetas usan una ilustración de referencia; no son fotografías. La estimación de precio no declara disponibilidad. El E2E cubre recorrido conectado, cursor y teclado; no se probó contraste con auditoría automatizada. SSR/SEO y autenticación contextual quedan en MPV2-07 y MPV2-06. La ejecución técnica no implica aceptación de producto.
