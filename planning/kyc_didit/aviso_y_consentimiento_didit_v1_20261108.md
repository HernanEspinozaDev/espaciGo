# Aviso y consentimiento separado para verificación con Didit — v1 candidata

- **Fecha:** 2026-11-08.
- **Versión técnica:** `didit-consent-v1` (16 caracteres; <= 20).
- **Estado:** **APROBADO como texto definitivo por el Responsable del Proyecto (2026-11-08) para MVP/Académico.**
- **Dependencia:** alineado con las decisiones aprobadas en `propuesta_retencion_kyc09_kyc10_20261108.md` y `revision_legal_privacidad_20261108.md`.
- **Parámetros definidos para MVP/Académico:** Responsable `EspaciGo (Proyecto MVP/Académico)`, canal de derechos en sección de privacidad de `/mi-cuenta`, documento admitido exclusivamente Cédula de Identidad chilena vigente (`ID`, `CHL`), para mayores de 18 años.[unverified]

## Texto informativo propuesto antes de la acción

### Verificación de identidad con Didit

Guardar tus datos de identidad en EspaciGo no inicia esta verificación ni envía datos a Didit. Para continuar, iniciaremos una acción separada y te redirigiremos al flujo alojado de Didit.

Con tu autorización, EspaciGo enviará a Didit tu nombre legal, fecha de nacimiento, RUN/RUT normalizado, país emisor y tipo de documento, además de una referencia técnica de la sesión. Didit usará esos datos para contrastarlos con el documento admitido por el flujo. El workflow seleccionado debe limitarse al tipo de documento ofrecido y mostrar aquí las capturas efectivamente requeridas, incluida selfie/video de liveness y cotejo facial si están activos.[1]

**EspaciGo no conservará copias de las fotos ni videos de identidad.** Didit procesa las capturas durante la comprobación. Cuando el backend de EspaciGo confirme la verificación desde una fuente autoritativa, la propuesta es solicitar inmediatamente a Didit el borrado de la sesión y eliminar la declaración/snapshot cifrado local. Para cumplir la regla de un RUT por cuenta podría conservarse, si se aprueba, un identificador protegido derivado del RUT para detectar duplicados; su finalidad y plazo deben informarse. Didit puede devolver datos multimedia en respuestas/webhooks según las salidas activas, por lo que esta promesa solo se podrá publicar tras probar que EspaciGo los descarta sin persistirlos ni registrarlos.[1][2]

La declaración cifrada de EspaciGo se conservaría solo mientras sea necesaria para comprobar la identidad y se eliminaría después de la confirmación autoritativa de `VERIFIED`; si la verificación queda pendiente, el máximo local propuesto es **30 días desde el guardado**, con purga dentro de las **24 horas siguientes** al vencimiento. Para las capturas en Didit, la regla propuesta es conservarlas solo mientras la verificación esté pendiente y solicitar borrado al confirmarse `VERIFIED`; el plazo automático de respaldo debe ser finito y completarse tras leer la configuración real. Los documentos públicos de Didit indican que, si no se configura retención por aplicación, el valor puede quedar ilimitado.[3][4]

Didit puede mantener registros de auditoría con un plazo separado.[3][4]

Si se habilitan plantillas biométricas, requieren una evaluación y regla de eliminación separadas.[4][5]

El entrenamiento de modelos y las funciones de fraude intercliente también son usos distintos; el control de entrenamiento no cambia la retención.[4][6]

Consulta la Política de Privacidad de `[razón social responsable]`, el aviso de privacidad de Didit y cómo ejercer tus derechos en `[canal de privacidad/derechos]`. Si no autorizas este flujo, no iniciaremos la sesión; podrás seguir usando los servicios de EspaciGo que no requieran esta verificación.

## Acciones afirmativas propuestas

Ambas casillas parten desmarcadas. La primera no se infiere del consentimiento de KYC-09 ni de haber guardado la declaración.

- [ ] **Autorizo a `[razón social responsable]` a enviar a Didit los datos de identidad indicados arriba para realizar la verificación que solicito y recibir el resultado necesario para informarme el estado.** Entiendo que EspaciGo y Didit tienen avisos y plazos de retención diferenciados.
- [ ] **Autorizo el tratamiento de las capturas necesarias para la verificación biométrica descrita**, únicamente mediante el flujo Didit y con los plazos/controles que se informan arriba.

