# Registro UX — continuación autónoma hasta entrega

## Autorización humana vigente

El usuario indicó continuar el plan hasta terminar, iterar entre implementación/revisión, aprobar internamente las decisiones visuales y publicar; revisará el resultado montado. No se solicitarán aprobaciones visuales intermedias. Se conservan los límites sobre secretos, pruebas locales, datos y operaciones backend/SQL/infraestructura. No se puede reemplazar un dato legal desconocido por uno ficticio ni fabricar evidencia de release.

## Cortes activos

1. Registro compacto y `/registro-exitoso`: escritor exclusivo de RegistrationScreen, resultado efímero cliente, página de resultado, ayuda secundaria lazy y pruebas propias. Estados 201/202 separados; correo fuera de URLs/SSR/storage; reemisión honesta; sin modificar documentos legales.
2. Verificación y recuperación: escritor distinto para ActionScreens, resultado `/correo-verificado` y pruebas propias. Sin entradas manuales de ID/token; conservar captura y purga de fragmento, confirmación voluntaria y autoridad Go. Recuperación exitosa oculta formularios y ofrece acceso.
3. Inspector de publicación read-only: verificar gates full reales para un cambio del mailer; no SQL, secretos, SMTP o infraestructura. No se reutilizan timestamps de evidencia vencida.
4. Mailer Go de marca: desarrollo y pruebas SMTP exclusivamente locales en worktree separado `/home/nandev/.hermes/cache/scratch/espacigo-mailer-ux-20261026`, detached en `715aca4`. No integrar sus cambios en `main` antes de resolver el gate de publicación correspondiente: el modo web exige paridad exacta del backend activo. La separación evita bloquear una entrega frontend segura por mezclar código Go prematuramente.

## Coordinación

No hay builds concurrentes: los escritores frontend ejecutan RED browser contra el standalone previo y pruebas unitarias de fuente; el padre reconstruye una única vez cuando ambos terminan y contrasta GREEN. La suite legacy se actualizará solo donde contradiga el nuevo contrato UX, sin quitar pruebas de dominio. Luego regresión global, HTTPS de sesión, privacidad, accesibilidad y revisión independiente. Solo los resultados reales pueden cerrar estos gates.

La base visual AuthShell ya fue contrastada: 14 UI focalizadas, 229 unitarias, 83 E2E/Axe con 45 skips y 11 HTTPS; revisión independiente aprobada. Estos números corresponden a la base visual anterior, no acreditan todavía las nuevas rutas en implementación.

## Dependencias que no se falsean

Los documentos Go actuales siguen conteniendo disclosures de evaluación, identidad ficticia y contacto no operativo. Se retiran del formulario los bloques completos y la explicación técnica añadida por la interfaz, pero no se alteran texto firmado, hashes, versiones ni metadatos para presentarlos como documentos definitivos. Un responsable/canal de soporte real no se deduce de las cuentas técnicas Git/GCP ni del correo personal mostrado por el usuario.

## Gate full: evidencia de esquema renovada realmente

El inspector detectó evidencia expirada (última captura histórica del 24 de octubre). El padre verificó los tres objetos existentes de GCS y confirmó el bloqueo; no reutilizó timestamps ni una copia local de otra base.

Se resolvió mediante el procedimiento canónico sobre la instancia/base identificadas: metadata Cloud SQL/bastión comprobada, SSH con clave de host ya conocida y verificación estricta, forward loopback propio, identidad existente `espacigo_migrator_login` y TLS. Preflight y captura en transacción **READ ONLY**; no migraciones, DDL, filas de negocio, grants ni cambios de infraestructura. La consulta versionada recogió catálogo/ledger y su verificador aprobó las 75 migraciones.

Evidencia capturada `2026-10-26T23:37:19.968832+00:00`, SHA256 `6102d0a5678f1e22f211ccbd1d9096162cc8a60ef3a8ecb2f3981cad66f8b238`; objeto `img01-schema-evidence/<sha256>.json`, generación `1793057879108058`. Se leyó de vuelta esa generación y se comprobó el hash de los bytes remotos. Detalle privado: `.local/deployments/registro-ux-schema-readonly.json`. El forward propio quedó cerrado y los dos archivos temporales de autenticación se retiraron.

El mailer conservará el remitente/configuración existentes: cambiar el asunto y formato de marca no requiere inventar otra identidad corporativa. No se retiran los disclosures legales ni se publica una razón social ficticia como real. El gate SMTP del pipeline comprueba configuración/metadata/acceso ya existentes; no autoriza enviar correos reales de prueba. Aún deben pasar código, pruebas, revisión y el build full fijado al SHA final, dentro de la ventana de seis horas. No modificar el default web del trigger ni conceder permisos para eludir gates.

