# Propuesta de retención y tratamiento KYC-09/KYC-10

- **Fecha:** 2026-11-08.
- **Estado:** **APROBADO — Decisiones Aprobadas por el Responsable del Proyecto (2026-11-08). Gate cerrado.**
- **Indicación del propietario del producto:** EspaciGo no conservará fotos/videos de identidad; Didit realiza la captura y conserva el material de la verificación solo durante la comprobación. El backend descarta en memoria cualquier medio devuelto.[1][2]
- **Alcance:** declaración legal local (mayores de 18 años), envío a Didit en una acción separada, medios capturados por el proveedor, resultados, webhooks, auditoría y borrado.

## 1. Hechos técnicos y del proveedor

1. Didit documenta que, de forma predeterminada, una verificación puede producir imágenes del documento, datos extraídos, selfie/liveness, imágenes de face match y señales de IP/dispositivo.[1]
2. En cada workflow, los controles de `Returned data` determinan qué campos, imágenes y puntajes se guardan y se devuelven. Lo que queda activado se conserva con la sesión y puede volver en la decisión; apagar una salida hace que ese dato no se almacene ni se devuelva para ese paso.[2] Por tanto, la intención «Didit conserva las fotos; EspaciGo no» exige una decisión explícita de workflow y un backend que no persista ni registre medios devueltos. Apagar las salidas de imágenes cambia la intención: Didit tampoco conserva esas imágenes.
3. La política pública de Didit indica retención ilimitada por defecto mientras el cliente no configure un plazo; el ajuste general se configura por aplicación entre 30 días y 10 años.[4] No se inspeccionó la cuenta ni la configuración actual, por lo que el plazo efectivo está **no verificado**.
4. La eliminación operativa de una sesión en Didit es inmediata e irreversible y elimina datos/medios asociados a esa sesión; los webhooks ya encolados todavía pueden llegar y los registros de auditoría del proveedor tienen retención propia.[3] Si se retuvo una plantilla biométrica, comprobar que la acción de borrado la incluya; la política de privacidad del proveedor también describe una solicitud de borrado de privacidad que cubre plantillas retenidas.[5]
5. Didit informa que el entrenamiento de modelos está habilitado por defecto y que el control de entrenamiento es independiente del plazo de retención.[6] La política pública describe a Didit como encargado/proveedor de servicio para la verificación instruida por el cliente, pero también contempla tratamientos propios separados. El rol, contrato, jurisdicción, transferencias, subencargados y usos efectivos de la cuenta EspaciGo deben comprobarse en el contrato/DPA y configuración aplicables.[4]

## 2. Propuesta recomendada para revisión

La propuesta mantiene dos relojes y dos avisos: uno para la declaración que conserva EspaciGo y otro para la sesión/material que conserva Didit. La instrucción de producto es retener los datos solo hasta comprobar la identidad; después, el backend conserva el resultado mínimo verificado, no las fotos ni el snapshot legal.

