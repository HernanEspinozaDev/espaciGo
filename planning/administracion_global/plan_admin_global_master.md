# Plan 1 — Administración global y cuenta maestra de EspaciGo

Fecha: 2026-10-14, America/Santiago. **Autorizado para desarrollar por cortes y PR.** Es trabajo nuevo posterior al cierre local sintético aceptado; no reabre aquel cierre ni autoriza GCP. La cuenta definitiva del propietario no se activa hasta identificar su cuenta de aplicación.

## 1. Diagnóstico verificado

Base integrada para GADM-02: `6dffcc92ae528d8a790956a8dd73960df1501802`, merge de PR #313 (GADM-01). HEAD revisado del PR: `0dcaa68e78d377d7ad5d0831366a73e5d72c3e3f`. GADM-02 está en revisión técnica en PR #314, rama `codex/marketplace/feature/gadm-02-account-publication-management`; ver evidencia reproducible en `matriz_capacidades_pruebas_cortes.md`. La aceptación de producto sigue pendiente.

- Existe `identity.RoleAdministrator = administrador`, además de arrendatario/arrendador; el registro solo concede arrendatario.
- Hay una cuenta administrativa de ensayo en la demo LOCAL-CLOSE-02; no se exportan sus credenciales ni se la convierte automáticamente en cuenta del propietario.
- El panel Angular tiene colas KYC/privacidad/moderación, avisos, cuentas por UUID, publicaciones, reservas, auditoría y reportes de ensayo.
- `GET /admin/local/accounts/{accountID}` devuelve cuenta minimizada por ID y registra acceso. No existe directorio global paginado de todas las cuentas.
- El listado administrativo actual de publicaciones solo consulta estado `activa`; no ofrece inventario completo con borradores/ocultas ni edición global.
- La UI declara explícitamente que puede bloquear/desbloquear y consultar roles, no asignarlos.
- Existen reportes de reservas, finanzas y disputas con intervalos y límites. No equivalen a un tablero global ni a métricas de visitantes/usuarios activos diarios.

Fuentes de código: `internal/identity/domain.go`, `internal/adminlocal/{service,queries}.go`, `internal/adminlocal/transport/http/handler.go`, `frontend/src/app/features/admin/`, OpenAPI. Son puntos de partida que se reutilizan; no reconstruir todo como otro sistema desconectado.

## 2. Resultado esperado

Una cuenta personal del propietario con rol de aplicación **`administrador_global`**, accesible desde el mismo login y un área privada de administración. Debe poder localizar y administrar cuentas y sus recursos, supervisar inventario/operación/finanzas de ensayo, revisar auditoría y exportar informes.

“Global” significa permisos administrativos explícitos sobre toda la aplicación; no superusuario de PostgreSQL, credenciales GCP, sesión de otro cliente ni capacidad de saltarse las reglas de negocio. Contraseñas originales, tokens, claves y hashes no se muestran. Las operaciones financieras e históricos se consultan/gestionan por las funciones del dominio, no por un editor libre de tablas.

## 3. Modelo de permisos y cuenta inicial

- Mantener `administrador` como rol existente de operación; no promocionar administradores actuales ni las diez cuentas administrativas del futuro dataset a global.
- Introducir `administrador_global` y un catálogo de capacidades Go: `accounts.list/read/update/block/roles`, `publications.list/read/update/moderate/retire`, `operations.read/manage`, `finance.read/operate`, `audit.read/export`, `kpi.read`, `admin_access.manage`. Afinar nombres finales en ADR y contrato.
- Denegar por defecto y validar capacidad, cuenta activa, bloqueo, modo restringido y recurso en **cada** petición y transacción relevante. La UI no es la autorización. No añadir un `if master => permitir cualquier método/ruta`.
- Integrar el rol nuevo con los controles Go/SQL existentes de administrador de manera explícita y comprobada; no basta actualizar un enum o el menú. Inventariar handlers, servicios, validadores, DTO de sesión, SDK y guards que hoy comparan literalmente `administrador`.
- Revocación de rol/bloqueo debe afectar el siguiente acceso aun con Bearer previo. Angular limpia datos privados, Blob URLs y operaciones pendientes al cambiar permisos/sesión; respuestas viejas no reintroducen datos.
- Bootstrap local de un solo uso que recibe el UUID de una cuenta registrada/verificada del propietario; comprueba destino/entorno, registra concesión y evita ejecución anónima o un endpoint público de promoción. Recuperación por procedimiento local auditable, no contraseña universal.
- Para desarrollar: maestro de ensayo en DB aislada y credenciales aleatorias excluidas 0600. Para activar la cuenta definitiva: solicitar al propietario correo o UUID de su cuenta por canal local; no inferirlo de HernanMEC/HernanEspinozaDev ni de una cuenta demo. Esto no bloquea las herramientas, pruebas ni pantallas.
- Proteger al último administrador global: no permitir quitar su rol/bloquearlo/eliminarlo dejando el sistema sin administración. Probar carreras concurrentes. Recuperación excepcional debe verificar identidad y quedar auditada.
- Acciones sensibles (roles, credenciales, baja, datos de identidad) requieren autenticación reciente/confirmación explícita y motivo, con controles de servidor; no solo un modal. Especificar mecanismo y TTL. MFA para administración expuesta externamente se diseña antes de esa exposición; no presentarlo como ya implementado.

