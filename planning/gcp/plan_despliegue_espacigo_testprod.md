# EspaciGo — plan de transición a Google Cloud

Fecha de comprobación: 2026-10-15. Estado: **planificación, sin recursos aprovisionados por esta tarea**.

## 1. Decisiones y límites del encargo

- El usuario solicita preparar el traslado de la aplicación y los datos de prueba a `espacigo-testprod`.
- **Catálogo público desde el inicio**, con búsqueda, categorías como filtros, detalle y fotos públicas autorizadas. No exigir cuenta EspaciGo para explorar.
- No se fija un tope monetario mensual, según la respuesta del usuario. Esto no elimina el inventario de costos, las cuotas técnicas ni el control de consumo.
- El usuario quiere utilizar la prueba gratuita sin actualizar a una cuenta de pago. **No activar/actualizar facturación de pago**. La parada al agotarse los créditos solo se puede asumir después de comprobar que la cuenta sigue siendo Free Trial; una cuenta de pago puede conservar crédito promocional y seguir cobrando después.
- Por ahora se autoriza planificación y comprobación de lectura. Crear recursos, habilitar APIs, publicar servicios, subir datos o cambiar IAM requiere la siguiente instrucción de ejecución; no se deduce de tener el proyecto creado.
- El nombre `testprod` no convierte este entorno en producción con personas, identidad, fotografías, contratos o dinero reales. Proveedores y datos siguen siendo de ensayo.
- La aceptación local ya dada se conserva; la aceptación del despliegue en GCP será una decisión humana distinta.

## 2. Destino comprobado mediante CLI, solo lectura

| Campo | Resultado observado |
| --- | --- |
| Proyecto | `espaciGo-testprod` |
| ID | `espacigo-testprod` |
| Número | `447373903153` |
| Estado | `ACTIVE` |
| Organización padre | `pasteleriahijitos-org`, ID `644690775054` |
| Facturación vinculada | Habilitada; cuenta abierta, nombre `Mi cuenta de facturación`, moneda CLP |
| Acceso de la sesión local consultada | `roles/owner` en este proyecto; no usar ese rol como identidad de ejecución |
| APIs principales de despliegue | No se observaron `run.googleapis.com`, `sqladmin.googleapis.com`, `secretmanager.googleapis.com` ni `artifactregistry.googleapis.com` entre las habilitadas |

La identidad de Google consultada, el ID de facturación y las salidas privadas están en `.local/planning/gcp/verificacion_proyecto_20261015.json` del checkout principal, excluido, modo 0600. No se publican tokens, claves ni cuentas de EspaciGo en este documento.

El saldo **CLP 203.136,51** y los **38 días** son información proporcionada por el usuario, no una lectura independiente del saldo actual. La CLI confirmó moneda y vínculo, pero no Free Trial frente a cuenta de pago ni vencimiento. El navegador de esta sesión no tiene una consola Google autenticada visible. Antes del primer aprovisionamiento se debe registrar la comprobación en Facturación → descripción general: tipo de cuenta, crédito y fecha de caducidad. No pulsar Activar/Actualizar.

No se inventaría un despliegue existente: la inspección de APIs no constituye un inventario completo de todos los recursos del proyecto u organización. No tocar otros proyectos ni infraestructura anterior, incluido cualquier proyecto ajeno de Terraform.

## 3. Fuente local que debe conservarse

