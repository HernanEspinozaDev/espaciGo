# Continuación F5: distintivos independientes y scanner — 2026-11-05

## Alcance y autorización

El usuario pidió ejecutar el orden recomendado: cerrar el distintivo y su ciclo visible, recuperar scanner y repetir el caso bloqueado, verificar localmente y presentar el resultado para su aceptación. Integración a `main`, F7 y cloud requieren autorización posterior, no inferida.

Al comprobar los puertos se detectó la base del ensayo y ClamAV detenidos. El usuario autorizó explícitamente: «Autorizar recuperación exacta del backup final». La recuperación se limitó al mismo destino `127.0.0.1:15434/espacigo_f5_gate_20261101`, imagen PostgreSQL 18.6 fijada y backup final del 2026-11-05T06:47:14Z. No se alternaron destinos ni se resetearon credenciales ni crearon cuentas.

## Recuperación comprobada

- Backup DB y storage: hashes verificados y lectura completa del dump con `pg_restore --file=/dev/null`.
- Roles de contrato preparados con el runner y SQL revisados; preview con rollback seguido de ausencia independiente, aplicación y restore transaccional con owners/ACL. El TOC omitió únicamente la creación de `tiger` y `topology`, ya instalados por las extensiones fijadas.
- Readback independiente: mismas dos cuentas y ledger83; V1–V83 coinciden con todos los checksums de las fuentes. Login existente y `SET ROLE espacigo_runtime` sin elevación comprobados con el passfile sin cambios.
- Los777 archivos retenidos coinciden byte a byte con el archivo de storage del backup; no fue necesario sobreescribirlos.
- API restaurada con perfil `local-http-worker-free`, reviewer local, sin booking trial ni workers generales; `/health/ready` devuelve200.

## Scanner: bloqueo histórico resuelto

Se restauraron las firmas oficiales respaldadas y se intentó la descarga oficial acotada. El updater histórico conservaba como NetworkMode una red retirada, por lo que se usó un updater efímero con la misma imagen fijada y volumen de firmas, sin conectar la API a egress. Al terminar se retiraron updater y red, verificados por lectura independiente.

- freshclam exit0: daily28173 →28174.
- `sigtool`: daily28174, build2026-11-05T06:26Z; scanner `VERSION` confirmó esa misma versión.
- INSTREAM real: control limpio `OK`, EICAR `FOUND`.
- Política máxima48h conservada, sin fabricar fechas ni estados.
- Backup nuevo de firmas oficiales: SHA256 `cde24d0153f773e3b3fed0235bb3f47c079b432e911b16ff6b3ab3c455d68bdc`, privado0600 fuera de tmpfs.
- Se reanudó exactamente el borrador cuyo escaneo había quedado bloqueado, usando el servicio y processor reales con rol limitado. Resultado `NEW_CASE_INSPECTED_REAL_SCANNED_SUBMITTED_RUNTIME_ROLE_PASS`. Readback independiente confirma expediente `enviada`, archivo `limpio` y cola de scan sin pendientes.

## Distintivo: registro intermedio previo a la integración

Contrato opcional nuevo `verification` en PublicSpace/PublicSpaceDetail, únicamente dos booleanos. No contiene propietario, expediente, archivo, revisor, fechas ni objetos privados. UI independiente del permiso de publicar; identidad y antecedentes del espacio no acreditan dominio, legalidad, garantía ni firma.

La primera hoja backend entregó solo el tipo/serialización: no se aceptó como implementación completa. Se encargó por separado la derivación SQL list/detail. La hoja frontend dejó errores de tipos que el padre corrigió mediante DTO→dominio explícito, sin casts para ocultarlos. Se añadieron regresiones de refresh que retiran los flags previos y texto accesible sin depender del hover; RED→GREEN.

Pruebas parciales ya ejecutadas: tipo/serialización Go; frontend typecheck,142 unitarias en una corrida anterior a las dos nuevas regresiones, dos regresiones adicionales PASS, lint sin errores (un warning preexistente de `img`) y build. No se infiere todavía prueba nativa del distintivo ni aprobación global de F5.

## Integración y regresiones posteriores

La consulta inicialmente entregada usaba `EXISTS` sobre cualquier aprobación. La prueba PostgreSQL real reprodujo nueve fallos: al revocar o vencer el expediente más reciente reaparecía una aprobación anterior. Se corrigió la selección para escoger primero la última decisión por ámbito y titular (`updated_at DESC,id DESC`, coherente con `CurrentForAccount`) y validar después estado, fechas, revocación y sustitución. Se corrigió también `identityCurrent`, que compartía el defecto. No se tocaron bytes de migraciones ni ACL.

