# Diagnóstico de campos simulados Sandbox

## Evidencia y límites

El usuario aportó texto de la consola de una nueva sesión Sandbox. La propia consola dice datos simulados y presenta tipo Passport, fecha/identificador extraídos distintos de expected_details y un nombre con carácter acentuado sustituido. No copiar aquí los valores personales, IP, identificadores ni contenido de documentos del adjunto.

Documentación oficial consultada: https://help.didit.me/getting-started/sandbox-and-testing . Explica que OCR/nombres/fechas/números/tipo son simulados, independientemente de la imagen, y que las capturas sí se almacenan. Por tanto este ensayo no demuestra un fallo OCR real, corrupción de la fecha enviada ni lectura real de una cédula chilena. No pasar a Live ni recopilar más medios reales por inferencia.

## Verificación del código ejecutada

- StartOrResume obtiene first_name,last_name,DOB,RUT de la revisión guardada y construye expected_details con paísCHL y tipoID. No usa el serial del documento como RUT.
- Nueva prueba sintética `TestDiditExpectedDetailsPreservesUTF8AndNationalIdentifier` ejecuta HTTPDiditClient.CreateSession con servidor local, decodifica el JSON y comprueba nombre compuesto acentuado/apellidos/fecha/RUT byte-equivalentes. PASS con race. La prueba existente del servicio desde declaración guardada también PASS. Esta es caracterización de transmisión, no prueba OCR ni preservación dentro de Didit.
- DTO DiditIDVerificationNode contempla identification_number y personal_number; no define document_number/MRZ/medios/serial. Snapshot cifrado de decisión conserva solo estado/booleans/flags/warnings/tiempo, no número de documento. No añadir serial a modelo persistido para resolver una discrepancia de Sandbox.
- La integración debe conservar distinción RUN/RUT frente al serial y exigir nacional identifier real correctamente mapeado; un pasaporte simulado sin personal_number no demuestra soporte de RUN chileno en Live. No cambiar comparador para equiparar el serial al RUN ni eliminar controles DOB/identidad para hacer pasar el ensayo.

## Próxima frontera

Prueba controlada con datos/escenario oficiales sintéticos y regresión de extracción de personal_number/RUN para CHL ID. Una validación OCR real de cédula chilena, incluidas tildes y DOB, requiere evidencia separada y autorización específica Live/biometría; no está cubierta por esta sesión Sandbox. Este diagnóstico no crea/edita sesiones remotas, no aplica migraciones, no reinicia servicios y no cambia formulario ni controles de aprobación.
