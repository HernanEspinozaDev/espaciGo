# Matriz de aceptación — MARKETPLACE-LOCAL-2

**Fecha:** 2026-10-13. **Estado:** criterios previstos; no ejecutados en V2.

Cada evidencia debe indicar commit probado, entorno, resultado y límite. Un test unitario con API simulada no acredita un recorrido conectado a Go. Cada fila puede necesitar varias regresiones.

| ID | Corte principal | Caso | Resultado requerido | Nivel/evidencia | Estado |
| --- | --- | --- | --- | --- | --- |
| PUB-01 | 00 | Inventario V1 | Guards, rutas, DTO, estados, perfiles y delta DB trazados a código. | Auditoría de fuente/contrato. | Pendiente |
| PUB-02 | 01 | Catálogo anónimo | HTTP200 sin cookies/Bearer, sin redirección a login. | Handler + Angular Router + E2E Go. | Pendiente |
| PUB-03 | 01 | Ficha por enlace directo | Pública en pestaña privada, incluido reload. | API + navegador. | Pendiente |
| PUB-04 | 01/04 | Publicación no visible | Borrador, oculta, eliminada y fixture restringido ausentes; detalle e imagen 404 genérico. | PostgreSQL + API. | Pendiente |
| PUB-05 | 01/08 | Cambio de elegibilidad/propietario | Revocación KYC, bloqueo, baja y moderación impiden nuevas lecturas sin exponer motivos. | Integración Go/DB. | Pendiente |
| PUB-06 | 01/08 | Minimización | Sin dirección precisa, contactos, ownerID, sesiones, KYC, rutas de archivo ni datos de ocupantes. | Asserts de DTO/JSON, HTML y binarios. | Pendiente |
| PUB-07 | 01/06 | Operaciones privadas | Sin Bearer siguen rechazadas; tercero no accede a reservas, contratos, pagos ni editor. | Contratos y handlers Go. | Pendiente |
| PUB-08 | 02 | Búsqueda textual | `q` usa PostgreSQL FTS español-unaccent, términos AND con stemming y ranking `relevance`; campos públicos únicamente; puntuación/stopwords sin lexemas da cero resultados. | Fixtures SQL + API. | Pendiente |
| PUB-09 | 01/05 | Categoría como filtro | `category_code` activo, único por primera versión; listado/enlace muestra solo esa categoría sin pantalla de acceso. | API + E2E. | Pendiente |
| PUB-10 | 02/03 | Región/comuna | Códigos válidos y sugerencias acotadas; zona ausente no inventada. | FK + API + editor. | Pendiente |
| PUB-11 | 02 | Atributos por categoría | Tipos/versiones correctos; boolean false válido; atributos ausentes no coinciden; perfil incompatible rechazado. | Dominio/DB/API. | Pendiente |
| PUB-12 | 01/02 | Precio base y estimado | Sin fechas se filtra/ordena tarifa base solo con `rate_unit` explícito; CLP por hora/día/mes no se mezcla. Total estimado es opcional, exige start/end y usa PriceUnits/TZ; filtros/sorts separados, inclusivos, min≤max. | Casos CLP, límites int64, modalidad y API. | Pendiente |
| PUB-13 | 02/06 | Dinero exacto | Valores int64 se conservan al renderizar/cotizar; estimación diferenciada de confirmación Go. | Contrato + Angular. | Pendiente |
| PUB-14 | 01/02 | Parámetros inválidos | q >120, códigos inactivos, unit price sin modalidad, rango invertido, intervalo incompleto, sort sin requisitos, page_size fuera 1–25 y cursor inválido dan 422 genérico; ficha/imagen no visible 404 genérico. | API. | Pendiente |
| PUB-15 | 01/02/08 | Paginación y orden | SQL `LIMIT n+1`, default 20/max25, cursor firmado no ligado a actor y fingerprint de filtros/orden/tamaño; sort default, unit price, estimated total, relevance según requisitos; empate ID estable. | Integración + inspección SQL/réplicas. | Pendiente |
| PUB-16 | 02/08 | Consulta pública sin efectos privados | No crea cotizaciones/ocupaciones ni ejecuta mutaciones por visita del rastreador. | Comparación de estado DB. | Pendiente |
| PUB-17 | 03 | Publicar desde editor | Anfitrión completa zona pública, publica y se encuentra en catálogo; actualización autorizada sin SQL manual. | E2E Go y UI. | Pendiente |
| PUB-18 | 03 | Migraciones compatibles | Base vacía y V54 existente funcionan; V1–V54/checksums/historial sin alteración. | PostgreSQL efímero. | Pendiente |
| PUB-19 | 04 | Imagen pública | Solo `imageID` marcado publicable en `/spaces/{spaceID}/images/{imageID}`; retirada, imagen ajena o evidencia privada no sirve. | Binario, API, hashes. | Pendiente |
| PUB-20 | 04/07 | Retirada de publicación/foto | Nueva petición de JSON/HTML/imagen y sitemap no expone contenido retirado; sin copia estática persistente. | Integración Go + SSR. | Pendiente |
| PUB-21 | 05 | URL y navegación | Filtros, chips, orden, limpiar, Atrás/adelante, ficha→resultados y enlaces compartidos consistentes. | Router real + E2E. | Pendiente |
| PUB-22 | 05 | Móvil/escritorio | Sidebar escritorio y filtros móviles accesibles; no overflow a 390 px; fotografías/tarjetas legibles. | Capturas con dimensiones/viewport. | Pendiente |
| PUB-23 | 05/08 | Accesibilidad | Teclado, Escape, foco inicial/retorno, etiquetas, errores, contraste y axe A/AA en vistas entregadas. | Automática + revisión manual. | Pendiente |
| PUB-24 | 05/08 | Estados asíncronos | Loading, sin resultados, caída API y cursor inválido diferentes; respuestas antiguas no pisan estado nuevo. | Unitarias + Go interrumpido en ensayo. | Pendiente |
| PUB-25 | 06 | Login contextual | Anónimo selecciona espacio, entra y vuelve al mismo paso; no envía POST automáticamente. | IdentitySession/Router reales + Go. | Pendiente |
| PUB-26 | 06 | Registro/verificación | Contexto permanece por registro/verificación/login o se ofrece recuperación segura; sin persistencia de Bearer. | E2E + Mailpit aislado. | Pendiente |
| PUB-27 | 06 | Retorno malicioso | Destinos externos, //, codificados/ambiguos y acciones desconocidas no provocan open redirect ni operación. | Pruebas del codec/Router. | Pendiente |
| PUB-28 | 06 | Estado cambia durante login | Tarifa/espacio/intervalo retirado o cuenta restringida revalidados con Go; no reserva inválida. | Integración + navegador. | Pendiente |
| PUB-29 | 06/08 | Sesiones y rutas reutilizadas | Cambio de ID/query invalida cargas; sesión nueva no hereda respuesta/401 o intento privado; logout conserva búsqueda pública. | Regresiones de carreras. | Pendiente |
| PUB-30 | 06 | Invitaciones existentes | `/reservas?contrato=...` sigue funcionando y tercero obtiene rechazo genérico. | Regresión #292. | Pendiente |
| PUB-31 | 07 | HTML sin JavaScript | Ficha entrega contenido principal público y metadatos; datos hidratados coherentes. | HTTP SSR + navegador JS desactivado. | Pendiente |
| PUB-32 | 07 | Estados HTTP | No visible devuelve 404; caída Go estado temporal apropiado; no falsear cero resultados. | HTTP SSR/API. | Pendiente |
| PUB-33 | 07 | Sitemap/canonical/indexación | Solo fichas visibles; filtros/cursor/privados noindex; canonical y enlaces con origen público correcto. | Parseo HTML/XML y retirada. | Pendiente |
| PUB-34 | 07/08 | Aislamiento SSR | Sin tokens técnicos, host Docker o datos privados; peticiones concurrentes no mezclan estado. | HTTP concurrente/config revisión. | Pendiente |
| PUB-35 | 08 | Coste y abuso | Consulta limitada, timeouts/429 correctos, planes SQL medidos con inventario sintético; no descarga total para paginar. | EXPLAIN/medidas documentadas. | Pendiente |
| PUB-36 | 08 | CORS/Compose | Navegador permitido, SSR sin Origin y OPTIONS válidos; frontend sin montajes de secretos. | Stack aislado/config resuelta. | Pendiente |
| PUB-37 | 08 | Regresión privada | Identidad, oferta, reserva, pago fake, contratos, privacidad y administración siguen autorizados. | Suite enfocada + consolidada. | Pendiente |
| PUB-38 | 09 | Demo y aceptación | Recorrido público→login contextual→reserva y retirada, commit fusionado/evidencias y aceptación explícita. | Registro del usuario y manifiesto. | Pendiente |

