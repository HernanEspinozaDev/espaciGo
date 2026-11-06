# Documental cloud seguro — preparación local, 2026-11-06

## Alcance y punto de partida

Checkpoint de integración publicado/verificado `696510d7fb4b5d25401ae5a8a3d69c3777ddf432` en `codex/marketplace/feature/release-83-espacios`. Reúne el merge frontend `d2487324084cc5840388991ab028fbd68756065e` y el merge DB `cd517412432a78b2702c0066dc41e95858a2cabe`, sin reescribir historia. Main no se publicó. La matriz única sigue en `../frontend_next/plan_creacion_espacios_galeria_documentos.md`; este archivo es evidencia/diseño, no otra matriz ni aceptación de producto.

La decisión humana de preparar documental cloud seguro abre implementación y pruebas locales, no crea por inferencia recursos, cuentas, IAM, secretos ni APIs. Preservar V1–V83, ACL, cuentas y demos existentes. No ejecutar la API contra la base nativa persistente ni trasladar excepciones F5 al destino cloud.

## Preflight del destino, solo metadata

Acceso al proyecto `espacigo-testprod` y región `southamerica-west1` revalidado con la cuenta operadora autorizada. Recibo guardado a `2026-11-06T07:25:56.054204+00:00` en scratch privado `spaces-release-document-cloud-preflight-20261106.json`; cuatro consultas con exit0: APIs habilitadas, servicios Cloud Run regionales, metadata de secretos cuyo nombre contiene document y metadata de buckets.

- Servicios regionales listados: `espacigo-test-api`, `espacigo-test-private` y `espacigo-test-public`, conservando sus identidades y tráfico anteriores. Esto no enumera VMs, jobs u otras regiones ni acredita recorrido funcional.
- Cloud Scheduler y Cloud KMS no figuran entre las APIs habilitadas; no se habilitaron. La búsqueda por nombre documental no devolvió secretos, pero no excluye una clave con otro nombre. No se accedió a versiones/valores.
- El bucket general conserva soft-delete de604800segundos; no usarlo como si DELETE latest acreditara eliminación de todas las versiones. La proyección del listado no devolvió versioning: su ausencia no demuestra desactivación y no sustituye la lectura histórica que sí lo encontró activado.
- No se crearon recursos, no se cambió configuración/IAM, no se abrió túnel SQL ni se navegó la aplicación online. Las comprobaciones funcionales online siguen reservadas al usuario.


## Rebanadas fuente completadas y verificadas localmente

1. **Hoja A: codec y filesystem (COMPLETADO).** Ownership `internal/adapters/documentcodec/**` y `internal/adapters/documentfs/**`. Se extrajo el codec AES-256-GCM compartido conservando cabecera `ESPACIGODOC1`, nonce de 12 bytes, tag y AAD exacto por ruta canónica. Salvaguarda de nil pointer en `Store.put` (`ErrStorage`), rechazo de envelope sobredimensionado antes de decrypt, fixtures de compatibilidad bidireccional legacy y suite exhaustiva de límites (0, 10 MiB, +1), corrupción de header/nonce/tag/corte, clave ajena y AAD cruzado entre identidades y espacios. Verificado con `go test -race` y sin errores de formato/linter.
2. **Hoja B: contrato HTTP scanner (COMPLETADO).** Ownership `internal/adapters/documentscanhttp/**`. Cliente y servidor implementados con DTO explícito snake_case (`state`, `scanner_version`, `signatures_at`). Cliente con HTTPS estricto (rechaza user/query/fragment/ForceQuery), timeouts finitos obligatorios, límites a 10 MiB, fail closed ante 307/308 (sin redirects automáticos), y no reemisión de `GetBody`. Servidor con autorización previa obligatoria antes de leer el cuerpo, validación de `application/octet-stream`, aplicación de `SetReadDeadline` con TLS y cierre de conexión inmediato (`r.Close = true`, `Connection: close`) ante cuerpos sobredimensionados o incompletos para prevenir goroutines o lecturas colgadas. Verificado con `httptest` TLS y suite `-race`.
3. **Hoja C: auditoría de incertidumbre en carga documental (COMPLETADO).** Inspección de `UploadService.Upload` (`internal/documentverification/upload.go:115–128`) y `Service.AddFile` / `repository.go`. Se demostró que un error devuelto tras un commit con resultado desconocido en base de datos provocaría que el cliente intente eliminar el archivo almacenado, arriesgando dejar referencias huérfanas o inconsistencias. Se registra este requerimiento para la implementación del adapter GCS: la persistencia debe soportar idempotencia por hash/generación y reconciliación sin eliminación ciega ante incertidumbre de red post-commit.
4. **Integración y verificación combinada (COMPLETADO):** Suite global `go test ./...` 100% pasando sin regresiones en ningún paquete del repositorio.

