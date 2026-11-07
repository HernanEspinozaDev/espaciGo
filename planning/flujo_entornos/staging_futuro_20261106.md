# Staging bajo demanda — propuesta diferida para primeros usuarios

Fecha: 2026-11-06. Estado: **futuro, fuera de la ejecución inicial**. La instrucción posterior del usuario pospone staging hasta la preparación de producción final y los primeros usuarios reales. El [plan vigente de desarrollo](plan_evolucion_20261106.md) tiene un solo destino cloud y no depende de este documento.

La revisión 2 completa permanece en el historial Git, commit `17789c36762c7c6c12ff252e17a4bd6c6f9f4d71`, archivo `planning/flujo_entornos/plan_evolucion_20261106.md`. No se habían implementado sus componentes.

Antes de abrir el producto a usuarios reales se deberá preparar un corte separado para:

1. Clasificar el destino productivo y separar datos/identidades/secretos/proveedores del staging sintético.
2. Cotizar y aprobar una instancia staging compatible, retenida y normalmente detenida; mantener PostgreSQL efímero en CI.
3. Promocionar el mismo manifest/digests probados desde staging a producción, con compatibilidad SQL y recuperación verificadas.
4. Implementar sesiones con reserva exclusiva, estado durable, fecha de vencimiento y finalizador; añadir vigilante independiente para fallo/cancelación/pérdida del runner.
5. Verificar cierre de SQL y ausencia de consumidores/carga fuera de sesión. Mínimo 0 de Cloud Run no impide que nuevas peticiones lo reactiven.
6. Probar expiración, sesiones concurrentes, finalizadores obsoletos, readback tras COMMIT incierto y alertas por cierre fallido; nunca apagar producción mediante este mecanismo.

Los parámetros anteriores de TTL de 2 horas, revisión cada 5 minutos y cierre de 10 minutos eran propuestas, no recursos configurados ni compromisos aprobados. Se revisarán con las necesidades reales del momento. Los costos de almacenamiento/backups y servicios asociados se cotizan aunque SQL permanezca detenido.

**El hito para retomar esta propuesta es la preparación para incorporar los primeros usuarios reales, antes de abrir ese acceso**, no un número de pushes o el cierre del pipeline inicial. El desarrollo cloud actual no se convierte automáticamente en producción final por recibir un despliegue desde main. La clasificación, migración de datos si corresponde y aceptación de producción requieren su corte explícito.