## Primer build integrado y contraste del padre

Los dos escritores de frontend finalizaron sin construir en paralelo. El padre corrigió formato de los archivos entregados, ejecutó tipos/unitarias y produjo el build unificado con las rutas `/registro-exitoso` y `/correo-verificado`.

El primer recorrido focalizado real sobre ese build obtuvo **7 PASS / 2 FAIL**: los fallos corresponden a un locator de ayuda que conserva el nombre anterior después de desplegarse y a un selector de confirmación de contraseña que también coincide con el botón mostrar/ocultar. El resto acreditó creación 201/202, estado neutral, captura/purga del fragmento, confirmación voluntaria y solicitud de recuperación.

La regresión global inicial obtuvo **80 PASS / 12 FAIL / 45 SKIP**. Hay selectores legacy contradictorios con el nuevo flujo y un fallo Axe real de jerarquía de encabezados. La lectura detectó también validación nativa de correo ausente y resultados efímeros que requieren purga más estricta al abandonar/cambiar flujo; no se consideran resueltos por el éxito de los escenarios felices.

Se asignaron tres correcciones con propiedad exclusiva: privacidad del resultado de registro; validación/accesibilidad/estado tardío de verificación y recuperación; y adaptación de `phase5-identity.spec.ts` preservando todos sus invariantes. El siguiente build vuelve a pertenecer exclusivamente al padre cuando terminen los escritores. El fixture público loopback propio continúa activo para este ensayo (`proc_9a272f6c6cbb`), sin acceso a datos persistentes.

## Mailer: entrega aislada y contraste inicial

El mailer se entregó en el worktree separado con MIME `multipart/alternative` (texto plano y HTML), CTA de verificación/recuperación, vencimiento procedente de Go y SMTP fake loopback con TLS y certificados efímeros. El padre inspeccionó ambos archivos y repitió **`go test -race ./internal/adapters/zohoauth -count=1` y `go test ./...`**, ambos exit 0. No se contactó SMTP externo ni se disparó una notificación real.

Antes de integrar, el padre detectó correcciones necesarias: el HTML usaba naranja distinto de la paleta aceptada, copy con garantías de protección no auditadas, asunto UTF-8 sin codificación RFC2047 y displayName legacy de pruebas. Se delegó un ajuste exclusivo de esos dos archivos, con pruebas de paleta, encabezados y remitente; la dirección/username/configuración SMTP no se cambian. Los mensajes de error de las pruebas tampoco deben volcar cuerpos/enlaces con secretos, aunque sean fixtures.

Se revisaron el pipeline full, el wrapper (solo web), el rewrite `/healthz` y la metadata actual de los servicios. El full usa el trigger existente con sustituciones explícitas de evidencia; no se cambia su default web. La configuración activa de servicios/red/runtime coincide con el procedimiento. El ajuste Go aún no está integrado en el checkout principal ni publicado.

## Contraste consolidado del frontend

El padre volvió a compilar después de las correcciones. Detectó que la limpieza de verificación seguía teniendo retención predeterminada de 1500ms: añadió un RED unitario real (3 PASS / 1 FAIL) y cambió ese default a microtask versionada; GREEN 4/4. También reprodujo el RED real de retorno inmediato a registro exitoso: el ciclo de unmount/StrictMode no bastaba para toda navegación. Se agregó una notificación opcional `onNavigateAway` en AuthShell/links legales, callbacks de salida en resultados y limpieza por `popstate`/`pagehide`. No se cambió la autoridad ni la sesión.

Sobre el nuevo build: **30/30 escenarios focalizados de registro, verificación y legacy Phase5 PASS**, **244 unitarias PASS**, **98 E2E/Axe PASS / 45 SKIP / 0 FAIL**, tipos/lint/formato en verde y **11/11 HTTPS de navbar/sesión**. La suite adicional HTTPS BFF obtuvo **13 PASS / 1 FAIL**, específicamente logout sin red; se delegó diagnóstico por etapa sin permitir omitir el caso o debilitar sus garantías. No se declara cerrado ese gate.

La inspección visual local aprobó internamente registro y resultado móvil/escritorio, sin overflow. Prueba de ayuda diferida medida: al desplegarla se solicitó **un chunk JavaScript adicional** que no estaba en las solicitudes previas. Se guardaron capturas sin secretos y reporte en `.local/testing/registro-ux-final/`; registro es simulación UI explícita, no un alta nativa ni un correo enviado. Se sustituyó después el rótulo técnico del requisito de largo de contraseña por copy comprensible, sin modificar la comprobación real de 72 bytes; el siguiente build debe recoger ese cambio.