## 4. Pantallas y operaciones

### A. Inicio: visión global

Tarjetas con valores reales de DB, ámbito, definición y `generated_at`; filtros de período y zona IANA. Acceso a directorio, publicaciones, reservas, finanzas y auditoría. Estado vacío/error/reintento; no tarjetas que aparenten métricas inexistentes.

### B. Directorio de cuentas

- Listado paginado keyset, búsqueda por UUID exacto y correo/nombre normalizados con límites, filtros por rol/estado/bloqueo/fecha. Evitar devolver el universo en una respuesta.
- Ficha con identidad mínima necesaria, roles, estado, alta, indicadores de verificación y conteos asociados de publicaciones/reservas/incidencias. Datos sensibles adicionales solo con permiso específico y motivo auditado.
- Pestañas de recursos de esa cuenta y su cronología; diferenciar acciones **hechas por** la cuenta de acciones **sobre** ella. Navegación por enlaces internos estables.
- Editar campos de perfil autorizados mediante allowlist; cambio de correo sigue verificación de identidad y no deja el correo nuevo validado automáticamente. Restablecer acceso por flujo de recuperación, nunca ver la contraseña o asignar una conocida desde la ficha.
- Bloqueo/desbloqueo, revocación de sesiones y gestión de roles explícitos. No convertir “baja” en DELETE físico inmediato si hay reservas/contratos/retenciones: usar los flujos de privacidad ya existentes y mostrar bloqueos, pendiente, retirada y residual.

### C. Inventario de publicaciones

- Todas las categorías y estados, con propietario, búsqueda, filtros, unidad, fechas y visibilidad efectiva. Distinguir `activa` interna de realmente visible en el catálogo público por elegibilidad, bloqueo y fotos.
- Ficha global: datos públicos/privados permitidos, atributos versionados, tarifas, disponibilidad, galería/portada y vínculos a reservas.
- Edición administrativa por campos permitidos y rutas dedicadas, conservando propietario real y actor administrador. No enviar el token del dueño ni usar impersonación.
- Publicar/ocultar/retirar/moderar y corregir contenido/tarifa/horario mediante servicios de dominio y validación; conservar contratos, cotizaciones y snapshots previos. No cambiar ocupaciones confirmadas ni importes históricos al editar una publicación.
- Baja con confirmación, motivo y explicación de restricciones/retención. Imágenes solo mediante el soporte de archivos actual; esta tarea no crea fotos reales o storage GCP por inferencia.

### D. Reservas, contratos e incidencias

Listas globales por actor/recurso/estado/fecha y acceso a los paneles ya implementados; aprobar/moderar/resolver solo operaciones administrativas previstas y vigentes. Correcciones nuevas requieren contrato de dominio, no escritura libre de estado. Tratar reclamaciones, privacidad, KYC y evidencia como acciones auditadas con datos minimizados.

### E. Auditoría por cuenta y global

- Filtros por cuenta objetivo, actor administrativo, recurso, acción, resultado, motivo, intervalo y correlation/request ID. Incluir recursos propios vinculados cuando la relación sea legítima; no reconstruir identidades anonimizadas/retiradas.
- Registrar consulta de ficha/datos privados/exportación y todas las mutaciones; actor real, objetivo, timestamp, motivo y diferencias de campos minimizadas. No registrar passwords, Bearer, texto privado entero o payloads completos.
- Eventos de cambios son append-only para la UI y quedan atómicos con la mutación. No permitir al maestro borrar su rastro ni editar ledger/contratos; conservación/cleanup siguen las políticas existentes documentadas.
- Exportaciones JSON y CSV con mismos filtros e intervalo fijo, paginación/snapshot coherentes, límite de tamaño y auditoría de descarga. Neutralizar fórmulas CSV, controlar concurrente, permisos y Blob URLs. Los archivos privados no son URLs públicas.

