# Corrección tras prueba manual — Mi cuenta / estado Didit

## Pedido y alcance

El usuario informó que ve dos versiones de la aplicación (dashboard frente a Mi cuenta), que el resultado del retorno no reaparece actualizado en Mi cuenta, y que aprobar manualmente una sesión Sandbox con advertencias no cambia lo visible. Pide mostrar estado y una acción para consultar/revisar nuevamente, y comprobar enlace con el formulario guardado.

La superficie canónica debe ser **Mi cuenta**, sin crear otra aplicación/dashboard. Antes de borrar o redirigir rutas, verificar su procedencia, usos y contratos existentes. Conservar datos y la sesión humana; no usar navegador del usuario. Investigación/ensayos locales, sin nueva sesión Didit, cambio remoto, migración, commit/push ni producción por inferencia.

## Distinciones obligatorias

- Estado del proveedor (p.ej. aprobado por revisor) ≠ evaluación EspaciGo de coincidencia ≠ elegibilidad Live. Mostrar las diferencias claramente sin forzar aprobación ni ocultar discrepancias.
- Sandbox simula identidades/resultados aunque reciba imágenes reales. La documentación oficial vigente consultada es help.didit.me/integration/sandbox-testing y help.didit.me/integration/create-session: la comparación `expected_details` puede producir advertencias de identificación/nombre/fecha. No atribuir el nombre simulado concreto al formulario sin trazar payload y sesión.
- Actualizar/consultar un intento existente no debe crear otro intento ni reenviar documentos. Una nueva revisión debe ser acción deliberada, con consentimiento y reglas de idempotencia/supersesión; no un reintento automático de una escritura incierta.

## Evidencia parental inicial

- Worktree espaciGo-main-sync, rama codex/marketplace/fix/kyc-06-save-identity-first, HEAD base274f37e483a078890e92382a159fa49fd23c2fe8; dirty previo preservado, incluido AGENTS.md ajeno.
- Procesos locales comprobados: API18090/PID120498, Next4201/PID105137, proxy4219/PID105185. No se leyeron pestañas ni se autenticó al usuario.
- Query agregada READ ONLY como APIlogin→runtime, filtrada por app/workflow Sandbox configurados: un intento IN_REVIEW, con sesión y declaración asociadas y decisión cifrada almacenada. Eventos status.updated: tres PROCESSED y uno IGNORED. No se imprimieron UUIDs de cuenta, PII, imágenes, tokens ni ciphertext. Esto demuestra recepción/procesamiento local, no la razón exacta de cada evaluación.
- `AccountVerificationCard` usa overview genérico de verificaciones en vez del endpoint de intento Didit. `VerificationIdentityForm` carga solo declaración guardada. `DiditService.GetStatus` devuelve estado persistido y badge, sin reconsulta al proveedor. Source-only hasta completar reproducción y seguimiento de los demás componentes.
- ProcessWebhook no trata IN_REVIEW como terminal; por tanto no aceptar esa hipótesis sin otra prueba. La evaluación estricta puede mantener revisión pese a Approved remoto con discrepancias. El snapshot mínimo cifrado ya distingue ProviderStatus y EvaluatedStatus, pero no se expone aún aquí como estado de usuario confirmado.

### Contraste autorizado de la sesión del ensayo (sin imprimir identidad)

- GET de decisión autoritativa de la única sesión local acotada al app/workflow Sandbox: proveedor **Approved**, mientras la base conserva IN_REVIEW. SessionID, vendor_data/correlación y workflow coinciden exactamente. No se creó ni editó sesión remota.
- Snapshot mínimo local descifrado únicamente en memoria, salida allowlist: ProviderStatus **In Review**, EvaluatedStatus IN_REVIEW, livenessfalse, faceMatchtrue y códigos de discrepancia. Por tanto se confirma snapshot local anterior al estado actual del proveedor, no solo un badge Sandbox filtrado.
- `expected_details` existe en la respuesta real con los campos esperados. Comparación privada con la declaración concreta asociada al intento: apellidos/fecha/RUT/país/tipo de documento coinciden exactamente; **nombres no coinciden por comparación literal**. No afirmar igualdad completa ni atribuir el desacuerdo a normalización hasta investigarlo. Ningún valor de identidad, clave, sesión URL o imagen se imprimió ni escribió en evidencia.

## Criterios de corrección