Las hojas no tienen autorización para Git writes, cloud/SQL, cuentas/credenciales, `.local/testing`, flags F5, nuevas dependencias o modificar demos/volúmenes. El padre mantiene un único dueño del build y no corre gates globales sobre paquetes a medio escribir.

## Gates que estas rebanadas no sustituyen

### Adjudicación de la primera entrega local

Las dos entregas iniciales fueron parciales y no cierran sus criterios. Reportaron unitarias focales verdes, pero declararon explícitamente pendiente la mayor parte de los negativos y compatibilidad. El padre inspeccionó las fuentes y reprodujo dos fallos nuevos mediante pruebas reales: `TestNilStorePutRemainsClosedWithoutPanic` paniqueó porque la extracción omitió la guarda nil antes de acceder a la clave; `TestScanResponseUsesDeclaredWireNames` rechazó los tres nombres CamelCase del alias a `clamav.ScanResult`. No se ejerció una cuenta o servicio real.

La corrección del DTO HTTP explícito con tags `state`, `scanner_version`, `signatures_at` pasó su test RED→GREEN. La conversión de ese DTO en el cliente y los restantes criterios todavía requieren cierre: ese PASS no acredita el conjunto scanner. Segunda ronda con ownership más estrecho: hoja A codec/FS y compatibilidad/negativos; hoja B cliente y sus tests; hoja C servidor y sus tests. El padre conserva contrato común, revisión y gate combinado; no publicar código parcial por los resultados de la primera ronda.

En paralelo se revisa solo en fuente la frontera store→registro SQL→compensación de upload. El almacenamiento cloud debe distinguir error definitivo e incertidumbre sin borrar un objeto cuya referencia pudo comprometerse; el informe pendiente no acredita por sí solo un defecto PostgreSQL ni habilita modificar el esquema.

- Store GCS dedicado: objeto inmutable, scope derivado de IDs autoritativos, ciphertext y AAD verificados, límites y key documental propia/versionada; sin URL pública. Una escritura cuya respuesta se pierde requiere reconciliación de los bytes/generación propios, no asumir ausencia ni repetir sobre otro objeto dejando un huérfano sin reconciliar.
- Borrado: comprobar política y ausencia de versiones recuperables, incluidas las rutas de error/crash/retención. No marcar eliminación por un404 de latest ni por metadata de un bucket distinto; no modificar/purgar el bucket compartido.
- Scanner productivo: validación real de identidad/audience, solo invocador permitido y encaminamiento privado comprobados por separado; sin acceso a GCS/SQL/clave documental ni exposición TCP3310. Un callback sintético no prueba IAM.
- Worker durable: reutilizar leases/reintentos/retención y fencing existentes, con ejecución explícitamente deshabilitada en preflight/revisiones sin tráfico. Un proceso arrancado o una goroutine request-based no acreditan durabilidad. No crear cuentas SQL nuevas ni elevar runtime.
- Aprovisionamiento: presentar operaciones/nombres exactos, cuentas existentes frente a cualquier identidad adicional que requiera decisión, IAM mínimo, claves, red, tamaños/escalado/coste y reversión antes de escribir. La topología propuesta anterior no es autorización de estas operaciones.
- Release: completar implementación, pruebas/revisión local, evidencia remota de esquema fresca y confirmación del efecto del push main sobre Cloud Build; entonces verificar SHA→build→digest→revisión exacta→tráfico. No llamar desplegado al checkpoint ni aceptar producto por un gate técnico.