- Diez escenarios reales del catálogo PASS después de RED: aprobación, última identidad/espacio revocados, vigencia vencida sin worker, documento vencido, aprobación futura y expediente sustituido. En todos se retiró KYC legado dentro de la misma transacción para acreditar publicación independiente. Fixtures de metadatos con rollback y ausencia posterior comprobada: no se presentan como inspecciones ni decisiones de producto.
- Cuatro escenarios PostgreSQL de `identityCurrent` PASS: vigente, revocada, vencida y documento vencido; sin resucitar aprobación anterior.
- El catálogo antes se montaba solo con `LOCAL_BOOKING_TRIAL`. Se añadió `LOCAL_PUBLIC_CATALOG_TRIAL` limitado al target loopback exacto y perfil worker-free, con clave estable privada separada y tests negativos. Las rutas de reserva/pago no se activaron.
- Go completo y vet PASS; frontend144 unitarias PASS, tipos/lint/API/formato PASS; lint conserva solo el warning anterior de `img`. Build PASS. Global E2E en4210:76PASS/53SKIP; la omisión adicional es el nuevo ensayo nativo opt-in, probado aparte.

### Recorrido nativo del distintivo

Cinco invocaciones Playwright reales PASS, sin mocks, con decisiones en la interfaz, BFF/Go/PG/storage y dos cuentas existentes:

1. Revocación normal de la identidad previamente aprobada para establecer el inicio sin distintivo; lectura pública sigue200 y lista conserva el anuncio.
2. Aprobación del nuevo expediente de identidad, con PDF limpio descargado: aparece «Identidad verificada» en detalle público anónimo.
3. Nuevo expediente real de espacio inspeccionado/escaneado/enviado; aprobación muestra ambos distintivos y revocación retira únicamente el de espacio.
4. Segundo expediente real de espacio, vigencia breve confirmada por UI: ambos visibles; vencimiento por reloj real sin worker retira el distintivo de espacio y conserva identidad. No se adelantó reloj ni reescribió SQL para simular expiración.
5. Revocación de identidad mediante UI: ambos distintivos ausentes, anuncio público y listado siguen accesibles.

AXE desktop/mobile y ausencia de overflow PASS en cada recorrido. Capturas públicas antes/aprobado/revocado/vencido retenidas en scratch; inspección visual del móvil aprobado confirma ambos distintivos, sin datos privados de expedientes ni PII. La captura no sustituye las lecturas API y pruebas anteriores. Los tests de refresh unitarios también cubren tarjeta/listado; no se afirma todavía una observación nativa específica de cada tarjeta del catálogo.

El primer intento de harness apuntó por error a un inventario de galería sin cuentas y falló antes de login; se seleccionó el inventario privado correcto por correspondencia de los dos UUID existentes. No se crearon cuentas ni cambiaron credenciales. El historial privado de grants se reconcilió por lectura independiente de revocación y auditoría ya persistidas antes de retirar exactamente la asignación vigente; no se improvisaron permisos SQL.

## SHA y límites

Revisión frontend independiente: sin defectos válidos,9pruebas focalizadas PASS. La objeción backend sobre `identityCurrent` y una identidad sustituida no es un estado válido: V80 impone `CHECK(superseded_at IS NULL OR purpose='space')`. Se agregó prueba PostgreSQL real, con savepoint, que comprueba SQLSTATE23514 y el nombre exacto de la constraint; los cuatro escenarios del helper vuelven a pasar. Re-revisión `deleg_c8b959fd`: retiró la objeción y no encontró otro bloqueo backend demostrable; revisión original `deleg_784b1991`. No se añadió una mutación artificial ni se alteró el esquema para sostenerla. Los informes son revisiones source-only y no sustituyen las pruebas PG/nativas ejecutadas por el padre.

Demo retenida: se creó otro expediente real de identidad (sin cuentas nuevas), inspeccionado, escaneado limpio y enviado; mismo grant exacto vigente por identidad. La sexta invocación nativa, modo `demo`, pasó sobre el standalone definitivo: login, PDF descargable, expediente aún enviado, detalle público anónimo sin distintivos y tarjeta real de explorar sin distintivos. AXE desktop/mobile PASS. No se sobrescribieron capturas históricas de reviewer. El proceso Next se reinició con entorno mínimo, sin variables DATABASE, passfile o clave de documentos; se verificaron PID/UID/ejecutable y entorno reales antes de conservar el recibo.