1. Un único recorrido visible canónico Mi cuenta, sin interfaz alternativa añadida para esta función; retorno y enlaces coherentes, preservar el resto de la aplicación.
2. Al entrar o volver a Mi cuenta/Verificaciones, recuperar estado del intento propio más reciente desde servidor, incluso tras recarga; no depender solo del query del retorno o memoria del navegador.
3. Mostrar pendientes/en revisión/aprobación de prueba/rechazo y error de consulta con semántica separada de badge Live y evaluación de identidad. Un error no equivale a rechazo ni aprobación.
4. Acción explícita de actualizar estado sin crear sesiones; definir la disponibilidad de revisar nuevamente según el estado permitido, sin consumir sesiones automáticamente.
5. Probar que datos guardados salen en expected_details, y que sesión/correlación/attempt corresponden; usar fixtures sintéticos en pruebas y leer solo metadatos permitidos del ensayo humano.
6. Reproducir revisión→aprobación y revisión→rechazo, casos con discrepancias, Sandbox≠Live y cambio/cierre de cuenta. Backend y frontend deben conservar autorización/aislamiento y no resucitar intentos terminales ni sobrescritos.
7. Revisar y probar cambios antes de reemplazar únicamente los procesos propios; no rebuild sobre standalone vivo. Conservar la prueba humana y registrar límites honestamente.

## Trabajo en curso

Delegación de investigación de solo lectura: frontend/navegación y backend/contrato, sin escritores concurrentes. Implementación se delimita tras resultados y una reproducción que falle por el síntoma concreto.

## Diseño de corrección delimitado

- Confirmado por historia: dashboard es una superficie starter antigua (41484bf), no creada en el corte manual. Se conserva compatibilidad de URLs mediante aliases hacia Mi cuenta, sin borrar páginas ajenas ni fabricar otra cuenta. Fuente modificada por delegado: dashboard/general/activity→Mi cuenta, security→Mi cuenta/seguridad; retorno canónico. Padre ejecutó7pruebas de alias +TypeScript+diff-check PASS. **Aún no está reconstruido el standalone**: no confundir fuente con pantalla entregada.
- Se implementará una consulta owner-scoped del intento más reciente y otra inspección explícita del mismo ID, sin crear sesiones, inventar eventos webhook, editar declaración ni otorgar aprobación durante GET. La vista separará `provider_status`, `evaluation_status`, estado persistido y badge. Una consulta no promoverá elegibilidad ni resucitará terminales/supersedidos. La reconciliación persistente seguirá siendo la ruta auditada actual; no se simula un webhook para actualizar una columna.
- Contrato propuesto: GET intent para último estado propio y GET intent/{id}/refresh para inspección real; DTO mínimo sin sesión proveedor, vendor_data, datos legales, URLs o imágenes. OpenAPI y prueba de contrato se preparan antes de generar cliente y asignar UI.
- La primera asignación backend se negó solo por el dirty compartido; no cambió archivos. Padre reiteró que ese baseline es autorizado y congelado por archivo, sin pedir al usuario un worktree nuevo ni descartar trabajo. Nueva asignación acotada continúa sobre ese mismo target.
- Comparación adicional privada confirmó que la diferencia del nombre tampoco se resuelve por trim/case/Unicode/diacríticos. No hay todavía prueba de su origen; se conserva como hallazgo, no como permiso para editar la declaración o aprobar.

## Gates implementados y bloqueo remoto observado después

- El ejecutor backend volvió a informar dependencias inexistentes. Padre verificó ruta/rama/HEAD y existencia real de servicio42177bytes/test32411bytes; rechazó ese diagnóstico sin pedir otro worktree. Se añadieron pruebas parentales reales y se obtuvo RED en resurrección de terminales, aprobación con declaración purgada y error proveedor sin sanear; GREEN después de correcciones acotadas. Prueba adicional confirma envío íntegro de nombre compuesto Unicode desde declaración a expected_details y lookup latest owner-scoped.
- HTTP primer ejecutor dejó pruebas RED antes de fallo400del proveedor del modelo. Padre conservó esas pruebas, completó DTO dedicado, auth/rutas/mux y añadió prueba de handler real con servicio/repositorio sintéticos: JSON allowlist, rawApproved/evaluaciónReview, foreign404sinconsulta y error502sincontenido privado. Race/vet verification/...+cmd/api+adaptadorPG PASS.
- Nueva evidencia nativa agregada: el intento está ligado a revisión1 y la declaración actual es revisión2. Se reprodujo el caso superseded en prueba; inspeccionar puede mostrar el resultado del proveedor del intento anterior, pero no aprobar la revisión nueva. No se cambió ninguna declaración del usuario.
- El primer preflight nativo READ ONLY hizo visible ese caso; después de corregirlo, la consulta real a Didit devolvió404. Repetición con Go y curl confirmó404, sin imprimir sesión/PII; clave del proceso original coincide con archivo privado. Outbox acotado a esa sesión:0solicitudes de borrado/0ACK. No afirmar quién la borró ni inventar aprobación: el test nativo de inspección real permanece **FAIL por recurso remoto no disponible**, no PASS ni mock sustituto. UI debe presentar error de consulta y permitir retry/revisión de datos, no convertirlo en rechazo ni ausencia de intento local.
- OpenAPI dos lecturas +codegen/allowlist204operaciones generados. Prueba de contrato y adaptador (con contratos anteriores) parentales14PASS. UI se asignó solo después de esas dependencias reales.
- API nueva instalada tras verificar y detener únicamente PID120498; proceso actual PID196339/handleproc_59a91247c72e, receipt privado api-status-process.json. Ready200 y ambos GET nuevos anónimos401. No nuevas sesiones/login/cuentas ni SQL de escritura ni workers activados por esta corrección.
- Frontend aún pendiente de la entrega UI, build y Playwright controlado; no se presenta código fuente como pantalla ya actualizada. Las pruebas futuras de aprobación/rechazo serán evidencia sintética de interfaz, separada del404 real de la sesión previa.

