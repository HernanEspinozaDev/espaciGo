# GCP-03 `g03r-1016`: matriz de cierre y siguiente trabajo

Fecha de evidencia: 2026-10-16. Esta matriz registra el ensayo sintético y la limpieza del run cloud. No representa aceptación de producto, restore de datos reales ni cierre completo GCP-03.

| Criterio | Estado | Evidencia / límite |
| --- | --- | --- |
| Fixture y restore PostgreSQL local aislado | Completo | Cinco ejecuciones finales PostgreSQL 18.6/PostGIS 3.6.4, network none, artefactos efímeros y diez comprobaciones por corrida; evidencia local excluida de Git. |
| Restore y verificación Cloud SQL sintéticos | Completo para fixture mínimo | Dos cuentas sintéticas, un espacio y dos ocupaciones; restore/verify del run pasó sus nueve comprobaciones. No es snapshot del origen ni cubre datos reales. |
| Retire Cloud Run | Fallido, sin reintento | Aplicación reportó `PHASE_FAILED`; container exit 1, proxy exit 0, estado de intento vacío. `commit_outcome_uncertain=false` no demuestra rollback; outcome SQL no resuelto. |
| Retiro de usuarios SQL IAM | Completo | API Delete aprobada; dos operaciones `DONE`, error nulo, usuarios ausentes. |
| Limpieza de objetos autorizados | Completo | Cuatro generaciones exactas borradas con precondiciones; bucket del run quedó sin objetos. |
| Stage3 de recursos run-scoped | Completo | PR #384 revisó HEAD `13e3ecb1fdafe6111d277761b6a2480a5c276743`; merge `e50fd4bb526bdd04992b00e8e807271345214f39`. Plan SHA `ea47a00c6330bf006b2f5e5abf2fc376f399b1cdaa428677bd089cbd40b753cc`, 0 add/0 change/30 destroy. State lineage preservado, serial 16, cero recursos gestionados. |
| Teardown Terraform idempotente | Verificado en configuración | Plan completo/refrescado tras apply: cero cambios y cero drift; no se aplicó. No se hizo un segundo apply de borrado. |
| Limpieza API y preservación de baseline | Completo según lecturas post-apply | Jobs, SAs, secret, bucket, DB e instancia del run ausentes; testprod y pilot01 `RUNNABLE`, VPC/subnet, Artifact Registry y backend Terraform conservados. Versionado del state backend habilitado. |
| Consulta Cloud SQL Operations durante borrado | Limitación registrada | Lectura adicional no pudo distinguir permiso de recurso ausente; no se reintentó. Apply principal tuvo éxito y las lecturas finales más el plan sin cambios confirmaron ausencia. |
| Credenciales locales del ensayo | Revisión acotada | Se inspeccionaron 120 artefactos preexistentes en `.local/testing/gcp03-1017`; todos estaban modo 0600. El barrido de nombres y marcadores comunes de valores credenciales no encontró candidatos; no se leyó payload de Secret Manager. State, planes y logs se conservaron como evidencia. |
| Snapshot autorizado del origen | Pendiente | No se leyó, exportó ni movió base, backup, archivo o dataset persistente. Requiere aprobación del dueño, alcance y plan separado. |
| Compatibilidad con snapshot/origen en Cloud SQL | Pendiente | Extensiones/roles/ACL/owners y comportamiento real post-restore; integración privada/TLS, app y reglas de reservas deben verificarse. |
| Retire SQL outcome | Sin resolver | No ejecutar nuevo `retire` ni deducir rollback. Requiere una estrategia observacional/read-only revisada antes de cualquier nueva operación. |
| Aceptación humana de GCP-03 | Pendiente | La evidencia técnica no equivale a aceptación. GCP-04 no se inicia ni se marca completo en este cierre. |

## Próximo paquete de trabajo, sin provisión

1. Conseguir autorización del snapshot origen: identidad del snapshot, consistencia, ventana, scope, retención y dueño.
2. Preparar manifest allowlist/hash y tabla de compatibilidad V1–V61 sin copiar contenido personal; reutilizar los resultados sintéticos como referencia.
3. Hacer una revisión de solo lectura de si una DB nueva en `pilot01` puede aislar roles/owners globales y carga. Si no, preparar alternativa de instancia privada efímera aislada.
4. Cotizar SKUs/región/cuotas vigentes y fijar una ventana/cota de costo; el cálculo previo de ocho horas es referencia no vinculante. Ningún plan se aplica en esta fase.
5. Someter snapshot, matriz, opción de topología, presupuesto, plan exacto, teardown y criterios de parada a revisión antes de solicitar cualquier cambio cloud.

La alternativa más barata no se presupone válida: una DB separa objetos de base, pero no los roles PostgreSQL del cluster ni la carga de CPU/IO. La revisión puede recomendar pilot01 solo si demuestra que esas dependencias no interfieren; de lo contrario, mantener la instancia efímera como alternativa con costo actualizado. Ninguna opción autoriza hoy una importación o provisión.