## 5. KPI iniciales y definiciones

Separar métricas de **stock vigente** de métricas de **flujos en un período**. Ventana por defecto 30 días, `[from, until)` y zona IANA; reutilizar límite 31 días de reportes existentes hasta justificar otro contrato/ensayo. Nunca sumar distintas tarifas como si fueran la misma unidad.

| KPI | Definición / fuente |
| --- | --- |
| Cuentas totales y por estado/rol | Cuentas registradas vigentes según política documentada; roles pueden solaparse, no sumar facetas para inventar total. “Estado activo” no significa DAU. |
| Altas por día | Fecha de registro dentro del período. No representar visitantes como registros. |
| Publicaciones por categoría/estado | Conteos únicos; separa activas internas, públicamente visibles, borradores y ocultas. Reutilizar política de visibilidad del catálogo. |
| Publicaciones por propietario | Distribución 0/1/2–5/6+ y ficha por cuenta, sin N+1 ni incluir identificación en KPI agregados. |
| Reservas por estado | Cohorte creada en período y estado vigente al generar reporte; rotular así, no fingir el estado que tenían al cierre histórico del día. |
| Aprobación/rechazo del anfitrión | Reservas con decisión documentada; aprobadas/(aprobadas+rechazadas), con numerador/denominador y período de decisión definidos. Sin decisión no entra en el denominador. |
| Cancelación | Canceladas en la cohorte definida / reservas de esa misma cohorte; indicar definición y no mezclar cohortes. |
| Operaciones completadas/cerradas | Transiciones persistidas y distintas, sin contar reintentos. Separar finalizada operativa de cerrada financieramente. |
| Volumen y devoluciones de ensayo | Pagos confirmados únicos y devoluciones completadas, separados de pendientes. Devolver CLP exacto; no usar Number JS para int64. |
| Comisiones y liquidaciones | Hechos persistidos de comisión y transferencias confirmadas; las garantías/autorizaciones no son ingresos. Rotular dinero/ledger de ensayo. |
| Reclamos/KYC/privacidad | Pendientes por estado y antigüedad, resoluciones y colas; sin exponer textos/evidencias en agregados. |
| Avisos fallidos y conciliaciones pendientes | Outbox/operaciones durables existentes. No afirmar salud total del sitio con solo un readiness. |

Si el dato necesario no se conserva, mostrar “no disponible” y documentar la instrumentación faltante: visitantes, sesiones simultáneas, conversión por visita, tiempo real, DAU/MAU y NPS no se deducen de la tabla de usuarios. No inventar ingresos/porcentajes ni generar analítica de tráfico como parte silenciosa de este plan.

## 6. Datos, Go/OpenAPI y Angular

Orden por corte: DB/migraciones → Go/OpenAPI → Angular → recorrido real. V1–V57 se conservan byte a byte. GADM-01 agrega V58 y los cortes posteriores revisarán el historial antes de agregar desde V59.

Diseñar cambios mínimos para rol/constraints, concesiones/auditoría, consultas e índices. Capacidad autoritativa en Go; una tabla por permiso o un motor ABAC genérico no son requisito si un catálogo explícito satisface el alcance. Recursos auxiliares del futuro soporte no se implementan aquí.

OpenAPI declara las operaciones implementadas por cada corte y el SDK generado debe corresponder a ese contrato. GADM-01 incorpora `/admin/global/me`, `/admin/global/overview`, `/admin/global/accounts` y `/admin/global/accounts/{accountID}` para el directorio de solo lectura. `/publications` y auditoría por cuenta/global siguen siendo propuestas de cortes posteriores.

Angular: features/selective hexagonal, fachadas/puertos/adaptadores, Signals, TypeScript estricto, SCSS/tokens. CSR/noindex/no-store en área privada, menús por capacidades confirmadas por Go, navegación accesible móvil/escritorio; sin HTTP en presentación, transporte en aplicación ni rutas/JSON arbitrarios editables desde un formulario.

Todos los cambios usan control de concurrencia (revisión/updated_at o versión acordada), errores de estado y transacciones; acciones reintentables idempotentes según contrato. Integrar limpieza de estado privado/401/403 y revocación de roles durante solicitudes en vuelo.

## 7. Cortes autorizados, revisión entre ellos

