# Guía — conectar Didit y preparar la integración KYC

Fecha: 2026-11-06. Guía de preparación; **el KYC del producto todavía no está implementado**. OAuth MCP está conectado para administración acotada; eso no sustituye la API REST de Go ni prueba el producto. Plan: `plan_kyc_didit.md`.

## A. Dos accesos diferentes

| Uso | Autenticación | Dónde debe vivir |
| --- | --- | --- |
| Hermes consulta/gestiona Didit por MCP | OAuth 2.1 + PKCE, login y consentimiento humano | Caché privada de OAuth gestionada por Hermes, nunca repositorio/chat |
| Go crea sesiones/recupera decisiones del KYC | API key de la aplicación Didit (`x-api-key`) | Secretos exclusivos del proceso backend |
| Go valida webhooks | Signing secret de la aplicación/destino configurado | Secretos exclusivos del receptor backend |
| Configuración de flujo | Workflow ID, application ID, entorno y versión | Configuración no secreta, verificando que no se acompañe de secretos |

El MCP no reemplaza al adaptador REST ni debe quedar como dependencia del runtime de EspaciGo. No se necesita API key para el MCP alojado. No habilitar Authentication de Didit ni reemplazar el login de Go.

## B. Estado del intento realizado

1. Se verificó el CLI instalado y se añadió el servidor personalizado por el CLI oficial. Un primer intento OAuth con 30 segundos expiró; se reintentó con 600 segundos y el usuario completó la autorización Didit.

   ```bash
   hermes mcp add didit --url https://mcp.didit.me/mcp --auth oauth --connect-timeout 600
   ```

2. La autorización se completó mediante OAuth/PKCE. Se descubrieron 194 herramientas y se habilitaron 11 de alcance KYC: lectura de aplicaciones/workflows/configuración/validación y lectura de decisión; escritura para crear sesión, crear borrador, editar grafo y publicar workflow. La allowlist fue verificada por lectura de configuración y `hermes mcp list`; `hermes mcp test didit` conectó correctamente.
3. Habilitar una herramienta concede capacidad, no ejecuta la operación. No se invocaron herramientas de escritura, no se crearon sesiones, no se publicaron cambios, no se consultaron sesiones ni datos de usuarios. Las escrituras requieren inspeccionar el destino/diff y autorización de la fase correspondiente.
4. Lectura realizada: organización/aplicación `Hotusoft (Sandbox)` y workflow `Compliance workflow`, publicado v1, Chile `CHL`, features `OCR + LIVENESS + FACE_MATCH`, acción `expected_details_mismatch_action=REVIEW`. `Hotusoft` no es el nombre del workflow. El payload reporta USD 0.35 como precio estimado del workflow; no confirma tarifa live ni facturación. No se imprimen ni guardan URLs tokenizadas.
5. El MCP se recarga con `/reload-mcp` o nueva sesión antes de usar herramientas en una conversación que no las tenga cargadas. El flujo anterior ya usó una sesión nueva para la lectura de workflow.

## C. Consentimiento MCP y próximos permisos

La autorización ya está completada. No volver a solicitar login ni pegar claves/códigos en chat. El comando de alta queda solo como referencia para una reinstalación autorizada:

```bash
hermes mcp add didit --url https://mcp.didit.me/mcp --auth oauth --connect-timeout 600
```

1. La allowlist actual está limitada a las 11 herramientas enumeradas en la evidencia KYC-00; no ampliarla globalmente.
2. Para el MVP, `didit_session_create` solo se utilizará en la fase sandbox autorizada, con aplicación explícita de Sandbox y payload sintético. Editar/publicar workflows no es necesario por ahora: el mismatch ya está configurado en `REVIEW`.
3. No utilizar didit MCP como runtime de EspaciGo. Go requiere API key y webhook secret privados por aplicación/entorno, separados del OAuth MCP.
4. Para repetir comprobación de conexión/configuración:

   ```bash
   hermes mcp list
   hermes mcp test didit
   ```

5. No usar credenciales API en el MCP, ni exportar sesiones del navegador o copiar tokens. Si una futura reautorización falla, registrar solo errores sanitizados.

**Estado de lectura:** ya se identificaron app/workflow, versión, módulos, países/documentos y acción de mismatch. La próxima comprobación pendiente no es editar el workflow, sino confirmar en una prueba sintética autorizada que Go envía `expected_details` y que el RUN/RUT se coteja con el campo extraído correcto. No hacer búsquedas globales de sesiones ni descargar documentos.

## D. Cómo preparar secretos del backend sin exponerlos

Todavía no hay un cargador Didit implementado en Go. Los nombres siguientes son una **convención propuesta**, no variables que la aplicación actual ya reconozca:

