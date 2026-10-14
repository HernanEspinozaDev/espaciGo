# Cierre del desarrollo local de EspaciGo — backend, Angular y DB

Fecha: 2026-10-14, America/Santiago. Estado: LOCAL-CLOSE-00 fusionado en PR #308; LOCAL-CLOSE-01 implementado en rama `codex/marketplace/fix/local-close-01-parity-contract`, PR #309 abierto y pendiente de revisión. HEAD de código ensayado `726b419824a26a79797561b6b77db6d92af0170e`; HEAD actual y autores verificados en GitHub PR #309. Baseline de `main`: merge #308 `a26e1dacbe23cbc4b73528c69ee0e6b4daf394ea`. Cierre integral todavía pendiente de evidencia y decisión humana. No es una autorización de despliegue ni de creación de recursos GCP.

## Objetivo y fuentes

Terminar el alcance **local y sintético** con una base reproducible, paridad funcional, contrato financiero compatible y acta verificable. Conservar la aceptación de la UX del marketplace registrada en #307; no convertirla en aceptación retroactiva de DOCKER-02 ni en lanzamiento productivo.

Fuente diagnóstica: auditoría original `planning/auditorias/revision_integral_local_y_gcp_20261014.md`, conservada sin modificación en el checkout habitual y en una copia excluida; sus propuestas no se ejecutan sin contrastarlas con el código. Resumen y corroboraciones del corte 00 se registran en la matriz/evidencia versionadas; no se publican los logs/JSON de origen. Baseline inicial: `28f63f3dcba5d1b6a57b47b0c229e0caf2bba7eb` (#307); LOCAL-CLOSE-00 se fusionó en #308 por `HernanEspinozaDev`, merge `a26e1dacbe23cbc4b73528c69ee0e6b4daf394ea`. LOCAL-CLOSE-01 parte de ese `main`. Si main avanza, registrar el nuevo baseline y revisar el delta antes de ensayar.

Contrastado contra el árbol de `origin/main`: `VerificationPort` carece de retirada; `nullableExactClpKeys` solo incluye `estimated_total_clp`. GitHub #267 está abierto y #269 se cerró administrativamente el 2026-10-12, con aceptación funcional expresamente incompleta. Cinco modificaciones frontend y los documentos de auditoría permanecen en el checkout habitual sin integrar; se preservaron en `.local/preservation/local-close-00-20261014/`, fuera de Git, con copia, patch, hashes y manifest. El checkout `main` habitual sigue en `c797f0e` y no se sincronizó porque conserva ese trabajo. El worktree de release limpio/canónico para estos cortes es `/home/nandev/.codex/worktrees/local-close-00-baseline-plan/espaciGo`, rama `codex/docs/chore/local-close-00-baseline-plan`, HEAD `28f63f3`.

Este plan está versionado como `planning/cierre_local/plan_cierre_desarrollo_local.md`, acompañado por `matriz_cierre_local.md` y una carpeta de evidencia por corte. La aceptación humana de MPV2-13 permanece acotada al estado UX/demo/código `736f81a` aceptado en #307; el PR #307 se fusionó por `HernanEspinozaDev` en `28f63f3`. Esa aceptación no cierra DOCKER-02 ni el cierre local integral. El documento de auditoría es diagnóstico, no instrucciones ejecutables: validar sus propuestas contra contrato/código vigente. La copia completa revisada no es necesaria en este PR; se publican solo los findings resumidos/verificados en el plan y la matriz, y se excluyen logs/JSON sin revisar.

## Estado conciliado al 2026-10-14

- **FE-15:** PR #292 fusionado por `HernanEspinozaDev` en `5405a82e90bc429aa89666ead430fe2cb39f102e`; Angular reemplazó el mock como frontend predeterminado. Evidencia anterior a esa fusión que dice “FE-15 no se inicia” se conserva como historial, no como estado actual. Toda nueva corrida usa Angular/Go/PostgreSQL vigentes, no exige el mock retirado.
- **MPV2-13:** PR #307 fusionado en `28f63f3dcba5d1b6a57b47b0c229e0caf2bba7eb`; el usuario aceptó el estado UX/demo/código `736f81a` el 2026-10-14. Es una aceptación de producto acotada, no una aprobación retroactiva de Docker ni del cierre local.
- **DOCKER-01:** #268 cerrada; PR #271 fusionado por `HernanEspinozaDev` en `e074d78f126cd3a5bbe9c108da26178bdb726ad4`, aceptado para el alcance de empaquetado/configuración.
- **DOCKER-02:** #269 está cerrada desde el 2026-10-12 por petición administrativa, pero su comentario de cierre declara incompletos liquidación posterior a 24 h, estado `cerrada`, historial, comprobante e idempotencia. #267 sigue abierto. No marcar DOCKER-02 aceptado hasta conciliar cada criterio con evidencia actual y obtener aceptación funcional explícita.
- **DOCKER-03:** #270 cerrada; PR #273 fusionado por `HernanEspinozaDev` en `b0f62b976c9586d260cea15ed9acc29ab12257a7` y aceptado dentro de su alcance. La evidencia del corte deja expresamente DOCKER-02 pendiente.
- Los recorridos de #307/FE-14, la automatización y la inspección estática son evidencias de alcance distinto. Reutilizarlos donde aplique requiere fijar criterio, SHA, delta y límite; no se cuentan como una nueva corrida ni como prueba de liquidación del stack DOCKER-02.

## Qué significa terminado

1. Los pendientes LOCAL-AUD-01, 02 y 04 están corregidos con pruebas conectadas y regresión.
2. LOCAL-AUD-05 queda resuelto mediante un árbol canónico de release limpio, cambios ajenos conservados y trazados.
3. LOCAL-AUD-03 tiene evidencia nueva por criterio de DOCKER-02, incluyendo liquidación, `cerrada`, historial, comprobante e idempotencia; ninguna casilla se completa solo porque un Issue/PR está cerrado.
4. Un único SHA de código probado, imágenes identificadas, contrato/SDK, V1–V57 y demo corresponden al mismo release. El acta posterior puede añadir solo documentación y debe acreditar que el código no cambió.
5. Se ratifican límites sintéticos, procedimientos de instalación/restore y pendientes de destino. Revisión técnica y fusión son distintas de aceptación humana final del cierre local.

## Ejecución y ramas

Tres cortes secuenciales, un PR por corte, base main; corregir comentarios en el mismo PR y esperar fusión antes del siguiente. Nombres nuevos `local-close-00`–`local-close-02`, sin reutilizar ramas MPV2 fusionadas. Incorporar esta extensión a AGENTS y enlazar el nuevo plan.

Desarrollo/commits/push/PR/respuestas: HernanMEC; revisión/aprobación/fusión y administración solicitada: HernanEspinozaDev. Mantener GPT-6 Luna con razonamiento medio en el chat de desarrollo. Comunicación bidireccional autorizada: avisar a revisión al publicar, corregir, terminar o bloquearse. No detenerse por decisiones rutinarias que el contrato ya resuelve.

### LOCAL-CLOSE-00 — base reproducible y matriz de cierre

Rama sugerida: `codex/docs/chore/local-close-00-baseline-plan`.

- Inspeccionar estado y fetch/prune. Crear worktree limpio desde origin/main; no ejecutar reset/pull ciego en el checkout habitual.
- Guardar copia y patch de los cinco cambios locales, con hashes y manifest, en almacenamiento local excluido; comparar con #307. Separar duplicados, cambios de producto no integrados y cambios ajenos. No mezclar esas modificaciones en los PR de cierre ni borrarlas. Si no puede hacerse fast-forward del checkout sin alterar trabajo ajeno, el worktree limpio de release será la base canónica y se documentará la deuda del checkout; no es necesario esperar permiso para trabajar allí.
- Preservar la auditoría original. Preparar una copia revisada para versionar si es útil; excluir logs/artefactos con cuentas, secretos o contenido privado.
- Versionar plan y matriz: cada LOCAL-AUD y cada criterio DOCKER-02 con fuente, prueba, SHA, resultado, límite, PR y condición de aceptación.
- Conciliar el estado vigente de FE-15, MPV2-13 y DOCKER-01/03 con sus merges/aceptaciones. Mantener resultados históricos fechados; no sustituirlos por números nuevos sin corrida.
- Identificar recursos retenidos y responsables; conservar las demos anteriores y `espacigo_pgdata`. No limpiar por prefijos amplios.

Salida: PR documental con base identificada, matriz utilizable, cambios locales preservados y ninguna afirmación de cierre integral todavía. Verificar enlaces, fuentes y diff; no repetir suites completas en este corte sin necesidad.

Registro LOCAL-CLOSE-00: PR #308 se fusionó en `main` por `HernanEspinozaDev`, merge `a26e1dacbe23cbc4b73528c69ee0e6b4daf394ea`. El corte fue documental: no ejecutó suites ni inició servicios. La copia verificable de cambios no integrados/auditoría local está descrita en `evidencia/local-close-00/README.md`.

### LOCAL-CLOSE-01 — paridad KYC, importes nulos y contraste

Rama sugerida: `codex/marketplace/fix/local-close-01-parity-contract`.

Orden dentro del corte: revisar datos y contrato → Go/OpenAPI si corresponde → adaptadores/puertos/fachadas Angular → UI → pruebas y evidencia. No modificar DB ni reglas financieras por defecto; si existe una necesidad real, migración aditiva desde V58 y pruebas vacío/upgrade, nunca reescribir V1–V57.

**LOCAL-AUD-01: retirada propia de evidencia KYC/KYB.**

- Usar el DELETE del titular ya existente, sin sustituirlo por el administrativo. Añadir operación al puerto/adaptador, fachada y acción accesible de confirmación; presentar permisos/estados según Go.
- Actualizar lista/historial cuando lo confirme Go, revocar Blob URL privada y limpiar selección correspondiente. No afirmar destrucción completa de copias si el backend solo retira evidencia lógica.
- Cubrir cancelación, éxito, error, retirada repetida según semántica real del contrato, cuenta/tercero, 401/403/404, logout/login misma cuenta, cambio de caso y respuestas tardías. Un resultado antiguo no elimina una evidencia nueva ni afecta sesión distinta.
- Mantener Bearer solo en memoria, errores genéricos contra terceros, transporte en infraestructura y claves de reintento solo si el contrato las requiere.

**LOCAL-AUD-02: contrato monetario nullable.**

- Auditar campos CLP y su nulabilidad en OpenAPI/modelos; aceptar `guarantee_expected_clp:null` cuando el contrato lo permite, sin ampliar todos los importes a nullable.
- Preservar ausencia, null y cero como estados distintos; no completar 50.000 ni convertir null en cero. Mantener int64 exacto sin pasar por Number.
- Pruebas codec y adaptador con respuesta histórica completa: null/ausente/cero, máximo int64, payload inválido y campos no nullable. Probar presentación tolerante sin inventar monto. Documentar que la reproducción era compatibilidad contractual, no un fallo universal del wire Go.

**LOCAL-AUD-04: placeholder.**

- Superficie uniforme o respaldo de texto con contraste calculado verificable; no basarse solo en axe ni en extremos del degradado. Revisar categoría con opacidad y texto normal.
- Conservar paleta, cuatro columnas desktop/lista móvil y selector 12/24/48 aceptados. Verificar visual real móvil/escritorio y foto retirada/error.

Salida: pruebas enfocadas Angular/Go pertinentes, lint/arquitectura/tokens, api:check sin servidor concurrente, build y recorrido Angular→Go→PostgreSQL para retirada propia/tercero. Capturas y resultados con SHA; matriz actualizada. No hace falta sustituir proveedores ni rediseñar la arquitectura.

Registro de ejecución LOCAL-CLOSE-01: PR #309 abierto desde `codex/marketplace/fix/local-close-01-parity-contract`, código probado en HEAD `726b419824a26a79797561b6b77db6d92af0170e`, sobre `main` `a26e1dacbe23cbc4b73528c69ee0e6b4daf394ea`. No se añadieron migraciones ni cambios Go/OpenAPI: el DELETE propietario ya estaba en contrato/autorización. Pasaron 80 pruebas Angular enfocadas, `go test ./internal/verification/transport/http`, lint/arquitectura/tokens, `api:check`, build SSR/prerender y E2E aislado 3/3 con base vacía, Angular→Go→PostgreSQL y capturas a 390×844/1365×900. El ensayo `espacigo_localclose01_726b41991ad6` usó puertos 20750–20753 y volumen dedicado que el runner retiró; estado/credenciales sintéticas 0600 eliminados. No tocó demos ni `espacigo_pgdata`. La medición de contraste calculada desde estilos computados fue 12,57:1 para ambos textos. Fallos iniciales de selectores E2E quedaron corregidos y el comando completo pasó al final; detalle, límites y capturas en `evidencia/local-close-01/README.md`. PR #309 solicita revisión de `HernanEspinozaDev`; no iniciar LOCAL-CLOSE-02 antes de fusionarlo.

### LOCAL-CLOSE-02 — regresión integral, Docker y acta final

Rama sugerida: `codex/marketplace/chore/local-close-02-final-validation`.

Un ensayo limpio y aislado sobre el código fusionado de LOCAL-CLOSE-01. Registrar instalación desde cero, migraciones vigentes, runtime/admin, imágenes, configuración, puertos y recursos antes del arranque. Reutilizar runners y fixtures de código, ampliando solo las comprobaciones faltantes.

- Gate backend completo, go vet e integraciones obligatorias con PostgreSQL real; declarar omitidos/fallos, no contar un go test sin DB como prueba de integración.
- Instalación Angular desde lockfile, lint/arquitectura, generación/verificación SDK, suite Angular, build SSR/prerender y auditoría de producción registrada por fecha. Mantener QA Go independiente de Node.
- Recorrido servido de titular, anfitrión, arrendatario, tercero y admin: identidad/KYC, retirada propia, oferta/calendario, catálogo público, reserva/pago fake/garantía, decisiones, firmas/PDF, check-in/out/recepción, disputa/descargo/resolución, conciliación/liquidación/cierre/comprobante y exportación privada. Cubrir ramas financieras relevantes mediante casos explícitos, no exigir que una sola reserva recorra estados incompatibles.
- **DOCKER-02:** evidencia concreta de límite antes de 24 h, éxito tras plazo, estado `cerrada`, historial, comprobante y repetición de liquidación/conciliación sin duplicar ledger u operación. Probar rechazo de terceros e importes exactos. Capturar respuesta y referencias sintéticas sin secretos.
- Usar reloj de negocio manual ya autorizado SOLO en overlay del ensayo: no cambiar hora del host, sesiones ni SQL histórico del volumen persistente. Mostrar hora lógica y zona IANA. Resolver aislamiento de fichero/ruta del reloj para esta corrida si el helper está fijado a otra demo; nunca reutilizar el reloj de una demo retenida.
- Arranque/reinicio, caída/restauración Go y workers, persistencia e idempotencia; recuperación/restore en DB y archivos propios. Revalidar DOCKER-03 afectado o documentar evidencia histórica aplicable con criterio y delta; no afirmar una nueva corrida que no se ejecutó.
- Catálogo ocho categorías y más de 48 ofertas, orden/filtros/paginación, retorno de login, SEO/SSR/fotos retiradas, navegación por teclado y contraste en 390×844/1365×900.
- Completar matriz con niveles diferenciados: unitario, API simulada, integración Go, E2E real y captura. Actualizar `local_docker.md`, FE-15, matriz PUB y número de operaciones obtenido del contrato actual; no completar 35 filas automáticamente.
- Producir acta `planning/cierre_local/acta_cierre_local.md`: alcance/exclusiones, SHA y hashes, comandos/resultados, cobertura y límites, incidencias, instalación/restore/rollback local, demos retenidas/responsable y pendientes GCP.

Salida: PR técnico revisable y demo final vinculada al código corregido. Solicitar aceptación humana del **cierre integral local** después de revisar evidencia; la aceptación anterior del marketplace se conserva pero no sustituye esta decisión. Hasta entonces #267 no se cierra; #269 conserva historial administrativo. Después de aceptación y fusión, registrar evidencia nueva en Issues según alcance y cerrar solo los criterios realmente cumplidos; no cerrar padres globales por inferencia.

## Entornos y seguridad de ejecución

Proyectos Compose, puertos, volúmenes y directorios exclusivos; validar configuración resuelta antes de crear recursos. Ningún ensayo usa `espacigo_pgdata` ni datos/secrets de demos ajenas. Credenciales sintéticas recreables en `.local/testing/`, permisos 0600, excluidas de commits/logs/capturas. Retirar recursos propios al acabar; si demo final queda retenida, documentar motivo y responsable. Conservar tests, fixtures y evidencia histórica versionada.

## Puerta de salida hacia GCP

El cierre permite pasar a **diseño de staging sintético**, no a producción con personas/fondos reales. Preparar después, como tarea distinta, `GCP-PREP-01`: topología SSR/API/DB/ejecutor/almacenamiento; IAM/secretos; compatibilidad real de PostgreSQL/extensiones; registro externo de bajas durable; workers y recuperación; límites públicos/429; URLs/domino/HTTPS, observabilidad, backup/restore/rollback y costes/límites acordados. Verificar decisiones contra documentación oficial vigente al diseñar el destino.

No crear recursos ni desplegar con este plan. Proveedores reales de pagos/correo/KYC/firma, textos legales, fotos reales y apertura pública siguen siendo decisiones y fases distintas. El staging cerrado puede mantener fakes explícitos; no basta ocultar avisos de ensayo para llamar al producto productivo.

## Encargo inicial al chat de desarrollo

Ejecutar la secuencia LOCAL-CLOSE-00 → 01 → 02 ya autorizada, manteniendo un PR abierto a la vez. LOCAL-CLOSE-00 (#308) ya se fusionó; LOCAL-CLOSE-01 es el corte actual y LOCAL-CLOSE-02 no empieza antes de fusionar PR01. Tras revisar y fusionar cada PR, desarrollo espera el encargo del chat de revisión para iniciar el corte sucesor, sin pedir una autorización nueva del usuario para la secuencia aprobada. En cada publicación, corrección, término o bloqueo, avisar al chat de revisión mediante `send_message_to_thread` antes de finalizar. No iniciar GCP: requiere otro alcance/encargo. Mantener pendiente la aceptación final del cierre local integral.
