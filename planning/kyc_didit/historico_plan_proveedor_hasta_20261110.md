# KYC Didit — confirmación de identidad de usuarios

## 1. Encargo, base y límites

- Fecha de actualización operativa: 2026-11-08 (fecha local del equipo). Estado: **implementación parcial; aceptación integral nativa pendiente**. §25 prevalece sobre las declaraciones históricas de completitud y restricciones sustituidas de §§9–24.
- Encargo actual: contrastar plan/código/evidencia y ejecutar por tareas las fases locales pendientes. El usuario autoriza datos sintéticos y modificaciones necesarias de datos/esquema en la base nativa indicada en §25, sin bases efímeras. No autoriza publicación, main, despliegue, Cloud SQL ni sesiones externas. Preservar los cambios locales preexistentes.
- Checkout revisado: `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD y ref local `origin/main` `274f37e483a078890e92382a159fa49fd23c2fe8`. El ref local no es una nueva consulta al remoto ni prueba del despliegue. Las siguientes referencias de KYC-00 son históricas.
- Rama local: `codex/marketplace/feature/kyc-00-didit-identidad`.
- Worktree: `/home/nandev/Documentos/projects/espaciGo-kyc-didit`.
- Base: `origin/main`, SHA `771134d165046b290b1f5ad3a1074a6357c2327f`, obtenido después de fetch. No partir del checkout histórico backend ni incorporar sus diferencias por inferencia.
- Entrega de este corte KYC-00: investigación oficial, contraste estático del código/esquema, diseño y guía MCP/secretos. Crear rama no autoriza publicación, integración a main, despliegue, migración ni pruebas con identidades reales.
- No se consultó una base viva. El inventario contrasta **migraciones versionadas**, Go, OpenAPI y Next; cualquier drift o versión efectiva de PostgreSQL queda pendiente de preflight autorizado.
- No se accedió a la aplicación online de EspaciGo, GCP, datos de usuarios, credenciales existentes ni pestañas del usuario. La lectura externa de Didit por MCP fue expresamente solicitada y se limitó a configuración del workflow; no equivale a probar el producto online.
- Lectura MCP de solo lectura completada: `Hotusoft` es la organización/aplicación, no el nombre del workflow. En `Hotusoft (Sandbox)` (app `806fe3f4-4c9b-4567-bf6f-ce8c3e688fce`) está publicado `Compliance workflow` (`a4b973d4-4578-44fc-8580-7554e39821ce`), v1, con `OCR + LIVENESS + FACE_MATCH`, país `CHL` y `expected_details_mismatch_action=REVIEW`. El payload informa precio estimado de USD 0.35; no es confirmación de tarifa live ni de un cobro. No se creó una sesión ni se modificó el workflow.

## 2. Conclusiones verificadas del proveedor

Fuentes oficiales consultadas el 2026-11-06; enlaces en §12. Las condiciones de la cuenta y su contrato prevalecen frente a una estimación pública.

1. **MCP y API son integraciones distintas.** El MCP alojado es Streamable HTTP en `https://mcp.didit.me/mcp`, autentica al usuario mediante OAuth 2.1/PKCE y puede leer/escribir en sus organizaciones. No admite API key como modo de autenticación. El backend del producto usa REST con `x-api-key`; no debe depender de Hermes ni de un token OAuth personal.
2. **Cambió la cuota gratuita.** Desde el **1 de noviembre de 2026**, la documentación sustituye las 500 verificaciones gratis/mes por USD 10 de crédito mensual, para organizaciones registradas con correo de trabajo, sin cubrir phone verification. El KYC completo publicado cuesta USD 0.33 con documento, passive liveness, face match e IP/dispositivo. La propia tabla estima unas 30 verificaciones completas/mes, no 500. Otros módulos y reintentos pueden cambiar el costo. No habilitar auto top-up ni comprar saldo en este corte.
3. **Sandbox separado por aplicación:** 500 creaciones de sesiones cada 24 horas por aplicación, sin facturación. Los resultados son simulados, pero **las imágenes, selfies y videos capturados sí se almacenan**; no usar personas/documentos reales ni cámara personal en una prueba automatizada.
4. `POST https://verification.didit.me/v3/session/` recibe `workflow_id`, `vendor_data`, `expected_details`, `language` y callback. Devuelve `session_id`, `url` y `session_token`. La URL y su token son secretos temporales, no deben aparecer en logs/analítica/evidencia.
5. `expected_details` se envía por sesión al crearla: no es un valor que se prellene en el workflow. Permite contrastar los atributos declarados con OCR; los desacuerdos generan warnings con acción configurable. El workflow sandbox comprobado tiene `expected_details_mismatch_action=REVIEW`. **EspaciGo aún debe enviar el snapshot del formulario y aplicar una compuerta propia en Go; el ajuste del workflow por sí solo no conecta el formulario ni demuestra que el RUT fue extraído/comparado correctamente.**
6. El reporte V3 devuelve arrays `id_verifications[]`, `liveness_checks[]`, `face_matches[]` ligados por `node_id`. Campos de documento/número personal pueden ser nulos. No tomar el primer elemento ni un `Approved` aislado como prueba completa.
7. Chile figura en la cobertura documental. Didit también documenta `chl_rut`, validación de RUT contra registros del Servicio de Registro Civil, como servicio adicional de database validation por USD 0.20 por consulta concluyente. Es **opcional**, diferente de OCR + prueba de vida; no viene incluido por inferencia en el KYC base ni habilita a afirmar una consulta estatal ya realizada.

**Decisión propuesta:** MVP de persona natural con cédula emitida en Chile, captura alojada por Didit, documento + liveness + face match. Reutilizar el workflow existente solo después de verificar que satisface el contrato. Mantener KYB, pagos, firma, listas AML y autenticación Didit fuera de este corte.

## 3. Flujo de usuario propuesto

1. **Mi cuenta → Verificar cuenta.** Mostrar «Cuenta no verificada» y CTA «Verificar cuenta», con navegación a «Verificación de usuario». Separar «Verificación de espacios»; no mostrar KYC/KYB como títulos para usuarios. Correo verificado no equivale a identidad verificada.
2. **Información y consentimiento.** Explicar finalidad, proveedor, datos compartidos, cámara, tratamiento biométrico, conservación y canal de revisión; consentimiento específico y versionado separado de marketing. No preseleccionarlo. Avisar antes de salir de EspaciGo. Menores/no elegibles y documentos no soportados reciben una salida informativa, sin cobro ni sesión remota.
3. **Formulario y guardado independiente** (§4). Prellenar solo identidad legal propia confirmada por Go; no dividir el nombre visible ni modificar el perfil público. CTA «Guardar datos de identidad» valida en cliente y Go y persiste declaración versionada y consentimiento. No llama a Didit ni redirige. Solo mostrar «Datos guardados» tras commit y lectura independiente autenticada. Debe sobrevivir recarga/reinicio del servicio y ser accesible solo a su propietario. Un guardado fallido o incierto no habilita el siguiente paso: ofrecer reconciliación explícita, sin repetir una mutación a ciegas.
4. **Confirmar y verificar, como segunda acción.** Mostrar resumen de los datos guardados con «Editar datos» y botón separado «Verificar identidad con Didit». Solo se habilita con revisión persistida vigente y permisos confirmados. Editar invalida la confirmación hasta guardar/leer de nuevo; no altera el snapshot de un intento anterior. Go deriva la cuenta de sesión y crea el intento desde esa revisión, no desde datos arbitrarios reenviados por UI. Bloqueo/idempotencia por cuenta, propósito, revisión y entorno; persistencia previa a la llamada externa y recuperación de resultado incierto. En la API del proveedor, Go envía el workflow aprobado y `expected_details` desde el snapshot con nombres, apellidos, nacimiento, emisor CHL, tipo admitido y RUN/RUT normalizado con DV. La versión/ruta exacta se ratifica en KYC-10. No enviar PII en `vendor_data`, URL o logs. Solo redirigir a una URL HTTPS del proveedor validada y realmente devuelta; sin credenciales o ante fallo, mostrar error seguro, nunca fabricar sesión.
5. **Captura en Didit.** Redirección de página completa, preferida para el MVP por cámara y compatibilidad móvil; Didit controla anverso/reverso, selfie/liveness y cotejo facial. No construir captura biométrica propia en EspaciGo. Liveness activo o pasivo según el workflow confirmado, sin prometer movimientos de cabeza si no están configurados.
6. **Retorno a EspaciGo.** Pantalla «Estamos comprobando el resultado». Ignorar `status` del query como autoridad. Resolver únicamente un intento propio y consultar Go; limpiar query de la barra cuando se resuelva la correlación. Si perdió sesión, login con retorno contextual allowlisted; nunca reasignar una sesión Didit a quien trae una URL.
7. **Recepción y cotejo asíncronos.** Webhook autenticado, inbox deduplicado y recuperación server-to-server de la decisión. Go valida app/entorno/workflow/versión/nodos, warnings y atributos extraídos contra el snapshot declarado. Los códigos de discrepancia (`FULL_NAME_MISMATCH_WITH_PROVIDED`, `DOB_MISMATCH_WITH_PROVIDED`, `COUNTRY_MISMATCH_WITH_PROVIDED`, `IDENTIFICATION_NUMBER_MISMATCH_WITH_PROVIDED`) mantienen el caso no-verificado hasta revisión/corrección; no basta el estado superior `Approved`. El webhook puede llegar antes del retorno, después, duplicado o no llegar. La UI admite recarga y reanudación desde otro dispositivo.
8. **Resultado:** aprobación efectiva + módulos requeridos + cotejo satisfecho → escudo verde con check y «Identidad verificada»; revisión/warning → reloj ámbar y «En revisión»; discrepancia corregible → «Revisa tus datos»; fallo técnico → reintento seguro, no «identidad rechazada»; rechazo → explicación segura y camino de revisión. No prometer tiempo fijo de aprobación humana.
9. **Mantenimiento:** expiración documental, revocación/bloqueo, cambio material de identidad y nueva decisión invalidante retiran el verde. No rescatar una aprobación histórica. El nombre público puede cambiar sin modificar la identidad legal.

No bloquear por defecto registro, edición de perfil o publicación por falta de KYC. El requisito de reserva es un gate separado (§7): el código actual exige revisión documental manual y no acepta un proveedor nuevo automáticamente.

## 4. Formulario para identidad chilena y cotejo

| Dato visible | Campo Didit de entrada | Política propuesta |
| --- | --- | --- |
| Nombres tal como aparecen en la cédula | `expected_details.first_name` | Todos los nombres en un campo; Unicode, espacios y guiones; no confundir con display name. |
| Apellidos tal como aparecen en la cédula | `expected_details.last_name` | Un campo de apellidos completos. No obligar a tener dos ni inferir límites entre apellido paterno/materno desde OCR. |
| Fecha de nacimiento | `expected_details.date_of_birth` | Fecha civil `YYYY-MM-DD`, sin conversión UTC; válida y no futura. Propuesta de producto: solo mayores de 18, pendiente de ratificación. |
| RUN/RUT de la persona | `expected_details.identification_number` | UI acepta puntos/guion y DV K/k; Go normaliza sin puntos, con guion y DV mayúsculo y valida módulo 11. Nunca usar el número de serie de la cédula como RUT. |
| País emisor | `expected_details.id_country` | Chile/`CHL` para este MVP; campo visible, fijo para la ruta chilena. |
| Tipo de documento | `expected_details.expected_document_types` | `["ID"]` en MVP; el catálogo usa otros nombres en la respuesta (`Identity Card`): no comparar códigos de entrada con labels de salida. |
| Nacionalidad | `expected_details.nationality` si se solicita | No deducirla del país emisor. Por minimización, omitirla del MVP salvo necesidad del workflow/política; una cédula chilena no obliga a nacionalidad chilena. |
| Consentimiento | registro interno + configuración de consentimiento Didit | Versión, finalidad, fecha servidor y cuenta. No inventar que `biometric_consent` reemplaza toda obligación local ni marcarlo otorgado sin acción válida. |

No pedir sexo/género, domicilio, estado civil, profesión, cuenta bancaria ni número de serie documental si no son necesarios para este objetivo. No editar correo desde este formulario. Documentos extranjeros/pasaporte: indicar «Esta etapa admite cédula emitida en Chile» y ruta de consulta/revisión, no forzar un RUT inexistente; ampliar en una decisión posterior con contrato propio.

### Motor de contraste

- Conservar **datos declarados**, **datos verificados** y **resultado del cotejo** como conceptos diferentes, con versión de la política. No sobreescribir el formulario silenciosamente con OCR.
- Normalizar comparación de nombres con Unicode, mayúsculas, espacios repetidos y tolerancia controlada a tildes/transliteración; preservar el original. No eliminar arbitrariamente apellidos ni aprobar por fuzzy matching genérico inventado. Umbral Didit revisado y casos chilenos sintéticos aprobados antes de automatizar; ambigüedad → revisión.
- Nacimiento exacto; RUT normalizado exacto y DV válido; país/tipo según workflow. `document_number` es el número documental y `personal_number` el identificador personal: **no asumir que son iguales**.
- Gate chileno: confirmar con reporte permitido/sintético representativo qué campo/subtipo entrega el RUN completo y su DV. El esquema genérico no prueba ese dato por subtipo. Si falta, está truncado o es ambiguo, no dar verde por nombre y nacimiento solamente; requerir revisión o evaluar `chl_rut` con aprobación de alcance/costo.
- Warnings de `FULL_NAME_MISMATCH_WITH_PROVIDED`, `DOB_MISMATCH_WITH_PROVIDED`, `COUNTRY_MISMATCH_WITH_PROVIDED`, `IDENTIFICATION_NUMBER_MISMATCH_WITH_PROVIDED` bloquean aprobación automática hasta resolución auditada. El workflow `Compliance workflow` v1 ya tiene `expected_details_mismatch_action=REVIEW`; Go mantendrá además su propia regla fail-closed y no confiará solo en el estado superior. No usar los mensajes crudos para exponer reglas antifraude ni identidad ajena.
- Una corrección durante un intento no modifica su snapshot. El usuario confirma los nuevos datos y crea una nueva revisión/intención; no crear otra sesión ciegamente porque Didit reutiliza una sesión no finalizada por `(workflow_id, vendor_data)` y no promete actualizar `expected_details` al reutilizarla. Reconciliar primero el intento anterior.

## 5. Estados e insignia

Los estados internos siguientes son **propuestos**, no enums existentes que se puedan enviar hoy al API.

| Estado Didit exacto / condición | Estado interno propuesto | UI/insignia |
| --- | --- | --- |
| Sin intento | `no_iniciada` | Escudo vacío gris; «No verificada». |
| `Not Started` | `pendiente_inicio` | Escudo vacío; «Continuar verificación». |
| `In Progress` | `en_proceso` | Escudo vacío + progreso; «Verificación en curso». |
| Retorno/webhook pendiente o error recuperable | `pendiente_confirmacion` | Nunca verde; «Comprobando resultado». |
| `In Review` o cotejo ambiguo/incompleto | `en_revision` | Reloj ámbar; no check verde. |
| `Approved` + todos los gates | `verificada` | Escudo verde relleno/check + texto. |
| `Declined` | `rechazada` | Explicación segura; opción de revisión según política. |
| `Resubmitted` | `requiere_accion` | Mostrar pasos que repetir; no tratarlo como aprobado. |
| `Expired` | `intento_vencido` | Nueva sesión explícita si procede; nunca confundir con vencimiento de identidad aprobada. |
| `Abandoned` | `incompleta` | Reanudar/crear intento según estado real. |
| `Kyc Expired` o vencimiento calculado localmente | `vencida` | Retirar verde, «Actualiza tu verificación». |
| Revocación local/bloqueo autoritativo | `revocada` | Retirar verde aunque haya un `Approved` viejo. |
| `Awaiting User`, estado desconocido o módulo requerido ausente en este KYC | `en_revision` / alerta técnica | Fail closed; investigar configuración, nunca aprobar. |

**Predicado autoritativo:** cuenta activa + identidad actual no revocada + entorno live para sello real + sesión/aplicación/workflow/versión esperados + decisión vigente + documento válido + nodos obligatorios aprobados de documento/liveness/face match + cotejo satisfecho + consentimiento aplicable + plazo local vigente. Nueva política/cambio requiere revisión explícita. En sandbox, mostrar siempre «Verificación de prueba» y excluirla de proyecciones públicas reales y de elegibilidad real.

No prometer propiedad del inmueble, ausencia de riesgo o garantía de pago. Insignias de identidad personal y documentación del espacio siguen siendo distintas. No reutilizar sin decisión la política manual de 24 meses para Didit; proponer vencimiento efectivo = menor entre vencimiento documental y plazo de renovación ratificado.

## 6. Datos, arquitectura y seguridad propuestas

### Persistencia

Definir el delta en KYC-01 tras inventario completo y revisión; nombres conceptuales, no tablas ya implementadas:

- Identidad legal declarada/versionada: cuenta, nombres/apellidos, nacimiento, identificador personal cifrado, país/tipo y consentimiento referenciado. Separada del perfil público.
- Intento de proveedor: UUID interno, cuenta, proveedor `didit`, entorno, application/workflow ID y versión, node IDs requeridos, provider session ID único, correlación opaca estable, snapshot/version, estado crudo y normalizado, tiempos, expiración y motivo seguro.
- Resultado mínimo: campos verificados necesarios cifrados, coincidencias por campo, riesgos allowlisted, versión de reglas, origen automático/manual, actor de revisión, revocación y supersesión. No guardar imágenes, videos ni plantillas faciales en PostgreSQL.
- Inbox de eventos: aplicación/entorno + `event_id` único, digest, tiempo de recepción y resultado de procesamiento; no log de cuerpos completos. Para recuperación, sobre cifrado mínimo con TTL o volver a consultar decisión, no persistir PII indiscriminadamente.
- Identidad efectiva: referencia a la decisión actual y su revisión. Transacción/auditoría atómica; CAS y bloqueo por cuenta/intento; historial inmutable. Evaluar índices únicos de intención activa y evitar colisiones entre sandbox/live.
- Para detección de duplicado de RUT, evaluar HMAC con clave separada/versionada, nunca SHA simple de un identificador enumerable. Conflicto no revela la cuenta ajena. Política de una identidad/múltiples cuentas pendiente de ratificación; no bloquear automáticamente por inferencia.

Toda migración aplicada permanece byte a byte. Elegir **la siguiente versión libre al implementar**, no reservar V84 por suposición. No aplicar SQL a PostgreSQL persistente/Cloud SQL desde este plan. Reusar controles de retención/exportación/supresión y actualizar inventario de procesadores; revisar DPA, residencia, transferencias, retención, eliminación y consentimiento con asesoría apropiada antes de live. Las afirmaciones comerciales de Didit no demuestran cumplimiento jurídico automático de EspaciGo.

### Backend y API

- Autoridad Go → puerto de verificación → adaptador Didit REST con timeout, límite de cuerpo, TLS y host fijo. Next mantiene BFF same-origin y sesión existente; cero claves Didit, acceso DB o tokens OAuth personales en el frontend.
- Operaciones conceptuales para diseñar en OpenAPI: leer identidad/resumen propio, preparar/confirmar declaración, iniciar/reanudar intento idempotente, consultar estado propio y solicitar revisión/corrección. Rutas definitivas pendientes del inventario; no presentarlas como endpoints montados.
- `vendor_data` derivado por Go de una referencia opaca estable de cuenta, sin RUT ni correo; metadata solo correlaciones no sensibles. No confiar en `owner_id`, workflow o entorno enviados por cliente.
- Dos niveles de idempotencia: intención local con snapshot inmutable + reutilización Didit no finalizada por workflow/vendor. Tras timeout de creación, reconciliar antes de repetir; después de sesión finalizada, la misma llamada puede crear otra. Respetar 429/Retry-After y evitar reintentos con cobros/duplicados.
- Callback fijo/allowlisted y URL remota HTTPS validada con hosts oficialmente confirmados, no cualquier URL del proveedor/cliente. Sin open redirect, SSRF, captura de token por analytics ni almacenamiento de link en localStorage/sessionStorage.

### Webhook y reconciliación