- Secreto: `DIDIT_API_KEY` (distinto por aplicación/entorno).
- Secreto: `DIDIT_WEBHOOK_SECRET` (según destino elegido; considerar rotación).
- No secreto: `DIDIT_WORKFLOW_ID`, `DIDIT_APPLICATION_ID`, entorno esperado y callback allowlisted.
- No secreto: versión de política/nodos y feature flag que permanece desactivado en live hasta aceptación.

### Preparación local posterior

- No solicitar ni recibir claves en el chat. Tú introduces sus valores directamente en un gestor de secretos o editor local privado.
- Si se elige archivo local: exclusivamente bajo `.local/testing/`, directorio `0700`, archivo `0600`, ignorado por Git **comprobado antes de escribir valores**. No crear `.env` público ni dejar claves en ejemplos. El agente verificará existencia/permisos/nombres necesarios sin imprimir contenidos.
- Confirmar primero si el worktree comparte la política privada de cuentas o necesita una referencia autorizada; no copiar políticas/credenciales entre worktrees por inferencia.
- Inyectar secretos solo a Go mediante un mecanismo de carga revisado; no pasarlos como argumentos visibles, headers de curl registrados, URLs, capturas, prompts de subagente, historial o salida de terminal.
- Next debe arrancar con allowlist de entorno separada y **sin** API key, webhook secret, credenciales DB ni passfile. Nunca usar prefijo `NEXT_PUBLIC_` para estos secretos.
- No reutilizar signing secret como clave de cifrado/HMAC de RUT. Cifrado y deduplicación requieren claves separadas y política de rotación.

### Preparación live posterior, no ejecutada

- Usar el gestor de secretos/identidad de ejecución del backend aprobado por la arquitectura; mínimos permisos y rotación. Configurar API key de aplicación live, nunca la sandbox ni token OAuth de Hermes.
- Registrar destino HTTPS de webhook solo cuando exista receptor verificado y se autorice infraestructura/red. No exponer la API privada completa ni registrar túneles públicos improvisados.
- No crear/configurar Secret Manager, IAM o Cloud Run desde esta guía. Seguir el runbook y obtener el alcance posterior correspondiente.

## E. Lista del workflow existente que necesitamos confirmar

- Organización y aplicación concretas; modo `sandbox` para primeros ensayos.
- Workflow KYC de persona, no KYB; ID estable, versión y node IDs.
- Chile `CHL`, documento `ID`, lados requeridos y formatos vigentes.
- Documento + passive/active liveness + face match habilitados y obligatorios. Confirmar IP/device si forma parte del precio citado; no asumir que el total del flujo creado es USD 0.33.
- Política de discrepancias de expected details en revisión/rechazo, no ignoradas.
- País emisor separado de nacionalidad; campos/nulabilidad reales RUN/RUT y DV.
- Vencimiento documental estricto y política de edad/renovación ratificada.
- Callback fijo y orígenes permitidos. Entrega `status.updated`, `data.updated`, y bloqueo de usuario si se usa.
- Consentimiento, retención y eliminación; no conservar plantillas faciales por defecto.
- Cuota/crédito efectivos y auto top-up desactivado salvo decisión humana.

## F. Pruebas previstas y límites

1. Primero tests locales sin llamadas externas, con cuerpos sintéticos y verificación real de firmas/cotejo/DB aislada.
2. Después, autorización específica para aplicación sandbox y canal webhook de ensayo. Consultar catálogo oficial de escenarios y usar documentos/medios de prueba, nunca cédula real ni selfie personal. Sandbox simula resultados pero conserva medios capturados.
3. Escenarios: aprobación, rechazo de documento, liveness, rostro, revisión, discrepancias, abandono/vencimiento, duplicados/desorden y revocación posterior. Distinguir resultado simulado de biometría real.
4. Integración completa navegador local → BFF → Go → proveedor sandbox → webhook → DB → insignia. No cerrar con un mock o un callback aislado.
5. Usuario realiza la aceptación online y eventual ensayo live cuando lo autorice por separado. Sin main/deploy ni SQL persistente por este documento.

## Referencias

- [Didit MCP overview](https://docs.didit.me/integration/mcp/overview)
- [Autenticación OAuth, scopes y roles](https://docs.didit.me/integration/mcp/authentication)
- [Instalación](https://docs.didit.me/integration/mcp/installation)
- [Hermes OAuth MCP](https://hermes-agent.nousresearch.com/docs/guides/manage-hermes-cloud-with-mcp)
- [API create session](https://docs.didit.me/sessions-api/create-session)
- [Sandbox](https://docs.didit.me/integration/sandbox-testing)
- [Precios vigentes](https://docs.didit.me/getting-started/pricing)