Release integrado: `2910491258007a103ed8d313c6202dec2ca4009d` (#323). Comprobación adicional de lectura de la base al preparar este plan:

- `espacigo_local` en `espacigo_pgdata`: 1.045 usuarios, 1.241 espacios, 1.228 reservas; V1–V61 aplicadas. PostgreSQL 18.6; base de aproximadamente 71 MB en esa observación, sin contar archivos externos.
- Dataset añadido: 1.000 cuentas, 1.200 publicaciones y 1.200 tratos; 960 publicaciones públicas, 120 ocultas y 120 borradores. Tratos: 600 cerrados, 240 pendientes de pago, 120 cancelados con devolución, 120 rechazados y 120 con reclamo abierto.
- Las 28 reservas originales quedaron conservadas. La cuenta maestra y su ancla de bootstrap se trasladan con su identidad; **no crear otro administrador global ni ejecutar bootstrap local sobre Cloud SQL**.
- PostgreSQL local tiene `btree_gist`, `unaccent`, PostGIS 3.6.4 y extensiones adicionales instaladas por la imagen (`fuzzystrmatch`, `postgis_tiger_geocoder`, `postgis_topology`, además de `plpgsql`). Distinguir lo usado por la aplicación de lo instalado incidentalmente; no borrar extensiones ni cambiar el origen para facilitar el traslado.
- Hora lógica final de la generación de tratos: `2027-04-07T04:00:00Z`. Son fechas simuladas; conservarlas y rotularlas. No retrasar/adelantar datos financieros por SQL ni poner ese reloj como hora global del servicio público.
- El dump **no contiene** todos los PNG, evidencia de operaciones, archivos privados externos ni claves de descifrado. Mantener mapas de UUID, manifiestos, hashes, versiones y material de recuperación privado.

El origen no se elimina, se reinicia ni se sustituye durante el traslado. El checkout principal tiene cambios ajenos y sigue intacto; trabajar desde worktree limpio de `origin/main`.

## 4. Arquitectura propuesta para el entorno de ensayo

| Pieza | Propuesta inicial | Condición de aceptación |
| --- | --- | --- |
| Región | Proponer `southamerica-west1` (Santiago), según la dirección ES2 existente; confirmar disponibilidad de servicios/edición y costo antes de crear | Misma región para API, SQL y buckets cuando sea compatible; no sustituir por otra región silenciosamente |
| Entrada pública | Angular SSR en Cloud Run, URL HTTPS gestionada inicialmente | Portada, catálogo, filtros y ficha pública funcionan sin login; conservar prerender y SEO público |
| Entrada privada de ensayo | Segunda entrada Cloud Run con la misma imagen Angular y acceso Google/IAP para propietario/evaluadores | La clave compartida de pruebas no permite a cualquier visitante usar las 10 cuentas administrativas ni la cuenta maestra |
| Gateway | Proxy de servidor con allowlist pública estricta; entrada privada proxy autenticado | La entrada pública solo reenvía operaciones públicas permitidas. No abrir `/auth`, reservas, administración o evidencia mediante bypass de rutas, codificación o cabeceras |
| API Go | Cloud Run invocable por identidades de servicio de las entradas, sin acceso anónimo directo | Autorización Go conserva Bearer y permisos. Autenticación servicio-a-servicio separada del Bearer de EspaciGo; no enviar credenciales de servicio al navegador |
| Base | Cloud SQL PostgreSQL 18, edición Enterprise elegida explícitamente, inicialmente zonal para ensayo | Validar extensiones, privilegios y restauración en el destino; no aceptar por mera coincidencia de versión mayor |
| Conexión | Red propia y conectividad privada; preferir conector Go de Cloud SQL con ADC y Direct VPC egress | TLS y credenciales correctas, pool acotado por máximos de instancias. Evaluar proxy TCP sidecar solo como alternativa explícita; la configuración actual no soporta socket Unix sin adaptación |
| Archivos | Cloud Storage privado con adaptador Go de objetos y precondiciones de generación | Sin pérdida tras reiniciar/escalar; hash, lectura autorizada, borrado recuperable y carreras probados. No declarar equivalente POSIX un montaje FUSE |
| Secretos | Secret Manager, identidades de servicio distintas para runtime, migrador y despliegue | Acceso mínimo; sin archivos JSON de service account, secretos en imagen o frontend. Claves de PDF existentes conservadas para leer datos trasladados |
| Trabajos de fondo | Primera opción: API con CPU asignada fuera de peticiones y mínimo 1 instancia; máximos/pool explícitos | Ensayar terminación, reinicio, leases y duplicados. Alternativa Jobs/Scheduler exige desacoplar y probar workers; no elegir escala a cero sin resolverlos |
| Migración | Tarea única independiente del arranque web | Migrador/restore separados del usuario runtime; reintento seguro y exclusión mutua |
| IaC/CI | Terraform en este repositorio, state remoto exclusivo; Artifact Registry; federación GitHub→GCP | Recursos identificados por proyecto exacto, plan revisable, imágenes por digest; no credenciales GCP permanentes en GitHub |
| Correo | Sumidero de ensayo restringido, nunca correo de prueba a destinatarios reales | Recuperación y verificación accesibles solo a evaluadores; no publicar Mailpit en Internet sin protección |

### Frontera pública y contraseña de ensayo

El usuario mantiene una contraseña común para las cuentas de prueba. El valor seguirá solo en la política privada local y, si se necesita en automatización cloud, en Secret Manager con acceso limitado; nunca en Git, datos enviados a otra IA, logs o capturas.

Publicar el catálogo no requiere publicar los endpoints de acceso a esas cuentas. Se propone separar entrada pública de entrada privada de ensayo, protegida adicionalmente con identidad Google/IAP. El enlace “Ingresar/postular” conserva espacio/retorno permitido y conduce al acceso privado. Esto no declara abierto el registro para usuarios reales. Una futura apertura general de cuentas requiere otra política de credenciales y alcance de producto.

La organización puede restringir miembros externos o acceso público. GCP-00 debe revisar políticas efectivas y configuración OAuth/IAP; no prometer que una cuenta Gmail externa queda autorizada solo por ser Owner. Si la política impide la topología, documentar la alternativa y sus costos antes de cambiar permisos.

## 5. Cambios de aplicación necesarios antes de desplegar

1. **Persistencia de archivos:** implementar adaptador Cloud Storage para los puertos existentes, manteniendo IDs/metadatos DB y la autorización pública/privada de Go. Incluir registro de privacidad/replay y sus garantías de escritura, no solo fotos de catálogo. Probar archivo ausente, timeout, escritura confirmada sin respuesta, limpieza y lectura desde dos instancias.
2. **Imagen Go:** la imagen `scratch` actual copia binario, migraciones y OpenAPI, pero no una colección de CA raíz. Agregar confianza TLS apropiada para APIs Google/Cloud SQL y verificar las conexiones HTTPS. Se conserva usuario sin privilegios y tzdata embebida.
3. **Configuración Cloud SQL:** adaptar `loadConfig`/conexión pgx a conector o alternativa seleccionada. Hoy exige contraseña por archivo y compone host/puerto TCP. Secret Manager puede montar archivos, pero eso no implementa el conector ni IAM de Cloud SQL por sí solo.
4. **Workers:** mantener expiración y conciliación normales del entorno cloud; no habilitar el flag que desactiva barridos globales para esconder tareas pendientes. El modo especial de importación fue un proceso separado de carga, no el perfil normal de la aplicación.
5. **Angular/gateway:** configurar puertos, origen público y base API real; si se usa `/api/v1` relativo, adaptar y probar sus validadores/configuración SSR. No trasladar `127.0.0.1` al navegador cloud. Reenviar Bearer con no-store y sin logs de credenciales; preservar rutas, guards y Go como autoridad.
6. **Perfil cloud de ensayo:** mantener proveedores fake explícitos sin introducir bypass de autorización, exponer fixtures administrativos o convertirlos en proveedores reales. Reloj normal real; no subir archivos de control local ni activar reloj manual en la API pública. Cualquier nueva simulación temporal se ejecuta en una API de ensayo aislada y con la protección de datos previos ya probada.
7. **Migraciones/roles:** Cloud SQL no ofrece un superusuario PostgreSQL real. Probar V60 y los grants/revocaciones/bootstrap sin elevar runtime. V1–V61 son inmutables; si hace falta cambio del modelo, solo migración nueva, con base vacía y upgrade.

## 6. Secuencia de cortes y evidencia

Cada corte usa una rama temporal y un PR hacia `main`; datos/contrato → Go → Angular → evidencia cuando atraviese capas. Desarrollo publica con `HernanMEC`; revisión/fusión con `HernanEspinozaDev`. Aviso al chat de revisión antes del final, con URL, HEAD, pruebas y límites; comprobar que el envío tuvo éxito.

| Corte | Trabajo | Entregable/gate para seguir |
| --- | --- | --- |
| GCP-00 | Este plan, identidad/proyecto/org/facturación, políticas, exposición pública, estimación y plan de apagado | Destino confirmado; tipo Free Trial/fecha/moneda comprobados antes de gastar; plan de recursos y autorización de ejecución. No crear recursos en este corte |
| GCP-01 | Portabilidad de configuración, TLS, puertos, Cloud Storage, conexión SQL, frontera pública/privada y modelo worker | Código y pruebas locales/contrato; distinguir emulación de integración GCP real. No Terraform apply |
| GCP-02 | IaC: bootstrap/state, APIs mínimas, IAM/federación, red, registry, secretos, buckets privados y SQL | Tras autorizar ejecución: revisar plan Terraform, edición/región y estimación; sin defaults Enterprise Plus/HA inesperados ni IAM público general |
| GCP-03 | Compatibilidad Cloud SQL y ensayo de restauración | Base vacía y restore de snapshot; V1–V61 exactas, PostGIS/índices/roles/grants/funciones/triggers, calendario/solapes y runtime sin superusuario; restauración verificable |
| GCP-04 | Backend y ejecución durable en destino | Readiness, migración como tarea, almacenamiento compartido, permisos, reinicio/leases, pool y límites; no cambiar origen local |
| GCP-05 | Angular SSR, gateway y HTTPS; catálogo público y entrada privada | Exploración anónima real; móvil/escritorio; publicación privada e imágenes 404; login diferido, retorno, 401/403 y administración protegida |
| GCP-06 | Copia final coherente de DB, archivos, claves y configuración | Backup nuevo; ventana de escrituras; conteos/UUID/hashes, 1.200 galerías y PDF/evidencia recuperables; sesiones/tokens de acción no trasladados como credenciales válidas |
| GCP-07 | Aceptación técnica, restauración, rollback, consumo y entrega | E2E en GCP, restablecimiento probado, enlaces y cuentas privadas entregados, costos observados y apagado documentado. Aceptación final humana; no producción real implícita |

No hay porcentaje ni fecha final inventada: la compatibilidad real de SQL, la frontera pública y el adaptador de archivos determinan el esfuerzo. Actualizar estado por evidencia en cada PR.

## 7. Método de traslado de datos y archivos

1. Fijar release/digest de origen y destino, esquema V61, inventario de todos los stores/claves y hashes. Conservar fuente JSONL y mapeos sin publicarlos. No regenerar el dataset ni cambiar sus UUID.
2. Ensayar PostgreSQL 18 destino con sus extensiones disponibles. Local PostGIS es **3.6.4**; la tabla oficial consultada indica **3.6.0 para Cloud SQL PostgreSQL 18**. Es un gate de compatibilidad, no autorización de downgrade en el origen. No copiar las tablas internas de extensiones o asumir igualdad de parche.
3. Preparar dump portable y restore controlado sin propietarios/ACL locales. Aprovisionar propietarios/grants explícitos; preservar funciones SECURITY DEFINER seguras y checksum de migraciones. Si hay que excluir objetos de extensión del TOC, generar manifiesto de selección revisado; no editar migraciones históricas para ocultar errores.
4. Exportar coherentemente DB y objetos durante una pausa de escrituras o snapshot coordinado. Transferir binarios a bucket privado con inventario ID→objeto/generación/tamaño/SHA. Trasladar claves de cifrado necesarias por canal de secretos; no cifrar con claves nuevas dejando PDFs antiguos ilegibles.
5. Las sesiones, tokens de verificación/recuperación y entregas de correo no se reutilizan como credenciales cloud. Definir y ejecutar su exclusión/invalidation en la copia destino, sin tocar el origen ni romper auditoría/retenciones. No enviar correos a personas reales desde el seed.
6. Verificar 1.045/1.241/1.228 y distribuciones por estado, categoría y unidad contra el snapshot de corte. Medir nuevamente si hubo cambios del usuario; no forzar esos números mediante DELETE. Conservar cuenta global única y ancla original sin ejecutar `activate-local` en GCP.
7. Comprobar las 960 portadas públicas y 240 publicaciones privadas, las 1.200 galerías por API autenticada, contratos/PDF, evidencia/garantías/comprobantes y aislamiento de terceros. Las 28 reservas anteriores conservan su snapshot.
8. Abrir el catálogo público solo tras comprobar rutas y acceso. El cambio de tráfico no sincroniza mágicamente dos bases: declarar cuál es la autoridad de la demo cloud y cómo se congeló/copió el origen. La base local permanece recuperable.

## 8. Costos, recuperación y salida

- Sin tope mensual solicitado. Preparar costo por 24 horas, 30 días y resto de prueba para SQL zonal, API con CPU continua/mínimo 1, entradas web, storage/versiones/backups, builds/registry, secretos, logging, red/IAP y posibles balanceadores. No reutilizar como tarifa actual el presupuesto histórico ES2 ni equiparar un seed de 1.000 usuarios con 1.000 concurrentes.
- Elegir explícitamente edición y tamaño SQL: para PostgreSQL 16+ la edición por defecto puede ser Enterprise Plus. Un prototipo económico no necesita HA o BigQuery/PubSub por defecto. No afirmar SLA, RPO/RTO o capacidad sin medir.
- El usuario no eligió un presupuesto numérico: no inventar uno ni crear una automatización de facturación. Las alertas de costo pueden proponerse como información; las alertas habituales no son una garantía de corte del gasto.
- Mientras Free Trial no esté comprobado, no prometer parada automática ni ausencia de cargos. No actualizar la cuenta. Al agotarse/caducar una prueba no actualizada pueden detenerse y finalmente eliminarse recursos/datos: mantener copia recuperable fuera del entorno de prueba y ensayar restore antes de depender de él.
- Configurar backups/PITR y protección de borrado según la instancia elegida. Guardar artefactos/digests y procedimiento de rollback. Rollback de aplicación y restore de base son operaciones diferentes; no hacer rollback SQL destructivo para revertir una imagen.
- Preparar inventario exacto de recursos `espacigo-testprod`, procedimiento de apagar aplicación y retirar recursos temporales. No `terraform destroy` global, borrar facturación/organización, claves del origen o volúmenes locales. La ejecución de un borrado de datos retenidos se revisa por separado.

## 9. Fuentes y decisiones originales

Diseño interno: `planning/referencias/ES2/03_06_infraestructura.md`, `planning/alcance_backend_local_y_transicion_gcp_20261007.md`, cierre local aceptado y `planning/dataset_loader_espacigo_1000.md`. La auditoría local del 2026-10-14 permanece como archivo ajeno en el checkout principal; no se añade a este corte. El estado actual V61/dataset sustituye sus conteos históricos para dimensionar esta migración, sin reescribir historia.

Documentación oficial consultada 2026-10-15:

- [Prueba gratuita y cuentas de pago](https://docs.cloud.google.com/free/docs/free-cloud-features).
- [Versiones PostgreSQL admitidas](https://docs.cloud.google.com/sql/docs/postgres/db-versions), [extensiones/PostGIS](https://docs.cloud.google.com/sql/docs/postgres/extensions) y [restricciones de usuarios](https://docs.cloud.google.com/sql/docs/postgres/users).
- [Exportación y restauración PostgreSQL](https://docs.cloud.google.com/sql/docs/postgres/import-export/import-export-dmp) y [elección de edición](https://docs.cloud.google.com/sql/docs/postgres/choose-edition).
- [Contrato de ejecución Cloud Run](https://docs.cloud.google.com/run/docs/container-contract) y [CPU/facturación](https://docs.cloud.google.com/run/docs/configuring/billing-settings).
- [IAP en Cloud Run](https://docs.cloud.google.com/run/docs/securing/identity-aware-proxy-cloud-run) y [Direct VPC egress](https://docs.cloud.google.com/run/docs/configuring/vpc-direct-vpc).
- [Cloud Storage FUSE](https://docs.cloud.google.com/storage/docs/cloud-storage-fuse/overview); un montaje no sustituye la validación de semántica del adaptador.
- [Precios Cloud Run](https://cloud.google.com/run/pricing), [Cloud SQL](https://cloud.google.com/sql/pricing?hl=en) y [presupuestos/alertas](https://docs.cloud.google.com/billing/docs/how-to/budgets).

**Siguiente acción autorizada en esta etapa:** revisar este plan y completar GCP-00. No comenzar infraestructura con costo ni migración cloud hasta recibir la instrucción de ejecución sobre un plan concreto y comprobar el estado de la prueba gratuita.
