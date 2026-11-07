# ADR-001 — Contrato de identidad chilena con Didit

- **Estado:** propuesta para ratificación en KYC-01; no implica integración implementada.
- **Fecha:** 2026-11-07.
- **Ámbito:** MVP de verificación de identidad de persona natural en Chile.
- **Contexto:** `plan_kyc_didit.md`, §§2–4, 6, 9 y 10.

## Decisión

El documento admitido por el MVP chileno será la **Cédula de Identidad chilena**, tipo Didit `ID`, con captura de **ambos lados**. La selección de `ID` limita el contrato de producto aunque el workflow permita otros documentos. RUN/RUT identifica a la persona; no debe confundirse con el número de serie de la cédula.

Esta decisión documenta el contrato objetivo. No afirma que EspaciGo ya cree sesiones, valide identidades ni esté autorizado para procesar datos reales.

## Contrato de `expected_details`

Go construirá `expected_details` desde una declaración legal confirmada por el usuario y una instantánea inmutable ligada a su cuenta e intento. El cliente no determina propietario, workflow ni entorno, y la identidad pública (`nombre_visible`) no sustituye estos datos.

| Dato declarado | Campo Didit | Valor/formato esperado |
| --- | --- | --- |
| Nombres legales completos tal como aparecen en la cédula | `first_name` | Todos los nombres en un campo; conservar Unicode, tildes, Ñ, espacios y guiones. |
| Apellidos legales completos tal como aparecen en la cédula | `last_name` | Apellidos en un campo; no imponer dos apellidos ni inferir su separación desde OCR. |
| Fecha de nacimiento | `date_of_birth` | Fecha civil `YYYY-MM-DD`, sin conversión de zona horaria. |
| País emisor del documento | `id_country` | `CHL`. Es país emisor, no una afirmación de nacionalidad. |
| Tipo documental esperado | `expected_document_types` | Lista con el único valor `ID` (`["ID"]`). La etiqueta de respuesta “Identity Card” no reemplaza el código de entrada. |
| RUN/RUT, incluido su dígito verificador | `identification_number` | Identificador completo con DV válido. La entrada puede aceptar formato chileno con puntos/guion; Go normaliza a formato estándar sin puntos y con guion, DV en mayúscula, antes de enviar/comparar. |

No enviar nacionalidad salvo que una necesidad aprobada y el workflow la requieran; nunca inferirla de `CHL`. No enviar el número de serie como RUN/RUT. El campo de respuesta `document_number` no se considerará equivalente a `personal_number` ni a `identification_number` sin evidencia del subtipo. Si Didit no devuelve el RUN completo y su DV en un campo permitido, la verificación no puede aprobarse automáticamente solo por nombre y nacimiento: revisión humana o decisión posterior sobre validación adicional.

Consentimiento debe registrarse con versión/finalidad y marca de tiempo del servidor **antes de crear cualquier sesión**; sin consentimiento válido no se crea sesión ni se llama al proveedor. Una edición posterior inicia una nueva instantánea/intención; no modifica silenciosamente los datos esperados de un intento existente.

## Política de discrepancias y cierre seguro

El workflow Didit comprobado para sandbox tiene `expected_details_mismatch_action=REVIEW`. Esa configuración es una primera barrera del proveedor, no sustituye la validación de EspaciGo ni demuestra por sí sola que el RUN/RUT fue extraído correctamente. Antes de usar otro workflow/versión/entorno debe confirmarse su configuración efectiva.

Go valida estrictamente la decisión recuperada y los atributos extraídos frente a la instantánea. Cualquier warning/discrepancia en nombres (`FULL_NAME_MISMATCH_WITH_PROVIDED`), fecha de nacimiento (`DOB_MISMATCH_WITH_PROVIDED`), país (`COUNTRY_MISMATCH_WITH_PROVIDED`) o número de identificación (`IDENTIFICATION_NUMBER_MISMATCH_WITH_PROVIDED`) **bloquea la aprobación automática**, incluso si el estado superior informa `Approved`. No se aprueba ante campos faltantes, nulos, truncados, ambiguos, tipo/país inesperado, DV inválido, workflow no esperado o resultado parcial/desconocido. Se conserva el original declarado; OCR no lo sobrescribe. El caso queda en revisión o requiere corrección y nueva intención reconciliada. Solo una decisión auditada y completa puede resolver una discrepancia conforme a política explícita; ningún mismatch da insignia verde por defecto.

Para nombres, solo se permitirá comparación normalizada conforme a reglas versionadas (Unicode, mayúsculas, espacios y tolerancia acotada a tildes/transliteración); coincidencia aproximada no autoriza aprobación genérica. Fecha civil y RUN/RUT normalizado con DV se comparan exactamente. No exponer mensajes crudos del proveedor, identidad ajena ni señales antifraude a la UI/logs.

## Privacidad, consentimiento y retención

- No poner PII —incluidos nombres, fecha de nacimiento o RUN/RUT— en URL de navegador, query/hash, `vendor_data`, analítica, telemetría, logs, errores o capturas. `vendor_data` será una referencia opaca estable derivada por Go. La URL temporal de sesión Didit tampoco se registra ni persiste en storage web.
- El formulario y su snapshot permanecen bajo control del servidor y se minimizan al fin de cotejo/auditoría. Next no conserva RUN/RUT ni nacimiento en `localStorage`, `sessionStorage` o URLs; purga borradores al salir, cambiar de cuenta/permisos o cerrar sesión.
- Registrar consentimiento afirmativo, versionado y timestamp de servidor antes de crear sesión; no preseleccionar ni confundirlo con consentimiento de marketing. Explicar finalidad, proveedor, captura de documento/biometría, conservación y vía de revisión antes de abandonar EspaciGo.
- EspaciGo no almacena imágenes, video ni plantillas faciales en PostgreSQL. Retención, exportación y supresión de declaraciones, eventos y resultados mínimos requieren política aprobada y revisión legal antes de datos reales/live; no se presume un plazo no ratificado.
- Aislar las pruebas sintéticas del entorno y datos reales. El sandbox puede almacenar medios capturados aunque simule resultados: usar únicamente documentos y medios sintéticos permitidos, nunca IDs/selfies reales ni cámara personal en automatización. Sandbox no confiere elegibilidad ni insignia real.

## Consecuencias y límites

El contrato habilita el diseño de persistencia/API de KYC-02 una vez ratificado; no autoriza todavía cambios Go/SQL, sesiones Didit, pruebas con personas/datos reales, publicación de workflow, producción ni alteración de gates de reserva. Quedan pendientes evidencia sintética permitida que confirme campo/subtipo RUN+DV, ratificación de edad/política de nombres/retención y la decisión de producto sobre reservas, conforme a `plan_kyc_didit.md` §§9 y 11.
