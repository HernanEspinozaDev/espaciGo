# Consentimiento para guardar identidad legal — versión candidata

- **Versión técnica:** `id-consent-20261108` (19 caracteres; dentro del máximo de 20 del contrato actual).
- **Fecha de aprobación:** 2026-11-08.
- **Estado:** **APROBADO como texto definitivo por el Responsable del Proyecto (gate legal/privacidad cerrado para MVP/Académico).**
- **Regla de elegibilidad aprobada:** estrictamente mayores de 18 años cumplidos; si no cumple, se rechaza de inmediato sin persistir ni llamar a Didit.

## Texto exacto propuesto para la casilla no premarcada

> Autorizo a EspaciGo a recopilar y tratar mi nombre legal, fecha de nacimiento y RUN/RUT para asociar mi declaración de identidad a mi cuenta y preparar una verificación de identidad que yo solicite. Esta declaración se almacenará cifrada hasta por 30 días desde que la guarde; vencido ese plazo, se eliminará dentro de las 24 horas siguientes. Guardarla no inicia una verificación biométrica ni envía mis datos a Didit u otro proveedor. Si elijo una verificación externa, antes recibiré información separada sobre el proveedor, los datos que se compartirán, su finalidad y conservación, y se me pedirá una autorización independiente. Para trazabilidad de seguridad, EspaciGo conservará por cinco años un registro mínimo del guardado (cuenta, referencia interna, fecha y correlación), sin mi nombre legal, RUN/RUT, fecha de nacimiento ni contenido cifrado. Puedo consultar la Política de Privacidad y ejercer los derechos que correspondan por los canales que allí se informan.

## Regla operativa propuesta

1. El plazo de 30 días calendario empieza en `consent_given_at` asignado por el servidor al confirmar el guardado. Cada nueva revisión obtiene su propio plazo; no prolonga automáticamente revisiones anteriores.
2. Desde `retention_at`, el servicio deja de devolver la declaración. Un mecanismo de retención debe borrar el ciphertext vencido en un máximo de 24 horas; la denegación de lectura sola no cuenta como eliminación.
3. El registro mínimo de auditoría usa el plazo fijo existente de cinco años, aceptado por el propietario del producto para este evento. No incluye campos de identidad, ciphertext ni versión del consentimiento; los identificadores de cuenta/recurso y la correlación siguen siendo metadatos vinculables.
4. Guardar la declaración no crea un intento ni transmite datos al proveedor. Un futuro inicio de verificación requerirá una acción y consentimiento separados; antes de implementarlo habrá que coordinar sus referencias/FK y retención con esta regla. El aviso KYC-10 candidato está en `aviso_y_consentimiento_didit_v1_20261108.md` y sigue pendiente de revisión.
5. La casilla comienza desmarcada y el backend registra versión y fecha del servidor solo tras una acción afirmativa. Rechazar el consentimiento impide guardar, pero no debe bloquear funciones ajenas a la verificación.

## Base y límite

Al 2026-11-08, la versión de la Ley 19.628 vigente hasta el 2026-11-30 (BCN, versión 2023-05-09) dispone en su artículo 4 autorización expresa, información del propósito y constancia escrita. La Ley 21.719 fue publicada el 2024-12-13 y entra en vigencia el 2026-12-01; incluye consentimiento libre, informado, específico, previo e inequívoco, revocable, además de finalidad, proporcionalidad, seguridad y supresión al terminar el plazo necesario. Este borrador no determina por sí mismo la base jurídica, la necesidad/proporcionalidad de cada dato ni el cumplimiento normativo; requiere revisión jurídica y de privacidad antes de activarse, con nueva revisión antes del 2026-12-01. Referencias oficiales: BCN, Ley 19.628, art. 4 (versión 2023-05-09); BCN, Ley 21.719, artículo primero (reformas a la Ley 19.628) y disposición transitoria primera (vigencia 2026-12-01).