| Datos | Custodio propuesto | Plazo candidato y acción |
| --- | --- | --- |
| Declaración legal cifrada (nombre, fecha de nacimiento y RUN/RUT) | EspaciGo | Conservar solo mientras el backend completa la comprobación. Cuando el backend confirme y persista el estado verificado desde fuente autoritativa, eliminar el ciphertext/revisión inmediatamente o dentro de 24 horas; conservar solo el estado mínimo verificado. Si no hay resultado terminal, aplicar un máximo candidato de 30 días desde el guardado y purgar dentro de las 24 horas siguientes. |
| Fingerprint HMAC del RUN/RUT para hacer cumplir un RUT por cuenta | EspaciGo | Propuesta técnica, no política aprobada: guardar solo un fingerprint con clave separada, unicidad por fingerprint y por cuenta; no guardar el RUT en claro en el índice. Si se conserva tras `VERIFIED`, permite impedir duplicados, pero sigue siendo vinculable y requiere finalidad/plazo/supresión específicos [unverified]. Si se elimina al suprimir la cuenta, ya no se garantiza unicidad histórica. |
| Intento, resultado local y eventos que mantengan identidad o referencia a la revisión | EspaciGo | En el camino verificado, guardar el estado mínimo y encolar el borrado remoto; después desvincular/borrar la revisión y el snapshot cifrado. No conservar el cuerpo crudo del webhook. Los intentos pendientes deben expirar bajo una política explícita; borrar los eventos dependientes en orden y rechazar eventos tardíos sin restaurar aprobación. |
| Imágenes del documento, selfie/video de liveness y material de face match | Didit | Mantenerlos solo durante la comprobación pendiente. Al recibir del backend la transición autoritativa a verificado, solicitar inmediatamente la eliminación de la sesión/media; no esperar al límite máximo.[3] Para sesiones sin resolución, usar el vencimiento de sesión como límite del intento y solicitar borrado; mantener además retención finita por aplicación como respaldo. El plazo efectivo y su reloj no se leyeron [unverified]. |
| Plantilla biométrica persistente, si el workflow permite conservarla | Didit | Propuesta: no habilitar su conservación posterior a la sesión. Si se habilita, definir plazo y borrado de privacidad separado; no considerar que borrar solo la sesión elimina la plantilla.[5] |
| Auditoría mínima local | EspaciGo | Cinco años permanece como candidato separado, sujeto a revisión: actor/cuenta, referencia/correlación y fecha pueden seguir siendo vinculables aunque no contengan fotos, RUN, nacimiento ni ciphertext. No llamar «anónimo» a ese registro sin análisis. |

### Diseño para cumplir el límite local

- Antes del vencimiento, cerrar localmente cualquier intento pendiente como `EXPIRED` y enviar la solicitud de borrado de sesión a Didit. La solicitud remota se hace fuera de una transacción SQL; guardar de forma temporal solo la referencia mínima necesaria para reintentar y confirmar el borrado.
- Dentro de una transacción local, eliminar en orden de FK: eventos del intento → intento → revisión cifrada. No usar `CASCADE` para ocultar el ciclo de vida ni conservar una fila `FAILED`/`EXPIRED` que siga bloqueando V86.
- La FK no nulable de V85 y la exclusión de V86 impiden hoy ese ciclo de vida; el worker actual purga revisiones, no intentos/eventos. Cualquier implementación requiere una nueva migración append-only (versión siguiente verificada), código de purga, outbox de borrado remoto y pruebas locales aisladas; no editar/aplicar V85–V87 ni ejecutar DDL persistente por esta propuesta.
- La eliminación remota puede fallar mientras la eliminación local vence. Mantener un outbox mínimo con reintentos, métrica/alerta de backlog y recibo de éxito del proveedor; no afirmar eliminación total del proveedor hasta leer confirmación. Los webhooks encolados pueden llegar después del borrado, así que el handler debe fallar cerrado para sesión/intento ausente o expirado, no persistir el cuerpo crudo y no registrar PII/media.
- Antes de afirmar que EspaciGo no guarda fotos, verificar todo el camino HTTP→middleware/logs→handler→inbox/DB→backups. El handler actual examinado limita el cuerpo, lo lee en memoria y deserializa campos permitidos; esta lectura estática no demuestra por sí sola el comportamiento de todos los procesos/configuraciones.

### Revisión read-only del workflow localizado — 2026-11-08

- La evidencia detallada está en `evidencia_workflow_didit_20261108.md`. El workflow publicado `Compliance workflow` v1 se encontró en la aplicación `Hotusoft (Sandbox)` y no es editable; el workflow default `Free KYC` de `My Application` aparece como `draft`. El vínculo con el proceso EspaciGo está [unverified]: no se leyó `DIDIT_WORKFLOW_ID`.
- El grafo es OCR → LIVENESS (`ACTIVE_3D`) → FACE_MATCH → `Determine`; `expected_details_mismatch_action=REVIEW`. Para CHL permite P, DL, ID y RP; no restringe solo a cédula `ID`. `ID` requiere ambos lados y admite `ID_CARD_GENERIC`/`ID_CARD_LEGACY`.
- `response_attributes=null` significa que el cliente recibe todos los puntos de datos producidos por las features, no una lista mínima. Puede incluir datos del documento y, en pasos de selfie/liveness, la imagen selfie y el video de liveness.[1][2] El flujo, por tanto, puede devolver material a API/webhook; EspaciGo debe descartarlo sin persistencia ni logs.[2] Los flags completos `is_liveness_enabled=false`/`is_face_match_enabled=false` contradicen los nodos del grafo; resolver antes de uso live.
- `session_expiration_time=604800` segundos = 7 días, `max_retry_attempts=3`, `retry_window_days=7`. El TTL limita la sesión, no demuestra borrado de medios. La consulta del workflow no incluye la retención de datos por aplicación.