- Endpoint público **solo para eventos Didit**, separado del BFF autenticado y de rutas administrativas. Su publicación/red/IAM es un gate posterior, no autorización de infraestructura implícita. No exponer toda la API privada para recibir un webhook.
- Verificar HMAC en tiempo constante y frescura `X-Timestamp` (ventana de 300 segundos según documentación), límites de body y tipo de contenido. Preferir `X-Signature-V2` con canonicalización Unicode/sorted JSON **exacta**, probada contra vectores oficiales; firma raw documentada como alternativa solo si se conservan bytes exactos. No usar firma Simple como único aval del decision body. Sin downgrade silencioso.
- Considerar rotación de secretos actuales/anteriores y solapamiento oficial; rechazar eventos falsos, desconocidos o del entorno/aplicación incorrectos. Validar correlación sesión→intento→cuenta desde DB, no reasignar a partir de metadata/vendor_data del evento.
- Suscribirse a `status.updated`, `data.updated` y, si la política los usa, eventos de bloqueo `user.status.updated`; manejar actualización posterior a la aprobación sin retener un verde obsoleto.
- Persistir recepción duradera antes de 2xx, procesar asíncronamente con reintentos internos acotados/dead-letter y alertas sin PII. No responder éxito si se perdió el evento antes de guardarlo.
- Duplicados y desorden: inbox deduplicado, reconciliación serializada por intento, consulta GET autoritativa y revisión local monotónica/CAS. Un Approved retrasado no puede revivir identidad revocada, vencida o supersedida. Fallos/estados no reconocidos mantienen fail closed.
- Didit documenta solo dos reintentos automáticos de webhook tras ciertos fallos; no depender de entrega infinita. Incluir trabajo de reconciliación acotado y refresh explícito propio, con límites de frecuencia; sin workers generales sobre DB persistente para probarlo.

## 7. Contraste inicial confirmado en la base de código

- `internal/verification/service.go:77`: constructor restringe proveedor a `local-fixture-v1`; `Start` crea evidencia `fixture:` y solo acepta `en_revision`. Agregar una API key no habilita Didit; requiere contrato/servicio nuevo o refactor explícito sin convertir fixtures en pruebas reales.
- `internal/identityverification/service.go:16`: propósito actual `identidad_documental`, método `manual_document_review_v1`. `RequireForBooking` (`:60`) y `currentFor` (`:79`) exigen aprobación manual, dos partes distintas, no revocación y plazo máximo manual de 24 meses. No renombrar Didit a ese método ni quitar el guard para hacer pasar una reserva.
- `frontend/src/features/verifications/ui/verifications-view.tsx:40`: ya existen badges de casos aprobada/en_revision/rechazada/revocada; el inicio (`:182`) recibe tipo, no declaración legal ni URL alojada. Es base reutilizable de presentación, no flujo Didit.

### Matriz de datos y consumidores comprobados

| Capa actual | Fuente versionada | Hallazgo y cambio requerido |
| --- | --- | --- |
| Cuenta | `db/migrations/V000001__m01_identity.sql:5` | `usuario` guarda identidad de acceso/correo/estado, no nombre legal, RUT ni nacimiento. No sobrecargar la cuenta de login con evidencia biométrica. |
| Perfil | `db/migrations/V000002__m02_profile_rights.sql:2` | `perfil_usuario` contiene `nombre_visible` y teléfono; no equivale a nombres/apellidos documentales. Agregar identidad legal separada, no transformar display name silenciosamente. |
| Verificación sintética | `db/migrations/V000003__m03_verification.sql:1` | Constraints fijan `local-fixture-v1` y evidencia `fixture:`; no cambiar su significado ni convertir casos históricos a Didit. |
| Elegibilidad sintética | `db/migrations/V000028__local_kyc_eligibility_and_history.sql:83`, `internal/adapters/postgres/verification/repository.go:122` | Aprobación/revocación manual de fixtures persiste elegibilidad/historia. No usar esta elegibilidad como certificado de biometría real. |
| Identidad documental manual | `db/migrations/V000079__document_verification_cases.sql:2`, `internal/identityverification/service.go:79` | Casos `identity`/`space`, archivos, permisos y decisiones; contratos/gates distintos. Integrar autoridad actual sin fingir revisión manual ni perder retención/auditoría. |
| Cambios posteriores documentales | `db/migrations/V000080__document_verification_space_ownership_supersession.sql:6`, V82/V83 | Supersesión, retención y concurrencia vigentes. No reemplazar estos mecanismos por un booleano nuevo que ignore su historia. |
| Alta API | `planning/openapi.yaml:2595`, `:10291` | `startSyntheticVerification` y `VerificationCreate` solo aceptan `type`, `additionalProperties: false`; no admite nombres/RUT ni devuelve contrato Didit. Definir operaciones nuevas explícitas y regenerar clientes. |
| Resumen API | `planning/openapi.yaml:2624` | Elegibilidad anunciada como sintética. Separar resumen efectivo de identidad del estado de fixture y de correo verificado. |
| Proyección pública Go/SQL | `internal/adapters/postgres/publiccatalog/repository.go:123` | `identity_verified` se calcula desde la última decisión documental terminal vigente; `space_verified` además depende de identidad y de evidencia del espacio. Reconciliar la nueva autoridad sin alterar publicación/propiedad ni resucitar una aprobación histórica. |
| Insignias Next | `frontend/src/features/public-catalog/ui/public-verification-badges.tsx:8` | Actualmente no renderiza nada sin flags; con flags muestra etiquetas neutras, no el escudo vacío/verde solicitado. Agregar el nuevo lenguaje visual con dato autoritativo; conservar distinción del espacio y disclaimers. |
| Pantalla privada | `frontend/src/features/verifications/ui/verifications-view.tsx:182` | Inicio sintético por tipo y badge de caso. Agregar el formulario/redirect/retorno y mantener historia sin aparentar que ya hay Didit. |

Inventario realizado sobre los **83 archivos de migración tracked** de la base SHA indicada; esto no demuestra que una base viva tenga V83 aplicada. Se contrastaron rutas y contenido en el worktree padre, no se tomó el resumen delegado como prueba suficiente. No hay un perfil legal estructurado de RUT/nacimiento en las fuentes inspeccionadas; los archivos documentales existentes no equivalen a ese modelo.

**Gate de producto pendiente:** ratificar si Didit habilitará reservas y bajo qué garantías; hasta entonces mantener el requisito manual actual sin bypass y comunicar la diferencia en UI.

## 8. Frontend previsto

- Mantener Next, layout Mi cuenta, Tailwind/tokens y componentes actuales; modificar superficies de identidad/verificaciones sin rediseño global.
- Feature tipada: UI → fachada → puerto → adaptador OpenAPI → BFF → Go. Estado visible y `can_start/can_resume/can_correct` desde servidor; no decidir autorización por color, rol o query string.
- Vista principal con resumen de identidad y pasos «Tus datos → Documento y cámara → Resultado». Formulario con labels, ayuda sobre RUN vs número de serie, errores asociados a controles y foco en primer error. Invalid input no crea intento ni hace llamada remota.
- Preservar borrador solo durante la sesión/pantalla; evitar persistencia de RUT/nacimiento en storage del navegador, HTML SSR, URLs o analítica. Purgar al logout, cambio de cuenta/permisos y unmount. No reutilizar respuestas tardías A/B/A.
- Estado de espera persistente del servidor; polling acotado/backoff solo visible, detener al salir; refresh explícito si demora. Retornar de cámara sin resultado no es aprobación.
- Insignia reutilizable privada/pública con texto accesible, no solo color ni hover. Público: exponer como máximo booleano de identidad vigente calculado en Go; nunca RUT, nacimiento, ID Didit ni motivo de rechazo. Leer la decisión actual antes de evaluar vigencia.
- Móvil/escritorio: 390×844 y 1365×900, sin overflow, objetivos táctiles de 44px, teclado, lectores y Axe completo; mensajes de cámara denegada, dispositivo incompatible, interrupción, abandono y falta de conectividad.

## 9. Plan por fases y gates

| Corte | Trabajo | Evidencia requerida para cerrar | Estado |
| --- | --- | --- | --- |
| KYC-00 | Investigación, rama, plan, inventario y guía de conexión | Fuentes oficiales fechadas, HEAD base, contraste estático y estado OAuth honesto | Completado (`evidencia_kyc00_20261106.md`) |
| KYC-01 | Decisiones Chile, workflow y contrato | ADR e investigación existentes; ratificación de campos/nodos, privacidad y reglas cerrada en §24 | Cerrado localmente (§24, `propuesta_retencion_kyc09_kyc10_20261108.md`); live sigue gate separado |
| KYC-02 | SQL y OpenAPI | V84/V85/V86/V87 y esquemas OpenAPI sincronizados; pruebas fresh/upgrade en PostgreSQL efímero | Cerrado localmente en KYC-09 (`evidencia_kyc09_20261108.md`) |
| KYC-03 | Go, inbox y reconciliación | Cliente v3, `DiditService`, repositorio V85, inbox durable, firma V2/Simple y `EvaluateDecision` fail-closed | Cerrado localmente en KYC-08–KYC-11 (`evidencia_kyc08_20261107.md`, `evidencia_kyc10_kyc11_20261108.md`) |
| KYC-04 | Next, retorno e insignias | Flujo en 2 acciones con readback, aislamiento A/B/A, retorno endurecido y copy separado de espacios | Cerrado localmente en KYC-12 (`evidencia_kyc10_kyc11_20261108.md`, `evidencia_kyc13_20261108.md`) |
| KYC-05 | Sandbox e integración local | Integración local DB + Go + BFF + Playwright verificada con mocks/stubs sin consumir cuota externa | Cerrado localmente en entorno sintético (`evidencia_kyc13_20261108.md`); sandbox externo real pendiente de autorización |
| KYC-06 | Revisión integral/preparación live | Matriz C01–C11 y cierre local R01–R14 por capa/entorno/SHA | Cerrado localmente en KYC-13 (`evidencia_kyc13_20261108.md`) |
| KYC-07 | Publicación/despliegue | Código integrado en main `274f37e`; versión desplegada, tráfico y validación humana no comprobados en esta revisión | Pendiente; diagnóstico/entrega KYC-14 |

Cada corte registra SHA, comandos, resultados, límites, recursos y siguiente gate. No declarar el KYC terminado por mocks verdes ni por instalar el MCP. La conexión OAuth no bloquea diseño/documentación ni autoriza configurar el workflow.

### Secuencia histórica de inicio — sustituida operativamente por §15

La secuencia siguiente conserva la intención original; ya existe implementación parcial. No usarla para reiniciar el trabajo ni para considerar cerradas las fases. El encargo actual es documental y el orden correctivo vigente está en §15.

1. **Cerrar KYC-01 antes de escribir código:** ratificar documento del MVP (cédula chilena), mayoría de edad, política de nombres, consentimiento/retención y el gate de reservas; dejar el contrato de `expected_details` y cotejo/mismatch en ADR; verificar el mapeo RUN/DV en evidencia sintética permitida. Mantener como hechos distintos el issuer `CHL`, la nacionalidad y el RUN/RUT.
2. **Primer corte de implementación: KYC-02.** Diseñar y revisar delta mínimo de identidad/intentos/auditoría e inbox, versión de migración libre solo tras inventario actualizado, constraints/ACL y OpenAPI tipado. No aplicar SQL persistente; preparar pruebas de base vacía y upgrade en entorno sintético aislado.
3. **KYC-03 Go y contrato:** iniciar sesión Didit desde Go con claves de aplicación sandbox inyectadas privadamente; fijar `expected_details` desde snapshot autenticado; hacer creación idempotente y recuperación; implementar verificación de firma, inbox, lectura autoritativa y cotejo de warnings/datos. Mantener feature flag live apagado. No usar MCP como runtime.
4. **KYC-04 Next:** formulario Chile, consentimiento, CTA, redirect/return y estados/insignias desde DTO Go. No persistir RUN/nacimiento en URL o storage del navegador; no cambiar perfil público para representar identidad legal.
5. **KYC-05 recorrido sintético en sandbox:** tras autorización específica, usar únicamente datos/medios sintéticos y el workflow publicado comprobado; probar par coincidente, mismatch de nombre/fecha/identificador, warning/missing fields, abandono, duplicado y recuperación. Verificar DB→API→UI y que ningún mismatch, resultado parcial ni sandbox dé check verde. Didit informa que el sandbox conserva medios capturados; no usar cédulas/selfies reales.
6. **KYC-06 revisión integral:** privacidad, coste efectivo por workflow/aplicación, controles de acceso, retención/eliminación, recuperación, accesibilidad y regresiones. Live, publicación, main y despliegue siguen gates separados con autorización expresa.

## 10. Matriz mínima de aceptación

- Formulario chileno: DV numérico/K, normalización, fecha inválida/futura, nombres compuestos, tildes/Ñ/guiones, un apellido y extranjeros con cédula chilena sin nacionalidad forzada.
- No crear sesión con inválidos/sin consentimiento; cuenta derivada del auth; acceso cruzado y sesión expirada denegados; CSRF/Origin BFF preservados.
- Snapshot estable; misma intención/llave idempotente no duplica ni actualiza datos a escondidas; timeout antes/después de creación y respuesta perdida reconciliados.
- ID/liveness/face match ausentes, fallidos, de nodo no esperado, sin RUT/DV, workflow cambiado o reporte parcial nunca dan verde.
- Coincidencia correcta; diferencia RUT/nacimiento; nombre ambiguo; mismatch warnings aunque top-level Approved; corrección auditada sin sobrescritura OCR silenciosa.
- Webhook falso/alterado, Unicode, firmas actuales/previas, replay, duplicado, desorden, evento previo a guardar sesión, desconocido, entorno cruzado, fallos de inbox y decisión GET.
- Retorno manipulado `status=Approved`, usuario cerró navegador y callback perdido no autorizan por sí mismos; login/recarga/nuevo dispositivo recuperan solo intento propio.
- Approved → revocado, Declined, Resubmitted, data.updated invalidante, Kyc Expired y expiración local retiran verde en perfil y catálogo; un evento viejo no lo restaura.
- Sandbox jamás concede badge real, reserva real ni se mezcla con live. El KYC fixture y revisión documental mantienen sus garantías/historia.
- Sin claves/URLs tokenizadas/PII en bundle, logs, errores, analytics o capturas. No duplicar imágenes/biometría en EspaciGo. Exportación/supresión incluyen los nuevos registros conforme política ratificada.
- Usuario ve el ciclo entero, no únicamente API; capturas sintéticas, readback independiente y accesibilidad. Fallos técnicos no se etiquetan como fraude/rechazo de identidad.

## 11. Dependencias y decisiones pendientes

1. **Lectura MCP de la app/workflow sandbox completada** (2026-11-06); la app live, precio contractual/crédito efectivo y destino de producción siguen sin verificarse. No listar sesiones personales por defecto.
2. Confirmar elegibilidad efectiva de crédito/plan y aplicaciones sandbox/live. El precio informado en el payload del workflow sandbox no demuestra tarifa live ni importe realmente facturado; no asumir crédito por haber abierto una cuenta.
3. Validar campos de RUN/DV por subtipo chileno; decidir `chl_rut` solo si imprescindible/justificado y con costo aceptado.
4. Ratificar documento inicial, mayoría de edad, nacionalidad opcional, política de nombres, duplicados y renovación; acceso de documentos fuera del MVP sin callejón sin salida.
5. Resolver requisito manual de reservas separadamente; identidad Didit no implica evidencia de propiedad del espacio ni KYB de empresa.
6. Revisar tratamiento legal/retención, revisión humana y permisos mínimos; no almacenar datos reales hasta aprobar esos gates.
7. Elegir entorno **local aislado** autorizado para implementación y prueba; preflight de DB/versiones y política privada antes de cuentas de ensayo.
8. KYC-00 fue documental; posteriormente se implementó e integró código parcial. Esta revisión no amplía autorizaciones anteriores ni ejecuta fases correctivas. Acordar los gates pendientes de §15 antes de retomarlas.

## 12. Fuentes oficiales

