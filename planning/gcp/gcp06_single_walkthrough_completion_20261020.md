# GCP-06: cierre reducido para un recorrido de ensayo

**Estado:** propuesta de desarrollo y gates; no es autorización para operar Cloud.
**Base documental:** `origin/main` en `d8e57fa72a3b41947f9d8597a4d321383af91fdf`.
**Destino acotado:** candidata existente `espacigo-gcp06-20261020-gcp06a-pg` / `espacigo_gcp06_candidate_20261020-gcp06a`, proyecto `espacigo-testprod` y servicios actuales `espacigo-test-api`, `espacigo-test-public` y `espacigo-test-private`.

## Objetivo

Completar una sola validación del producto usando la candidata ya importada y una cohorte aproximada de diez usuarios/datos sintéticos. No repetir el dump, transferencia ni importación SQL ya reportados. La candidata se conserva aislada; la instancia histórica `espacigo-testprod-pg`, producción, personas reales y GCP-07 quedan fuera.

Este documento convierte el objetivo en una secuencia corta, con un entregable por gate. La importación reportada como `DONE` no acredita el ledger, conteos, extensiones, propietarios, ACL ni acceso runtime. Los PR #459, #460 y #461 están fusionados; #461 agregó inspección SQL local de solo lectura y preparación de wiring Terraform. El plan dirigido documentado por #461 no es aplicable: no hay una versión activa confirmada del secreto runtime ni verificación independiente del login/rol runtime y sus ACL. Cloud Run aún no usa la candidata.

## Secuencia propuesta

### 1. Verificación privada de la candidata

Usar únicamente una sesión/canal privado aprobado y la instancia/base fijadas arriba. El inspector de #461 puede resumir catálogos, extensiones, propietarios y privilegios efectivos; no lee datos de aplicación y por sí solo no verifica filas de `schema_migrations` ni conteos de tablas. Completar la evidencia con consultas fijas de solo lectura que informen:

- ledger: cantidad de versiones, mínimo, máximo, duplicados y continuidad esperada V1–V64;
- conteos agregados de tablas contra el manifiesto sanitizado existente, sin filas, correos, UUID ni valores de negocio;
- extensiones/versiones, propietarios, ACL de base/esquema/relaciones/secuencias/rutinas y privilegios efectivos del login/rol runtime;
- identidad de sesión, destino, hora UTC y resultado por comprobación.

El resultado será un informe agregado, con hashes de consulta/manifiesto y `PASS`/`STOP` por comprobación. No inferir éxito desde metadatos de Cloud, estado `DONE` de la importación ni un plan Terraform. Si no existe autenticación privada válida, o el rol disponible no puede ejecutar una consulta sin elevarse, detenerse y preparar un plan exacto revisable para habilitar ese canal. No cambiar `postgres`, crear o rotar credenciales, crear grants/roles, habilitar IAM DB auth ni aplicar Terraform bajo este PR.

**Gate:** no avanzar a runtime mientras ledger, conteos, extensiones, ownership, ACL y acceso runtime no tengan evidencia SQL independiente y satisfactoria.

### 2. Preparar una cohorte recreable de aproximadamente diez usuarios

Después de pasar el gate 1, seleccionar solo datos sintéticos y recreables para el recorrido: aproximadamente diez cuentas y los registros mínimos que necesite el flujo elegido. Usar los flujos de aplicación existentes para registro/verificación y el mecanismo de correo de ensayo; no importar cuentas humanas, credenciales del entorno local, sesiones, tokens de recuperación ni contenido privado. No generar correos reales ni dejar credenciales en Git, evidencia o logs. La cuenta de administración global, si hace falta para el recorrido, se trata como bootstrap separado, auditado y aprobado; no se crea implícitamente como parte del seed.

El manifiesto del seed registra tipos/cantidades y un identificador de corrida, nunca secretos ni datos personales. La carga se realiza solo en la candidata luego de revisión y autorización del plan exacto. No reescribir migraciones ni ejecutar una migración completa como sustituto de este seed.

**Gate:** el seed debe poder repetirse o retirarse sin tocar datos históricos ni otras instancias; verificar el resultado con conteos agregados.

### 3. Conectar temporalmente el API de ensayo

Solo después de verificar el catálogo y preparar credenciales runtime válidas, regenerar un plan Terraform remoto con estado actual, recurso/API exactos, versiones numéricas reales de secretos, imagen/digest y costo estimado. Revisar que el diff cambie únicamente el destino SQL y el secreto runtime que necesita `espacigo-test-api`; conservar imagen del API, servicios web, URLs, ingress, IAM no relacionado y tráfico fuera del cambio. Registrar el SHA del plan y la revisión estable previa como reversa.

El plan completo Terraform no es aceptable si incluye la destrucción del recurso histórico del inspector. Usar un plan dirigido únicamente si su alcance y state quedan revisados independientemente; no aplicarlo por estar dirigido. Cambiar el destino, Secret Manager, IAM, tráfico Cloud Run o aplicar Terraform requiere autorización explícita posterior al plan exacto. No conectar el frontend directamente a SQL.

**Gate:** una respuesta de salud del API no basta; debe demostrar que la revisión estable usa la instancia/base candidatas y el rol runtime esperado antes del recorrido.

### 4. Ejecutar un solo recorrido y conservar reversa

Con el API verificado contra la candidata y los dos frontends actuales sin cambios, ejecutar una única sesión de ensayo con datos sintéticos: abrir el flujo público elegido, autenticar el usuario de ensayo por el flujo normal en la superficie privada y verificar que la lectura/acción prevista llega al API conectado a la candidata. Definir antes de la ventana un flujo de producto existente y sus comprobaciones; no afirmar pagos, correos reales ni otras capacidades que no formen parte del flujo. Guardar URL/revisiones/digests y resultados saneados, sin capturas con secretos o identidades reales.

La reversa de aplicación consiste en restaurar la revisión estable anterior del API y verificar salud; no restaura ni sincroniza escrituras de la candidata. La candidata permanece retenida para diagnóstico y no se destruye en este corte. Un rollback de datos requeriría un plan separado; no usar `terraform destroy` global ni restaurar sobre la instancia histórica.

## Límites, evidencia y siguiente acción

Este PR solo deja el plan y la matriz alineados con el objetivo reducido. No crea usuarios/datos, no abre una sesión SQL, no consulta ni altera Cloud, no crea o rota credenciales, no cambia IAM/Secret Manager/Cloud Run, no aplica Terraform, no transfiere datos y no incurre gasto Cloud. La propuesta de canal privado todavía debe cotejarse con la identidad disponible y el inventario de permisos; no hay canal ni credencial SQL confirmados como utilizables.

El paquete local de verificación privada y SQL de agregados está en [GCP-06: paquete de verificación SQL privada](gcp06_private_verification_package_20261020.md), con manifiesto/hash de consultas y límites de Fase A/Fase A2. Es preparación para revisión, no evidencia de lectura Cloud: el manifiesto saneado privado no estaba en el worktree y su hash sigue pendiente. Ninguna operación Cloud se inicia hasta que el usuario autorice por separado el plan exacto y exista un canal/actor permitido. GCP-06 sigue parcial y GCP-07/aceptación siguen pendientes.
