# Master Plan: Infraestructura, Base de Datos y Backend (Go)

Este documento define la arquitectura técnica, los componentes en Google Cloud Platform (GCP) y la hoja de ruta para construir el núcleo backend de un marketplace transaccional. El alcance de este documento cubre infraestructura, bases de datos (operativas y analíticas) y el desarrollo de la API, dejando el frontend principal para una etapa posterior.

---

## 1. Arquitectura de Infraestructura (Google Cloud Platform)

La infraestructura será provisionada como código (IaC) utilizando **Terraform**.

*   **Compute (Procesamiento):** 
    *   **Cloud Run (Backend):** Contenedor Docker único ejecutando la API en Go. Escalabilidad automática de 0 a N instancias.
*   **Almacenamiento y Bases de Datos:**
    *   **Cloud SQL (PostgreSQL):** Base de datos principal (OLTP). Fuente única de la verdad para transacciones, usuarios, publicaciones, pagos y contratos.
    *   **Cloud Storage:** Buckets privados y públicos para almacenamiento de activos multimedia (imágenes de publicaciones, PDFs de contratos).
*   **Analítica y Big Data (OLAP):**
    *   **BigQuery:** Data Warehouse central. 
*   **Mensajería y Telemetría:**
    *   **Cloud Logging:** Captura automática de la salida estándar (stdout) del contenedor de Go para auditoría.
    *   **Cloud Pub/Sub:** Bus de mensajería para ingerir eventos de alto volumen (vistas de catálogo, impresiones de anuncios) hacia BigQuery.
    *   **Datastream (CDC):** Replicará en tiempo real tablas clave (pagos, contratos) de Postgres hacia BigQuery para cruce analítico.

---

## 2. Diseño del Backend (Go Modular Monolith)

El backend se construirá como un **Monolito Modular** en un único contenedor Docker. Esto garantiza transacciones ACID seguras en memoria y elimina la latencia de red entre servicios.

### Estructura Interna del Código
La lógica se dividirá en paquetes (`packages`) independientes, permitiendo que si un módulo crece, pueda separarse a futuro:
*   `/internal/usuarios` (Autenticación, perfiles, CRM básico)
*   `/internal/catalogo` (Publicaciones, búsquedas, gestión de Cloud Storage)
*   `/internal/transacciones` (Pagos, generación de contratos)
*   `/internal/disputas` (Resolución de conflictos)
*   `/internal/telemetria` (Ingesta de eventos y logs estructurados)

### Interacción con la Base de Datos
Se compartirá un único pool de conexiones (`pgxpool`) en toda la aplicación:
1.  **Transaccional (`pgx` + `sqlc`):** Para operaciones críticas (pagos, contratos, usuarios). Garantiza consistencia, bloqueos de seguridad e inmutabilidad de tipos.
2.  **Dinámico (`pgx` puro / Builder ligero):** Exclusivo para el buscador del catálogo, permitiendo filtros variables (precio, categoría, etc.).

---

## 3. Estrategia de Registros y Analítica

### A. Auditoría y Seguridad (Logs)
*   El backend emitirá logs estructurados en formato JSON.
*   Cloud Logging lo captura nativamente y un "Log Sink" (Sumidero) lo envía a **BigQuery** para análisis forense, rastreo de cuentas vulneradas o auditorías transaccionales.

### B. Telemetría y Aceleradora Premium (Eventos)
*   El backend expondrá un endpoint (ej. `POST /api/v1/telemetry`) para recibir eventos masivos.
*   Go enviará estos eventos a **Pub/Sub**, el cual los insertará en **BigQuery**.
*   **Métricas Premium:** Go consultará BigQuery para devolver al CRM del vendedor estadísticas (alcance, vistas, CTR) únicamente si su suscripción en Postgres figura como activa.

---

## 4. Instrucciones para Agentes de Código (IA)

Para desarrollar este proyecto con asistentes como Cursor, Claude Code o Windsurf, se utilizarán archivos de reglas (ej. `.cursorrules` o `agent.md`) en la raíz del proyecto. Estos archivos dictarán a la IA:

*   **Regla de Arquitectura:** "No sugieras crear microservicios. Todo el código va en `/internal` dentro de un mismo proyecto Go."
*   **Regla de Base de Datos:** "Para transacciones financieras, usa siempre `sqlc` y el patrón de inyección del objeto `db.Queries` con `tx, _ := pool.Begin()`. Nunca uses ORMs como GORM."
*   **Regla de Respuestas HTTP:** "Usa un estándar JSON para todas las respuestas de la API, separando claramente `data` y `error`."
*   **Regla de Logs:** "Usa siempre `log/slog` (librería estándar de Go) para emitir logs en formato JSON en los controladores."

---

## 5. Hoja de Ruta de Desarrollo (Backend First)

### Fase 0: Entorno Local y Prototipado (ACTUAL)
*   Usar `docker-compose` para levantar PostgreSQL localmente.
*   Inicializar proyecto en Go y conectar a BD local.
*   Construir flujos básicos (CRUD) y servirlos temporalmente con `html/template` y **HTMX** desde el mismo Go para probar interacciones (botones, formularios) sin necesidad de un frontend separado.

### Fase 1: Cimientos del Backend (Go + sqlc)
*   Diseñar el diagrama Entidad-Relación base.
*   Configurar migraciones SQL y generación de código con `sqlc`.
*   Crear los archivos `.md` de reglas para la IA.
*   Implementar autenticación y perfiles de usuario.

### Fase 2: Núcleo Transaccional (ACID)
*   Diseñar esquema SQL para Pagos, Contratos y Disputas.
*   Implementar la lógica combinada en Go: Procesar un pago y crear un contrato dentro de la misma transacción (Rollback automático).
*   Probar exhaustivamente en local.

### Fase 3: Infraestructura como Código (Terraform)
*   Ahora que el código local funciona, escribir módulos de Terraform para levantar la VPC, Cloud SQL (PostgreSQL), Storage y Cloud Run en GCP.
*   Empaquetar la app Go en un Dockerfile de producción y desplegar.

### Fase 4: Integración de Archivos y Catálogo
*   Integrar SDK de Google Cloud para subir imágenes de publicaciones a Storage de forma segura.
*   Crear la API del buscador dinámico del catálogo usando `pgx` nativo.

### Fase 5: Trazabilidad y Analítica
*   Actualizar Terraform: Desplegar Pub/Sub, dataset de BigQuery y Datastream (CDC).
*   Implementar `slog` en todo el backend para logs estructurados.
*   Crear la API de telemetría y las consultas en Go hacia BigQuery para exponer métricas premium.