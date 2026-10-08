# LOCAL-PRIV-01A2 — recorrido local V24

Fecha: 2026-10-08. Checkout sincronizado con `main` en `a695fcd` (merge de PR #190). V24 se aplicó incrementalmente con `bash scripts/dev-env.sh up -d`; la consulta de `schema_migrations` devuelve 24 y database, backend, mock y Mailpit quedaron saludables. Se preservaron `espacigo_pgdata`, secretos y datos previos. No se ejecutó `clean`, no se deshabilitaron restricciones y no hubo trabajo de GCP.

## Reproducción

Con la pila local arriba, ejecutar:

```bash
bash scripts/verify-local-privacy-dispute.sh
```

El comando crea y verifica tres cuentas nuevas mediante registro y Mailpit en la primera ejecución; en ejecuciones siguientes reutiliza las mismas cuentas. Asigna `arrendador` al anfitrión y `administrador` a una cuenta distinta usando PostgreSQL local; el arrendatario conserva solamente `arrendatario`. Crea una reserva/fixture sintética aislada para cada ejecución y conserva el historial anterior.

Las credenciales se generan localmente y se guardan fuera del repositorio en `~/.local/state/espacigo/local-priv-01a2-test-accounts.json` (archivo `0600`, directorio `0700`). No se imprimen ni se copian al repositorio. La cuenta administradora no comparte identidad con el anfitrión o el arrendatario.

## Resultado observado

El comando pasó tres veces, reutilizando las mismas tres cuentas y generando una reserva y una incidencia nuevas en cada recorrido:

- Una reserva pendiente de pago fue consultada por anfitrión y arrendatario.
- Solo el anfitrión pudo abrir `ensayo_privacidad`; el arrendatario consultó la incidencia. Anfitrión y arrendatario recibieron `403` al intentar cerrarla; el administrador independiente pudo consultar la cola y cerrarla con `ensayo_finalizado`.
- La revisión administrativa de ambos participantes detectó `reserva_activa` y `disputa_abierta`, además del pendiente independiente `matriz_retencion_historicos_incompleta`.
- Tras cerrar, ambas revisiones conservaron `reserva_activa` y el pendiente de retención, y eliminaron únicamente `disputa_abierta`. La reserva siguió en `pendiente_de_pago` hasta que el arrendatario usó la API para cancelarla; el anfitrión vio el estado actualizado. Luego las revisiones dejaron de detectar obligaciones, manteniendo pendiente la matriz de retención.
- La supresión no se ejecutó. La baja/desidentificación sigue deshabilitada.

Resultados de la última ejecución: reserva `e28a5ac5-10ca-41b8-a743-a05ceb0b6d40`, incidencia `e0cb3305-3b38-4ba0-89eb-04a0f3352b56`. Las ejecuciones anteriores también pasaron con IDs distintos.

No se eliminaron las cuentas ni fixtures sintéticos M06 existentes: conservan reservas e historiales de recorridos aceptados y no fue necesario sustituirlos para esta prueba. Los nuevos datos se limitan al espacio/fixture y reservas sintéticos asociados al grupo local `LOCAL-PRIV-01A2`.

#189 cumple los criterios de esta subentrega y puede aceptarse. #185 y #40 permanecen abiertas; la retención histórica y su fundamento no se ratifican con este recorrido. Los plazos propuestos en `planning/propuesta_fundamentos_plazos_privacidad_local.md` siguen pendientes de decisión.