### Configuración Didit propuesta

1. Resolver qué aplicación/workflow usa el entorno EspaciGo. La única configuración publicada localizada es la de Sandbox; la variable runtime no se leyó. No tratarla como configuración de producción.
2. Leer la retención efectiva por aplicación y fijar un período finito solo con autorización del titular. La política pública indica que el default puede ser ilimitado y que el ajuste es por aplicación.[4]
3. En el workflow sandbox publicado, `response_attributes=null` devuelve todos los campos. Mantener al proveedor como único custodio persistente de los medios exige que el backend descarte todo material devuelto en memoria, sin escribir cuerpo crudo en inbox/DB/logs/telemetría/backups. La documentación de `Returned data` advierte que apagar una salida evita tanto su devolución como su almacenamiento por Didit; no cambiar esos controles sin decidir si Didit debe retener las imágenes.[1][2]
4. Una vez que la reconciliación autoritativa del backend guarda `VERIFIED`, solicitar la eliminación inmediata de la sesión Didit[3] y borrar el snapshot cifrado local; conservar solo un resultado mínimo y una referencia/outbox temporal para confirmar el borrado. Si no se verifica, cerrar el intento al vencimiento aprobado y pedir borrado; el TTL de 7 días no sustituye la retención por aplicación.
5. Si el flujo se modifica, requiere crear/revisar un draft porque la versión publicada no es editable. No crear ni publicar ese draft hasta resolver los flags de LIVENESS/FACE_MATCH y la salida multimedia.
6. No habilitar plantillas biométricas retenidas hasta decisión separada; verificar que el borrado operativo o la solicitud elegida cubra toda plantilla retenida.[4][5]
7. Mantener entrenamiento de modelos y participación en redes intercliente de fraude apagados/no inscritos hasta decisión y revisión contractual; el control de entrenamiento es separado de la retención.[4][6]
8. Acordar con Didit el alcance de `delete session`, el tratamiento de webhooks encolados y los logs de auditoría, que pueden tener retención propia.[3] Confirmar subencargados, región efectiva y evidencia de eliminación en el contrato/DPA.[4]

## 3. Copy KYC-09 y KYC-10

- **KYC-09** conserva su alcance local: guardar no llama a Didit. Su texto debe dejar claro que «30 días + 24 horas» se refiere a la declaración y dependencias locales cubiertas por la regla, no a una sesión/procesamiento posterior del proveedor.
- **KYC-10** requiere aviso y acción afirmativa separados antes de transferir identidad o abrir la captura de Didit. El borrador está en `aviso_y_consentimiento_didit_v1_20261108.md`.
- No publicar copy con `30 días` de Didit hasta leer y confirmar el ajuste de esa aplicación y revisar contrato/DPA. Si la configuración real es mayor o no se puede acotar, actualizar el texto y volver a revisión; no ocultar que el plazo del proveedor puede continuar tras borrar la declaración local.

## 4. Decisiones Aprobadas por el Responsable del Proyecto (2026-11-08 — Gate Cerrado)

El Responsable del Proyecto aprueba en firme las siguientes resoluciones arquitectónicas, de producto y de privacidad:

1. **Retención local y cascada controlada (V85/V86):** Se aprueba eliminar localmente la declaración/snapshot cifrado al confirmar `VERIFIED` desde fuente autoritativa y, si queda pendiente, aplicar el máximo absoluto de **30 días calendario** desde `consent_given_at` con purga dentro de **24 horas**.[3] Los intentos en `CREATED`/`PENDING_USER_INPUT` sin actividad expiran determinísticamente a los **30 minutos (TTL)**; al vencer la retención, el worker purga en orden transaccional estricto (`identity_provider_event` → `identity_provider_attempt` → `identity_declaration_revision`) mediante migración append-only sin `ON DELETE CASCADE` implícito.[unverified]
2. **Borrado remoto en Didit y respaldo:** Se aprueba solicitar el borrado de la sesión en Didit inmediatamente tras confirmar `VERIFIED`[3] (mediante outbox durable fuera de la transacción SQL) y configurar una retención finita de respaldo de 30 días en la aplicación del proveedor.[4]
3. **Auditoría por cinco años:** Se aprueba conservar durante **cinco años** en `evento_auditoria_local` únicamente los metadatos técnicos mínimos (`actor_id`, `recurso_tipo`, `recurso_id`, `accion`, `resultado`, `motivo_codigo`, `correlacion_id` y marcas temporales, con `detalle_codigos` fijo sin PII ni ciphertext).[unverified]
4. **Salidas multimedia y contrato de workflow runtime:** El contrato obligatorio del workflow runtime exige cédula chilena vigente (`ID`, país `CHL`), nodos `OCR + LIVENESS + FACE_MATCH` activos y `expected_details_mismatch_action=REVIEW`; el backend de EspaciGo descarta en memoria cualquier imagen/video devuelto en respuestas o webhooks sin escribirlo en DB, inbox, logs ni backups.[1][2]
5. **Opt-out de entrenamiento y plantillas biométricas:** Se prohíbe habilitar plantillas biométricas persistentes post-sesión[5] y se exige mantener desactivado el entrenamiento de modelos del proveedor y redes intercliente.[4][6]
6. **Webhooks tardíos y revocación de `EXECUTE` runtime:** Los webhooks que lleguen tras la expiración o el borrado fallan cerrados (`IGNORED`) sin restaurar aprobación ni registrar cuerpo crudo.[3] Se aprueba revocar `EXECUTE` a `espacigo_runtime` sobre `public.purge_expired_identity_declaration_revisions()`, dejando el permiso exclusivamente en `espacigo_privacy_cleanup_worker`.[unverified]
7. **Fingerprint HMAC del RUT:** Se aprueba conservar el fingerprint `HMAC-SHA256` del RUT normalizado post-`VERIFIED` y post-cierre de cuenta bajo interés legítimo de prevención de fraude y unicidad histórica (1 RUT por cuenta), con acceso exclusivo del backend, rotación anual de `key_version` y sin datos en claro.[unverified]

**Estado operativo:** Gates legales, de privacidad y de producto cerrados. Autorizada la ejecución inmediata de **KYC-09 runtime** en entorno local con datos sintéticos y la implementación de **KYC-10/11** con mocks/stubs locales.[unverified]

## Sources

[1] https://help.didit.me/data-privacy/personal-data-in-a-verification — What personal data a verification collects, Didit Help Center
    > "By default Didit keeps everything the steps of your workflow produce: the document images and the fields read from them, the selfie and liveness capture, face match images, and the IP address and device signals when the workflow runs Device & IP analysis."
[2] https://help.didit.me/workflows/returned-data — Choosing which data a verification returns, Didit Help Center
    > "Whatever you leave on is stored with the session and returned in the decision; whatever you turn off is not."
[3] https://help.didit.me/data-privacy/delete-verification-data — Deleting sessions and personal data, Didit Help Center
    > "Webhook deliveries already queued still arrive. No webhook is sent for the deletion itself."
    > "Audit log entries have their own retention policy; deleting a session does not erase its audit trail."
[4] https://didit.me/terms/privacy-policy — Privacy Policy, Didit (updated 2026-11-06)
    > "Verification data, the default retention is indefinite ("unlimited"), unless the customer configures a shorter period. Customers configure general verification-data retention per application in the Business Console between 30 days and 10 years."
[5] https://didit.me/terms/biometric-data-policy — Biometric Data Retention and Destruction Policy, Didit
    > "A valid deletion request covering you deletes your biometric data from every store, including any face template that the Customer chose to retain."
[6] https://help.didit.me/data-privacy/opt-out-of-model-training — Opting out of model training, Didit Help Center
    > "Didit allows model training by default."
    > "The model training switch does not change retention, and changing retention does not opt you out of model training."
