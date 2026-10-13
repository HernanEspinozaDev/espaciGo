# Evidencia MPV2-06

## Alcance probado

Recorrido FE-14 conectado de Angular a Go y PostgreSQL en un proyecto Compose efímero: visita anónima, catálogo con filtros, detalle, cancelación de acceso y retorno a filtros, login contextual, revalidación de horario/tarifa, cotización explícita y creación de reserva solo tras confirmación. También se probó registro, verificación por Mailpit y retorno posterior al mismo detalle. La regresión de imagen pública de MPV2-04 se ejecutó dentro de la misma suite.

Resultado: `FE14_PUBLIC_ONLY=1 bash scripts/test-frontend-e2e.sh` — 6/6 pruebas aprobadas. Cuatro cuentas sintéticas y dos publicaciones elegibles se crearon para la base desechable. Los logs no contenían correos ni contraseñas. El script retiró Compose, volumen y credenciales de ensayo al finalizar; no usó `espacigo_pgdata`.

Pruebas de frontend: `npm test -- --watch=false` (232/232), `npm run lint` (incluye estilos y arquitectura), `npm run api:check` y `npm run build` con prerender de tres rutas. El build se ejecutó en serie después de `api:check`, porque la generación concurrente de SDK elimina temporalmente archivos generados.

No se modificaron Go, OpenAPI ni migraciones. La cotización y reserva reutilizan las operaciones privadas existentes. No se probó pago real, SSR del detalle, visita/postulación ni GCP. La cuenta debe poder reservar; cuentas restringidas y roles incompatibles reciben explicación sin continuar la operación. Go revalida al cotizar y al solicitar reserva.

## Capturas

Ambas capturas corresponden al detalle con sesión contextual retornada; la acción de reservar requiere selección de horario, cotización y confirmación separadas.

| Archivo | Viewport | Documento | SHA-256 |
| --- | ---: | ---: | --- |
| `fe14_mpv2_06_context_desktop.jpg` | 1365×900 | 1365×1315 | `3e2e39c89c51676a39d2bc94ad303069f94fd28412be555acbdffe7bc376e5af` |
| `fe14_mpv2_06_context_mobile.jpg` | 390×844 | 390×2151 | `d76e12eb1fe439ec0dd68c01bffab452abfb82411f24fee5074d51ab179e2445` |

Los JSON adyacentes guardan viewport, dimensiones de documento y HEAD de código probado. Las pruebas iniciales se ejecutaron sobre la base `fe8d720e1ce0e2e6f296fcf01953f334a5adec52`; la evidencia se regenerará contra el commit final del corte antes de abrir el PR.
