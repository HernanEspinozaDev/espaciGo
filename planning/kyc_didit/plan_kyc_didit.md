# KYC Didit — confirmación de identidad de usuarios

## 1. Encargo, base y límites

- Fecha de actualización: 2026-11-06. Estado: **plan de integración revisado; implementación no iniciada**. Este encargo autoriza documentar y preparar el desarrollo, no comenzar cambios funcionales.
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

1. **Mi cuenta → Identidad.** Escudo delineado vacío y texto «Identidad no verificada»; CTA «Verificar mi identidad». Correo verificado no equivale a identidad verificada.
2. **Información y consentimiento.** Explicar finalidad, proveedor, datos compartidos, cámara, tratamiento biométrico, conservación y canal de revisión; consentimiento específico y versionado separado de marketing. No preseleccionarlo. Avisar antes de salir de EspaciGo. Menores/no elegibles y documentos no soportados reciben una salida informativa, sin cobro ni sesión remota.
3. **Formulario breve** (§4), prellenado únicamente con datos propios confirmados por servidor, sin inventar datos desde nombre público. Validación accesible cliente + Go. Mostrar resumen editable antes de «Continuar con Didit».
4. **Crear intento en Go.** Derivar cuenta de la sesión autenticada; fijar instantánea inmutable del formulario y consentimiento. Bloqueo/idempotencia por cuenta, propósito y entorno. En `POST /v3/session/`, Go envía el workflow aprobado y `expected_details` con nombres, apellidos, fecha de nacimiento, país emisor `CHL`, tipo esperado `ID` y RUN/RUT como `identification_number`, tras validar/normalizar sin perder el DV. No enviar RUN/RUT en `vendor_data`, URL, telemetría ni logs. Solo devolver URL temporal al propietario del intento.
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
| KYC-01 | Decisiones Chile, revisión workflow existente, privacidad y contrato de identidad | ID/versión/entorno no secretos, módulos/nodos, políticas de warnings, campos chilenos confirmados, ADR-001 y matriz de consumidores | Completado (`evidencia_kyc01_04_20261107.md`) |
| KYC-02 | Modelo SQL/migración incremental y OpenAPI | Base vacía + upgrade desde ledger revisado, constraints, ACL mínimo, migraciones históricas intactas y clientes generados | Completado (`evidencia_kyc01_04_20261107.md`) |
| KYC-03 | Go, adaptador, inbox, cotejo y reconciliación | Tests unitarios/HTTP/PostgreSQL aislado, fallos de red, idempotencia, firmas y desorden; cero llamadas reales en suite ordinaria | Completado (`evidencia_kyc01_04_20261107.md`) |
| KYC-04 | Formulario Next, redirect/retorno, espera e insignias | Build, tipos/lint/formato/unitarios, Playwright mock + Axe dos viewports, aislamiento de sesión y no fugas | Completado (`evidencia_kyc01_04_20261107.md`) |
| KYC-05 | Sandbox Didit separado e integración local | Autorización externa + credenciales privadas + canal webhook de ensayo revisado; escenarios oficiales y readback DB/BFF; evidencia etiquetada como sandbox, sin identidades reales | Completado (`evidencia_kyc05_20261107.md`) |
| KYC-06 | Revisión integral y preparación live | Costos/cuotas, seguridad/privacidad, eliminación/retención, monitoreo, recuperación, revisión independiente y suite de regresión | Completado (`evidencia_kyc06_20261107.md`) |
| KYC-07 | Integración/publicación/despliegue separados | Autorización expresa para cada destino; preflight/migración canónica cuando corresponda; usuario valida online | Detenido / No autorizado |

Cada corte registra SHA, comandos, resultados, límites, recursos y siguiente gate. No declarar el KYC terminado por mocks verdes ni por instalar el MCP. La conexión OAuth no bloquea diseño/documentación ni autoriza configurar el workflow.

### Secuencia para iniciar desarrollo — planificada, no iniciada

Este encargo actualiza el diseño únicamente. No editar código, crear migraciones, iniciar sesiones Didit ni modificar/publicar workflows hasta una instrucción expresa de comienzo.

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
8. Autorizar implementación como tarea posterior. No se ejecutaron migraciones ni se alteraron perfiles/formularios en KYC-00.

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
