# LOCAL-M04-PUBLICATION-DELETE-01 — decisión y contrato técnico

Issue #258, hija de LOCAL-CLOSE-01/#246, RQF-091/092. Política local ratificada el 2026-10-11. Implementación en revisión en una rama derivada de `main` posterior a PR #261. Este documento no cierra M04 ni #246.

## Regla de producto vigente

- Solo se retira una publicación activa u oculta; un borrador no usa este endpoint.
- Antes de cualquier cambio, la transacción toma los bloqueos del titular y del espacio usados por las operaciones de reserva/publicación. Después de los bloqueos consulta `clock_timestamp()` y verifica reservas no terminales, ventana de reclamo, disputa abierta, pagos/devoluciones, garantía, decisión financiera, liquidación y conciliaciones pendientes. Un bloqueador devuelve 409 y no cambia publicación ni archivos.
- La comprobación del espacio bajo `FOR UPDATE` serializa el retiro con solicitudes nuevas, y el bloqueo de titular serializa cambios financieros y acciones de reserva. La operación no cancela reservas ni mueve fondos.
- Un retiro elegible deja un tombstone `eliminada`, retira título, descripción, dirección, atributos, configuración semanal y simulaciones privadas. La tarifa sin referencias a cotizaciones se elimina; las versiones referenciadas se conservan. El catálogo y las rutas de owner dejan de exponer el espacio.
- Se conservan el ancla técnica, historial de publicación, cotizaciones/reservas/ocupaciones y snapshots, contratos, pagos, devoluciones, garantías, reclamos, decisiones, liquidaciones e historiales que existan. No se borran filas transaccionales en cascada ni se cambian fechas/plazos originales.
- Fotos activas se marcan retiradas y sus bytes se encolan en el cleanup durable existente. Candidatos reservados se marcan para limpieza. Un error al eliminar bytes conserva fila/referencia y reintenta con el worker; la baja no afirma que el borrado físico ya ocurrió.
- La operación es idempotente por espacio/titular: un retry devuelve el recibo original. Otra publicación/cuenta no cambia. El recibo y el ZIP del titular documentan los grupos retirados/conservados y la cantidad en limpieza.

## Esquema y límites de privacidad

V50 añade el estado `eliminada` sin borrar V1–V49 y una fila append-only `espacio_eliminacion_historial_local`. Su proyección incluye espacio técnico, actor titular, fecha, correlación, conteo de archivos y grupos estructurados. No copia contenido libre ni identificadores de participantes. La fila queda exportada como `space_deletion_history` del propietario.

No se crea plazo de retención para el ledger de eliminación; ese plazo requiere decisión aparte. Cotizaciones convertidas y transacciones conservan los plazos ya ratificados, y las cotizaciones no convertidas conservan sus vencimientos existentes. La eliminación no reinicia ningún plazo. La retención productiva, anonimización integral, proveedores y GCP quedan fuera.

## Validación enfocada

La integración PostgreSQL del corte usa el runner desechable desde V1 con el rol runtime. Verifica el 409/no-mutación ante reserva no terminal y garantía pendiente, retiro de una publicación con reserva terminal/liquidación confirmada, acceso de propietario y ocultamiento de la eliminada, preservación de snapshot/historial, replay idempotente, exportación de la sección propia y PNG retirado con fallo transitorio/recuperación durable. El mock compila con TypeScript y ofrece la acción únicamente al rol anfitrión para publicaciones activas u ocultas.

No se ejecuta la regresión LOCAL-1 hasta completar #259. No se monta ni altera `espacigo_pgdata`; no se ejecuta GCP.