| Corte | Entrega | Condición de terminado |
| --- | --- | --- |
| GADM-00 | Inventario de capacidades actuales, ADR, matriz permisos/operaciones y plan versionado | Fusionado en PR #312, merge `2589aa1139232f419cf1d1aebdd79a69c1f4007b`; sin concesiones de privilegios ni cuentas nuevas. |
| GADM-01 | Rol global/capacidades, V58, bootstrap/recuperación, sesión y directorio de cuentas con totales reales | Fusionado en PR #313, merge `6dffcc92ae528d8a790956a8dd73960df1501802`, HEAD revisado `0dcaa68e78d377d7ad5d0831366a73e5d72c3e3f`. La cuenta real y aceptación de producto siguen pendientes. |
| GADM-02 | Gestión de cuentas/publicaciones: edición allowlist, bloqueo/sesiones/roles bajo política, estados y recursos por propietario; moderación, publicación, ocultamiento y retiro según dominio | Acciones Angular→Go→DB; motivo y autenticación reciente verificados por servidor; auditoría atómica, control de versión/idempotencia, snapshots y ocupaciones conservados, privacidad/retención respetadas y carreras cubiertas. Correo verificado no editable desde ficha. Supresión exige transferencia previa del UUID bootstrap. V1–V58 inmutables; V59 solo si el modelo lo exige. |
| GADM-03 | Tablero KPI y auditoría por cuenta/global, exportaciones JSON/CSV | Definiciones/fuentes/intervalos exactos, cifras contrastadas con DB, sin doble conteo/N+1 ni exposición de secretos, paginación/export coherentes. |
| GADM-04 | Regresión y entrega de la cuenta maestra con guía/demo e inventario de capacidades | Pruebas de permisos y recorridos completos, dataset de ensayo recreable, evidencia por SHA, aceptación del propietario; ningún soporte o GCP activado. |

Ramas: `codex/marketplace/<feature|fix|chore>/gadm-00-descripcion` y sucesores, una rama temporal/PR por corte sobre main; no ramas por capa ni reutilizar ramas fusionadas. Si el hook requiere patrón, ampliarlo de forma acotada y probada sin deshabilitar identidades. Iniciar solo00; revisión envía el sucesor tras fusión, sin pedir otra autorización de la secuencia al usuario.

## 8. Pruebas y entrega

- Matriz positivo/negativo: global, administrador previo, anfitrión, arrendatario, anónimo, bloqueado y modo restringido. Los permisos globales no se conceden por enviar un campo role en registro/update ni por modificar Angular.
- Revocación con token previo, autoedición/último master, dos administradores concurrentes, cuenta objetivo distinta de actor, exportación/auditoría por propietario, filtros y CSV seguro. Evidencia privada no reaparece tras logout/cambio de permisos.
- Datos: base vacía/upgrade/migraciones históricas; fixtures recreables con usuarios de varios roles, publicaciones multicuenta/multicategoría y operaciones con distintas obligaciones. No usar los archivos que está generando la otra IA ni alterar el kit excluido sin otro encargo.
- Cuentas de ensayo propias y credenciales 0600 excluidas; no elevar a nadie en `espacigo_pgdata` ni usar privilegios de la demo existente para bootstrap de nuevos entornos.
- Pruebas Go/PostgreSQL y Angular, lint/arquitectura/api:check/build; E2E servido, capturas 390×844/1365×900; presupuestos SQL/EXPLAIN y tiempos de listados/KPI ante al menos 1.000 cuentas técnicas de ensayo, sin generar contenido del dataset del usuario. Describir qué se ejecutó realmente.
- PRs/commits/push/respuestas por HernanMEC; revisión/fusión por HernanEspinozaDev; role script antes de commits/publicación y verificación en GitHub. Aviso a este chat al publicar/corregir/terminar/bloquear.
- Preservar checkout habitual con cambios ajenos, auditoría, `.local/datasets/espacigo-1000/`, demos retenidas y `espacigo_pgdata`; worktree limpio desde main.

## 9. Relación con soporte y futuro destino

Soporte es un plan separado **no autorizado a ejecutar todavía**. Este plan global solo deja una frontera de capacidades reutilizable; no crear cuentas de soporte, endpoints, tickets ni permisos de soporte. GCP sigue sin encargo: no crear proyectos, recursos, proveedores o despliegues.

Referencia técnica consultada 2026-10-14: OWASP Authorization Cheat Sheet, permisos mínimos, denegación por defecto y controles en cada petición: https://cheatsheetseries.owasp.org/cheatsheets/Authorization_Cheat_Sheet.html . Las decisiones de producto de este documento son propuestas adaptadas a EspaciGo, no capacidades implementadas.