- [MCP: propósito y endpoint](https://docs.didit.me/integration/mcp/overview)
- [OAuth, scopes y roles MCP](https://docs.didit.me/integration/mcp/authentication)
- [Instalación MCP](https://docs.didit.me/integration/mcp/installation)
- [Precios vigentes y cambio del 1 de noviembre de 2026](https://docs.didit.me/getting-started/pricing)
- [Sandbox y escenarios](https://docs.didit.me/integration/sandbox-testing)
- [Crear sesión, expected_details e idempotencia](https://docs.didit.me/sessions-api/create-session)
- [Recuperar decisión](https://docs.didit.me/sessions-api/retrieve-session)
- [Modelo V3 y campos](https://docs.didit.me/reference/data-models)
- [OpenAPI Didit](https://docs.didit.me/openapi-25.json)
- [Advertencias de documento y mismatch](https://docs.didit.me/core-technology/id-verification/warnings-id-verification)
- [Estados exactos](https://docs.didit.me/integration/verification-statuses)
- [Firmas, eventos y reintentos de webhook](https://docs.didit.me/integration/webhooks)
- [Cobertura documental](https://docs.didit.me/reference/supported-documents-id-verification)
- [Chile: validación RUT adicional](https://docs.didit.me/api-reference/database-validation/chile/rut)
- [Hermes MCP y OAuth](https://hermes-agent.nousresearch.com/docs/guides/manage-hermes-cloud-with-mcp)

Documentos relacionados: `guia_conexion_y_operacion.md`, inventario/evidencia del corte. Las fuentes externas describen al proveedor; las decisiones de arquitectura/UX de este documento son propuestas de EspaciGo, no funcionalidades del proveedor ni capacidades ya desplegadas.

## 13. Revisión correctiva del avance — 2026-11-07

Revisión estática del checkout indicado en §1, no auditoría de una base viva ni del proveedor o despliegue. No se revalidaron precios, cuotas o documentación externa: §2 conserva su fecha de consulta y debe refrescarse antes de decisiones de proveedor. Se conservarán las evidencias anteriores; sus afirmaciones incompatibles quedan explícitamente supersedidas por esta revisión, sin borrar historia ni reutilizar sus PASS como cierre E2E.

### Lo que existe y se conserva

- V84 define `identity_verification_attempts` y `didit_webhook_inbox`; la existencia del SQL no demuestra que el handler escriba allí ni que esté aplicado en un destino.
- OpenAPI define creación/consulta de intentos; Go contiene normalización de RUT, cliente HTTP, comparador y handlers. Next contiene tarjeta, formulario y retorno. Hay pruebas unitarias/HTTP; son activos recuperables, no prueba de recorrido completo.
- `274f37e` contiene cambios visibles de copy y tarjeta en main. Los ajustes locales de `consent_given` y `redirect_url` no están en ese commit: son trabajo pendiente del encargo anterior, no una fase terminada.
- La ejecución anterior informó typecheck, lint sin errores con cuatro advertencias, 477 pruebas aprobadas y una omitida, build y handler Go en verde. No se repitieron aquí; no cubren por sí mismos guardado durable, seguridad entre cuentas, webhook real ni versión servida.

### Registro de errores y brechas verificados en fuente

Prioridades: P0 = seguridad/atribución o aprobación incorrecta; P1 = recorrido/persistencia/contrato; P2 = trazabilidad/calidad de entrega. Las filas describen código observado, no explotación demostrada en un sistema desplegado.

| ID | Prioridad | Evidencia en el repositorio | Brecha y corrección requerida | Fase |
| --- | --- | --- | --- | --- |
| R01 | P1 | `frontend/src/features/verifications/verification-flow.tsx`, `handleSaveProfile` y `submit` | Botón único; cambia displayName, divide nombres públicos e ignora error de perfil. Sustituir por declaración legal separada y dos acciones con readback; no hacer públicas las identidades legales. | 09, 12 |
| R02 | P1 | `internal/verification/transport/http/handler_didit.go:18–20,119–121` | Intentos en mapa global volátil; tablas V84 desconectadas de esa ruta. Añadir repositorio durable y pruebas de reinicio/múltiples instancias. | 09 |
| R03 | P1 | `verification-api.ts`, structs de `handler_didit.go` y `planning/openapi.yaml:7530–7562` | En main consentimiento/URL difieren; corrección local solo parcial. Consulta aún espera `badge_status`/account_id, Go devuelve `badge_result`; ID generado es hex de 32 caracteres, contrato exige UUID y cliente exige longitud 36. Unificar contrato generado y tests de frontera reales. | 09, 12 |
| R04 | P0 | `handler_didit.go:102–117,133–165`; `handler.go:78–95` | Cuenta autenticada no se asigna al intento ni se verifica al consultarlo. Usar principal de servidor en creación y cada lectura/reanudación; probar denegación cruzada, no solo autenticación. | 08, 09 |
| R05 | P1 | `handler_didit.go:71–99` | Sesión/URL inventada por defecto; fallo remoto puede acabar en 201. Eliminar fallback de runtime, configuración ausente debe fallar cerrada, mocks solo por inyección de pruebas. Manejar timeout incierto sin duplicar sesiones. | 08, 10 |
| R06 | P0 | `handler_didit.go:187–190,198–210` | Valor de secreto fallback embebido; timestamp ausente o inválido no se rechaza. Eliminar fallback, revisar exposición y rotar si correspondiera con autorización, sin copiar el valor al plan. Exigir configuración y timestamp válido; probar vectores por variante de firma. | 08, 11 |
| R07 | P0 | `handler_didit.go:225–255` | Asigna evento al único intento sin correlación; permite APPROVED sin decisión completa. Eliminar ambos atajos y exigir correlación persistida y decisión autoritativa validada. | 08, 11 |
| R08 | P0 | `handler_didit.go:142–149,168–267` | No usa inbox durable/deduplicación ni consulta decisión remota en webhook; badge VERIFIED no excluye SANDBOX. Implementar persistencia, reconciliación, entorno, supersesión/revocación y fail-closed. | 11 |
| R09 | P1 | `internal/verification/didit.go:86–90,122–129`; §2 | Cliente usa rutas v2 y host sandbox diferente de los previstos por el plan v3. Verificar contrato oficial vigente y evidencia sintética del workflow; no parchear por suposición ni confundir tests con API real. | 10 |
| R10 | P1 | `account-verification-card.tsx:38–40`; `verification-flow.tsx`, selección kyc | Tarjeta/formulario consumen casos sintéticos como estado de identidad y atribuyen aprobación a Didit, arriendos/cobros. Crear resumen efectivo en Go con procedencia/entorno; retirar promesas no respaldadas y mantener gates manuales. | 11, 12 |
| R11 | P1 | `frontend/src/app/verificaciones/retorno/page.tsx:17–59`; cliente de consulta | Exige attempt_id y session_id, pero correlación/callback no están cerrados; espera DTO incompatible. Reanudar por cuenta propia, retorno seguro, refresh acotado y aislamiento A/B/A, sin confiar en query. | 10, 12 |
| R12 | P1 | `handler_didit.go:48–69`; V84:10–19; `verification-flow.tsx:29` | Decoder sin los límites/unknown-fields del handler común; nacimiento solo no vacío, longitudes no alineadas y versión de consentimiento del formulario excede varchar(20). Alinear dominio, OpenAPI y DB antes de conectar persistencia. | 09 |
| R13 | P1 | `internal/verification/didit.go:142–146` | Se accede a d.Warnings antes del guard nil; caso de decisión ausente puede provocar panic. Test negativo y corrección del orden sin ocultar datos faltantes. | 08 |
| R14 | P2 | `evidencia_kyc06_20261107.md:8–31` y tabla histórica de §9 | Numeración/descripción de fases difiere del plan; se afirma deduplicación/seguridad sin ruta durable. Mantener IDs del plan, añadir alcance/exclusiones y cerrar por criterio verificable, no por conteos generales. | 13 |
| R15 | P2 | `cloudbuild.yaml:12–30` | Main activa calidad y release; CI verde no confirma rollout. Falta recibo exacto build→SHA→digest→revisión→tráfico. Investigar error real del pipeline sin atribuirlo a caché ni al usuario por inferencia. | 14 |

## 14. Contrato funcional del recorrido mejorado

Secuencia de aceptación: **Mi cuenta → Cuenta no verificada → Verificar cuenta → Tus datos → Guardar datos de identidad → Datos guardados → Verificar identidad con Didit → Captura externa → Comprobando resultado → Estado autoritativo**.

| Situación | Comportamiento obligatorio |
| --- | --- |
| Sin identidad guardada | Formulario vacío o declaración propia leída; nombres/apellidos, nacimiento, RUN/RUT, país/tipo admitidos y consentimiento versionado. Nunca deducir apellidos desde displayName. |
| Guardar | Solo operación propia en Go/DB; cero peticiones al proveedor. Deshabilitar doble envío; asociar errores a controles. Error no se disfraza de éxito. |
| Guardado confirmado | GET independiente confirma cuenta/revisión y datos; mostrar resumen y acción separada para Didit. Recarga conserva datos desde Go, no desde storage del navegador. |
| Editar | Invalidar revisión confirmada para iniciar; guardar nueva revisión sin modificar snapshot anterior ni reactivar automáticamente una identidad revocada. |
| Iniciar Didit | Backend comprueba cuenta/revisión, consentimiento vigente, idempotencia y entorno. Snapshot inmutable y URL validada; timeout incierto se reconcilia sin POST automático. |
| Retornar o cerrar cámara | No aprobar desde query; consultar intento propio. Permitir retomar desde Mi cuenta sin conservar tokens en URLs o storage ni depender de la instancia. |
| Consulta fallida | Mostrar «No pudimos consultar el estado» y acción explícita; no convertir error en «Cuenta no verificada», rechazo o aprobación. |
| Resultado | Separar no iniciada, datos guardados, en curso, revisión, rechazo, vencida/revocada y fallo técnico; sandbox siempre «Verificación de prueba», nunca sello real. |
| Espacios | Sección diferenciada de documentación de espacios con sus permisos y autoridad actuales. Didit verifica persona: no acredita propiedad, empresa, pagos ni derechos sobre espacios por inferencia. |

Las rutas nuevas de declaración/lectura/inicio se diseñan en OpenAPI en KYC-09: esta tabla no anuncia endpoints implementados. UI→fachada→puerto→adaptador generado→BFF→Go; sin transporte HTTP directo en presentación. Purgar datos privados y abortar/publicar con fencing en logout, cambio de cuenta/permisos o selección.

## 15. Fases correctivas nuevas y orden de ejecución

Se conservan KYC-00–07 y las fases correctivas KYC-08–14. KYC-08 a KYC-13 se completaron localmente en la rama de corrección con evidencia en `evidencia_kyc08_20261107.md`, `evidencia_kyc09_20261108.md`, `evidencia_kyc10_kyc11_20261108.md` y `evidencia_kyc13_20261108.md`; KYC-14 (publicación/despliegue) y el ensayo con sandbox externo real permanecen detenidos hasta autorización expresa. No renumerar el historial ni llamar KYC-05 a un nuevo cambio UX. Cada cierre requiere evidencia propia y actualización de §9/registro Rxx. No iniciar una fase dependiente mientras falte su prerrequisito; documentación/diagnóstico read-only de entrega puede prepararse en paralelo sin publicar.

| Fase | Alcance acotado / dependencia | Criterios de salida y evidencia |
| --- | --- | --- |
| KYC-08 — Contención y seguridad básica | **Completada localmente**; R04–R07 y R13 corregidos en handler/servicio; preservar mecanismos existentes de cuenta/documentos. | Regresiones RED→GREEN para aislamiento de lectura, secreto ausente, timestamp ausente/vencido, evento desconocido, `Approved` sin decisión, `d=nil` y fallo de proveedor. Se implementa `X-Signature-V2` con canonicalización recursiva Unicode/número, firma sintética golden, raw-body fallback completo y encabezados de secreto previo opcional; binding/frescura del timestamp se prueban. `go test -count=1 -race ./internal/verification/...` PASS; `go vet ./internal/verification/...` PASS; `git diff --check` PASS. Evidencia: `evidencia_kyc08_20261107.md`. |
| KYC-09 — Declaración durable y contrato único | **Completada localmente** (tras 08). Guardar/leer identidad y consentimiento (`id-consent-20261108`, 18+) separados del perfil público; intentos vinculados por cuenta/revisión; cifrado AES-256-GCM con AAD y rotación `v1`→`v2`. | OpenAPI generado (`202` operaciones) y gateway compatibles; `newRuntimeIdentityDeclarationService` y `NewHandlerWithDeclaration` en `cmd/api`; pruebas `save→GET→reinicio→GET`, cuentas distintas, rollback, revisiones concurrentes y auditoría en PostgreSQL 18 efímero (`120` resultados DB). Evidencia: `evidencia_kyc09_20261108.md`. |
| KYC-10 — Inicio/reanudación Didit fiable | **Completada localmente con mocks/stubs** (tras 09). Cliente Didit v3 inyectado, snapshot desde servidor, unicidad de RUT por cuenta y correlación callback; cero consumo de cuota real. | `DiditService.StartOrResume`, idempotencia por `Idempotency-Key` e intento activo, TTL de 30 min para sesiones colgadas, bloqueo de colisión de RUT entre cuentas (`409 rut_already_claimed`) y persistencia en `identity_provider_attempt`. Evidencia: `evidencia_kyc10_kyc11_20261108.md`. |
| KYC-11 — Inbox, decisión y estado efectivo | **Completada localmente con mocks/stubs** (tras 09/10). Firma V2/Simple, inbox durable (`identity_provider_event`), dedupe, consulta autoritativa `GetSessionDecision`, cotejo estricto y borrado tras `VERIFIED`. | `ProcessWebhook` con deduplicación durable, cifrado de decisión mínima sin medios, bloqueo de `SANDBOX` en modo `LIVE`, revocación por supersesión de declaración y solicitud de borrado remoto vía outbox. Evidencia: `evidencia_kyc10_kyc11_20261108.md`. |
| KYC-12 — UX completa en dos acciones | **Completada localmente** (tras 09–11). Flujo en 2 pasos (`Guardar datos de identidad` -> `Editar datos` -> `Verificar identidad con Didit`), fencing por `accountId` y `/verificaciones/retorno` endurecido. | Playwright local escritorio/móvil (`29` pruebas E2E + Axe en verde) y `482` pruebas Node (`tooling/*.test.mjs`): guardado sin proveedor, readback/recarga, edición, consentimiento separado Didit, aislamiento `A->logout->B->logout->A`, conflicto 409 sin redirección y recuperación ante error 503 y fallo técnico `FAILED`. Evidencia: `evidencia_kyc10_kyc11_20261108.md` y `evidencia_kyc13_20261108.md`. |
| KYC-13 — Revalidación integral y sandbox | **Completada localmente en entorno aislado** (tras 08–12). Matriz C01–C11 y cierre local de brechas R01–R14 verificados en Go (`-race -count=1`), PostgreSQL 18 efímero (`120` pruebas) y Playwright/BFF (`29` pruebas). | Matriz `criterio→test→capa/entorno→SHA→resultado` en `evidencia_kyc13_20261108.md`. Sandbox externo real de Didit, futura migración append-only `V000088` y entorno Live permanecen detenidos hasta autorización expresa. |
| KYC-14 — Main, pipeline y aceptación separada | Código completo/revisado para publicar; diagnóstico read-only del fallo previo independiente. Seguir skill versionada release y runbooks vigentes, no secuencia cloud manual. | Publicación de rama y main con autorización, identidad/hook y refs verificadas; CI del mismo SHA. Localizar trigger/build exacto, clasificación docs/web/full, paso/error y recibos/lock antes de reintentar. Verificar imagen/digest/provenance, Ready y tráfico a revisión del SHA exacto; esquema por ledger/postcheck solo vía circuito autorizado. Registrar error y resolución, o bloqueo real. Usuario verifica online Mi cuenta→guardar→Didit→resultado; metadata no equivale a aceptación. |

Para cada fase: dueño, archivos/capas, dependencias, datos sintéticos permitidos, SHA probado, comandos/resultados completos, skips, límites y recibo de revisión. Toda escritura externa exige lectura del destino después; no declarar fase cerrada desde un resumen delegado. Entorno exacto de DB/API y política privada se verifican antes de un ensayo; no sustituir bases o crear cuentas por inferencia.

### Decisiones que no se deben inventar

- Live: aplicación/workflow y precio/retención/consentimiento vigentes, política de renovación, nombres ambiguos, revisión humana y duplicidad. Decidir elegibilidad de reservas/pagos por separado; ningún copy concede esos permisos.
- Persistencia: el candidato V85 y su contrato están documentados y alineados con el handler/API actual; no están aplicados. Siguen como decisiones explícitas la duración de retención, ciclo operativo de claves, recuperación de URL/sesión e idempotencia/reconciliación del proveedor. Mantener V1–V84 byte a byte y V85 sin aplicar hasta gates de compatibilidad, autoridad, backup y autorización del destino exacto; no añadir PII por comodidad.
- Operación: ingress del webhook, configuración/secretos runtime y observabilidad sin PII se revisan sin ampliar IAM o exponer API privada. Rotación no significa borrar historia Git por iniciativa propia.
- Despliegue previo: causa **no determinada**. Que main deba desplegarse obliga a inspeccionar su ejecución, no a declarar rollout ni fabricar una causa. Esta edición documental no consulta GCP ni inicia otro despliegue.

## 16. Estado de la revisión documental (snapshot anterior a la ejecución)

- «Implementado» requiere camino de producción conectado; «guardado» exige commit/readback durable propio; «probado» identifica capa/entorno; «en main» exige ref remoto; «desplegado» exige cadena SHA/digest/revisión/tráfico; «aceptado» requiere usuario. Son afirmaciones diferentes.
- En su momento, esta revisión cambió únicamente el plan; la ejecución posterior de KYC-08 está registrada en §17. Los cambios frontend preexistentes siguen locales y sin commit. No se ejecutó migración, sandbox, publicación o despliegue.
- Siguiente fase: KYC-09 — persistencia durable y contrato unificado. No se ha integrado a `main` ni confirmado despliegue.

## 17. Ejecución correctiva KYC-08

- Se cerró localmente KYC-08 en la rama `codex/marketplace/fix/kyc-06-save-identity-first`, con evidencia en `evidencia_kyc08_20261107.md`. La evidencia cubre pruebas Go locales con secretos sintéticos, no Didit real, persistencia ni despliegue.
- Corregidos en fuente: propiedad/lectura cruzada en el handler actual, rechazo por secreto/timestamp ausente, malformado o vencido, correlación exacta sin asignación oportunista, no aprobación sin `decision`, fallo cerrado de configuración/proveedor y nil-safe de `EvaluateDecision`. Firma V2 canonicalizada, raw-body fallback completo y header de secreto anterior opcional tienen pruebas sintéticas; la frescura se valida tanto en header como en body.
- Pendientes: secreto runtime externo y rotación no ejecutados; falta contrastar canonicalización con entrega/vector real sandbox. Intentos y webhook siguen en memoria; persistencia durable, inbox e idempotencia son KYC-09/11, no producto listo.
- KYC-09 discovery se amplía por el contrato candidato V85: el handler Didit vigente sigue usando `memoryAttempts`; inicia la llamada externa antes de guardar el intento y no inyecta repositorio para V84/V85. V85 define snapshot cifrado/eventos mínimos pero no conecta runtime. `declarationcrypto.Keyring` implementa AES-256-GCM con AAD y carga local 0600; no es gestión de claves de despliegue ni rotación operativa. Ver `../gcp/contrato_v85_identidad_20261107.md` para interfaz y secuencia propuesta. V85 bloquea si tablas V84 tienen filas y revoca runtime sobre ellas: compatibilidad condicionada, no upgrade universal.
- Subgate documental de contrato KYC-09: **cerrado para diseño**, sincronizado con el esquema V85 y el estado real del handler/API (sin repositorio V84/V85 durable); fija brechas de ID/DTO, declaración-first y URL de sesión, más autoridad por destino y compatibilidad legacy. La implementación Go/OpenAPI, persistencia, inyección y pruebas sigue abierta. No implica que V85 esté desplegada ni que el consumidor servido sea compatible.
- Estado KYC-09: **parcial/abierto**. Contrato de persistencia y evidencia desechable no satisfacen implementación Go, API coherente, commit→readback independiente, reanudación post-reinicio, idempotencia antes de Didit ni pruebas DB autorizadas. V84 permanece byte a byte; la política de release sin cambio. Cloud SQL fue leída en READ ONLY (V1–V83 exactas/V84 pendiente); el snapshot nativo sigue V76 y es anterior. No se aplicó DDL a ninguno de los destinos.
- Siguiente paso: implementar por cortes pequeños el repositorio/servicio Go y contrato OpenAPI contra el diseño revisado, usando solo fixtures sintéticos/harness aislado; probar save→GET autenticado, persistir intento idempotente antes de HTTP externo (fuera de transacción) e inbox antes de 2xx. El ensayo de código no autoriza DDL: usar las tablas V85 solo en pruebas desechables hasta revisar compatibilidad, autoridad y autorización por destino. Sandbox sigue inelegible para sello/beneficio real.

## 18. Avance local KYC-09 (2026-11-08)

- Avance de código en `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8`: códec/AAD, repositorio PostgreSQL V85, servicio de declaración con lectura exacta post-commit, handler HTTP opt-in y contrato OpenAPI generado. La evidencia detallada, comandos/resultados y límites está en `evidencia_kyc09_20261108.md`.
- Verificado localmente: pruebas Go con race y vet, integración de servicio/repositorio y V86 en el runner PostgreSQL desechable (117 resultados requeridos, ninguno omitido), y generación/check OpenAPI. El runner usó un contenedor propio efímero; esta fase no consultó ni modificó destinos persistentes.
- **KYC-09 permanece parcial/abierto**: `cmd/api` aún no inyecta el servicio. A solicitud expresa del usuario se creó el copy candidato y la versión `id-consent-20261108` (19 caracteres), con retención propuesta de 30 días más purga dentro de 24 horas; la propuesta requiere revisión legal/privacidad. V86 implementa la primitiva durable de purga (máximo 100 filas, referencias protegidas y runtime sin `DELETE` directo), pero no hay scheduler/worker ni política activa y no se probó en destino persistente. El usuario aceptó cinco años para los metadatos mínimos del ledger de auditoría. La auditoría atómica, su rollback y POST→GET HTTP→PostgreSQL se prueban solo en el harness desechable. No se ejecutó UI, Didit, publicación ni despliegue.
- No adelantar KYC-12 ni declarar “Datos guardados” en una interfaz hasta revisar legalmente el copy, definir y verificar por separado la invocación programada/acotada de la purga, montar Go con configuración aislada y comprobar lectura/guardado bajo la misma cuenta. No truncar ni reutilizar el token antiguo del formulario; el copy/version candidato está en `consentimiento_declaracion_identidad_v1_20261108.md`. No aplicar DDL persistente; V86 es append-only y solo se probó en harness desechable.
- Siguiente paso: en un corte separado, decidir y probar la invocación local acotada de V86, luego configurar/injectar el servicio y probar reinicio/readback. Revisar el consentimiento y la política con asesoría de privacidad antes de mostrar a usuarios. KYC-10/11 continúan bloqueadas hasta completar KYC-09.

## 19. Addendum de avance KYC-09 — 2026-11-08

- Actualiza solo el estado operativo de KYC-09; §18 se conserva como snapshot previo. En el checkout `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8`, se añadió al código del worker existente una llamada condicional a `PurgeExpiredRevisions`, limitada a 100 filas por ciclo y solo cuando `DATABASE_SET_ROLE=espacigo_privacy_cleanup_worker`. No cambia el intervalo ni el ciclo existente y no demuestra una garantía de eliminación en 24 horas.
- V87 agrega el `EXECUTE` directo mínimo para el rol de limpieza y está registrado como `security-tightening`, `automatic: false`, review-required. V86/V87 solo se probaron en harness efímero; ninguna se aplicó a PostgreSQL persistente o Cloud SQL. No se ejecutó el worker, `cmd/api`, Didit ni despliegue.
- Pruebas de la slice worker/V87 registradas: `bash scripts/test-migration-runner-postgres.sh full` PASS; `GOTOOLCHAIN=local go test -count=1 ./cmd/privacy-cleanup-worker` PASS; `GOTOOLCHAIN=local go vet ./cmd/privacy-cleanup-worker ./internal/migrator` PASS; `python3 scripts/gcp/test_check_migration_ci.py` PASS (4 pruebas). El harness no establece estado de ningún destino persistente.
- **KYC-09 continúa parcial/abierto**: `cmd/api/main.go` sigue montando `verificationhttp.NewHandler(...)`, no `NewHandlerWithDeclaration`; no existe keyring runtime configurado para esa ruta. Falta decidir el gate de activación, realizar prueba integrada en entorno local aislado autorizado y demostrar reinicio real del proceso API con readback. No mostrar el formulario ni recibir datos personales hasta cerrar la revisión legal/privacidad del consentimiento y retención candidata.
- Siguiente orden seguro: revisar legal/privacidad antes de activar o mostrar el consentimiento; completar después la inyección de servicio/keyring y sus pruebas de runtime bajo entorno aislado autorizado. La invocación de purga está implementada en código, pero cualquier ejecución de worker o aplicación persistente de V86/V87 requiere sus gates y autorización exactos. KYC-10/11/12 no se declaran iniciadas ni completadas por este addendum.

### Revalidación local del addendum — 2026-11-08

- En el mismo checkout/HEAD base, después de actualizar la evidencia, se repitieron `GOTOOLCHAIN=local go test -count=1 -race ./internal/verification/... ./internal/adapters/postgres/identitydeclaration`, `GOTOOLCHAIN=local go vet ./internal/verification/... ./internal/adapters/postgres/identitydeclaration ./internal/migrator`, `GOTOOLCHAIN=local go test -count=1 ./cmd/privacy-cleanup-worker`, `GOTOOLCHAIN=local go vet ./cmd/privacy-cleanup-worker ./internal/migrator`, `python3 scripts/gcp/test_check_migration_ci.py` y `bash scripts/test-migration-runner-postgres.sh full`; todo terminó con exit 0. Python: 4 pruebas. Harness: 120 resultados DB requeridos, ninguno faltante/omitido; V1–V87 fresh y upgrades V75/V76 a V87 en contenedor efímero con red/puertos/volúmenes externos deshabilitados; retiro del contenedor propio verificado. `git diff --check` también pasó.
- Esta revalidación no ejecutó `cmd/api` ni el worker y no aplicó DDL a destinos persistentes; no cierra el gate legal, runtime/readback ni KYC-09.

### Preparación del gate legal/privacidad — 2026-11-08

- Se preparó `revision_legal_privacidad_20261108.md` como matriz de preguntas/evidencia para asesoría, no como dictamen ni aprobación. Contrasta la versión actual de Ley 19.628 (hasta 2026-11-30) con la versión modificada por Ley 21.719 cuya vigencia comienza 2026-12-01, y registra el alcance transitorio de Ley 21.806 sin atribuirle cambios a las reglas sustantivas de consentimiento.
- El checklist pide resolver responsable/contacto y política enlazada, licitud/necesidad por campo, formalidad/evidencia/revocación del consentimiento, separación de proveedor/biometría, 30 días/24 horas, backups, metadatos auditables cinco años, derechos y menores. No cambia el copy ni ratifica ninguna decisión.
- Gate humano sigue abierto. No activar ni mostrar el consentimiento, recibir datos personales, ejecutar worker, exponer la ruta, aplicar V86/V87 a un destino persistente, crear sesiones Didit o publicar/desplegar sin las revisiones/autorizaciones correspondientes.

### Regresión del estado API deshabilitado — 2026-11-08

- Se añadió `TestIdentityDeclarationRouteFailsClosedWhenServiceIsNotConfigured`: con principal autenticado, GET y POST por el handler estándar devuelven 503 `service_unavailable`, `Cache-Control: no-store` y no reflejan el cuerpo. El punto de montaje actual en `cmd/api/main.go` usa `verificationhttp.NewHandler(...)`, que deja el servicio de declaración nil.
- `GOTOOLCHAIN=local go test -count=1 ./internal/verification/transport/http -run '^TestIdentityDeclaration(RouteFailsClosedWhenServiceIsNotConfigured|RoutesRequireAuthenticatedHandlerConfiguration)$'` — PASS. Es una prueba del handler/contrato local, no una ejecución de `cmd/api` ni prueba de runtime/proceso.
- Esto refuerza el fail-closed mientras sigue pendiente la revisión legal; no cierra la inyección/keyring ni readback tras reinicio.
- Tras añadirla, se repitieron `GOTOOLCHAIN=local go test -count=1 -race ./internal/verification/... ./internal/adapters/postgres/identitydeclaration` y `GOTOOLCHAIN=local go vet ./internal/verification/... ./internal/adapters/postgres/identitydeclaration ./internal/migrator`; ambos PASS. `git diff --check` PASS.

### Hallazgo de compatibilidad de retención — 2026-11-08

- Revisión estática de V85–V86 confirma que V85 hace `identity_provider_attempt.declaration_id NOT NULL` con FK a la declaración y V86 omite toda declaración referenciada por cualquier intento; el worker no purga intentos. Las pruebas V86 esperan explícitamente conservar una declaración referenciada. Por tanto, si KYC-10 crea un intento para una revisión, el plazo candidato de «30 días + eliminación en 24 horas» no está garantizado mientras esa referencia persista.
- No se cambió la migración ni el copy: la solución afecta retención/anonimización de intentos y snapshots y requiere decisión legal/producto. Resolver antes de KYC-10 y antes de aprobar el copy; no aplicar `CASCADE` ni ampliar retenciones por inferencia.

### Revisión estática de privilegio de purga — 2026-11-08

- V86 concede `EXECUTE` de la función `SECURITY DEFINER` tanto a `espacigo_runtime` como al rol de limpieza; V87 conserva ambos grants. En el código revisado, el único consumidor de producción es `cmd/privacy-cleanup-worker`, que asume el rol dedicado. La prueba V86 también valida explícitamente que runtime puede ejecutar la función.
- La función no recibe parámetros y solo borra revisiones vencidas, no referenciadas y en lotes máximos de 100; no hay `DELETE` directo para runtime. Queda como hallazgo de mínimo privilegio: confirmar si el permiso de runtime es deliberado/necesario; si no, preparar una nueva migración append-only review-required para revocarlo. No se alteraron V86/V87 ni se cambió ningún ACL de destino.

## 20. Propuesta de tratamiento de medios Didit — 2026-11-08

- El propietario del producto especificó que EspaciGo no conserva fotos de identidad y que Didit las recibe/retiene. Se registra como intención de producto; la configuración efectiva de la cuenta/workflow Didit no se leyó ni verificó.
- Se crearon `propuesta_retencion_kyc09_kyc10_20261108.md` y `aviso_y_consentimiento_didit_v1_20261108.md` con una propuesta para separar la retención local de la del proveedor, un aviso/consentimiento KYC-10 separado y decisiones explícitas para salidas multimedia, webhooks, borrado, auditoría, biometría y entrenamiento. Sus afirmaciones del proveedor están citadas a documentación oficial; los borradores no son dictamen ni aprobación.
- Propuesta para revisión, no valor activo: declaración local hasta 30 días desde guardado y purga dentro de 24 horas; los intentos/eventos locales que la referencien deben expirar y eliminarse en el mismo límite. Para Didit se propone una retención finita por sesión, como máximo 30 días desde su creación, con solicitud de borrado anterior al cerrar/revocar. El plazo público por aplicación y el modo `Returned data` deben verificarse en la app/contrato antes de mostrar copy; no se accedió a la consola ni se cambió configuración.
- El handler actual examinado lee el webhook acotado en memoria y decodifica campos permitidos; esta inspección estática no verifica middleware, logs, almacenamiento operativo ni configuración de todos los procesos. KYC-10 debe probar que no se guardan medios en inbox/DB/logs/telemetría/backups.
- Esta propuesta no resuelve la FK no nulable de V85 ni la exclusión de V86. Para cumplir la retención local se necesita diseño versionado append-only y pruebas para intentos, eventos, borrado remoto y webhooks tardíos. No se modificaron migraciones ni código, no se inició una sesión Didit y no se ejecutó DDL/worker/API por esta tarea.
- **Gate humano permanece pendiente:** revisar el plazo de proveedor/local, el consentimiento biométrico, la configuración de medios y usos secundarios, la relación contractual/transferencias, el audit log vinculable y el borrado. KYC-09 runtime y KYC-10 no quedan desbloqueadas por crear estas propuestas.

## 21. Lectura read-only del workflow y borrado tras verificación — 2026-11-08

- Este addendum complementa §20, que registra el estado antes de la consulta read-only. `evidencia_workflow_didit_20261108.md` conserva el resultado resumido y su alcance; no contiene URL temporal ni secretos.
- Didit MCP listó 7 workflows en 2 aplicaciones. El `Compliance workflow` v1 publicado está en `Hotusoft (Sandbox)`, no es editable; `Free KYC` aparece como default en `My Application` pero en estado `draft`. No se leyó `DIDIT_WORKFLOW_ID`, así que el vínculo con el proceso EspaciGo y la existencia/configuración live no quedan demostrados.
- El grafo sandbox es OCR→LIVENESS `ACTIVE_3D`→FACE_MATCH→`Determine`, con `expected_details_mismatch_action=REVIEW`. Para CHL admite P/DL/ID/RP, no solo ID; las salidas son `response_attributes=null` (todos los puntos de datos). Además, el objeto completo indica `is_liveness_enabled=false` y `is_face_match_enabled=false`, discrepante con los nodos/features del grafo. Resolver selección de documentos, flags y salidas antes de uso live.
- `session_expiration_time=604800` segundos (7 días), `max_retry_attempts=3` y `retry_window_days=7`; la expiración de sesión no es borrado/retención de medios. El endpoint de workflow no reveló la retención de la aplicación Didit.
- Nueva instrucción de producto: conservar imágenes/datos solo durante la comprobación; cuando el backend reconcilie desde fuente autoritativa y persista `VERIFIED`, solicitar borrado inmediato de la sesión en Didit, eliminar la revisión/snapshot cifrado local y mantener solo estado mínimo verificado. No borrar al retorno del navegador ni a un webhook aislado. Si queda pendiente, cerrar por vencimiento aprobado y solicitar borrado; un outbox temporal y la gestión de webhooks encolados deben asegurar el borrado sin retener el snapshot.
- V85 mantiene `declaration_id NOT NULL` y V86 protege cualquier declaración referenciada. La transición de éxito a estado mínimo sin declaración requiere diseño/migración append-only y pruebas de expiración, FK, borrado remoto, reintentos y webhook tardío. No se tocó la configuración Didit, código, migraciones ni base; no se creó sesión.
- Estado: evidencia de workflow preparada; propuesta de retención/copy actualizada. Revisión legal/privacidad, semántica de flags con Didit, retención de aplicación, contrato/DPA y lectura del selector runtime siguen pendientes; KYC-09 runtime/KYC-10 permanecen bloqueadas.

## 22. Aclaración de producto: cotejo con formulario y RUT único — 2026-11-08

- El propietario confirma que el formulario de identidad es la base prevista para la comprobación: Go deberá enviar por sesión como `expected_details` la declaración guardada; Didit coteja identidad/documento y Go evalúa la decisión junto con liveness y face match. El backend deberá marcar `VERIFIED` únicamente si coinciden los datos requeridos del formulario y pasan todos los controles obligatorios. El `REVIEW` observado es la acción configurada ante discrepancias; no sustituye este contrato ni significa que falte el cotejo.
- Si hay discrepancia, no se concede `VERIFIED`; se pide corregir/reverificar desde la cuenta autenticada después de reconciliar el intento anterior. No crear sesiones repetidas a ciegas ni permitir que un intento viejo apruebe datos de una nueva revisión.
- El RUT es el identificador único de la identidad en este formulario. Antes de imponer unicidad entre cuentas, auto-vincular cuentas o revelar que existe otra cuenta, falta cerrar la política exacta de duplicado/recuperación y el diseño de mínimo dato (propuesta técnica: índice HMAC versionado con clave separada, nunca índice/hash simple del RUT). El RUT no sustituye autenticación ni prueba de control de una cuenta.
- Esta aclaración alinea el flujo con el contrato ya descrito en `adr_001_contrato_identidad_didit.md` y §§4, 6 y 10 del plan; no cambia el estado de implementación ni los gates legal/persistencia/proveedor. El runtime no se probó, no se creó sesión y no se cambió el workflow.

## 23. Unicidad de RUT en formulario y verificación — 2026-11-08

- **Regla de producto aclarada por el propietario:** una cuenta puede tener un solo RUT y un RUT no puede quedar vinculado a más de una cuenta. La unicidad se valida al guardar/registrar el formulario y vuelve a comprobarse antes de iniciar/reconciliar la verificación. Una colisión con otra cuenta rechaza el registro; el mismo RUT dentro de su cuenta existente permite reanudar o reverificar después de reconciliar el intento anterior. Nunca crear una segunda cuenta ni reasignar una identidad por semejanza de datos.
- **Diseño recomendado:** normalizar y validar el RUT con DV en Go; derivar un fingerprint HMAC versionado con una clave dedicada, separada de cifrado, credenciales y webhook. Persistir el fingerprint en un registro privado con unicidad global del fingerprint y unicidad por cuenta, actualizado transaccionalmente junto con la declaración. No indexar/loguear el RUT en claro ni usar SHA simple. La restricción de DB es la autoridad contra carreras; una consulta previa de disponibilidad solo mejora UX y no sustituye el `INSERT`/constraint.
- Reservar el fingerprint solo tras autenticación de la cuenta y consentimiento requerido. Una reserva aún no verificada debe tener vencimiento y liberación idempotente después de cerrar/expirar el intento, para que una declaración abandonada o fraudulenta no bloquee indefinidamente al titular real. Al verificar, revalidar que el fingerprint pertenece a esa misma cuenta y que la revisión no fue sustituida. Una sesión solo se inicia desde el snapshot servidor guardado; no confiar en RUT/campos reenviados por el cliente.
- **Cotejo:** el texto del formulario se compara con los campos OCR/documentales de la decisión usando `expected_details` de esa revisión; face match compara la selfie con el retrato del documento y liveness comprueba la captura. No son la misma comparación. Go concede `VERIFIED` solo tras decisión autoritativa completa, documento `ID`/emisor esperado, coincidencias de RUT+D.V., fecha/nombres según reglas ratificadas, liveness y face match aprobados, sin warnings críticos ni intento/app/workflow/version cruzados. Un mismatch queda no verificado; permitir corregir y reverificar en la misma cuenta solo después de reconciliar el intento previo, sin sobrescribir su snapshot ni crear sesiones a ciegas.
- **UX y privacidad:** el propietario propuso «Este RUT ya está registrado, ingrese otro». Recomiendo no pedir “ingrese otro RUT”: puede facilitar enumeración de cuentas y animar a usar el RUT de otra persona. Mensaje preferido: «No se pudo registrar este RUT. Si corresponde a tu identidad, inicia sesión en la cuenta asociada o contacta a soporte». Limitar frecuencia, no confirmar qué cuenta contiene el RUT y ofrecer recuperación desde un canal autenticado. La redacción final queda sujeta a revisión legal/privacidad.
- **Implicación de retención:** borrar la declaración cifrada, imágenes y datos de proveedor tras `VERIFIED` no borra el fingerprint HMAC si se necesita impedir que el RUT se vincule a otra cuenta. Ese fingerprint sigue siendo un identificador seudónimo y requiere finalidad, plazo, acceso, rotación y supresión aprobados; si se elimina, ya no puede garantizarse la unicidad histórica. Resolver esta retención en `kyc09-legal` antes de activar la regla.
- **Estado de esquema:** V85 guarda la declaración cifrada y su relación con `account_id`; no contiene índice de unicidad de RUT. V000001 hace único el correo normalizado, no el RUT. Cualquier registro/fingerprint requiere diseño de migración append-only en la siguiente versión libre **después** de reconciliar el inventario/ledger del destino; no modificar ni aplicar V1–V87 desde esta propuesta.
- **Cobertura local futura:** pruebas sintéticas de mismo RUT/misma cuenta (reanudación idempotente), mismo RUT/cuentas distintas (conflicto sin sesión), dos altas concurrentes (solo una gana), normalizaciones equivalentes, expiración/liberación de reserva, reintento legítimo, mismatch de OCR/RUT, face match/liveness fallidos, rotación de HMAC y ausencia del RUT/fingerprint en logs. Ninguna se ejecutó en este cambio documental.

## 24. Flujo consolidado del formulario, unicidad RUT y resultado Didit — 2026-11-08

Este addendum reúne el flujo UX/API y las mejoras propuestas en §§21–23. Es el contrato objetivo; no certifica que esté conectado en runtime ni reemplaza los gates legales, de persistencia, proveedor o producto.

### Recorrido usuario → servidor → Didit

1. **Abrir verificación desde Mi cuenta.** Mostrar el estado propio que entrega el backend y «Verificar identidad». No usar nombre público/display name como identidad legal. El formulario chileno del MVP admite cédula `ID`; el workflow sandbox permite más tipos, así que la restricción del MVP debe aplicarse también en Go y verificarse en el workflow runtime antes de habilitarlo.
2. **Validar y guardar formulario (KYC-09).** En sesión autenticada, validar nombres, fecha, país/tipo y RUT/DV en Go; normalizar el RUT antes de calcular su fingerprint. Guardar declaración cifrada, consentimiento/versionado y vínculo de cuenta. Esta acción no llama a Didit ni crea sesión. Confirmar el commit con lectura servidor-autoritativa; solo entonces mostrar «Datos guardados» y habilitar la segunda acción.
3. **Comprobar unicidad al guardar.** Consultar si el RUT normalizado está asociado a otra cuenta para dar respuesta rápida, pero hacer cumplir la regla con una restricción única de DB en un fingerprint HMAC versionado, separado de la clave de cifrado y de secretos Didit. La inserción/transacción debe ganar o perder atómicamente ante dos altas simultáneas. Una colisión de otra cuenta rechaza el guardado y no crea sesión; la misma cuenta puede continuar/reverificar con su RUT tras reconciliar el intento anterior. No reasignar ni enlazar cuentas por semejanza.
4. **Iniciar verificación (KYC-10) como acción separada.** Go carga la declaración guardada propia; no acepta que el navegador reemplace RUT/nombres/fecha, cuenta, workflow o entorno. Revalida que el fingerprint de RUT siga perteneciendo a esa cuenta y revisión; persiste/idempotentiza el intento antes de la llamada externa y llama a Didit fuera de la transacción. Envía `expected_details` desde el snapshot inmutable de esa revisión. No enviar PII en `vendor_data`, URL, logs ni analítica.
5. **Captura alojada de Didit.** El proveedor procesa documento, liveness y face match según el workflow/version aprobados. El navegador vuelve a una pantalla «Estamos comprobando»; query/callback del browser no autoriza el estado ni dispara borrado.
6. **Cotejo de backend (KYC-11).** `expected_details` compara lo declarado con los atributos OCR/documentales. Face match contrasta selfie con retrato del documento; liveness es otro control. Go consulta/reconcilia la decisión autoritativa, valida aplicación/entorno/workflow/versión/nodos/estado, tipo `ID` y país `CHL`, coincidencia exacta del RUT y su dígito verificador (DV), fecha/nombres según reglas aprobadas, liveness y face match aprobados, y ausencia de warnings críticos, datos faltantes o intento superado. `expected_details_mismatch_action=REVIEW` es una ruta de revisión ante discrepancias, no un reemplazo de la compuerta Go.
7. **Resultado.** Solo si la reconciliación completa corresponde a la revisión actual, Go persiste el estado `VERIFIED` y la UI muestra «Identidad verificada». Ante mismatch, el estado no queda verificado: explicar que debe corregir/reverificar, permitir hacerlo desde la misma cuenta tras cerrar/reconciliar el intento previo y crear una nueva revisión/intent idempotente. Nunca sobrescribir el snapshot usado por una sesión ni iniciar sesiones repetidas a ciegas. El retorno puede quedar pendiente si webhook/consulta falla; la insignia lee únicamente estado actual del servidor.

### Unicidad, mensaje y datos mínimos

- Aplicar la misma normalización de RUT en guardado, chequeo de duplicado, creación/reanudación de intento y cotejo contra lo extraído del documento. Un RUT de otra cuenta se rechaza antes de llamar a Didit; el usuario dueño puede reanudar o volver a verificar dentro de su cuenta. Una corrección de RUT no es «ingresar otro» para pasar la comprobación; requiere resolver titularidad/cuenta por el flujo autorizado.
- El propietario pidió el mensaje «Este RUT ya está registrado, ingrese otro». Como opción preferida de privacidad/producto, usar una respuesta que no confirme qué cuenta contiene el RUT ni aliente a probar otro identificador: «No se pudo registrar este RUT. Si corresponde a tu identidad, inicia sesión en la cuenta asociada o contacta a soporte». La decisión/copy final queda para revisión legal/privacidad; cualquier respuesta de conflicto debe ser uniforme, rate-limited y no incluir RUT/cuenta en logs.
- Tras `VERIFIED`, borrar la declaración/snapshot cifrado y solicitar de inmediato el borrado de la sesión Didit. Conservar solo estado de verificación y metadatos aprobados. Para seguir imponiendo unicidad puede ser necesario mantener un fingerprint HMAC del RUT; es identificador seudónimo, no anónimo. Su finalidad/plazo/supresión requieren aprobación legal. Si la política exige borrar también ese fingerprint, queda explícito que no se podrá prevenir la reutilización histórica tras el borrado.
- Para reservas `PENDING` del fingerprint, fijar vencimiento, liberación idempotente y protección contra abuso; no dejar que un intento abandonado bloquee indefinidamente al titular. No exponer el `EXISTS` de otra cuenta a una API pública sin decisión de producto/seguridad.

### Workflow observado y límites

- La lectura read-only identificó `Compliance workflow` v1 publicado en `Hotusoft (Sandbox)`, no editable; el vínculo con `DIDIT_WORKFLOW_ID` de runtime no está verificado. El workflow configurado devuelve todos los puntos con `response_attributes=null`; la respuesta/webhook puede incluir medios. Los flags `is_liveness_enabled=false` y `is_face_match_enabled=false` contradicen los nodos. El workflow admite P/DL/ID/RP para CHL, aunque el contrato del MVP es solo ID.
- No cambiar ni publicar el workflow desde este plan. Antes de uso live, resolver app/workflow/version runtime, discrepancia de flags, documentos permitidos, salidas multimedia y retención por aplicación. Probar que EspaciGo descarta imágenes en HTTP/middleware/inbox/DB/logs/telemetría/backups; un TTL de sesión de 7 días no demuestra borrado.

### Persistencia, aceptación y gates

- **Brecha actual:** el handler estático de inicio Didit construye `expected_details` a partir de campos de la petición (`internal/verification/transport/http/handler_didit.go:50-100`); no demuestra el recorrido declarado guardado→readback→inicio. `cmd/api` y el runtime no han probado esa vinculación. Corregirla en KYC-10 solo después de cerrar los gates de KYC-09.
- V85 no tiene fingerprint único de RUT; V85 fija `declaration_id NOT NULL` y V86 excluye revisiones referenciadas. La purga tras verificación y la unicidad persistente requieren rediseño append-only de claim/snapshot/intento/outbox y transiciones; no editar/aplicar V1–V87. Elegir una futura versión solo después de reconciliar el inventario/ledger exacto y obtener autorización para el destino.
- Criterios de aceptación local sintética: mismo RUT en la misma cuenta permite reanudar; mismo RUT en cuentas distintas produce conflicto sin crear sesión; dos altas concurrentes dejan un solo registro; formatos equivalentes de RUT convergen; un RUT/nombre/fecha discordante, nodo faltante, warning crítico, face match/liveness fallido o decisión parcial nunca producen `VERIFIED`; una revisión nueva no hereda una aprobación vieja; tras `VERIFIED` se purga el snapshot y se prueban la solicitud/reintentos de borrado remoto sin guardar medios; webhooks tardíos o duplicados no restauran el estado.
- **Resoluciones Aprobadas por el Responsable del Proyecto (2026-11-08 — Gates Legales y de Producto Cerrados):**
  1. **Edad mínima:** estrictamente mayores de 18 años cumplidos; si no cumple, se rechaza de inmediato (`422` / `ErrInvalid`) sin llamar a Didit ni persistir datos, purgando cualquier dato del intento.
  2. **Retención y conflicto V85/V86:** se adopta la Opción 1 con cascada controlada transaccional en el worker (máximo absoluto de 30 días desde `consent_given_at` y purga en 24 h expirando y borrando en orden `identity_provider_event` → `identity_provider_attempt` → `identity_declaration_revision` mediante migración append-only sin `CASCADE` implícito, y borrado inmediato del snapshot local + solicitud de borrado de sesión Didit vía outbox al alcanzar `VERIFIED`).
  3. **Fingerprint HMAC del RUT:** se conserva el `HMAC-SHA256` del RUT normalizado post-`VERIFIED` y post-cierre de cuenta bajo interés legítimo de prevención de fraude/unicidad histórica (1 RUT por cuenta), acceso exclusivo backend, rotación anual de `key_version` y sin RUT en claro.
  4. **Permiso de purga:** se revoca `EXECUTE` a `espacigo_runtime` sobre `public.purge_expired_identity_declaration_revisions()`, dejándolo exclusivamente en `espacigo_privacy_cleanup_worker` (mediante migración append-only).
  5. **Auditoría (5 años) y Consentimientos:** quedan aprobados en firme los textos definitivos de consentimiento KYC-09 (`id-consent-20261108`) y KYC-10 (`didit-consent-v1`) y los metadatos mínimos de `evento_auditoria_local` por 5 años sin PII ni ciphertext.
  6. **Cotejo de RUT y Nombres (KYC-10/11):** extracción del RUT del campo estándar de documento nacional chileno devuelto por Didit (`ID`, `CHL`), normalización y validación Módulo 11 en backend; casos ambiguos en RUT o coincidencia parcial de nombres no se aprueban automáticamente y pasan fail-closed a revisión/rechazo controlado sin usar `chl_rut` de pago en este MVP.
  7. **Workflow de Runtime y Reintentos:** contrato estricto para runtime (cédula chilena `ID` vigente, `OCR + LIVENESS + FACE_MATCH` obligatorios, `expected_details_mismatch_action=REVIEW`, descarte en memoria de medios y sin retención para entrenamiento del proveedor ni plantillas biométricas persistentes) y TTL determinista de 30 minutos para sesiones colgadas antes de permitir nuevo intento idempotente.
  8. **Autorización Operativa Explícita:** autorizada la implementación y ejecución local de **KYC-09 runtime** (`cmd/api`, keyring, persistencia y pruebas sintéticas locales) y el desarrollo de **KYC-10/11** con mocks/stubs locales sin consumir cuota real de Didit.
- **Estado KYC-09 Runtime (2026-11-08):** implementado y verificado localmente en Go y PostgreSQL efímero (`ValidateAdultBirthDate` 18+ en `internal/verification`, `newRuntimeIdentityDeclarationService` y `NewHandlerWithDeclaration` en `cmd/api`, keyring 0600 con rotación `v1`→`v2`, consentimiento `id-consent-20261108`, retención de 30 días y pruebas de guardado→reinicio→readback en `cmd/api/identity_declaration_runtime_test.go` y `internal/migrator/identity_schema_v85_test.go`). Ver detalle en `evidencia_kyc09_20261108.md`.
- **Estado KYC-10 / KYC-11 / KYC-12 y Contrato Frontend (2026-11-08):** implementados y verificados localmente con mocks/stubs y en PostgreSQL efímero sin consumir cuota de Didit (`internal/verification/didit.go`, `internal/verification/didit_service.go`, `internal/verification/didit_service_test.go`, `internal/adapters/postgres/identitydeclaration/provider_attempt.go`, `internal/migrator/identity_schema_v85_test.go`, `internal/verification/transport/http/handler_didit.go`, `cmd/api/identity_declaration_runtime.go`, `frontend/src/features/verifications/verification-api.ts`, `frontend/src/features/verifications/verification-flow.tsx`, `frontend/src/features/account/ui/account-verification-card.tsx`, `frontend/src/app/(workspace)/mi-cuenta/verificaciones/page.tsx`, `frontend/src/app/verificaciones/retorno/page.tsx` y `frontend/tests/phase6-account.spec.ts` — 29 pruebas Playwright E2E + Axe en verde y 120 pruebas DB en harness efímero). Ver detalle en `evidencia_kyc10_kyc11_20261108.md`.
- **Estado KYC-13 Revalidación Integral Local (2026-11-08):** completada en entorno local aislado sin llamadas al proveedor real; matriz `criterio -> test -> capa/entorno -> SHA -> resultado` (C01–C11) y cierre local de brechas `R01–R14` documentados en `evidencia_kyc13_20261108.md`. El ensayo con sandbox real externo, la futura migración append-only (`V000088`) y la publicación/despliegue (`KYC-14` / `R15`) permanecen detenidos hasta autorización expresa.

## 25. Ejecución sobre una única base local — autorización y revisión vigentes

Fecha local verificada: **2026-11-08**. Los nombres de evidencias creadas previamente con fecha 20261109 se conservan; no representan una consulta futura ni cambian la fecha local. HEAD base del árbol con modificaciones: `274f37e483a078890e92382a159fa49fd23c2fe8`, rama `codex/marketplace/fix/kyc-06-save-identity-first`.

### 25.1 Alcance autorizado y simplificación

- Único destino DB de desarrollo e integración KYC: **`127.0.0.1:5432/espacigo_dms_stage_20261020`**, socket `/var/run/postgresql`, OID observado `16388`, motor observado PostgreSQL 18.6. Confirmar identidad antes de escribir.
- Instrucción humana vigente: se pueden escribir, borrar y modificar los datos y schema necesarios de esta base local, crear datos sintéticos y continuar por etapas para completar KYC. No volver a tratar el permiso DML/DDL local como bloqueado; esta instrucción sustituye las prohibiciones anteriores para este destino.
- No crear/usar bases efímeras ni repetir fresh-install/upgrade en otro PostgreSQL como gate de este desarrollo. Mantener tests históricos, unitarios y de contrato; las pruebas reales nuevas se ejecutan directamente en esta base con fixtures identificables y cleanup limitado. Una comprobación ya vigente no se repite salvo cambio de código/esquema o necesidad de verificar un efecto.
- Preparar política privada nueva del corte si falta la histórica; no pedir al usuario restaurarla para avanzar. Cuentas y datos sintéticos propios, sin modificar cuentas reales/ajenas ni exponer credenciales. Backup privado validado antes de DDL o cambios destructivos. Usar migraciones versionadas append-only y no editar ledger.
- La autorización no se extiende a otra base, Cloud SQL/GCP, proveedor externo, personas/documentos reales, publicación, main ni despliegue. No convertir la autorización de desarrollo en aprobación jurídica ni aceptación de producto.

### 25.2 Contraste confirmado y cierres reabiertos

| Área | Estado que sí está demostrado | Falta para cierre nativo |
| --- | --- | --- |
| Esquema local | Preflight read-only: V1–V85, 85/85 nombres/checksums coinciden con fuente; inventario V1–V88 | Ruta revisada para V86–V88 y deltas posteriores necesarios; backup/postcheck |
| Declaración KYC-09 | Servicio, cifrado, repositorio y handler presentes; unitarios/race/vet verdes | Guardar y leer con conexión nueva en base nativa; cuenta propia y limpieza verificada |
| Autenticación DB | Roles y SET ROLE definidos | Login API intentado con passfile por defecto: `fe_sendauth: no password supplied`; no equivale a password rechazado. Resolver sin elevar runtime ni reciclar secretos |
| Unicidad RUT KYC-10 | Runtime usa `NewMemoryRutClaimRegistry`; V85 no contiene claim único | Claim transaccional persistente al guardar y al verificar; carreras/reinicio/expiración |
| Retención KYC-11 | V86 omite revisiones con FK; worker tiene invocación en código; V88 revoca runtime | Borrado tras VERIFIED, cascada controlada de dependencias y reintento durable de borrado remoto; no basta outbox en memoria |
| UI KYC-12 | Flujo de dos acciones y tests con respuestas interceptadas | Recorrido local BFF→Go→esta DB; recarga/reinicio, error y aislamiento; proveedor falso inyectado |
| KYC-13 | Evidencia histórica por capas conservada | Se reabre el cierre integral nativo; mocks y DB efímera no prueban este destino |
| KYC-14 | Sin autorización de publicación/despliegue en este encargo | Gate externo separado; no bloquea terminar desarrollo local |

§9 y §15 contienen cierres históricos por capa, no aceptación integral de este destino. V85 está aplicada localmente; las frases anteriores que la daban por pendiente se conservan como snapshots y no son el estado operativo. §24 contiene decisiones atribuidas a revisiones anteriores; no sustituye evidencia de implementación ni aprobación legal externa.

### 25.3 Tareas pequeñas, dependencias y salida verificable

| Tarea | Fases | Trabajo | Salida / estado |
| --- | --- | --- | --- |
| N01 | Todas | Fijar autorización, destino único, preservar cambios, contrastar plan/fuentes | Revisión inicial completada; brechas confirmadas en §25.5. Desarrollo y aceptación nativa continúan abiertos |
| N02 | 09 | Política sintética propia y acceso local separado runtime/migrador; backup validado | Resuelto para el ensayo: ambos logins reales verificados, passfiles privados separados, backup validado y dos cuentas sintéticas creadas/verificadas por servicio normal e inventariadas. Receipts y credenciales solo privados; límite máximo dos alcanzado |
| N03 | 09 | Ejecutar guardar→conexión nueva→lectura, cifrado, aislamiento y cleanup en DB nativa | Resuelto en base nativa con -race: guardar→pool nuevo→lectura A (200), GET cruzado como B (404 sin PII), relectura A intacta, cleanup por journal previo sin residuo y readback independiente verificado. Cuentas conservadas, ledger88 intacto. Evidencia 2026-11-09 |
| N04 | 02,09,11 | Revisar y aplicar secuencialmente V86–V88 mediante ruta local versionada; cerrar FK/retención con siguiente versión libre si corresponde | Esquema nativo resuelto en secuencia hasta V90 (§25.10, §25.17–18): V86–V88, claims V89 y purga/outbox V90 aplicadas/verificadas; historial inmutable, permisos temporales retirados. No acredita worker programado ni SLA real |
| N05 | 10 | Claim HMAC persistente y unicidad por cuenta/RUT, guardado atómico, concurrencia/reinicio y liberación | Resuelto en base nativa con -race: V89 aplicada secuencialmente (ledger 89 filas, SHA validado), `identity_rut_claim` activa con permisos mínimos runtime, `MarkVerified` con idempotencia, wiring en runtime y prueba nativa de ciclo de vida PASS sobre cuentas autorizadas. Evidencia en `evidencia_native_v89_20261109.md` |
| N06 | 10,11 | Persistir idempotencia/reconciliación, resultado mínimo y outbox de borrado; impedir reactivación por evento tardío | Resuelta en destino nativo con `-race`: rollback inducido íntegro; retry `Duplicate=true`; decisión/claim/evento/outbox/purga atómicos; recuperación en pool nuevo; borrado y ACK sintéticos, retención ≥30 días; readback sin residuos; grants temporales revocados y runtime intacto. Evidencia `evidencia_native_n06_20261109.md`, §25.18 |
| N07 | 12 | Entrada local explícita sin workers generales, auth sintética y proveedor falso sin fallthrough exterior; BFF/Go/DB reales | Resuelta localmente: login HTTPS real con cuentas sintéticas, guardar201/readback200/recarga200, intento falso201/consulta200, A→B→A sin PII cruzada, logout204/sesión401 y cookies retiradas; harness y probe exit0, cleanup por journal y revocación/readback independientes PASS. §25.20 |
| N08 | 12,13 | Recorrido visible completo y regresión nativa por criterio C01–C11; retención solo de fixtures propios | Cerrada localmente en alcance técnico sintético: A–N completan matriz por capas. K supersesión/SANDBOX sin badge LIVE; L retirada de aprobación previa; M nuevo intento tras TTL; N purga real bajo rol worker100+1 con rollback. Ledger91, cleanup/revocación/readback y regresión final PASS. §25.34 y cierre_native_n08_20261109.md. Sin aceptación humana, proveedor externo, SLA, publicación ni despliegue |

Orden N01→N02→N03; N04–N06 por dependencias reales de esquema/contrato; N07→N08 cuando persistencia esté lista. Las auditorías de fuentes pueden realizarse en paralelo; un solo operador escribe en la base. No declarar fases cerradas antes de su salida verificable. Mantener aquí resultados de cada tarea y enlaces a evidencia, sin repetir resúmenes como pruebas.

### 25.4 Ejecución inicial del nuevo alcance

- Autorización/destino único registrados en `AGENTS.md`; política privada de este corte creada y validada con archivo 0600/directorio 0700 e ignorada por Git. No contiene credenciales ni adopta cuentas anteriores.
- Preflight administrativo read-only repetido antes del backup: DB nativa exacta, OID 16388, motor 180006, ledger de 85 filas V1–V85. No se escribió en tablas.
- Backup privado `.local/testing/kyc-native/pre-kyc-v85-20261108.dump`, custom format, 8286348 bytes, 0600 sobre ext4; `pg_restore --list` y parse completo `pg_restore --exit-on-error --file=/dev/null` terminaron exit 0. SHA-256 `ee1827698341516f49877111730cd94749fed0ef5139ccc9b9894a5cae8c457c`. Esto no es un ensayo de restauración ni respalda roles globales del cluster.
- Intento explícito TCP de `espacigo_api_login` con el passfile por defecto, sin leer su contenido: `fe_sendauth: no password supplied`. Es un bloqueo de material de conexión, no de autorización humana ni de esquema; no se cambió contraseña existente ni ACL para esconderlo.
- Auditoría independiente de backend y ruta operacional local en curso; su resultado debe contrastarse antes de aplicar deltas. No se creó base efímera ni se ejecutó Didit externo.

### 25.5 Resultados de auditoría y correcciones priorizadas

Las dos auditorías de fuente terminaron sin cambios ni consultas DB. Su observación de política ausente quedó superada por N02: ya se creó la política propia del corte. No se vuelve a bloquear el trabajo esperando el archivo anterior. V86–V88 no son prerrequisito de guardar/leer declaraciones V85; N03 debe realizarse antes de cambiarlas para no retrasar el primer recorrido nativo.

1. **N06-A, seguridad prioritaria — cerrada en fuente y pruebas unitarias:** eliminado el fallback de `GetSessionDecision` hacia el cuerpo del webhook; sin sesión persistida o con GET fallido no se evalúa/aprueba. Padre corrigió los falsos positivos de fixture y repitió `go test -race -count=1 ./internal/verification/...`, `go vet ./internal/verification/...`, `gofmt -d` y `git diff --check`: PASS. Fixture completo evaluado independientemente como APPROVED; sesión realmente retirada/restaurada, evento FAILED único, claim no verificado y cero borrados comprobados. El mismo evento FAILED se procesa tras recuperar GET; una respuesta autoritativa In Review prevalece sobre webhook Approved. Dos experimentos Go overlay, sin tocar producción, restablecieron el fallback y quitaron el guard de sesión respectivamente: ambas mutaciones fallaron en las aserciones previstas. Sin DB ni proveedor externo; no cierra inbox/reconciliación durable.
2. **N05, unicidad persistente:** `cmd/api/identity_declaration_runtime.go` selecciona `NewMemoryRutClaimRegistry`; `Repository.CreateRevision` guarda declaración/auditoría sin claim RUT. Falta la restricción única global y por cuenta, actualización atómica al guardar, revalidación al reconciliar y expiración/liberación probadas en la base nativa.
3. **N06-B, aprobación atómica:** `SaveAttemptDecisionAndCompleteEvent` persiste intento/evento, pero `MarkVerified` se invoca después y su error se ignora. Claim/estado mínimo/borrado local/outbox necesitan una transición durable transaccional con CAS, no varios efectos sin reconciliación.
4. **N06-C, borrado durable:** `pendingDeletions` es memoria de proceso; tras restart se pierde. V85 conserva FK no nula a declaración y V86 omite revisiones referenciadas. Falta delta append-only y outbox transaccional/idempotente; no afirmar cumplimiento de borrado inmediato ni SLA de 24 h.
5. **N02/N03, conexión y fixture:** el test obliga EUID postgres mientras necesita passfiles de operador para los logins; corregir esa incompatibilidad sin copiar secretos entre usuarios. El preflight peer es administrativo/read-only, no login API demostrado. Revisar también lectura del ciphertext que hoy hace `adminPool` y validar que el UUID provenga de la política privada, no solo `EXISTS(usuario)`.
6. **N04, runner:** `native_local.go` y el supervisor actual tienen ceiling V85; `native_worker_bootstrap.go` es helper sin recogida de catálogo ni wiring CLI. Implementar la ruta V86+ antes de DDL, con autoridad, lock, backup y postcheck. No ampliar el ceiling por un reemplazo numérico ni ejecutar el helper como si fuera un runner operativo.

Los gates de la prueba nativa deben adaptarse al ledger exacto cuando avance después de V85, conservando nombres/checksums y rechazo de drift; aplicar migraciones antes de ese ajuste haría fallar el test por precondición, no por persistencia.

Nota de revisión N06-A: el error delegado `verification not found` durante el reintento era un defecto de fixture: se guardaba una copia del intento, se vaciaba su sesión y luego se restauraba esa misma copia ya vacía. El padre separó el original del intento sin sesión y el reintento pasó sin modificar producción. No se registra aquel error como defecto demostrado del repositorio ni como bloqueo nativo.

### 25.6 Continuación automática por tareas (2026-11-08)

- **Acceso migrador desbloqueado:** `cmd/dbmigrate/native_local.go` designa un passfile privado distinto del default. Se validaron solo sus metadatos (archivo regular propio 0600, directorio 0700), sin imprimir/leer contenido. Autenticación TCP real contra la base exacta, transacción READ ONLY y `SET LOCAL ROLE espacigo_schema_owner` devolvieron `session_user=espacigo_migrator_login`, `current_user=espacigo_schema_owner`. El intento API con ese archivo siguió en `no password supplied`; no se reinició ninguna contraseña.
- Lectura de catálogo cluster: sin sesiones activas de esos dos logins en el momento de consulta; dependencias observadas solo de catálogo compartido. Esto no autoriza cambiar contraseñas ni prueba ausencia de clientes inactivos. Rol cleanup NOLOGIN seguro presente; login worker ausente.
- **Corrección de dependencia N04:** V87 no exige crear el login worker: sus guards admiten ausencia de membresías y solo restringen aristas si existen. No bloquear la aplicación V87 por ese bootstrap; la autenticación/ejecución de un worker posterior sí es tarea distinta.
- En implementación paralela con archivos exclusivos: N05 adaptador de claims y nueva migración V89 (sin aplicar ni afirmar atomicidad al guardar); N03 separación del preflight administrativo de las conexiones API/migrador; N04 supervisor acotado V86–V88 reutilizando migrador versionado y backups. Padre conserva la operación DB y la revisión/pruebas combinadas. No se crean bases efímeras.

### 25.7 Revisión de entregas N03/N05

- El candidato V89 y el adaptador de claims son fuente nueva, **no aplicada ni conectada** al guardado. Revisión parental detectó que `Claim` sobrescribía el error del SELECT con el DELETE posterior, saltándose el INSERT cuando no existía claim; tests iniciales verdes no ejercitaban esa rama. La segunda entrega delegada tampoco corrigió la condición de selección ni agregó cobertura. Padre añadió `rut_claim_transaction_test.go`, observó RED real (`inserts=0 updates=1 commits=1` y conflicto perdido), corrigió la condición a `lookupErr == nil`, comprobó RowsAffected del UPDATE y trasladó cleanup de transacción al helper real. GREEN con fake estricto para INSERT ausente, error SELECT, claim pendiente/confirmado propio, dueño distinto y duplicado 23505. `go test -race -count=1` y vet de adaptador/verificación/cmd/api PASS; DB no ejecutada. V89 sigue pendiente de revisión de permisos, fingerprint/versiones y compatibilidad con borrado del snapshot; no declarar unicidad persistente cerrada.
- La entrega inicial N03 solo cambió validadores TCP y dejó un `t.Skip` incondicional con código muerto detrás. No se aceptó como implementación del preflight. Padre implementó el parser JSON de identidad/read-only/ledger V1–V85 con regresión RED→GREEN (test independiente con `-race`, vet PASS), añadió la invocación externa `sudo -u postgres psql` en READ ONLY y eliminó el pool administrativo de las lecturas de negocio. El ciphertext y la presencia de la cuenta se consultan por el pool runtime; la autenticación real sigue siendo prerrequisito no satisfecho, y el recorrido persistente **no se ejecutó**.
- N04 se redujo a un tracer bullet completo V86 con entrada CLI propia y staging exacto, reutilizando `RunWithRoleInRange(85,86)`. El supervisor Python se descartó antes de crear archivos al no tener una ruta CLI acotada. Implementación de ese CLI en curso; no hay DDL aplicado.

### 25.8 CLI V86: recuperación de entrega parcial y gate de revisión

- La entrega delegada dejó flags/funciones sin completar y el paquete no compilaba. Padre terminó el despacho temprano exclusivo del modo V86, rechazo de flags ajenos/operandos antes de configuración, validación de passfile propio/path/0600/directorio0700, DSN loopback exacta y preflight READ ONLY de OID 16388/motor180006/login migrador/owner/flags/membresía/CREATE requerido. No se usa login simulado ni se concede autoridad automáticamente.
- Pin V1–V86 del snapshot: SHA-256 de filas `version name checksum\n` = `d3ca3619312168dddebe1b7a722252d24381efcfd6bb60349a4dbd86007ec807`. V86 mantiene checksum `af864b793d8f4cbc9cf28dbfe65b8ed6d2388a0ade9a739fc8b480470bd3dc8e`. Staging excluye V87 y posteriores; aplicación por `RunWithRoleInRange(85,86)` y postcheck exacto V86.
- `go test -race -count=1 ./cmd/dbmigrate -run '^TestNativeV86'`, `go vet ./cmd/dbmigrate` y `git diff --check` PASS. Incluye subprocess del CLI que rechaza combinaciones, confirmación incompleta, operandos antes/después y separador `--` antes de leer configuración de conexión inválida.
- **No ejecutado sobre la base:** revisión independiente de esta ruta en curso; falta cerrar sus hallazgos y preparar/revocar la ventana temporal de CREATE con readback y backup. Estos tests no prueban semántica SQL ni permiten afirmar V86 aplicada. V87–V89 siguen fuera de este CLI.

### 25.9 V86 aplicada y verificada en el destino nativo

La entrada anterior conserva el estado previo a revisión. La evidencia vigente es `evidencia_native_v86_20261108.md`: **V86 aplicada**, ledger 86 filas V1–V86, checksum exacto, función del owner con search_path fijo y CREATE temporal retirado comprobado mediante conexión nueva. El primer ensayo falló por un conteo de membresías duplicado e incompleto; no ejecutó SQL de migración y retiró CREATE. Se diagnosticó contra el contrato fuente completo y se corrigió, sin cambiar roles/ledger. El segundo ensayo terminó exit 0.

Hallazgos de revisión tratados: la afirmación de que pgx no consume PGPASSFILE fue refutada con fuente instalada y prueba sintética; los checks de target/OID/identidad se añadieron a la conexión bajo lock y a la transacción, conservando el gate canónico de autoridad. No se usó una credencial alternativa ni se concedió privilegio permanente.

V86 conserva el grant runtime intermedio hasta V88; no se ejecutó la purga ni worker. Backup posterior V86 validado para avanzar a V87. La ruta de una sola versión V87/V88 está en implementación fuente; no saltar directo a V88. El recorrido KYC nativo sigue pendiente y no queda demostrado por aplicar esta función.

### 25.10 V87–V88 aplicadas; N02 desbloqueada y N03 en preparación

Evidencia: `evidencia_native_v87_v88_20261108.md`. Padre completó el tail delegado parcial con staging exacto, flags, checksums/fingerprint y postcheck; los tests focalizados CLI/race/vet/diff-check pasaron. V87 se aplicó sin CREATE ni login worker; backup nuevo y validado; luego V88 con ventana CREATE del owner retirada/verificada. Readback independiente final: ledger88, runtime EXECUTE=false, worker EXECUTE=true, owner CREATE=false, sin PUBLIC en la función. No se invocó purga ni worker.

El login API tenía password sin inicializar (`IS NULL`, sin consultar valor/hash). Se configuró únicamente su primera credencial con guard de no sobrescritura y destino nativo exacto; passfile nuevo propio privado0600 e ignorado. Autenticación TCP real APIlogin→runtime PASS. No se reinició una contraseña existente. Migrador y API usan passfiles explícitos separados.

Se actualizó la prueba al preflight V1–V88 y política con UUID autorizado/verified=true. El preflight por sudo en subprocess falló sin iniciar DML; se sustituyó por el login migrador ya autorizado en READ ONLY. `TestNativeIdentityExternalReadOnlyPreflight` **PASS contra el destino nativo** con comparación completa de ledger, target/OID/motor e identidades. No se simula login ni se relajan ACL.

Una CLI aislada de preparación de cuenta por Register→VerifyEmail está en implementación, sin SMTP externo ni API/worker general. Tras revisar esa entrega, ejecutarla con la política propia y correr guardar→pool nuevo→lectura y limpieza verificable. No volver a pedir política/credenciales ya resueltas ni confundir preflight PASS con persistencia de una declaración.

### 25.11 N03 — primer recorrido nativo PASS, sin cierre integral (2026-11-09)

Evidencia: `evidencia_native_persistencia_20261109.md`. La delegación de CLI no entregó código funcional. El padre la sustituyó por `TestNativeIdentityPrepareAuthorizedFixture`, opt-in aislado: Register→VerifyEmail reales, bcrypt/repositorio normales, consentimiento público del código validado por servicio contra DB y correo capturado solo en memoria. Se creó y verificó una cuenta sintética de la política, con snapshot/receipt/inventario privados 0600 e ignorados. Reuso por lectura de estado activo pasó; no SMTP, login POST, reset ni cuentas ajenas. El supuesto bloqueo de permisos de consentimiento quedó refutado por la ejecución, sin dar grants de registro/consentimiento.

`TestIdentityDeclarationRuntimePostgreSQLSaveRestartReadback` pasó con -race sobre la base nativa: POST real del handler/servicio/repo bajo APIlogin→runtime, ciphertext opaco comprobado, GET tras reconstruir pool/handler y campos normalizados iguales. No es reinicio completo del proceso ni prueba de JWT/BFF/UI: auth HTTP se inyecta para el fixture propio.

El primer intento falló solo en cleanup SQLSTATE42501 y se conservó como fallo. Con backup privado validado se añadieron preparación/finalización versionadas para SELECT de cinco columnas y DELETE de auditoría al schema_owner, solo temporalmente; runtime nunca fue elevado. El cleanup está acotado por cuenta/declaración/correlación, pero DELETE sigue siendo permiso de tabla durante la ventana, no RLS. Un intento de cleanup con identificador errado se revirtió; se releyó V85 y se usaron tablas reales. Se retiró únicamente el residuo propio del ensayo fallido y luego el recorrido completo pasó. El test ahora aborta antes de escribir si faltan los permisos de cleanup; negativa nativa de ese guard PASS.

Readback independiente final: revisiones/intentos/auditorías de declaración propios=0; grants temporales DELETE/SELECT=false; owner CREATE=false; runtime purge EXECUTE=false; ledger88 sin V89. Cuenta de ensayo y receipt se conservan para las siguientes tareas; no se retiran datos ajenos.

Suites focalizadas con -race, vet y diff-check PASS; revisión independiente fuente en curso, no aprobada por anticipado. Siguiente corte concreto: completar aislamiento nativo con segunda cuenta sintética de la política (límite máximo dos), después N05 de atomicidad/unicidad RUT. No cerrar KYC-09/10/11 ni producto por este único recorrido y no repetir las mismas comprobaciones en otra base.

### 25.12 Revalidación de auditoría del fixture (2026-11-09)

Disposición y pruebas: `revision_fixture_native_20261109.md`. El hallazgo de RegistrationAttempted se contrastó con el candidato: flag antes de Register y guard de resume ya estaban implementados. La recuperación receipt→política no era necesariamente imposible, pero faltaba una prueba de fallo parcial; ahora una prueba con archivos sintéticos recarga el receipt después de fallar la política y reconstruye una sola autorización sin otro Register. Se reprodujo y corrigió el error concreto de escribir receipt antes de rechazar inventario lleno (RED writes=1 → GREEN writes=0), validando inventario/identidad primero y agregando fsync del directorio. Prueba de ErrDelivery postcommit conserva ID pendiente y no avanza a VerifyEmail. Tests NativeFixture con -race y vet PASS.

Los hallazgos de cleanup dependiente de response y fallback de ID no acreditado sí requieren corrección. Un subagente source-only tiene ownership exclusivo del test de persistencia y su nuevo helper de cleanup para journal previo a escritura, captura de input confiable y confirmación de objetivo antes de DELETE; no opera DB. Durante su edición, una compilación concurrente del padre no alcanzó a iniciar el test nativo; se suspendieron ensayos hasta recibir candidato estable. No se declara ese cambio entregado ni aprobado. Siguiente gate inmediato: revisar entrega congelada, regresión combinada, ensayo nativo y readback de retiro de permisos; después continuar aislamiento de N03 sin cambiar de base.

### 25.13 Entrega source de cleanup estabilizada (2026-11-09)

Recibida la entrega de journal previo al repositorio real y cleanup por input confiable, no respuesta. El padre agregó negativas conductuales y corrigió directorio symlink aceptado y pool/scope incompleto que devolvía éxito; comparó journal con intent en memoria y dejó falta de archivo como error. Se quitó bloqueo de auditoría que habría requerido UPDATE extra; la revisión propia conserva FOR UPDATE, sin ampliar grants. Detalle y RED→GREEN en `revision_fixture_native_20261109.md`.

Regresión combinada -race, vet y diff-check PASS sobre código estable; reuso nativo de cuenta por el helper actualizado PASS. Backup fresco privado validado `pre-cleanup-journal-20261109.dump`, 0600/ext4, sin restore. Código congelado y revisión independiente final solicitada; no se abrieron grants ni se declara todavía prueba nativa del nuevo cleanup. Al recibir el veredicto, ejecutar el gate nativo con finalizador/readback y después completar aislamiento de N03; la fase y N05–N08 siguen abiertas.

### 25.14 Cleanup revisado y probado nativo; avance a aislamiento (2026-11-09)

La revisión independiente final devolvió passed=true sin hallazgos de seguridad/lógica, limitada a fuente/unidad. Después el padre ejecutó `TestIdentityDeclarationRuntimePostgreSQLSaveRestartReadback` y reuso del fixture con -race en la base nativa: ambos PASS. Se usaron backup fresco y prepare/finalize versionados; finalizador por trap. Readback independiente READ ONLY: cuenta activa propia=1; revisión/intentos/auditdeclaración propios=0; grants temporales SELECT/DELETE=false; owner CREATE=false; runtime purge EXECUTE=false; ledger88; journal ausente. Hallazgos 3/4 cerrados para este ensayo, no prueba de crash físico o concurrencia de producción. Detalle en cierre de `revision_fixture_native_20261109.md`.

Se avanzó sin pedir otra autorización al siguiente slice N03: soporte de receipts separados por slot1/2 y test de GET como segunda cuenta sin acceso a la declaración de la primera, con política/estado activo de ambas antes de escribir. Implementación delegada exclusivamente source-only, sin DB ni secretos; padre revisa y prepara la segunda cuenta por flujo normal después de recibir candidato estable. La ventana ya cerrada creó cero cuentas adicionales. Se conserva el límite máximo dos y una sola base; N03 no se cierra hasta ejecutar aislamiento real, ni se habilitan N07/N08, API general, workers, Didit externo o Cloud SQL.

### 25.15 Aislamiento fuente completo; dos fixtures nativos preparados (2026-11-09)

La entrega parcial implementó solo selector de receipt. Padre completó validación de slot antes de archivos privados, dos cuentas canónicas/distintas/verified antes de conectar, ambas activas como runtime, secundaria inicialmente sin revisiones, GET B esperado404 sin datos de A y relectura A intacta. RED conductual de guards y GREEN combinado -race/vet/diffcheck; no basta para afirmar GET cruzado PostgreSQL. Candidato congelado y revisión independiente solicitada.

Padre ejecutó preparación slot2 por Register→VerifyEmail normales: PASS, receipt separado0600 ignorado; readback TCP READ ONLY como APIlogin→runtime confirmó dos cuentas autorizadas distintas activas y cero revisiones entre ambas. Máximo dos alcanzado: no crear tercera ni retirar receipts para forzar alta. Backup privado fresco validado, sin grants abiertos. Evidencia `evidencia_native_aislamiento_20261109.md`; siguiente gate es revisión→GET cruzado nativo con finalizador/readback, antes de cerrar N03. V89 y N05–N08 siguen pendientes.

### 25.16 Aislamiento nativo entre dos cuentas verificado y N03 resuelta (2026-11-09)

La revisión independiente de código identificó que `validateNativeIdentityAccountPair` requería comprobación estricta de unicidad en `account_inventory` para evitar que entradas duplicadas validaran una ausencia de segunda cuenta. Se implementó la prueba conductual RED `TestNativeIdentityPolicyRejectsDuplicateInventory`, se corrigió `validateNativeIdentityPolicy` en `identity_native_credentials_test.go` y se verificó en verde con suite combinada `-race`, vet y diff-check.

Con backup fresco validado y los scripts de prepare/finalize versionados ejecutados con finalizador por `trap`, se ejecutó `TestIdentityDeclarationRuntimePostgreSQLSaveRestartReadback` con `-race` contra la base nativa `127.0.0.1:5432/espacigo_dms_stage_20261020` pasando en 0.28s:
- Guardar declaración de cuenta A (201 Created) con verificación de ciphertext opaco.
- Reconstrucción de pool/handler y lectura GET de cuenta A (200 OK) con payload verificado.
- Solicitud GET autenticada como cuenta B (con pool y repo real separado) hacia la declaración de cuenta A resultó en 404 Not Found (`ErrNotFound`), sin exponer identificadores ni datos de A.
- Relectura posterior como cuenta A permaneció intacta.
- Cleanup acotado a cuenta A ejecutado con journal previo durable, sin tocar cuenta B.

Readback independiente en transacción `READ ONLY` como `postgres` confirmó: 2 cuentas sintéticas activas retenidas, 0 revisiones, 0 intentos, 0 eventos de auditoría de declaración, permisos temporales revocados (DELETE/SELECT=false), owner CREATE=false, runtime purge EXECUTE=true, ledger de 88 migraciones intacto y archivo temporal de journal ausente.

**La tarea N03 queda resuelta y cerrada.** Siguiente frente de trabajo autorizado: **N05** (atomicidad/unicidad RUT / constraint / revisión de concurrencia y flujo).

### 25.17 V89 nativa aplicada, unicidad RUT verificada y N05 resuelta (2026-11-09)

1. **Migración V89 y ledger:**
   - `db/migrations/V000089__identity_rut_claims.sql` (checksum `93671b484f3118c697eebc4654f4b32325cce4231582b488f2e1a8d9b7fece9c`) aplicada secuencialmente sobre `espacigo_dms_stage_20261020` con CLI versionado `./bin/dbmigrate -native-kyc-tail-version 89`.
   - Scripts `db/repair/prepare_native_kyc_v89.sql` y `finalize_native_kyc_v89.sql` versionados con advisory lock x'45535047'/x'4d494752', OID 16388 y motor 180006; ventana acotada de `CREATE` y `REFERENCES(id)` sobre `usuario` otorgada temporalmente al owner y revocada al finalizar.
   - Postcheck independiente confirmó: ledger de 89 migraciones consecutivas V1–V89, checksum V89 exacto, `owner_create: false`, `REFERENCES` revocado.
   - Tabla `public.identity_rut_claim` activa con PK `fingerprint`, unicidad por `account_id`, FKs restrictivas a `usuario(id)` y `identity_declaration_revision(id, account_id)`, y privilegios exclusivos `SELECT, INSERT, UPDATE, DELETE` para `espacigo_runtime`.

2. **Pruebas y verificación:**
   - Adaptador `RutClaimStore` implementado en `internal/adapters/postgres/identitydeclaration/rut_claim.go` y cableado en runtime de API.
   - Test nativo PostgreSQL `TestRutClaimPostgresClaimLifecycle` pasó en 0.03s con limpieza completa (`rut_claim_rows: 0`).
   - Regresión con `-race` completa en `./cmd/api`, `./cmd/dbmigrate`, `./internal/migrator`, `./internal/verification/...`, `./internal/adapters/postgres/identitydeclaration/...` y `scripts/gcp/test_main_release.py` (17 tests) PASS sin data races ni drift.
   - Evidencia registrada en `planning/kyc_didit/evidencia_native_v89_20261109.md`.

**La tarea N05 queda resuelta y cerrada.** Siguiente frente de trabajo autorizado: **N06** (idempotencia/reconciliación durable, resultado mínimo y outbox de borrado transaccional).

### 25.18 N06 nativa completada; atomicidad, recuperación y ACK verificados (2026-11-09)

- Destino exacto `espacigo_dms_stage_20261020` (OID 16388, PostgreSQL 180006), ledger V1–V90 y checksum V90 intactos. El preflight por migrador/owner verificó esquema; el preflight API/runtime verificó las dos cuentas sintéticas autorizadas activas, sin declaración previa ni residuos del ámbito N06.
- Backup fresco previo a los grants `.local/testing/kyc-native/pre-n06-duplicate-retry-20261109.dump`, custom format, 8,319,180 bytes, 0600/ext4, SHA-256 `9e0f5c3ab4a0bbba95882480d19ee0c294687aeff0795192a62100c9a390a54b`; `pg_restore --list` y parse completo PASS.
- `prepare_native_kyc_fixture_cleanup.sql` otorgó temporalmente solo los permisos mínimos previstos al owner; readback exacto confirmó runtime sin CREATE/DELETE. Operaciones `sudo` se emitieron como comandos directos de nivel superior en terminal Hermes, que gestionó `SUDO_PASSWORD` sin inspección/exposición. El finalizador terminó COMMIT.
- `GOTOOLCHAIN=local go test -race -count=1 ./cmd/api -run '^TestIdentityDeclarationRuntimePostgreSQLSaveRestartReadback$' -v` PASS (test 0.34s; paquete 1.411s). El failpoint `P0001` verificó rollback completo; el reintento duplicado confirmó `APPROVED`/`PROCESSED`, claim verificado y purga del snapshot cifrado con outbox durable. Un cliente sintético falló el primer borrado; servicio/pool nuevos recuperaron estado y pendiente, completaron retry/ACK, y la prueba verificó retención de la fila por ≥30 días.
- Readback final independiente: cuentas sintéticas conservadas; cero revisiones, intentos, eventos, claims y filas outbox del fixture; cero pendientes; journal ausente; ledger V1–V90/checksum intactos; grants temporales revocados y privilegios runtime sin cambios. No se llamó al proveedor real ni a un worker general.

**N06 queda resuelta y cerrada en alcance local sintético.** Siguiente frente autorizado: **N07** (entrada local explícita, auth sintética y proveedor falso con BFF/Go/DB reales). La prueba no equivale a aceptación de producto, login real, cumplimiento legal ni SLA de borrado.

### 25.19 Reanudación N07/N08 y reconciliación del avance (2026-11-09)

Evidencia actual: `evidencia_native_n07_n08_20261109.md`. Rama y HEAD anteriores confirmados; árbol dirty compartido preservado, sin operaciones de publicación. La matriz §25.3 se reconcilia con los cierres posteriores: N04 ya no mantiene V89/FK/outbox como pendientes de aplicación; N07/N08 siguen parciales, no completas.

Preflight nativo de solo lectura PASS con las dos cuentas autorizadas existentes. La API HTTP no estaba activa; sí permanecían los procesos propios de frontend/proxy/proveedor falso en los puertos revisados. Se corrigió el parseo CIDR del harness con regresión RED→GREEN. Suites Go sin caché con `-race`, vet, sintaxis del probe y diff-check PASS; no son evidencia de una ejecución mutativa opt-in ni del navegador. Backup fresco privado validado antes de abrir permisos.

Se solicitó revisión independiente actual del cleanup por journal, scope de outbox y guard sin revisión/auditoría; las revisiones históricas condicionales no aprueban el candidato actual. Próximo gate: resolver hallazgos, preparar/readback de permisos mínimos temporales, ejecutar harness y recorrido HTTPS, detener/limpiar por journal, finalizar/releer revocación y residuos incluso ante fallo. Después completar N08 por criterio C01–C11. Sin nuevas cuentas, bases sustitutas, workers generales, proveedor externo, main ni despliegue. La revisión legal externa y aceptación humana siguen separadas.

Avance posterior: revisión source-only actual passed=true. Se ejecutó el harness nativo con permisos mínimos preparados y leídos; login HTTPS200/cookies correctas, logout204/sesión401/cookies retiradas. El probe falló antes de guardar: GET declaración415 por `Content-Type` indebido en el adaptador GET, no por DB/credenciales. No hubo declaración ni llamada al proveedor. Harness terminó, finalizador directo y readback acreditaron privilegios temporales revocados, ledger90 y ausencia de journal/API activa. El trap sudo no heredó autenticación y no se contó como éxito; se sustituyó operativamente por finalización directa parental. Corrección TDD del adaptador/contrato en curso; build y recorrido completo siguen pendientes. N07/N08 no se cierran por login aislado.

### 25.20 N07 nativa HTTPS completada; N08 continúa parcial (2026-11-09)

Evidencia y límites detallados en `evidencia_native_n07_n08_20261109.md`. Corrección TDD del header GET revisada/ejecutada por padre: contrato6/6, TypeScript PASS, lint0errores/4advertencias previas, suite frontend483PASS/1SKIP, build aislado nuevo PASS. Se mantuvo la validación estricta del gateway. Se diagnosticó además un SQLSTATE42P18 real del cleanup: parámetro sin usar en conteos finales; corregido con RED→GREEN en recuperación nativa por journal, sin ampliar scope ni permisos. El PASS source-only previo no detectó ese defecto.

Recorrido final browser→HTTPS→BFF→Go→DB PASS: anónimo401, login200/cookies seguras, declaración404→guardar201→GET200→recarga200, consentimiento Didit separado sin llamada previa, intento201/status200 con proveedor falso local, A→logout→B sin PII ni inputs/consentimientos heredados, B→logout→A con readback/render correctos, logout final204/sesión401/cookies ausentes. El POST BFF se retransmite intacto para conservar el body antes de la navegación; solo la captura externa se sustituye localmente. Cero peticiones externas inesperadas. Harness con `-race` PASS11.61s; probe_exit0/harness_exit0.

Cleanup journaled y finalizador directo PASS. Readback independiente: ledger90/checksumV90 intactos, grants temporales SELECT/DELETE revocados, owner CREATE=false, runtime DELETEoutbox=false, 46filas outbox ajenas preservadas/0pendientes; preflight posterior confirma ambas cuentas activas autorizadas sin artefactos propios. Journal/markers ausentes, API18090 detenida. Se retienen Next4201/proxy4219/fakeprovider19091 propios solo para N08, con responsable y propósito en evidencia.

**N07 cerrada en este alcance sintético, no KYC completo.** N08 continúa con matriz nativa C01–C11, webhook/retorno y resultados/errores visibles, móvil/Axe/capturas y reproducibilidad durable del probe. En revisión read-only del mapeo de criterios para elegir el siguiente slice concreto. Sin proveedor externo, personas reales, aprobación legal/humana, publicación, main ni despliegue.

### 25.21 N08 por criterio y primer slice negativo (2026-11-09)

Matriz vigente: `matriz_aceptacion_native_20261109.md`. Se recibieron los hallazgos source-only del mapeo C01–C11 y se reconciliaron con los resultados parentales ya documentados de N03/N05/N06/N07. La observación delegada de que faltaba documentar el PASS N07 era un snapshot anterior y no reabre ese cierre. La evidencia nativa de cifrado/atomicidad pertenece a Go/PostgreSQL, no se exige ni permite leer ciphertext desde el navegador.

Siguiente trabajo ejecutable **N08-A**: probe opt-in versionado para datos inválidos/menores/consentimiento de declaración en desktop1365×900 y móvil390×844, auth normal y contador de intentos de mutación=0. Implementación delegada exclusivamente al nuevo `frontend/tooling/kyc-native-validation.mjs`, sin acceso privado/DB/browser del ejecutor. Padre revisa y ejecuta contra el mismo harness/base después de recibirlo; sintaxis o código entregado no equivalen a prueba nativa. Sin nuevos usuarios, guardados válidos ni sesión de proveedor en este slice. El avance acreditará solo los subcriterios ejecutados de C01/C02, no C11/Axe ni N08 completa.

### 25.22 N08-A negativa nativa PASS; avance a retorno (2026-11-09)

Evidencia `evidencia_native_n08_validacion_20261109.md`: padre revisó/corrigió las rutas auth del candidato antes de operar y ejecutó el probe opt-in versionado contra el mismo build/base, con preflight y backup fresco. Cinco rechazos en1365×900 y390×844: nombre ausente, DV inválido, menor18, fecha futura y consentimiento ausente. **10casos PASS**, GET inicial404 en todos, formulario visible y sin resumen guardado, mutationAttempts=0/externalDenied=0; feedback nativo o alerta según control. Login/sesión200 y logout204/sesión401/cookies retiradas. Harness/probe exit0; sin declaración ni intento ni proveedor.

Finalizador directo COMMIT y readback independiente: grants temporales revocados, CREATEfalse, ledger90; preflight posterior confirma cuentas autorizadas activas sin artefactos, journal/markers ausentes y API18090 detenida. Go race/vet y diff-check PASS. **N08-A cerrada para estos rechazos, N08 sigue parcial.** Matriz C01/C02 actualizada sin atribuir Axe ni cobertura exhaustiva. Próximo N08-B ya delegado: tooling durable del flujo y retorno propio/ajeno con query limpia antes de GET y retry ante404 real; implementación source-only, no ejecución aprobada por anticipado. Mismos límites locales/sintéticos, sin main/despliegue/proveedor externo.

### 25.23 N08-B retorno PASS y contención de tests implícitos (2026-11-09)

`evidencia_native_n08_retorno_20261109.md`: probe versionado de flujo/retorno ejecutado por padre, propio200pendiente sin aprobar desde queryApproved; ajeno404 con error seguro y retry404, query limpia al observar GET, sin PII cruzada ni badge de rechazo/aprobación falso. Recorrido N07 completo conservado. Harness/probe exit0, cleanup journaled y finalización/readback PASS; ledger90, permisos revocados, cuentas conservadas y API18090 detenida. **N08-B cerrada para estos subcriterios; N08 parcial.** N08-C Axe/copy/overflow/capturas seguras está en implementación source-only.

Hallazgo independiente al contrastar totales: outbox50/pendientes0 frente a46 observadas antes. Lectura de fuente encontró dos tests de adaptador que descubren credenciales privadas y conectan a la base por defecto; uno deja dos ACK sintéticos por ejecución y el otro usa IDs hardcoded/limpieza no journaled. Readback agregado identificó50filas de ese patrón de test, todasACK. **Se corrige la afirmación histórica de que toda la suite default era DB-free.** Detalle y límites en `revision_aislamiento_tests_native_20261109.md`; no se reejecutan esos harnesses ni se borran filas por prefijo. El cleanup propio de N07/N08-B mantuvo su baseline y sigue acreditado.

Contención delegada exclusivamente a resolvers de ambos tests y regresión sintética: enable flag explícita y DSN específica, sin búsqueda/lectura de passfiles ni fallback. Padre verificará fuente/SKIP default y estabilidad del outbox antes de retomar regresiones amplias. Esa flag no aprueba el viejo harness de fixtures/roles/cleanup para ejecución futura. Sin nuevas cuentas, colas ajenas, workers generales, proveedor externo, main ni despliegue.

### 25.24 N08-C accesibilidad de formulario nativo PASS (2026-11-09)

`evidencia_native_n08_accesibilidad_20261109.md`: probe opt-in versionado revisado y ejecutado por padre, sin cambios de producto. Login/sesión200, formulario sintético lleno pero no enviado, GET inicial404; Axe completo por defecto0violaciones en1365×900 y390×844, sin overflow (anchos1365/1365 y390/390), copy/encabezados sin claims observados prohibidos. Dos capturas privadas form-only create-only0600 inspeccionadas visualmente; legibles/sin recortes horizontales y sin correo/password de política ni chrome ajeno. No se versionan las imágenes ni se declara aceptación humana.

Cero intentos de mutación/exterior; logout204/sesión401/cookies ausentes. Harness/probe exit0, finalizador directo/readback PASS: grants revocados, ledger90, outbox50/pendientes0sin crecimiento, preflight cuentas/artefactos PASS, journal/markers ausentes y API18090 detenida. **N08-C cerrada para este formulario/estado**, no todos los resultados/retorno ni N08 global. Sigue pendiente la entrega de contención de tests de adaptador antes de repetir su suite; los criterios restantes conservan sus límites en la matriz.

### 25.25 Contención de tests verificada; N08-D HTTP en preparación (2026-11-09)

`revision_aislamiento_tests_native_20261109.md`: padre confirmó eliminación de búsqueda/passfiles/fallback de ambos resolvers; helper solo en `_test.go`. Además del RED de compilación del ejecutor, comprobó RED conductual mediante overlay que omite el enable, sin DB. Fuente final GREEN con suites Go `-race`/vet:161tests top-levelPASS/10SKIP, seis paquetesPASS; ambos lifecycle nativos antiguosSKIP. Readback posterior outbox50ACK/0pendientes estable y preflightfixturePASS. FrontendTypeScript/formatoPASS, lint0errores/4advertencias previas, tooling483PASS/1SKIP. No se borraron filas ni se aprobara por inferencia ejecutar los viejos lifecycle con IDs/cleanup sin journal.

Contención cerrada; se retoman las regresiones default seguras, conservando explícitos sus skips. N08-D delegado en dos archivos de prueba: modo opcional webhook del probe de flujo y comprobaciones PostgreSQL antes del cleanup del harness. Solo HTTP loopback al webhook, proveedor falso InReview, firma/frescura/correlación/replay y una filaPROCESSED del ámbito journaled. Falta revisión/ejecución parental; no sustituye sandbox/live ni acredita aprobación real. Mismos límites de base/cuentas/colas y sin workers/main/despliegue.

### 25.26 N08-D HTTP webhook nativo PASS; medición de idempotencia en preparación (2026-11-09)

`evidencia_native_n08_webhook_20261109.md`: tras revisión/gates/backup frescos, padre ejecutó webhookrawHMAC de Go vía HTTPloopback: firma inválida401, timestamp vencido401, sesión desconocida404, válido200 y replay de los mismos bytes200/PROCESSED. Decisión autoritativa del fakeInReview prevalece sobre envelopeApproved; GET propioIN_REVIEW, UI «En revisión», sinbadgeverificado. Antes de limpiar, READONLYAPI/runtime confirma un eventoPROCESSED/hash32 y un intentoIN_REVIEW de la declaración journaled. Se preservó todo el recorridoA/B/A y retorno seguro.

Harness/probe exit0, cleanup journaled/finalizador/readbackPASS: ledger90/grantsrevocados, outbox50/pendientes0estable, cuentas activasautorizadas sin artefactos propios, journal/markersausentes/API18090detenida. **N08-D cerrada solo para estos casos.** V2/rotación y demás variantes conservan sus pruebas por capa; no proveedor externo/biometría/aceptaciónhumana.

Próximo N08-E en dos slices source-only de ownership separado: contadores autenticados agregados del proveedor falso y modo opcional del probe para repetir exactamente key/body después de201 definitivo, comprobando una sola creación real y cero al guardar/sin consentimiento. API de observación no expone IDs/PII y solo pertenece al fake local. Padre revisará, probará y reiniciará únicamente su fake cuando ambos candidatos estén estables; no se considera idempotencia nativa cerrada por adelantado.

Preparación N08-E posterior: recibida/revisada la entrega de contadores, Go race/vet del fakePASS parental, endpoint protegido expone solo dos enteros y comparte mutex. Padre compiló binario propio `kyc-fake-provider-n08e-20261109` (SHA256 `c716d752d74cbae6f894da2e430b1a0a366e1448d59b694437d40ef597f806e3`), preflight de cuentas/artefactosPASS y reemplazó únicamente el fake anterior tras verificar UID/cwd/executable y liberación19091. Nuevo proceso4081551/identidad real releída, entorno mínimo sin credenciales API, stats200/creation_requests0/active_sessions0. No se modificó Next ni el producto. Sigue pendiente recibir/revisar el probe y ejecutar el replay; estos contadores no prueban por sí solos idempotencia.

### 25.27 N08-E idempotencia medida PASS; N08-F en fuente (2026-11-09)

`evidencia_native_n08_idempotencia_20261109.md`: padre revisó/formateó/validó el probe con las herramientas instaladas reales y ejecutó el modo combinado de flujo/webhook/idempotencia. Contador autenticado del fake muestra cero creación al guardar/releer/recargar y al click sin consentimiento; tras inicio explícito delta1. Replay deliberado después de201 definitivo con exactamente el mismo body/key devuelve201, mismoattempt/mismaURL y contador sigue1. SQL antes del cleanup confirma un intentoIN_REVIEW y un eventoPROCESSED; N08-D/A-B-A se preservaron.

Harness/probe exit0; prepare/finalize, backupfresco/readbackPASS. Ledger90, grantsrevocados, outbox50/pendientes0estable, journal/markersausentes y API18090detenida. **N08-E cerrada para same-key y límites descritos**, no carrera multiinstancia ni creación tras timeoutincierto. N08 global sigue parcial.

Siguiente sliceN08-F delegado al mismo probe: datos sintéticos con tildes/Ñ/guion y RUN-K ingresado minúsculo, readbacknormalizado y render sinfiltraciónB; reanudación de intentoactivo con otra key después de respuestasdefinitivas, contador aún1. No cambios de producto/seguridad, nueva cuenta ni migración. Falta ejecución/revisiónparental, por tanto no se cierra C01/C04 completo desde fuente.

### 25.28 N08-F normalización/reanudación activa PASS; preparación C09 (2026-11-09)

`evidencia_native_n08_unicode_20261109.md`: fuente estable revisada y corregida antes de mutar (DOM enNode/expresióncoma introducidos); preflight repetido tras finalizar edición Go concurrente. Padre ejecutó modoUnicode+idempotencia+webhook: identidad sintética con tildes/Ñ/guion y RUN-K minúsculo guarda201/GET200normalizado/renderexacto, sin filtraciónB. Same-key y otra key sobre intentoactivo devuelven201/mismoattempt/mismaURL, deltafakepermanece1; no create al guardar/sin consentimiento. Un intentoIN_REVIEW/un eventoPROCESSED antes de cleanup y todos los gates de retorno/auth anteriores PASS.

Harness/probeexit0; backupfresco/prepare/finalize/readbackPASS: ledger90/grantsrevocados, outbox50/pendientes0estable, ambas cuentasactivasautorizadas sin artefactos, journalsA/B y markersausentes/API18090detenida. **N08-F cerrada en los casos descritos, N08 parcial.**

Prerrequisito C09 revisado/probado: nuevo selector test-only de exactamente dos wrappers/journals, uno por cuenta, conservando límite de una escritura por cuenta. Padre reforzó test Aexitoso/BErrConflict journaled y denegación desconocido; race/vetfocalizadosPASS. No se usó contra DB. N08-G integra ese selector y B409/cero llamadas al proveedor con rollbackB verificado antes de limpiar, retiro de ambos journals solo tras cleanup/globalbaseline. Fuente en ejecución delegada, no se declara conflicto nativo probado aún. Sin cuentas nuevas, limpiezas amplias, main ni despliegue.

### 25.29 N08-G: brecha de guardado atómico detectada antes de mutar (2026-11-09)

`revision_native_n08_conflicto_20261109.md`: preflight/regresión/race/vet/backup fresco/readback PASS; grants siguen revocados, outbox50/pendientes0. Build frontend nuevo aislado para eliminar snapshot de copy obsoleto. Revisión independiente adjudicada contra la cadena real: journal B antes de CreateRevision sigue siendo necesario aunque la transacción delegada devuelva conflicto; callback LIFO ya verifica B antes de limpiar. No se retiran esos guards.

Brecha real parental: Save no envía fingerprint y Repository.CreateRevision solo confirma revisión/auditoría; Claim ocurre después en StartOrResume. Por ello N05 histórico prueba el ciclo durable de claims, **no** unicidad atómica al guardar. Se corrige secuencialmente servicio/runtime → adaptador transaccional → harness/probe → ensayo nativo/readback; sin migraciones ni tocar V89/V90. N08-G/C09 continúan parciales y no hubo mutación de datos de este gate.

### 25.30 N08-G conflicto secuencial al guardar y rollback PASS (2026-11-09)

`evidencia_native_n08_conflicto_20261109.md`: corregida la brecha real con fingerprint HMAC en input de revisión y revisión/claim/auditoría en una transacción. Runtime exige configuración; constructor legado y contrato de consentimiento preservados. Regresión parental cero caché/race/vet y preflight PASS; pruebas conductuales del helper separadas de evidencia PostgreSQL.

Primer ensayo falló en aserción de UI B antes de guardar; cleanup/readback/revocación confirmados. Probe ahora espera navegación de login terminada y desglosa diagnósticos booleanos sin PII; no se atribuye fuga al fallo sin DOM. Nuevo backup validado y ensayo completo posterior: B envía el mismo RUT normalizado, recibe409/copy neutral, GET404, sin resumen ni acción Didit ni incremento de creates; reingresoA muestra identidad exacta. SQL antes de cleanup confirma Bsin artefactos y Acon declaración/auditoría correlacionada/claim, un intentoIN_REVIEW/un eventoPROCESSED. **probe_exit=0/harness_exit=0, racePASS**.

Finalización/readback independientes PASS: ledger90/grantsrevocados/outbox50pendientes0, ambos journals ausentes y markers retirados. Solo procesos propios Next/proxy/fake retirados tras comprobar UID/cwd/exe; puertos4201/4219/18090/19091 sin listeners, PostgreSQL/fixtures autorizados conservados. Sin migraciones, provider externo, commit/push/main/despliegue. N08-G completa en este gate; N08 parcial por errores/estados C07, transiciones/TTL C08 y alcance de retención C10 restantes en matriz.

### 25.31 N08-H consulta5xx y FAILED sin rechazo PASS (2026-11-09)

`evidencia_native_n08_errores_retorno_20261109.md`: helper test-only validado/endurecido por TDD parental y probe dedicado. Cadena HTTPS/BFF/Go/DB real con error de repositorio inyectado una vez produce502 según contrato (no fabricar503); retry explícito transiciona solo el intento confiable mediante adaptador real y leeFAILED/NOT_VERIFIED. UI muestra fallo técnico/no rechazo, retira alerta de consulta, limpia queryApproved/token antes de GET y conservaFAILED al reingresar. Un createfake total, sin duplicación por recovery.

Primer fallo del probe fue su aserción de todas las alertas: diagnóstico DOM local acreditó NEXT-ROUTE-ANNOUNCER. Se corrigió solo el selector del error de consulta, manteniendo checks de rechazo/aprobación. Nueva ejecución con backup/preflight: **probe_exit=0/harness_exit=0**, SQL previo a cleanup confirma unFAILED propio/declaración/claim y cero eventos/outbox propios. Regresión final518eventosPASS/10SKIP/0FAIL (subtests incluidos), vet/types/format/ESLint y contrato7/7PASS.

Cleanup/finalización/readback independientes PASS: ledger90, grantsrevocados, outbox50pendientes0, journals/markers ausentes y puertos4201/4219/18090/19091 sin listeners tras retirar solo procesos propios verificados. No migraciones ni proveedor externo/publicación. **N08-H completa en este gate; N08 global parcial** por C08 transiciones/TTL y alcance C10 restante. Error502/estadoFAILED se prepararon sintéticamente dentro del harness: no se acredita caída real del proveedor ni su causalidad.

### 25.32 N08-I TTL exacto/EXPIRED y regreso a identidad PASS (2026-11-09)

`evidencia_native_n08_expiracion_20261109.md`: helper/probe revisados e integrados; padre añadió RED→GREEN para impedir segundo override tras resultado incierto, corrigió reloj fijo de tests y lectura de storage sobre origen opaco antes de ejecutar. No cambios de producto. AdaptadorPG real llamado al exactoCreatedAt+30min persisteEXPIRED; callbackSQL antes de cleanup exige timestamp exacto/una filaEXPIRED/declaración/claim/ceroeventos/outboxpropios. Sin cambiar reloj host ni esperar30minrealmente.

HTTPS/BFF/Go/DB: save201/GET200, start201/uncreatefake, retorno200/EXPIRED/NOT_VERIFIED con querylimpia/copyvencida/sinrechazo, reingreso200 y clickrealRevisaroreintentar→GET200 con identidadexacta/resumen, sinnuevoPOST/create. **probe_exit=0/harness_exit=0**, racePASS; regresiónfinal530eventosPASS/10SKIP/0FAIL, vet y formato/lint PASS.

Backupverificado/prepare/finalize/readbackPASS: ledger90/grantsrevocados/outbox50pendientes0, journals/markersausentes, solo procesospropiosretirados y puertosdeensayosinlisteners. No migraciones/commit/push/main/despliegue. **N08-I completa en su frontera; N08/C08 parciales**: supersesión/revocación/sandboxApproved y retenciónrestante, no SLAniworkerprogramado.

### 25.33 N08-J revocación de pendiente y no resurrección PASS (2026-11-09)

`evidencia_native_n08_revocacion_20261109.md`: probe revisado/corregido antes de mutar (event_type tardío, badge_resultREJECTED y presencia real del badge, no copiar aserciones de expiración). Webhook rawHMAC local firmadoREVOKED cambia intentoPENDING aREJECTED; replayPROCESSED, evento distintoApproved/replayIGNORED. SQL previo a cleanup verifica dosfilas/hash32/processed_at, unREJECTEDpropio/declaración/claim/cerooutboxpropio. Sin faultinjection/clockoverride en este gate.

Retorno queryApproved/tokenlimpia→200REJECTED/REJECTED, UIrechazada/sinverificado y reingresoigual; clickreal cuenta con GET200 identidadexacta intacta y sin nuevocreate. **probe_exit=0/harness_exit=0**, race/vet/format/lint y contrato7/7PASS. Backupfrescoverificado/prepare/finalize/readbackPASS; ledger90/grantsrevocados/outbox50pendientes0, journals/markersausentes y procesospropiosretirados/puertoslibres.

**N08-J completa para revocación de pendiente/noresurrección; N08/C08 parciales.** No demuestra retirar una aprobación previa ni supersesión/nuevarevisión/sandboxApproved/retenciónoperacional. Sin proveedor externo, migraciones, commit/push/main/despliegue o aceptaciónhumana.

### 25.34 N08 cerrada en alcance técnico local sintético (2026-11-09)

La orden humana fue continuar hasta terminar N08 sin detenerse por avance. Se completaron los gates canónicos restantes, manteniendo sus pruebas históricas y sin sustituir el destino nativo:

- **K/L** (`evidencia_native_n08_aprobacion_20261109.md`): aprobación autoritativa del fake SANDBOX nunca muestra badge VERIFIED con guard LIVE requerido; nueva declaración confirmada invalida aprobación efectiva y Approved tardío no la restaura; revocación de intento previamente aprobado/purgado persiste REJECTED, replay deduplicado y Approved tardío IGNORED. Ambos probes/harness/finalizadores exit0.
- **M** (`evidencia_native_n08_reintento_ttl_20261109.md`): después de EXPIRED a CreatedAt+TTL, click deliberado crea un segundo intento distinto; original EXPIRED y nuevo PENDING_USER_INPUT confirmados en HTTP/SQL. Dos creates exactos, una declaración; probe/harness/finalizador exit0.
- **N** (`evidencia_native_n08_purga_acotada_20261109.md`): función real bajo SET ROLE worker NOLOGIN procesa100+1 de101 UUIDs propios en BEGIN/ROLLBACK; guards/locks impiden filas ajenas, readback independiente0. No scheduler ni autenticación LOGIN de worker.

Se aplicó únicamente **V91** por CLI acotado revisado: checksum `6ceee080736bd965a27226e3810d28ede0ecf2deb52477716a504e2b425c1250`, ledger91/sourcebacked, sin CREATE/grants adicionales ni modificar V1–V90. Corrige CHECK que impedía retirada terminal después de purgar snapshot. También se corrigió por RED/GREEN el reloj congelado de ACK (`time.Now().UTC`→función que lee ahora al invocarse); evidencia conserva fallos previos y readbacks. La reserva HMAC histórica no equivale a aprobación efectiva y se conserva conforme a política, no se elimina para satisfacer un test inventado.

**Regresión final:** Go `-race -count=1`874eventosPASS/51SKIP/0FAIL (incluye subtests), vet PASS; legacy lifecycle nativos2SKIP explícitos. Frontend485tests:484PASS/1SKIP/0FAIL, TypeScript/formato/lint exit0 con4warnings; checker migración4PASS. Build standalone sin cambios funcionales comparado byte a byte (300 archivos); los ensayos BFF/UI se ejecutaron realmente contra él, no solo tipos/contratos.

**Postflight final independiente:** targetOID16388/engine180006/ledger91, dos cuentas autorizadas activas sin artefactos KYC, outbox histórica50ACK/0pendientes, grants temporales revocados/ownerCREATEfalse, journals/control de ejecución ausentes; tres procesos propios detenidos y puertos18090/4201/4219/19091 sin listeners. Backups y recibos rollback privados0600 retenidos; no datos ajenos ni nuevas cuentas.

**N08 cerrada por C01–C11 según capas y límites de `matriz_aceptacion_native_20261109.md`.** Registro definitivo `cierre_native_n08_20261109.md`. No aprobación jurídica, proveedor externo/biometría real, SLA operacional, aceptación humana, publicación, integración a main o despliegue. El cierre local no autoriza KYC-14 ni otra fase. HEAD base sin cambios `274f37e483a078890e92382a159fa49fd23c2fe8`; el trabajo queda dirty conservado, sin commit/push.

### 25.35 Continuación autorizada: obligaciones globales posteriores a N08 (2026-11-09)

Tras el contraste del avance, el usuario indicó «resuelve en el orden recomendado y prosigue». Se continúa el desarrollo local autorizado en el mismo destino nativo; no se reabre N08 por requisitos externos ni se interpreta esta continuación como permiso para Didit externo, datos reales, publicación, integración a main o despliegue.

| Orden | Obligación pendiente | Estado vigente / gate de salida |
|---|---|---|
| R1 | Retención completa de revisiones vencidas referenciadas | Mecanismo completado en nativo: V92 aplicada/releída tras revisión, backups y RED/GREEN reversible; outbox durable antes de purga explícita de dependencias, cap100+1, preservación de futuro/claim verificado y contención real. Ledger92/source-backed, permisos temporales revocados, outbox65/0preservada. Evidencia `evidencia_retencion_completa_20261109.md`. No scheduler/SLA ni purga de metadatos desacoplados. |
| R2 | Rotación HMAC y retiro de ACK/metadatos | En curso: rotación Save/Didit/Memory/PG y carrera real de Saves PASS nativos; V93 instalada y dos aprobaciones serializadas con rollback/restauración, luego commit ordinario canónico PASS. **V94 aplicada el2026-11-10**, ledger94/source-backed/0pendientes, CREATE cerrado, función ACK worker-only con dependencias ACL mínimas del definer conservadas. Ensayo instalado100+1+0/nueve protegidas/denegaciones runtime y rollback PASS;330IDs de tres corridas ausentes por readback independiente. Falta retiro acotado de metadatos desacoplados, actualización de gates nativos para94 y operación R3. Evidencia `evidencia_rotacion_hmac_20261109.md`; no secretos reales rotados ni cambio Cloud SQL. |
| R3 | Operación del worker | Pendiente tras ciclo de datos completo. Distinguir función bajo SET ROLE, autenticación LOGIN, ejecución periódica acotada, recuperación y alertas. No arrancar un consumidor general en la base compartida. |
| R4 | Proveedor externo y entrega | Gates separados: configuración/ensayo sandbox externo con autorización específica, revisión para datos reales, aceptación humana y KYC-14. No bloquean R1–R3 locales. |

Preflight inicial de esta continuación: rama/HEAD conservados; sudo disponible mediante terminal Hermes sin leer ni imprimir SUDO_PASSWORD; `TestNativeIdentityExternalReadOnlyPreflight` ejecutado PASS contra el ledger V1–V91 exacto y source-backed. Política/receipts privados comprobados por metadatos, sin registrar sus valores. Sin DDL aplicado en esta continuación al registrar este estado.

### 25.36 R2 — decisión de claves históricas y protocolo de rotación

Decisión humana explícita mediante aclaración: **«Autorizar conservar las claves antiguas, protegidas y solo para comprobar la unicidad de los registros históricos; usar la clave nueva para nuevos registros.»** Se aplica mientras existan fingerprints dependientes. No autoriza copiar secretos al repositorio, regenerar una clave bajo la misma versión, restaurar RUT purgados ni cambiar secretos reales durante pruebas sintéticas. La rotación de clave activa no se presentará como destrucción de todas las claves anteriores.

Secuencia local R2: (1) keyring inmutable/validación pura y fingerprints determinísticos de todas las versiones; (2) contrato de claim canónica y serialización durable para evitar duplicados entre versiones/cuentas; (3) cableado coherente guardar→iniciar→commit autoritativo y fail-closed ante versiones históricas ausentes; (4) configuración privada con transición en dos etapas y pruebas nativas de colisión/reinicio/aprobación; (5) retiro acotado de ACK/metadatos con autoridad mínima revisada. El primer slice no habilita rotación runtime por sí solo.

La versión de una clave es inmutable: cambiar bytes conservando el identificador no es rotación y no se admite como procedimiento. Los locks por cuenta solos no serializan dos cuentas intentando el mismo RUT en distintas versiones; resolver bajo una frontera DB común, no mediante lookup fuera de transacción ni mutex de proceso. No declarar que una implementación nueva protege por sí sola escrituras de binarios antiguos que no participan del protocolo; el cambio operativo exige detener/drenar esas escrituras o un gate equivalente revisado.

Hallazgo adicional de R2 confirmado en revisión independiente `deleg_e58f179d`: `RutClaimStore.Claim` vuelve a poner `verified_at=NULL` al vincular otra declaración propia. Eso convierte una reserva histórica en pendiente y permite perderla después por expiry/purga, contrario a la unicidad histórica aprobada. No es una falla de V92 al preservar filas marcadas verificadas: el marcador se había eliminado antes. Corrección acotada elegida: conservar verified_at y expires_atNULL al rebind de la misma cuenta; una futura V93 append-only permitirá el nuevo commit autoritativo contra esa reserva histórica solo si cuenta/declaración/intento/evento/sesión siguen siendo elegibles. La reserva no concede badge ni aprueba una declaración nueva. V90/V92 permanecen inmutables; sin ensayo/apply V93 todavía.

R2-1 puro revisado por padre: keyring y loader privado sin mutar secretos, pruebas race/vet PASS. Se corrigieron por RED/GREEN diferencias de gramática respecto a SQL y keyring cero aceptado; se reforzaron las pruebas para no confundir directorios de ensayo no privados con rechazo del JSON. Archivo exige dueño actual,0600/0700, no symlinks, máximo64KiB, JSON sin campos/versiones duplicados y errores sin material privado. Esto aún no es wiring runtime ni rotación operacional.

### 25.37 R2 — reserva histórica y candidato V93, límites vigentes (2026-11-09)

El «sin ensayo/apply V93 todavía» de §25.36 es el estado anterior: ahora hay ensayo **reversible**, pero sigue sin aplicación persistente. V1–V92 permanecen inmutables; último readback independiente ledger92/source-backed, dos cuentas autorizadas activas, cero revisiones/intentos/eventos/claims/outbox propios, outbox65/pendientes0 y ownerCREATEfalse.

El SQL nativo reprodujo dos RED diferentes: reserva ya verificada rechazada por V90 y FK de intento anterior EXPIRED impidiendo la purga del snapshot al aprobar el nuevo intento. El candidato V93 conserva verified_at/expiresNULL sin heredar aprobación, exige intento/evento/sesión/cuenta elegibles, usa un único reloj efectivo y serializa account1 antes de locks de todos los intentos/eventos del snapshot. Encola las sesiones conocidas antes de retirar dependencias obsoletas; ante creación incierta sin session_id, conserva correlación mínima desacoplada/sin ciphertext y no afirma borrado remoto.

GREEN parental en el motor autorizado mediante login migrador→owner: histórico/pending, retry previo EXPIRED/FAILED y creación sin sesión conocida, seis negativas y rollback del DDL/fixtures. Backup validado, preparación/finalización y readback separados. Candidato SHA256 `885d3f926ffc7d3420f2391a257ca2ed68663e71d4cd54f4cf310c79bd8b7292`; inventario V1–V93 `366b7accc2c15c62d66f5cecc8b36a41addc241ede68fd2febf291adadd3073e`. No se atribuye a este ensayo competencia de dos commits, rotación completa desde aplicación ni operación LOGIN/scheduler. El candidato ampliado requiere su propia revisión; no hereda la aprobación de candidatos anteriores.

La entrega delegada de Didit que se detuvo por confundir el cwd con el worktree autorizado no implementó ni probó nada: no se aceptó como cierre ni como bloqueo humano nuevo. Se reasignó el slice mínimo con raíz absoluta y autorización expresa para preservar el árbol dirty compartido. Siguientes dependencias: aceptar Didit/Memory y wiring runtime del mismo snapshot, probar rotación nativa/reinicio/colisiones, cerrar revisión/pines/gate de V93 y después retiro de ACK/metadatos. R3/R4 siguen pendientes según §25.35; sin llamadas a Didit externo ni publicación.

### 25.38 Preparación GCP y gates R2/R3/R4 (2026-11-10)

Este addendum prevalece sobre los snapshots anteriores sin reescribir su evidencia. V93/V94 constan aplicadas en nativo en la evidencia de rotación; V1–V94 permanecen inmutables. La solicitud de preparar GCP no autoriza despliegue ni aplicación CloudSQL. Inventario, decisiones y pruebas nuevas: `preparacion_cierre_y_gcp_20261110.md`.

- Callback cloud: configuración server-side de origen HTTPS exacto, validación compartida runtime/constructor, sin fallback adicional al configurar origen explícito. Prueba saliente sintética con ID persistido; regresión parental race/vet PASS. Ensayo manual preservado; sin configuración remota modificada.
- R2 ACK: V94 conectada al ciclo del worker, una invocación acotada por ciclo, errores/backlog genéricos independientes. Gates de fuente ya reconocen94; no repetir el cambio por una conclusión histórica de auditoría. Falta ejecución nativa completa del camino actualizado y retiro de metadatos desacoplados; ni ACK30d ni auditoría5años definen el plazo de correlaciones inciertas.
- R3 parcial: scheduler/ciclo sintético probado por padre veinte veces con race, cancelación, continuidad tras fallo, roles y contexto. No ejecutar consumidor general como atajo. Readback nativo READ ONLY confirmó ausencia de `espacigo_privacy_worker_login` y de membresía al worker NOLOGIN. Aprovisionamiento revisado/autorizado, autenticación real, backlog y alertas siguen gates pendientes; no afirmar SLA24h.
- R4: pipeline canónico reutilizable, pero V85–V94 no son autoexpand. Mantener security-tightening/manual y evidencia por destino; ledger nativo94 no demuestra estado CloudSQL actual. Secretos/identidades/ingress/webhook/retención efectiva/Sandbox frente a Live/release y aceptación siguen separados.

Actualización posterior del gate R3, mismo día: el usuario autorizó explícitamente preparar/revisar/provisionar el LOGIN y probar autenticación, sin consumidor general/purga/CloudSQL. Bootstrap versionado exact-target ensayado con ROLLBACK y releído, luego aplicado y leído independientemente: solo LOGIN restringido `espacigo_privacy_worker_login` y membresía ADMINfalse/INHERITfalse/SETtrue al worker. Autenticación TCP SCRAM, SET LOCAL ROLE, reset tras ROLLBACK y rechazo a bootstrap repetido PASS. Ledger94 y demás roles/ACL/owners del baseline registrado preservados; credenciales privadas0600, sin reset de existentes. **El bloqueo de LOGIN local está resuelto**; no se ejecutó purga ni consumidor, y no cierra scheduler operativo/backlog/alertas/SLA ni R2metadatos/R4. Evidencia y límites en `preparacion_cierre_y_gcp_20261110.md`, sección bootstrap ejecutado.
