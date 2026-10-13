# Evidencia MPV2-04 · Fotos y ficha pública

**Fecha:** 2026-10-13. **Base:** merge de MPV2-03 #297, `0fe7785352ba91972d4ec403d24b4cd085d5477e`. **Rama/PR:** `codex/marketplace/feature/mpv2-04-public-images-detail`, #298. **HEAD de código probado:** `145067b907f79c4d557c7cf7c0c0a293814af173`.

## Recorrido FE-14 conectado

El ensayo de revisión usó el proyecto Compose aislado `espacigo_fe14_44fef7d8d012`, API/Angular/Mailpit en puertos `18780/18782/18781` y volumen PostgreSQL `espacigo_fe14_44fef7d8d012_pgdata`. La configuración resuelta no montó `espacigo_pgdata`. El script retiró contenedores, red, volumen y estado temporal al terminar; las cuentas sintéticas y sus credenciales temporales también se eliminaron.

Resultado: **2/2 pruebas FE-14 aprobadas**. El anfitrión generó tres PNG sintéticos: dos declarados públicos (una portada y una imagen de galería) y uno privado, excluido del DTO. Un visitante sin Bearer consultó la ficha y ambas imágenes; el endpoint binario respondió `image/png`, `Cache-Control: no-store` y `X-Content-Type-Options: nosniff`. El anfitrión retiró una foto; su URL respondió 404 y al recargar la ficha permaneció la otra imagen cargada correctamente. Esto prueba retirada de una foto, no ocultación de la publicación completa. Los logs del Backend no incluyeron correo ni contraseña sintéticos.

## Verificaciones

- `go test ./...` — aprobado.
- PostgreSQL/PostGIS desechable, `scripts/test-local-comm-mailpit.sh ./internal/adapters/postgres/spaces -run '^TestLocalPublicationRequiresEffectiveKYCAndRecordsOwnerTransitions$' -count=1` — aprobado con V56. Prueba GET 404 genérico/no-store para foto privada por defecto, asociación cruzada, ID de evidencia KYC, publicación revocada, cuenta bloqueada, elegibilidad KYC revocada, espacio oculto/eliminado y foto retirada; verifica lectura binaria anónima con headers y que propietario publica/retira mientras tercero no puede hacerlo.
- `npm test -- --watch=false` — 204/204 pruebas aprobadas.
- `npm run api:check`, `npm run lint` y `npm run build` — aprobados en esta revisión.
- FE-14 aislado: `FE14_PUBLIC_ONLY=1 FE14_EVIDENCE_OUTPUT_DIR=planning/marketplace_publico_v2/evidencia/mpv2_04 bash scripts/test-frontend-e2e.sh` — 2/2 aprobadas, con dos fotos públicas, una privada y retirada/recarga.
- GitHub Actions de PR #298, ejecución [38423547641](https://github.com/HernanEspinozaDev/espaciGo/actions/runs/38423547641) — frontend aprobado: lint, `api:check`, unitarias, build y `npm audit --omit=dev`.

## Capturas

Los JSON adjuntos registran viewport CSS y dimensiones del documento. En capturas de página completa, las dimensiones JPEG son mayores que el viewport por el alto del documento.

| Captura | Viewport CSS | JPEG | SHA-256 |
| --- | ---: | ---: | --- |
| Ficha pública, escritorio | 1365×900 | 1365×1390 | `0069b3bfc33fa854fa0ddc796ac6eb223b94d531110cb9effaaf1dd2c43b4788` |
| Ficha pública, móvil | 390×844 | 390×1892 | `f24de2c7c6bb1a6664a4786b61e8064f88290717a429caff0b55aff46e1fb394` |
| Galería anfitrión, escritorio | 1365×900 | 1365×900 | `af6e173c06683b703030c888f01b11e32959d36870a9ad070b1803fcbd078fe5` |
| Galería anfitrión, móvil | 390×844 | 390×2896 | `de43cc355e66a3f479726793bc0f477d2a8909c65a28ec53fc4ab748c483245e` |
| Búsqueda visitante, escritorio | 1365×900 | 1365×1570 | `d3da12380f2a96e6d64f164003d1ceb4e846e4dba75cbddd10218af796eb5e37` |
| Búsqueda visitante, móvil | 390×844 | 390×2620 | `33ea25200a81e70dc5bcaf372bd5a24078b0c2a6889fd52c31db67a7109fada6` |
| Catálogo público, escritorio | 1365×900 | 1365×1773 | `448c7799ccc0b5e97b91fa1ba8e8ab8915031baad48ed63965fec1d0ae9b8d75` |
| Catálogo público, móvil | 390×844 | 390×2887 | `335622e62472998295703dcf738daf83f56e052c2233a19087474dd4815ac2cd` |

## Límites

Las imágenes son fixtures sintéticos, no fotografías reales. No se probaron GCP, mapas, pagos ni SSR. La retirada impide nuevas lecturas del servidor; no revoca copias que un visitante ya descargó. La fusión técnica y esta evidencia no implican aceptación de producto.