**Acciones:** `Cancelar` · `Iniciar verificación con Didit`

La acción «Iniciar verificación» solo se habilita después de las autorizaciones requeridas y una lectura servidor-autoritativa de la declaración guardada. El backend debe registrar versión exacta del aviso, marcas afirmativas, fecha del servidor y referencia de la declaración; no crear una sesión ante una autorización ausente o vencida.

## Nota interna: revisión read-only del workflow

- La inspección está documentada en `evidencia_workflow_didit_20261108.md`. Se encontró `Compliance workflow` v1 publicado en `Hotusoft (Sandbox)`, no editable. El `DIDIT_WORKFLOW_ID` del runtime no fue leído; no está confirmado que este sea el workflow usado por EspaciGo.
- El grafo del workflow sandbox permite OCR → LIVENESS (`ACTIVE_3D`) → FACE_MATCH y `expected_details_mismatch_action=REVIEW`. Para CHL permite pasaporte, licencia, cédula (`ID`) y residencia (`RP`), no solo cédula. Alinear documento permitido con el alcance de producto antes de publicar este aviso.
- `response_attributes=null` indica devolución de todos los puntos de datos de las features; puede incluir datos de documento y, con selfie/liveness, imagen y video. El aviso no debe afirmar «EspaciGo no conserva fotos» hasta verificar con pruebas que no se almacenan en ningún proceso/salida. Los flags completos de LIVENESS/FACE_MATCH contradicen los nodos del grafo y requieren aclaración.
- El workflow expira la sesión en 604800 segundos (7 días), con 3 reintentos dentro de 7 días. Eso no prueba eliminación de medios ni es el plazo de retención de la aplicación Didit.
- El requisito nuevo de un RUT único por cuenta puede necesitar que EspaciGo conserve un fingerprint HMAC después de borrar el snapshot verificado. No afirmar que después de la verificación solo queda el estado hasta que legal/privacidad apruebe finalidad y plazo de ese identificador residual y se refleje en el texto.

## Condiciones para quitar placeholders o mostrar este aviso

1. Confirmar con asesoría si la autorización debe ser única o granular, qué base/finalidad corresponde y qué información específica exige el tratamiento de capturas/biometría. Este borrador no afirma suficiencia jurídica.
2. Leer en modo de solo lectura la aplicación/runtime seleccionado y la versión exacta: retención por aplicación, plantilla biométrica, entrenamiento, fraude intercliente, entidad contratante y región. La lectura actual solo confirmó el workflow sandbox; no usarlo como evidencia de producción.
3. El workflow sandbox publicado actualmente tiene `response_attributes=null`, que devuelve todos los datos de las features. Didit indica que apagar salidas puede evitar tanto su devolución como su almacenamiento; el modo elegido debe resolver explícitamente el requisito «Didit retiene mientras se verifica; EspaciGo no guarda copias».[1][2]
4. Probar que el cuerpo del webhook y toda respuesta que pueda contener medios no se escriben en DB, inbox, log, telemetría, captura ni backup; guardar únicamente campos allowlist del resultado. Las pruebas deben incluir un payload sintético con datos de imagen y comprobar ausencia en todas las salidas persistentes.
5. Configurar una retención finita antes de la primera sesión: la política pública indica retención ilimitada si el cliente no fija un período y configuración por aplicación de 30 días a 10 años.[4] Eliminar una sesión puede dejar llegar webhooks que ya estaban encolados y no elimina por sí solo registros de auditoría del proveedor.[3]
6. Definir la solicitud y evidencia de borrado remoto, incluyendo cualquier plantilla biométrica conservada; la política del proveedor documenta un procedimiento de borrado que cubre plantillas retenidas.[3][5]
7. Confirmar contrato/DPA, entidad que presta el servicio, región y subencargados; la política pública describe funciones de Didit como encargado/proveedor y otros tratamientos propios separados.[4]
8. Verificar el control de entrenamiento por separado: Didit informa que el entrenamiento está habilitado por defecto y que apagarlo no cambia la retención.[6]
9. Resolver y divulgar la conservación temporal o posterior del identificador de unicidad de RUT; el borrado del RUN/RUT en claro no equivale a borrar su fingerprint vinculable.

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