Revisión independiente final frontend `deleg_88aacdc0` aprobada: JSON válido con `passed: true`, sin hallazgos de seguridad, lógica o sugerencias. Contrastó PII efímera, purga de fragmentos, 201/202 y estados neutrales, épocas, StrictMode y eventos de salida, sin cambios de autoridad de sesión. Mailer y revisión/cierre del runner siguen separados; no se anticipa su aprobación ni publicación.

## Integración del mailer y vista previa segura

El ajuste del mailer fue contrastado por el padre con `go test -race` y suite Go completa. Se detectó y reprodujo además un RED de ancho HTML mal escapado (`100%%` en el resultado); se corrigió el formato y se añadió regresión de porcentaje único. Los dos archivos se integraron en el checkout principal solo después de comprobar alcance exclusivo y aplicación limpia del diff; `go test -race` volvió a pasar allí.

La plantilla tiene asuntos RFC2047, rechazo de CRLF, display de marca Auth sin «pruebas» conservando el buzón, alternativas texto/HTML, paleta aprobada y copy sin garantías inventadas. Se generó desde el renderer Go una vista previa con **enlace inerte `#`, sin credenciales de acción**, y se comprobó en Chromium local a 390px y 1365px sin overflow. El padre inspeccionó la captura móvil y aprobó internamente esa dirección visual. Capturas `correo-mobile.png` y `correo-desktop.png` bajo `.local/testing/registro-ux-final/`; no demuestran entrega en una bandeja real.

La revisión independiente Go `deleg_9c94c9b5` quedó aprobada: JSON válido con `passed: true`, sin hallazgos de seguridad, lógica o sugerencias. Confirmó MIME/Quoted-Printable/RFC2047, anti-CRLF, escaping, TLS estricto y preservación de contratos sin fugas de detalles. Se encargó también un ajuste mínimo de trazabilidad del pipeline full: sus tres deploys no fijaban etiquetas de SHA/build y podían heredar metadatos antiguos; solo se agregan las etiquetas y su prueba, sin cambiar permisos, red o modos de publicación. Ese ajuste aún requiere entrega y contraste.

El gate adicional HTTPS BFF quedó resuelto y repetido por el padre: **14/14 PASS**, con comprobaciones de marcador pendiente, purga de la pestaña compañera, reanudación online y sesión 401 antes de restaurar. El fallo estaba en consultar `localStorage` desde la pestaña que Chromium sustituyó por una página interna de error al intentar navegar sin red; ahora se observa el origen seguro de la pestaña compañera. No se modifica producción ni se eliminan garantías. Esto no promete renderizado offline de `/acceso` sin una aplicación cacheada.

Después de incorporar el último copy, el padre repitió tipos/lint/formato/unitarias/build y regresión global con exit 0; se mantienen 244 unitarias y 98 E2E/Axe con 45 skips. La revisión independiente Go y el ajuste de etiquetas full siguen pendientes de entrega/contraste. Se identificó además un único punto global pendiente del plan: el workspace no incluía enlaces legales, aunque marketplace y AuthShell sí. Se delegó reutilizar el footer compacto al final del layout sin cambiar header, sesión o contenido privado, con RED browser y posterior build del padre.

El ajuste de etiquetas full se contrastó por el padre: diff de exactamente tres flags `--labels`, uno por deploy API/web pública/web privada, sin cambios de red, secretos, variables, IAM o probes. La primera ejecución local encontró PyYAML ausente; se resolvió en un venv de scratch usando la versión **6.0.3 ya fijada en el repositorio**, sin instalar globalmente ni modificar dependencias. Luego pasaron el contrato nuevo (un test con tres subtests), los nueve tests de frontera web y `validate_cloud_build_transition.py` (ocho pasos válidos; full dos imágenes, web una). No se ha ejecutado aún ningún build remoto de este rediseño.

## Cierre técnico local y publicación autorizada

El footer global quedó integrado fuera de Header y PrivateSessionBoundary, con un caso RED previo y su GREEN en el build del padre. Se detectó una ambigüedad de selectores legacy en Phase6: «Privacidad» también coincidía con el nuevo enlace legal. Se acotaron las dos búsquedas a la navegación «Secciones de mi cuenta», sin alterar la UI ni los asserts de ARCO/exportaciones/Axe.

Resultado final del padre sobre el último build: **244/244 unitarias, 99 E2E/Axe PASS / 45 SKIP / 0 FAIL, 11/11 HTTPS de navbar y 14/14 HTTPS BFF**, tipos/lint/formato/build en verde; Go suite completa y race del mailer en verde; tests de release y validador Cloud Build en verde. Revisiones independientes frontend y Go aprobadas, y cambios finales de footer/etiquetas contrastados por el padre. Diseño aceptado internamente bajo la autorización humana de continuar hasta publicar.

