# Evidencia KYC-00 — planificación e investigación

Fecha: 2026-11-06. Alcance: plan y guía KYC actualizados, conexión MCP, lectura de workflow; **sin implementación**.

## Base y aislamiento

- Worktree creado: `/home/nandev/Documentos/projects/espaciGo-kyc-didit`.
- Rama: `codex/marketplace/feature/kyc-00-didit-identidad`.
- HEAD base: `771134d165046b290b1f5ad3a1074a6357c2327f`.
- Se ejecutó `git fetch origin --prune`. El worktree limpio de main ya coincidía con origin/main; `merge --ff-only origin/main` informó «Ya está actualizado».
- Checkout backend histórico y demás worktrees preservados. La rama nueva no sigue origin/main como upstream de publicación. No commits, push, PR, integración ni despliegue.
- Solo archivos nuevos bajo `planning/kyc_didit/`; ningún código funcional o migración modificado.

## Investigación y contraste

- Fuentes oficiales Didit consultadas mediante búsqueda y HTTP de sus páginas Markdown y OpenAPI. Índice: `https://docs.didit.me/llms.txt`.
- Matriz y enlaces en el plan. Precios: se detectó explícitamente el cambio vigente desde 2026-11-01; no se tomó el video/cuota anterior como contrato actual.
- Inspección delegada de solo lectura, contrastada en el padre sobre SQL, Go, OpenAPI y Next. `git ls-files db/migrations` y lectura programática confirmaron 83 archivos tracked; no constituye consulta ni aceptación del ledger de una DB viva.
- Campo `expected_details` y códigos documentales comprobados contra `https://docs.didit.me/openapi-25.json` descargado. Se verificaron `first_name`, `last_name`, `date_of_birth`, `id_country`, `nationality`, `identification_number`, `expected_document_types` y código `ID`.
- Referencias locales `ruta:línea` comprobadas por existencia y rango de línea. Las ubicaciones son de este HEAD, no de una base remota ni de otro worktree.

## MCP: resultado real

- El primer intento OAuth de 30 segundos expiró; el reintento por CLI oficial con 600 segundos llegó a Didit y el usuario completó OAuth/PKCE. Se descubrieron 194 herramientas.
- Allowlist exacta verificada: `didit_org_list_applications`, `didit_session_create`, `didit_session_get_decision`, `didit_workflow_list`, `didit_workflow_get`, `didit_workflow_get_graph`, `didit_workflow_get_feature_config_schema`, `didit_workflow_validate_graph`, `didit_workflow_edit_graph`, `didit_workflow_create_draft`, `didit_workflow_publish`. Son permisos disponibles; no implica que se hayan ejecutado escrituras.
- `hermes mcp list`: Didit habilitado, 11 seleccionadas. `hermes mcp test didit`: salida 0, transporte HTTP conectado en 4665 ms. No se mostró ni copió ningún secreto.
- Lectura MCP de `didit_workflow_list` y `didit_workflow_get`, sin sesiones/personas: `Hotusoft` es nombre de organización/app, no de workflow. App `Hotusoft (Sandbox)` (`806fe3f4-4c9b-4567-bf6f-ce8c3e688fce`); workflow `Compliance workflow` (`a4b973d4-4578-44fc-8580-7554e39821ce`), publicado v1, `CHL`, `OCR + LIVENESS + FACE_MATCH`, y `expected_details_mismatch_action=REVIEW`. Documentos permitidos para CHL incluyen ID (2 lados), RP, DL y P; el MVP propuesto limitará la UI a ID.
- El payload del workflow reportó USD 0.35 estimados; no se comprobó tarifa de aplicación live ni hubo cobro. Ningún workflow se creó/editó/publicó; ninguna sesión de verificación se creó. No se descargaron documentos ni datos personales.

## Comprobaciones ejecutadas

- `git diff --check`: sin errores para tracked. Se revisaron además los documentos nuevos explícitamente, ya que no aparecen en el diff tracked.
- `git diff --no-index --check /dev/null <documento>`: sin salida de errores de whitespace. Este modo puede devolver 1 por existir diferencias; la primera comprobación lo interpretó erróneamente como fallo, se corrigió el wrapper para aceptar 0/1 únicamente sin diagnósticos y se reejecutó con PASS.
- Verificador Python de fences Markdown y referencias locales: PASS.
- Aserciones contra esquema OpenAPI Didit para los campos propuestos: PASS.
- `hermes mcp list` y `hermes mcp test didit`: PASS de conexión/configuración; no es prueba de KYC de usuario.
- Plan/guía/evidencia actualizados solo como documentación. No se ejecutaron tests Go/Next/DB ni builds; no hubo llamadas de sesión, cámara, webhook ni prueba funcional de cotejo. Sus gates permanecen futuros.

## Entrega y siguiente gate

- `plan_kyc_didit.md`: propuesta end-to-end, matriz actual/delta y fases de implementación.
- `guia_conexion_y_operacion.md`: OAuth MCP, preparación segura de secretos y checklist del workflow existente.
- Estado KYC-00: documentación, conexión OAuth y lectura acotada del workflow completadas; **integración funcional no implementada**. KYC-01 queda parcial hasta ratificar decisiones de producto/privacidad y confirmar el mapeo RUN/RUT con evidencia sintética.
- El usuario autorizó establecer el flujo y planificar el comienzo del desarrollo, pero indicó expresamente **no iniciarlo todavía**. No se modificó código, SQL, workflow remoto ni datos. No hay commit/push, integración a main ni deploy.
- Próximo paso cuando el usuario autorice iniciar: cerrar KYC-01; después comenzar KYC-02 (modelo/migración revisada + OpenAPI) y seguir Go → Next → ensayo sandbox sintético conforme al plan. Live y publicaciones siguen separados y no autorizados.
