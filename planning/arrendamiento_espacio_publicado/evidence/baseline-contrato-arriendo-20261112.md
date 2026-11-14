# Baseline local — arriendo desde una publicación

- Fecha: 2026-11-12 (-03).
- Worktree: `/home/nandev/Documentos/projects/espaciGo-rent-published-space`.
- Rama: `codex/marketplace/feature/rent-published-space-flow`.
- Base inspeccionada: `75c8c21d30c70a4fd63f6109664d5c2a063410df` (rama local `main`; un commit documental por delante de `origin/main`).
- Método: lectura estática de fuentes versionadas y documentos. No se inició API/Next/PostgreSQL, no hubo login, requests, mutaciones, consulta online ni acceso a datos privados.

## Hallazgos verificados en fuente

| Necesidad | Fuente observada | Conclusión y límite |
|---|---|---|
| Mostrar anuncio y precio | `frontend/src/app/(marketplace)/espacios/[id]/page.tsx:57-77,143-165,181-185` | La página carga ficha pública minimizada y advierte que precio/estimación no confirman disponibilidad ni cotización. Termina indicando que la ficha no ofrece reserva/pago. No hay aún CTA de arriendo en esa página. |
| Cotizar desde una ruta | `frontend/src/app/(workspace)/reservas/nueva/page.tsx:1-18`; `frontend/src/features/bookings/ui/booking-quote-card.tsx:21-35,96-124` | Existe ruta que toma `space_id`/`title` y tarjeta de cotización. Su existencia no demuestra que se alcance desde una publicación ni que el flujo sea comercial. |
| Destino real del adaptador booking | `frontend/src/features/bookings/infrastructure/bookings-openapi-adapter.ts:27-42,82-98` | La cotización y solicitud usan `/local/booking-trial/*`; la UI muestra badge «Ensayo» (`booking-quote-card.tsx:151-165`). No conectar ese adapter a una ficha pública como si fuera arriendo productivo. |
| Contrato OpenAPI de ensayo | `planning/openapi.yaml:4972-5005` y `5006-5062` | La operación se nombra `quoteAuthorizedLocalBooking`, se etiqueta `M06 local` y documenta fixture/ensayo; no constituye un endpoint comercial. El flujo de solicitud usa idempotencia, pero dentro de ese alcance local. |
| Montaje Go de booking | `cmd/api/main.go:481-537` | La lectura fuente muestra el servicio y el handler local de reserva bajo `/api/v1/local/booking-trial/`. Este inventario no encontró, en lo inspeccionado, una API pública/comercial de booking equivalente; no afirma ausencia de todas las rutas sin una revisión posterior exhaustiva de mounts. |
| Decisión vigente | `planning/decisiones/adr_objetivo_espacios_contratacion_cobros.md:7-11,15-27,31-40` | El ADR ratificado define una cuenta con ambos papeles, identidad vigente antes de emitir contrato, bloqueo al acordar intervalo/precio, reserva sin firma como camino base, firma opcional y garantía diferida. Exige resolver proveedor, plazos, fondos y conciliación según etapas. |
| Prototipo M06 previo | `planning/prototipo_local_m06_reserva_simulada.md`; `planning/decisiones_m06_reserva_local_simulada.md` | Describe fixtures privados, pago fake y prueba local. Sus reglas y resultados son específicos del ensayo; no sustituyen al ADR ni prueban capacidad comercial. |
| Evidencia histórica del frontend | `planning/frontend_next/evidence/fase-8-reservas-operacion.md` (corte 2026-10-25) | Reporta pruebas controladas de UI y una prueba native configurada para un baseline anterior. No se extrapola a este HEAD ni al circuito desde publicación pública; la propia evidencia etiqueta el pago como simulado. |

## Conclusión del baseline

Hay piezas reutilizables de catálogo público y de reservas de ensayo, pero el camino conectado desde publicación pública hacia una cotización/reserva comercial no está acreditado. El primer desarrollo funcional depende de cerrar el contrato comercial y las reglas autoritativas en Go/OpenAPI; no es seguro habilitar un botón que derive a las rutas locales de ensayo.

## Límites

Esta evidencia no es ejecución ni readback de API, Go, PostgreSQL, storage, identidad, contrato, proveedor o fondos. No se inspeccionó la aplicación desplegada. Los hallazgos se limitan a las fuentes citadas en la base local indicada.