Backup final retenido fuera de tmpfs `20261105T080824Z`: DB SHA256 `f08ddde9842bcce658740813936d16da4bb39938efe03e42f0ce96c94436be58`; storage SHA256 `4870e0a73bb9edd8456cf8ea2680b62a4ebfad1d09300d547e03d78fc5b21430`. Validado dump completo, hashes y ledger83. Los recursos de esta demo se conservan para revisión humana; no detener contenedores tmpfs ni borrar volúmenes/backups. Responsable: agente padre durante el corte; retiro únicamente por autorización posterior y procedimiento de ensayo.

Bases: backend `c201a2f4cf58e985f76dd311e5ccf9e099b737cd`, frontend `07a07bdef598a5dab2fdbd6e86fc343e8d1e4c38`. Código definitivo probado y revisado backend `ce7aa563d40316448cc116e4d52e90c3e487a317`, frontend `62310202e9b5cfbc5f80db906966cee9368d541f`. Los commits siguientes son evidencia/documentación/capturas públicas, no cambio funcional. Se conservan la evidencia previa de creación, galería, publicación/republicación y los cierres ACL/codec/BFF. Ninguna nueva migración ni alteración de ACL. Los refs documentales finales se verifican y entregan tras push normal a ambas ramas; no se infieren CI remoto ni aceptación humana. Integración a main y F7 no ejecutados.

## Reproducción de la demo y límites operativos

- URL reviewer: `https://127.0.0.1:4203/mi-cuenta/revision-documental`; catálogo: `/explorar`. La demo pendiente conserva un expediente de identidad enviado/limpio con asignación vigente; el anuncio público sigue visible sin distintivos porque las decisiones usadas en la regresión quedaron revocadas o vencidas. No se revirtieron esas decisiones para aparentar un estado de producto.
- Configuración, inventario, selección de fixture, claves y passfiles permanecen exclusivamente en `.local/testing/`, modo privado; no copiarlos al frontend ni imprimirlos. Los launchers y un binario con digest verificado se conservaron bajo el directorio privado `space-gallery-demo/retained-f5-launch/` fuera de scratch. Un reinicio requiere comprobar recibos/PID/UID/ejecutable/target y el estado tmpfs, no ejecutar ciegamente un PID histórico. El perfil API es `local-http-worker-free`, con `LOCAL_DOCUMENT_REVIEW_TRIAL=1`, `LOCAL_PUBLIC_CATALOG_TRIAL=1` y clave privada estable mediante `LOCAL_PUBLIC_CATALOG_CURSOR_KEY_FILE`. `LOCAL_BOOKING_TRIAL` permanece ausente.
- Build frontend `pnpm build`, copia standalone de `public` y `.next/static`; Next escucha solo4204 con `API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1`. TLS loopback4203 y forward PG15434 separados. Next no recibe credenciales DB ni claves documentales.
- Harness versionado: `pnpm exec playwright test --config playwright.public-verification-native.config.ts`. Exige opt-in `PHASE_PUBLIC_VERIFICATION_NATIVE_E2E=1`, raíz/inventario/fixture privados explícitos y `PHASE_PUBLIC_VERIFICATION_SCENARIO` (`identity`, `space-revoke`, `space-expire`, `identity-revoke` o `demo`). Los primeros cuatro consumen decisiones del fixture y no deben repetirse como si fueran read-only. `demo` comprueba el pendiente y PDF sin decidir. El fixture público de demo debe seleccionar el espacio activo existente del mismo titular, no inferir publicación de un borrador usado para el documento.
- Capturas públicas versionadas nuevas: `f5-public-badges-approved-mobile-20261105.png` y `f5-public-badges-expired-mobile-20261105.png`; las capturas anteriores se conservan sin sobrescribir.
- La vigencia48h del scanner sigue corriendo: la actualización no es un bypass permanente. Si caduca, debe fallar cerrado y renovarse por descarga oficial acotada autorizada, retirando egress después.
- Siguiente puerta humana: revisión y aceptación del resultado local. Después, autorización específica para integración a main/release F7; no ejecutar reservas, firma, cobros o garantía por esta evidencia.