El fixture propio fue detenido. No se crearon cuentas, recuperaciones o envíos SMTP reales de prueba, no se usó navegador del usuario y no se modificaron esquema, datos, AGENTS o documentos legales versionados. El worktree aislado del mailer se conserva como evidencia; sus dos archivos fueron integrados y contrastados en el checkout principal.

**Implementación y validación local completadas; publicación en curso después de este commit.** El corte incluye Go y requiere `_DEPLOY_SCOPE=full` con la evidencia real fijada por URI/generación/hash. El trigger automático conserva su default `web`: si rechaza la paridad por el cambio Go, es una detención esperada antes del despliegue, no motivo para quitar el guard. Se ejecutará entonces una única invocación full del trigger existente sobre el SHA remoto exacto y sin otro build activo. El informe de entrega se leerá de vuelta desde Cloud Build/Artifact Registry/Cloud Run y se conservará en `.local/deployments/registro-ux-release-<sha>.json`; este texto no adelanta un resultado remoto.

Los textos legales y sus versiones se mantienen como indicó el usuario, con sus disclosures reales; retirar contenido técnico incondicional del formulario no los convierte en documentos jurídicos definitivos ni inventa una identidad corporativa.

## Publicación funcional comprobada

Se publicó `1b4b4249ef45c5fa9f98c1c16f271c5e581a0c63` por push directo a `main` con identidad verificada. Ambos workflows GitHub terminaron SUCCESS. El automático web `8202cac4-9e05-4a74-be36-01da144e4f35` detuvo correctamente la paridad por los dos archivos Go cambiados, sin desplegar. Luego se invocó una única vez el trigger existente en modo full, con evidencia fijada: build `ef5bd95d-357e-4ab0-86e9-a08e473ef096` y child `5a3286f5-57d8-4484-803a-9b4fca3264f3`, ambos SUCCESS.

- API: `sha256:a46ebd80621bdb6e4b1cfa47b2e5f15e9c8915c358b1b7472c1f00a86786de43`, revisión `espacigo-test-api-00022-7bz`.
- Web: `sha256:f5b700fa8210c0b3fa0de12bbb78785dc348357f1f4bd3e2656b3d29537b32ca`, revisiones `espacigo-test-public-00031-7gw` y `espacigo-test-private-00030-zc8`.

El contraste posterior NO se detuvo en SUCCESS: detectó que el full heredó tráfico web fijado a revisiones anteriores. Las nuevas revisiones tenían imagen/SHA/build correctos y Ready True, pero cero tráfico. Se verificaron esas revisiones exactas y se promovieron explícitamente con `--to-revisions=REVISION=100`, nunca `--to-latest`. La lectura posterior acredita **las tres revisiones correctas Ready y 100% de tráfico**, proveniencia de ambas imágenes al child real, IAP privado y ausencia de invocadores públicos en privado/API.

HTTP read-only comprobó en el dominio público los contenidos nuevos de acceso, registro, ambos resultados neutrales y footer de cuenta; health 200 y sesión anónima 401. Sin crear cuentas ni probar envío a una bandeja real. Reporte privado completo: `.local/deployments/registro-ux-release-1b4b4249ef45c5fa9f98c1c16f271c5e581a0c63.json`.

La regresión del pipeline full quedó corregida para sus tres servicios: preparación `--no-traffic` con revisión determinista por build, lectura JSON de esa revisión exacta, verificación de digest/Ready/SHA/build y promoción explícita a 100%. Se preservaron runtime, secretos, red, IAM y probes. El parser se ejecuta con Python aislado (`-I`) para que `PYTHONOPTIMIZE` no pueda desactivar las validaciones. El formatter compuesto de la primera propuesta fue rechazado por contraste con el CLI real y no se publicó.

Contraste final del padre: **31 pruebas de scripts GCP PASS**, incluidas cinco pruebas de contrato/guard con casos de drift/Ready/JSON/CLI; las cinco también pasan con `PYTHONOPTIMIZE=1`. El parser exacto extraído del YAML validó en modo read-only el JSON real de las tres revisiones publicadas. El validador de transición conserva ocho etapas. Esta corrección no altera Go ni la interfaz: su commit de cierre usa el despliegue automático web con guard de paridad, sin un segundo full o captura SQL innecesarios. La evidencia final de ese cierre se conserva bajo `.local/deployments/` con el SHA real verificado después del push.
