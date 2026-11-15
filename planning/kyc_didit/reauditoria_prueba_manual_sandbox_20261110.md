# Reauditoría para prueba manual local con Didit Sandbox — 2026-11-10

## Veredicto y alcance

La infraestructura HTTPS fija está creada; el usuario confirma que editó el destino webhook existente en Didit, guardó la nueva URL y mantuvo su secreto. No se requiere rotar la clave API o el secreto por ese cambio. La confirmación de consola es del usuario, no una lectura independiente del agente.

El recorrido manual todavía no está listo: no hay API/frontend escuchando y el perfil específico de ensayo local exige el proveedor falso. Falta una preparación acotada de runtime para Sandbox externo; no es necesario rehacer el flujo KYC ni completar todo R2/R3 para este ensayo limitado. No se han creado sesiones externas ni cuentas nuevas en esta auditoría.

## Evidencia actual

- Rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8`, árbol dirty preservado.
- `systemctl --user is-active espacigo-didit-tunnel.service`: active.
- HTTPS `didit.testingpage.store/api/v1/verifications/didit/webhook`:502; listeners8080/18090/4201/4219 ausentes. No receptor funcional todavía.
- Ingress fijo del recibo `evidencia_webhook_fijo_cloudflare_20261110.md`: solo webhook hacia8080, resto404. No cambiar Didit otra vez para resolver un puerto local: alinear el origen remoto o usar un relay local restringido hacia el puerto de API autorizado, después de definir el perfil.
- Archivo histórico de credenciales existente en el worktree `espaciGo-kyc-didit`, `.local/testing/didit_sandbox.env`: regular propio0600. Se comprobó únicamente presencia de nombres con valor no vacío: API_KEY, WEBHOOK_SECRET y WORKFLOW_ID presentes; APPLICATION_ID/ENVIRONMENT/WORKFLOW_VERSION ausentes en ese archivo. No se imprimieron valores, no se copiaron ni rotaron secretos, ni se probó autenticación. El mismo archivo no existe en el worktree activo main-sync: no lanzar un `source` de ruta relativa suponiendo que son el mismo checkout.

## Código versus configuración versus ejecución

| Frontera | Estado y evidencia | Trabajo mínimo pendiente |
|---|---|---|
| Transporte público | Túnel fijo activo; webhook ya editado según usuario | Backend real detrás del origen8080; comprobación de firma y recepción después |
| Perfil aislado | `cmd/api/worker_profile.go:36–58`: local-kyc-worker-free fija18090 y credenciales/proveedor falso | Perfil separado o extensión explícita opt-in para Sandbox externo, sin aflojar el perfil falso ni usar standard con workers |
| Cifrado/HMAC | El perfil aún exige secreto HMAC legacy; runtime de declaración ya tiene keyring | Perfil nuevo debe validar el contrato privado vigente y snapshot compartido; no inventar secretos ni rotar los existentes |
| Cliente API | `internal/verification/didit.go:24–25` conserva host sandbox separado; `identity_declaration_runtime.go:115–117` acepta overrideDIDIT_BASE_URL | Host de servicio verificado por documentación y preflight; DNS parental: verification.didit.me resuelve, verification-sandbox.didit.me no. Configurar el host válido explícitamente o corregir default con prueba propia; no diagnosticar esto como clave incorrecta |
| Binding | `identity_declaration_runtime.go:119–125` usa app/version defaults cuando faltan | Configurar explícitamente app806fe3f4-4c9b-4567-bf6f-ce8c3e688fce, SANDBOX y workflow/version correctos; todavía falta contrastar el workflow actual, sin confundir appID con workflowID |
| Registro/correo | Código de mailer local/SMTP en cmd/api/main.go:236–251 | Definir correo de ensayo y lectura del enlace por el usuario; no simular envío real ni verificar una cuenta por SQL |
| Frontend/retorno | E2E local histórico PASS con HTTPS4219 y proveedor falso | Levantar build actual, TLS confiable y frontend origin coherente; callback del navegador distinto del webhook público |
| Resultado | Guards Sandbox≠Live existentes | Mostrar/observar decisión de prueba sin otorgar elegibilidad real ni desactivar guard para obtener verde |
| Verificación end-to-end | Solo evidencia sintética previa | Probar receptor firmado, después recorrido manual: registro/correo→guardar/GET→inicio explícito→Didit→webhook→GET autoritativo→UI/readback |

## Lo que ya no es bloqueo

- No falta crear otro workflow REST para cambiar el webhook.
- No falta un hostname público fijo ni un nuevo secreto webhook.
- No se considera la retención de correlación incierta pendiente de decisión: `decision_privacidad_sandbox_20261110.md` ya la fija; falta implementación R2.
- V94 aplicada y probada no demuestra ni impide por sí sola la conectividad externa.

## Preparación propuesta en orden

1. Implementar y probar el perfil separado Sandbox externo worker-free y su política de carga de credenciales; conservar target nativo, clave privada/cifrado y reglas existentes.
2. Preparar launcher con entorno allowlist, sin secretos de API/DB en Next. Leer las credenciales existentes privadamente desde su ubicación autorizada; no moverlas/rotarlas por inferencia.
3. Definir app/workflow/version, baseURL, frontend/callback y correo de ensayo explícitos.
4. Alinear túnel con el listener real, conservando la URL pública y catch-all404. Levantar únicamente recursos propios, sin consumidores generales de la DB.
5. Preflight: readiness, auth/registro habilitado, webhook sin firma denegado, ninguna operación inválida modifica estado. Ningún receptor ficticio para sustituir la API.
6. Entregar al usuario URL local y pasos manuales; el usuario opera navegador y credenciales. Un nuevo registro no permite saltarse el inventario/política de datos del ensayo. Documento/cámara reales son una autorización de datos distinta de probar con material sintético.
7. Confirmar recepción/decisión/readback sin registrar PII, y documentar el resultado real. Si hay discrepancia con el contrato V3 o workflow, corregir el camino preciso, no fabricar aprobación.

## Pendientes globales que no se deben mezclar con habilitar la prueba

### Adjudicación de revisión independiente recibida

Revisión `deleg_e65b33b4` source-only recibida; padre confirmó directamente dos omisiones adicionales del reporte inicial:

- **Correo es un bloqueo de código del perfil dedicado, no solo configuración SMTP.** `cmd/api/main.go:798–801` construye auth con `zohoauth.UnavailableMailer{}` en `runLocalKYCOnly`. Por eso la existencia de SMTP en el arranque general no habilita registro→correo en este perfil. El perfil manual necesita una entrega/captura local explícita y probada; no usar standard ni verificar usuarios mediante SQL para esquivarlo.
- **Retorno no está cableado en el inicio durable.** `didit_service.go:624–636` llama CreateSession sin poblar el campo opcional Callback. Si el workflow configura un retorno externo, no se ha comprobado que vuelva al frontend local. Para garantizar el recorrido solicitado, fijar callback permitido en el perfil/servicio o verificar y documentar el retorno ya configurado en el workflow; no sustituirlo por la URL webhook, que no sirve páginas de UI.

La recomendación mínima es un perfil separado `local-kyc-didit-sandbox-manual` (nombre propuesto, todavía inexistente) con ruta durable ya montada, credenciales privadas existentes, host API permitido, binding exacto de aplicación/workflow, retorno frontend y entrega de correo acotada. No relajar los guards del fake, no usar el fallback legacy de handler que persiste en memoria, no habilitar workers ni conceder badgeLive por resultadoSandbox. Las pruebas V3/firma unitarias existentes no acreditan el shape exacto de una respuesta o webhook real del workflow seleccionado.

La revisión independiente no ejecutó tests ni conexiones. Este addendum es auditoría; no implementa el perfil/callback/mailer ni inicia una sesión externa. Rectifica expresamente la descripción anterior de correo como mera configuración.

- R2: retiro de metadatos desacoplados con auditoría de incertidumbre y preservación de aprobación vigente.
- R3: worker/login/scheduler/backlog/alertas/SLA operacional, incluido cableado de retiroACK.
- R4/KYC-14: proveedor externo probado, validación humana y entrega/publicación/despliegue separados.
- Privacidad Sandbox observada:1año/Automático/Eliminar con la sesión. Objetivo30d/Siempre/Eliminar con la sesión no aplicado por el agente. No afirmar promesa de30d del proveedor con el valor actual1año.
