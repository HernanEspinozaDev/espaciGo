# LOCAL-FIN-SETTLEMENT-01 — política de ensayo local

**Issue:** #236, hija funcional vinculada a #109 y #110. **Estado:** implementación en PR separado; pendiente de revisión/aceptación. Solo cuentas, reservas y operaciones sintéticas locales.

## Reglas ratificadas

- `liquidacion_local_0pct_v1` aplica a cotizaciones y reservas nuevas: comisión parametrizada por basis points, con snapshot de versión, moneda CLP y tasa (0 bps en esta política local). No se reescriben snapshots históricos ni se calcula comisión sobre garantía.
- La liquidación es elegible desde el check-out persistido + 24 horas, con reserva `finalizada`, sin reclamos abiertos, devoluciones pendientes ni otras obligaciones financieras sin conciliar. La reserva cancelada no se liquida. El Backend adquiere bloqueos de cuentas → reserva → garantía/check-out en el orden del módulo, toma el reloj después y vuelve a comprobar las condiciones dentro de la transacción que persiste la intención.
- El importe usa arriendo fake confirmado menos devoluciones confirmadas y comisión local. No incluye garantía, capturas por daño ni liberaciones. CLP se mantiene como entero.
- Se persiste una única intención/operación antes de llamar al adaptador local. Éxito, rechazo, timeout, reinicio y reintentos reutilizan la operación. Timeout permanece `por_conciliar`; no representa pago. Reconciliación administrativa resuelve el resultado persistido y no inicia otra transferencia. SMTP/proveedor externo no se integra.
- Solo un éxito fake confirmado genera `comprobante_ensayo` JSON versionado con `ENSAYO LOCAL — SIN VALIDEZ FISCAL — SIN MOVIMIENTOS REALES`. Puede contener un UUID técnico interno; no folio fiscal, RUT ni elementos visuales de boleta. No sustituye un documento fiscal válido.
- La exportación propia incluye snapshots, liquidaciones, historial y comprobantes. La baja queda bloqueada ante liquidación pendiente o incierta. Los hechos financieros cerrados mantienen el tratamiento local ratificado de 24 meses desde el último cierre financiero; la retención física/restauración más allá de ese alcance sigue según la matriz. No se afirma anonimización.

## Fuera de esta decisión

Liquidación/custodia real, proveedor o sandbox, comisión productiva, pago de promoción, liquidación real al anfitrión y documento tributario válido. La política de comisión 0 % solo permite ensayar el contrato financiero.

Campañas, promociones, `campana`, `orden_promocion`, `derecho_reporte` y NPS (`respuesta_nps`) continúan diferidos por MAP-07: referencias RQF-183/RQF-236 y RNF-018/026/029 cuando correspondan, CU-52/HU26–HU27; HU24/HU29 permanecen sin requisito funcional autorizado. No se atribuye cumplimiento a su diferimiento ni se crean productores/datos para ellas.