### Revisión de integración UI

- Primera entrega conectó las tres superficies pero omitió las pruebas de presentación/navegador. Padre detectó que la acción de refresco en Mi cuenta seguía consultando latest aunque ya conocía un ID; corrección del mismo ID, abortos y casos DECLINED/terminales/Sandbox se asignaron junto con las pruebas faltantes. No se aceptó el PASS del adaptador como prueba UI.
- La entrega también retiró el enlace canónico recién corregido del retorno. Prueba de navegación reprodujo RED6PASS/1FAIL; padre restituyó Volver a Mi cuenta y quitó el botón duplicado de abrir formulario, conservando una sola acción. Regresión de navegación/adaptador13PASS y TypeScript PASS después.
- La versión de frontend en ejecución permanece intacta hasta congelar los cambios y ejecutar build+browser de la misma entrega. No se alteró la sesión ni se generó una verificación nueva para resolver estos defectos.

## Entrega montada y verificación final

- Panel corregido mantiene ID conocido al refrescar y tras error, aborta lecturas desmontadas y distingue DECLINED/terminales; aviso Sandbox en todos los resultados. Presentación3+adaptador6+navegación7:16PASS parentales. TypeScript, ESLint focal y Prettier focal PASS. No se ejecutó toda la regresión global frontend ni prueba específica A→B tardía en navegador; el key por identidad y generación/abortos fueron revisados, no sustituyen esa prueba pendiente.
- Se verificó UID/cwd de Next propio105137 antes de SIGTERM, puerto4201 libre, build Next producción con entorno allowlist PASS, assets copiados y nuevo proceso203563/handleproc_a3a4f3c4c2ad. Proxy4219/API196339/buzón se preservaron. Receipt privado frontend-status-process.json con ausencia de secretos backend comprobada. Las cookies/sesiones de la base no fueron reseteadas.
- Playwright aislado controlado contra el artefacto nuevo4201: primer run2PASS/1FAIL porque el test trataba retorno proveedor como return_to de login y esperaba revisión pese a fixtureDECLINED. Se corrigió solo el test a loginMiCuenta→navegaciónretorno y resultado real esperado; también se arregló el mock502 para interceptar la ruta refresh (antes404 habría dado falso positivo). Segundo run **3PASS**: mismo intento/reload sinPOST, retornoDECLINED/limpiezaquery/refresco y error502 recuperable sin mutación. Esto es evidencia UI con fixtures sintéticos, no aprobación real de proveedor ni login humano.
- GET HTTPS independiente de los cuatro aliases confirma destinos Mi cuenta/seguridad correctos, APIready200. El frontend ya está montado en https://127.0.0.1:4219. El usuario puede recargar su propia pestaña sin nueva URL ni otro dashboard.
- Límite externo conserva evidencia: la sesión remota anterior devolvió404; no se recuperó ni recreó. La interfaz muestra fallo de consulta, mantiene el último resultado conocido solo junto al aviso de error si existía y permite consultar de nuevo o abrir el formulario existente. No se afirma nueva aprobación ni sincronización persistente de un evento que no llegó.
- HEAD base274f37e483a078890e92382a159fa49fd23c2fe8 con dirty compartido preservado; sin commit/push, main, migraciones, cambio de credenciales, base o workers. Diferencia de nombre explicada parcialmente por revisión1 frente a revisión2; no se modificaron identidades para hacerlas coincidir.

## Reapertura por aprobación Sandbox no reflejada automáticamente

- Corrección humana: Didit sí muestra aprobado; el problema es frontend sin reflejarlo. No volver a atribuir ese síntoma a Sandbox ni pedir otra sesión/documento.
- Consulta real actual del último intento: HTTP200/proveedorApproved, sesión/correlación coincidentes, declaración vinculada actual. El404 histórico corresponde a un intento anterior, no es el bloqueo actual.
- Se ajustó únicamente el selector del test nativo opt-in para elegir el último intento por created_at/id, preservando alcance app/workflow/Sandbox y sesión PostgreSQL READ ONLY. Ejecución de GetLatestStatus real **PASS**: providerAPPROVED, evaluationIN_REVIEW, persistedIN_REVIEW, badgeIN_REVIEW, mismo intento y sin cambios de estado/eventos. Así se confirma que el servicio ya obtiene la aprobación; la evaluación estricta no debe esconder ese resultado de proveedor en UI.
- Fuente/frontend desplegado actualizado comprobado: panel solo leía al montar o pulsar botón; no tenía actualización automática por foco/visibilidad ni polling. Se asignó corrección acotada y regresión de cambio InReview→Approved sin pulsar botón, conservando mismo ID/noPOST y guard Live. No se presenta como solucionado hasta build y browser contra artefacto nuevo.

