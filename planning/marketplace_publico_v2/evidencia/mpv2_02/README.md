# Evidencia MPV2-02 — búsqueda pública FE-14

Recorrido ejecutado el 2026-10-13 con el árbol de trabajo de la corrección del PR #296, basado en `0a45507c8176db4d4c39b64b5fa17e47d0f28fc5`. La prueba levantó Angular, Go, Mailpit y PostgreSQL en un proyecto Compose desechable con volumen exclusivo; el fixture público, su zona `America/Santiago` y su atributo Wi-Fi son sintéticos. La página llamó al Go real sin cabecera Authorization. Se comprobó búsqueda textual, región 13, comuna 13123, categoría oficina, Wi-Fi, intervalo horario, mínimo estimado CLP 18.000 y orden por total estimado, además de una ficha minimizada sin dirección precisa.

Las dos capturas de búsqueda se tomaron desde la misma página filtrada, en viewports distintos. Dimensiones del documento y viewport se guardan junto a cada imagen. SHA-256:

| Captura | Viewport | Documento | SHA-256 |
| --- | ---: | ---: | --- |
| [Escritorio](fe14_public_search_desktop.jpg) | 1365×900 | 1365×1773 | `5ce71bd7be51652a6656be1fe9ec2d0f0d7eb8adf3d45d82e28eb202a6ec7ff1` |
| [Móvil](fe14_public_search_mobile.jpg) | 390×844 | 390×2887 | `07bcd5acf0a5eb6e431202ee7c798185f7185ffa1676576352d4539210c3585b` |

La corrida FE-14 terminó con un caso anónimo aprobado. Se retiraron los contenedores, red, volumen y credenciales temporales del ensayo; permanecen intactos los recursos persistentes `espacigo-database-1` y `espacigo_pgdata`. Esto acredita este fixture y recorrido local; no acredita asignación de comunas desde la interfaz, contenido/fotos reales, producción ni aceptación de producto.
