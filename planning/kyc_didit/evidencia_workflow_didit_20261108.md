# Revisión read-only del workflow Didit — 2026-11-08

## Alcance y método

- Fuente: herramientas Didit MCP `didit_workflow_list`, `didit_workflow_get` y `didit_workflow_get_graph`, llamadas en modo de solo lectura el 2026-11-08.
- Se listaron workflows en 2 aplicaciones (7 filas; 0 aplicaciones omitidas/no listables). Se siguió el nombre exacto `Compliance workflow` registrado en el plan KYC y se leyó su configuración por UUID y aplicación exacta.
- No se abrió la consola/navegador, no se leyó una API key ni variable de entorno, no se modificó el workflow, no se creó/publicó un draft y no se creó una sesión.

## Workflow inspeccionado

- Organización: `Hotusoft`.
- Aplicación: `Hotusoft (Sandbox)`; UUID `806fe3f4-4c9b-4567-bf6f-ce8c3e688fce`.
- Workflow: `Compliance workflow`; UUID `a4b973d4-4578-44fc-8580-7554e39821ce`; versión 1; estado `published`; `is_editable=false`; no es default.
- En la aplicación `My Application`, el workflow default listado es `Free KYC` en estado `draft`; no se confirmó un workflow KYC publicado para esa aplicación.
- El valor runtime `DIDIT_WORKFLOW_ID` y la aplicación seleccionada por el proceso EspaciGo no se leyeron. No se puede afirmar que `cmd/api` use el workflow sandbox inspeccionado.

## Configuración observada

1. El grafo publicado es lineal: OCR → LIVENESS (`ACTIVE_3D`) → FACE_MATCH → status `Determine`. La lista resumida del workflow también muestra OCR + LIVENESS + FACE_MATCH; AML e IP analysis no figuran en esa lista.
2. `expected_details_mismatch_action=REVIEW` en el objeto de configuración.
3. `response_attributes=null`. La semántica devuelta por la herramienta Didit indica que null/missing significa todos los puntos de datos producidos por las features, no una lista mínima; puede incluir datos del documento y, si se ejecutan pasos selfie/liveness, la imagen selfie y el video de liveness. No enumero nombres de campos porque la respuesta no proporciona el catálogo.
4. País CHL permite cuatro categorías documentales: pasaporte (`P`, un lado), licencia (`DL`, dos lados), cédula/tarjeta de identidad (`ID`, dos lados; `ID_CARD_GENERIC` y `ID_CARD_LEGACY`) y residencia (`RP`, dos lados). La configuración **no limita** el flujo a cédula chilena `ID` solamente, aunque `ID` está permitido y exige ambos lados.
5. `session_expiration_time=604800` segundos (7 días), `max_retry_attempts=3`, `retry_window_days=7`. El tiempo de expiración de sesión no es una política de borrado/retención de medios. La consulta del workflow no expuso un plazo de retención de datos de la aplicación.
6. Hay una inconsistencia que requiere aclaración: el grafo y la lista indican nodos/features LIVENESS y FACE_MATCH, pero el objeto completo también trae `is_liveness_enabled=false` y `is_face_match_enabled=false`. No normalizar ni ignorar esa discrepancia; confirmar su semántica con Didit antes de cualquier uso live.

## Impacto en el requisito de no conservar fotos tras verificar

- El workflow sandbox actualmente devuelve todos los puntos de datos (`response_attributes=null`). Por tanto, la intención “Didit conserva las capturas; EspaciGo no” **no está demostrada por la configuración**: la respuesta/webhook puede incluir medios. Esto no elimina el cotejo ya propuesto: Go envía por sesión `expected_details` desde la declaración guardada; el backend compara la decisión/documento con esa instantánea y exige documento, liveness y face match aprobados. Solo la coincidencia completa permite `VERIFIED`; un mismatch permanece en revisión y requiere corrección/reverificación controlada. El backend debe descartar bytes multimedia sin persistirlos ni registrarlos, o el workflow debe cambiarse a una salida compatible con la retención elegida. Apagar un control `Returned data` puede también evitar que Didit almacene el medio; no asumir que solo afecta al consumidor.
- Propuesta de ciclo de vida: mantener la declaración cifrada y material del proveedor solo mientras el backend verifica; al reconciliar desde fuente autoritativa todos los requisitos y guardar `VERIFIED`, solicitar borrado inmediato de la sesión Didit, eliminar la declaración/snapshot cifrado y conservar únicamente el estado mínimo de verificación conforme al plazo aprobado. No borrar por el retorno del navegador ni por un webhook aislado.
- Si la sesión queda pendiente, cerrar/expirar por la política aprobada; el TTL de 7 días sirve como dato de la sesión, no como prueba de que Didit haya eliminado medios. Solicitar y confirmar borrado por separado; webhooks ya encolados pueden llegar después.
- V85 hace `identity_provider_attempt.declaration_id NOT NULL` y V86 no purga declaraciones referenciadas. Para borrar la declaración tras éxito sin perder el estado mínimo de verificación, hace falta diseñar una nueva transición/migración append-only y cubrir intento, eventos, outbox de borrado remoto, idempotencia y webhook tardío. No se cambió SQL/código ni se aplicó DDL.

## Resultado

La inspección fue de lectura solamente. El workflow publicado localizado está en Sandbox y no es editable; el vínculo del runtime a esa aplicación no se verificó. No se recomienda iniciar otra sesión ni exponer el flujo hasta resolver el response-all, la discrepancia de flags, el alcance de documentos, la retención por aplicación y el borrado en estado VERIFIED.