### Corrección automática montada

- Entrega revisada: actualización por focus/visibility visible y polling10segundos acotado6lecturas, sinPOST ni intentos nuevos, detención ante terminal/error; control de solapamiento y purga401/403. La prueba de polling por reloj y el caso A→B siguen sin ejecución específica: no afirmar cobertura total de esos caminos.
- Padre ejecutó presentación/adaptador/navegación16PASS, TypeScript y ESLint focal PASS. Detuvo únicamente Next propio203563 tras verificar UID/cwd; build producción con entorno allowlist PASS y assets copiados. Next actual233563/handleproc_759c9eccd1ae; receipt privado frontend-auto-status-process.json. Proxy/API/buzón/sesión persistente preservados.
- Playwright contra build definitivo: **3PASS**. Incluye Mi cuenta con revisión→aprobación del mismo intento mediante evento focus, sin pulsar consulta y sin POST; retornoDECLINED/limpiezaquery;502recuperable. Es interfaz con fixtures controlados, no prueba autenticada de la sesión humana.
- Verificación posterior independiente: HTTPS4219/mi-cuenta/verificaciones200, APIready200, nuevo proceso/artefacto y ausencia de secretos backend en Next comprobados; diff-check PASS. Resultado real del servicio para el intento humano actual ya había sido APPROVED en READ ONLY, sin modificar decisión ni declaración. No pedir repetir verificación ni adjuntar medios por esta corrección.

## Aclaración humana y decisión del resultado de ensayo

- El usuario confirmó URL4219/mi-cuenta/verificaciones y que **sí aparece Aprobado por Didit**, pero esperaba transición a cuenta verificada. Se rectifica el diagnóstico: la comunicación de esa aprobación está funcionando; el desacuerdo restante es la semántica del estado final de prueba, no ausencia demostrada de REST.
- Comprobación REST real adicional: login200 y sesión200 de fixture sintético autorizado mediante BFF HTTPS, GETlatest404not_found para ese fixture sin intento propio y logout204 de esa única sesión de diagnóstico. No se accedió a sesión/pestañas/credenciales del usuario ni se confundió ese404 con fallo proveedor. GETs anónimos por Go/Next/HTTPS401enambasrutas confirmaron protección, no equivalen a lectura autenticada humana.
- Decisión literal seleccionada en aclaración: **«Mostrar ‘Cuenta verificada — prueba Sandbox’ al aprobar Didit, sin conceder elegibilidad ni permisos reales»**. Alcance: presentación Sandbox solamente, a partir de provider_statusAPPROVED autoritativo; mantener separada la evaluación de coincidencia, badge_result persistido, claims y permisos. No alterar Live ni aprobar por callback/webhook no validado. Si evaluación sigueIN_REVIEW, mostrarlo como comparación pendiente sin ocultar la aprobación simulada.
- Implementación visual mínima asignada con regresiones para Sandbox/Live/noPOST; fuente aún no montada hasta revisión/build/browser. No migración ni cambio de estado SQL, claves, permisos o provider por esta decisión.

### Resultado de la decisión montado

- Sandbox+providerAPPROVED muestra literalmente **Cuenta verificada — prueba Sandbox**, con aprobación simulada/sinreservas,pagos,permisosreales y comparación de datos aún en revisión cuando corresponde. No se modificó badge_result, evaluación, claims, DB o Live.
- Padre ejecutó presentación5+adaptador6=11PASS, TypeScript y build producción PASS. Detuvo solo Next propio233563 con UID/cwd comprobados, copió assets y montó Next258569/handleproc_83bc27dfa5e1. Receipt privado frontend-sandbox-verified-process.json; HTTPS4219/verificaciones200.
- Playwright del artefacto nuevo **3PASS**, incluyendo actualización automática a la etiqueta Sandbox elegida, advertencia de permisos, ceroPOST, rechazo y fallo502recuperable. La prueba de navegador sigue siendo controlada; los GET REST reales anteriores y la confirmación humana de Aprobado enDidit sustentan la comunicación, no se fabricó una aprobación nueva.
- Recursos API/proxy/buzón y sesión humana preservados. Sin commit/push/main/despliegue ni nueva verificación. El usuario solo requiere recargar para descargar este bundle.