## Registro por corte

| Corte | PR / HEAD / merge | Datos | Go/OpenAPI | Angular | Evidencia / límites | Decisión |
| --- | --- | --- | --- | --- | --- | --- |
| MPV2-00 | [PR #294](https://github.com/HernanEspinozaDev/espaciGo/pull/294); HEAD actual en PR | Sin migración; delta documentado | Operaciones públicas/visibilidad/search contract documentados; implementación pendiente | Rutas/guards/SSR/retorno auditados; implementación pendiente | Contrato `contrato_mpv2_00.md`; sin pruebas runtime | Pendiente de revisión técnica |
| MPV2-01 | Pendiente | Pendiente | Pendiente | Pendiente | Pendiente | Pendiente |
| MPV2-02 | Pendiente | Pendiente | Pendiente | Pendiente | Pendiente | Pendiente |
| MPV2-03 | Pendiente | Pendiente | Pendiente | Pendiente | Pendiente | Pendiente |
| MPV2-04 | Pendiente | Pendiente | Pendiente | Pendiente | Pendiente | Pendiente |
| MPV2-05 | Pendiente | Pendiente | Pendiente | Pendiente | Pendiente | Pendiente |
| MPV2-06 | Pendiente | Pendiente | Pendiente | Pendiente | Pendiente | Pendiente |
| MPV2-07 | Pendiente | Pendiente | Pendiente | Pendiente | Pendiente | Pendiente |
| MPV2-08 | Pendiente | Pendiente | Pendiente | Pendiente | Pendiente | Pendiente |
| MPV2-09 | Pendiente | Pendiente | Pendiente | Pendiente | Pendiente | Pendiente |

## Bloqueos de cierre

Bloquean aceptación: lectura que exige login; filtración de datos privados; borradores/retiradas visibles; eliminación de permisos privados; búsqueda/filtros aparentes; precio sin unidad o inexacto; retorno inseguro; migraciones históricas modificadas; ausencia de recorrido real o evidencia crítica; SSR vacío/no indexable donde se declaró funcional.

Los límites fuera de alcance (fotos reales, mapas/proveedores externos, visitas/postulaciones, GCP) se registran como futuros. No sirven para justificar brechas en el catálogo público, la autenticación contextual o la protección de datos que sí forman parte de V2.
