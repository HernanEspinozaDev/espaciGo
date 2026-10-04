# Entorno local aislado: database, backend y mock

## Propósito y límites

Contrato de planificación para que una implementación posterior pueda ofrecer `docker compose up` desde un checkout limpio y validar RNF-035 junto con el aislamiento del frontend mock. No crea Dockerfiles, compose, código ni contenedores. El backend sigue siendo un monolito modular Go; PostgreSQL es la fuente de verdad. El mock es temporal, consume solo la API HTTP pública y no es un cliente privilegiado.

## Topología y límites de red

```text
                         red edge (solo HTTP)
 Navegador ────────────────┬───────────────────
                           │                   │
                    mock-frontend          backend
                    (estático)                 │
                           │                   │
                           └── API HTTP ───────┘
                                       │
                                  red data
                                       │
                                  database

Regla de flujo: mock-frontend → backend → database
Prohibido: mock-frontend → database (sin ruta, DNS ni credencial)
```

Servicios Compose previstos: `database` (PostgreSQL), `backend` (API Go) y `mock-frontend` (servidor estático mínimo). `database` se conecta únicamente a `data`; `backend` a `edge` y `data`; `mock-frontend` solo a `edge`. La red `edge` no debe conectar directamente a PostgreSQL; no publicar puertos de DB al host por defecto. El backend es el único dueño del acceso a DB. Compose no sustituye el control de acceso de aplicación.

## Puertos y acceso local

Contrato inicial, sujeto a los puertos ya elegidos por la implementación: publicar al loopback del host la API en `127.0.0.1:8080` y el mock en `127.0.0.1:8081`; PostgreSQL no se publica. Dentro de Compose, API y DB se comunican por DNS de servicio y puerto interno, no por `localhost`. No usar `network_mode: host`, `links` ni puertos DB expuestos a interfaces externas. Si se necesita inspeccionar PostgreSQL, hacerlo mediante un perfil/habilitación local explícita, bind a loopback y sin ampliar la red del mock.

## Variables y secretos

Solo configuración no secreta en el entorno Compose: `APP_ENV=local`, `HTTP_ADDR=:8080`, `DATABASE_HOST=database`, `DATABASE_PORT=5432`, `DATABASE_NAME=espacigo_test`, `DATABASE_USER` de mínimo privilegio, `CORS_ALLOWED_ORIGINS=http://localhost:8081`, `API_BASE_URL=http://localhost:8080/api/v1` y `LOG_LEVEL=debug` (nombres propuestos; alinear con el bootstrap). Las variables del mock no contienen DSN, usuario/clave de DB ni credenciales de proveedores. El servidor estático recibe la URL pública del API, no una dirección interna de DB.

No incluir secretos reales en compose versionado, imágenes, logs, argumentos, documentación ni valores por defecto compartidos. Credenciales locales de DB de test: sintéticas, exclusivas del entorno y suministradas por mecanismo local ignorado por Git o secret store; nunca reutilizar producción/desarrollo compartido. El backend recibe sus credenciales solo en runtime. No registrar DSN ni volcados de entorno. No añadir `DATABASE_URL` al contenedor del mock.

## Readiness, salud y apagado

- `database`: healthcheck con `pg_isready` contra la DB de test y el usuario configurado; estado healthy solo cuando acepta conexiones.
- `backend`: endpoint de liveness independiente de DB y readiness que falla mientras DB no esté disponible o migraciones requeridas no hayan terminado. El proceso debe reintentar conexión con límite/backoff o salir con error claro; nunca anunciar ready antes de poder atender.
- `mock-frontend`: healthcheck HTTP del recurso estático servido.
- Dependencias de arranque ordenan inicialización (backend espera DB healthy; mock puede esperar backend ready), pero healthchecks también deben reflejar fallos posteriores. El mock no debe contener lógica de retry que oculte indisponibilidad de API.
- Cierre ordenado: backend deja de aceptar tráfico, drena solicitudes y cierra pool; DB persiste su volumen local. Reinicio del stack no debe requerir datos de producción ni pasos manuales no documentados.

Migraciones: usar el migrador versionado del proyecto al iniciar backend o como paso one-shot claramente dependiente de DB healthy, nunca desde el mock. Respetar su contrato: historial/checksum inmutable, lock y aborto claro ante deriva. En entorno de test, la inicialización debe ser reproducible desde DB vacía.

## Base de datos de test

Nombre aislado `espacigo_test` (propuesto); volumen dedicado y distinto de cualquier entorno real. Solo datos sintéticos, seeds deterministas y limpieza/recreación documentada. El harness de pruebas debe validar nombre seguro y rechazar URLs/hosts no autorizados antes de borrar o migrar datos; PostgreSQL real, no SQLite, para semántica de integración. Pruebas paralelas usan esquema/DB aislada por worker o ámbito equivalente, sin compartir estado mutable. Compose local no debe conectar ni importar datos reales.

## CORS y origen API

El navegador carga el mock desde `http://localhost:8081` y llama a la API pública `http://localhost:8080`; el backend permite explícitamente ese origen local. No usar wildcard junto con credenciales. Responder preflight para métodos/headers realmente soportados, incluyendo Authorization y headers de contrato (p. ej. request/correlation e idempotency cuando corresponda). Exponer solo headers necesarios al cliente. En otros ambientes, lista de orígenes explícita por ambiente; no confiar en CORS como autorización. El API valida autenticación y permisos por recurso como define el contrato HTTP/OpenAPI.

## Contrato mock → API

El mock usa `fetch` a `API_BASE_URL` y solo rutas HTTP/JSON públicas declaradas en OpenAPI. En navegador, `API_BASE_URL` debe ser una URL alcanzable desde el host del navegador (localhost); `backend` como hostname Compose solo es resoluble entre contenedores. Sin proxy que dé al mock acceso a redes internas, sin SQL, socket Docker, credenciales DB, SDK interno ni acceso directo a proveedores/storage. API caído debe aparecer como error visible; no responder con fixtures que aparenten éxito salvo fixtures explícitamente identificados para pruebas unitarias del mock.

## Checklist para ticket de implementación

- [ ] Crear servicios `database`, `backend`, `mock-frontend` y redes con mínimo privilegio según el diagrama; demostrar que mock no resuelve ni conecta a DB.
- [ ] Compose y configuración documentados permiten `docker compose up` desde cero y repetición tras `down` (aclarar cuándo se conserva o elimina volumen).
- [ ] DB no publicada al host por defecto; API/mock publicados solo en loopback con puertos documentados y sin colisiones inadvertidas.
- [ ] Variables de configuración coherentes con bootstrap; sin secretos en repo/imagen/logs y sin variables de DB en mock.
- [ ] Healthchecks reales y dependencias por condición healthy/ready; fallo de DB o API resulta visible y estado ready correcto.
- [ ] Migraciones aplican desde DB vacía con el migrador acordado; deriva/fallo bloquea readiness.
- [ ] DB de test PostgreSQL aislada, seeds sintéticos y guardas anti-destrucción de DB externa; no SQLite para integración.
- [ ] CORS acepta el origen local exacto, preflight necesario y no wildcard con credenciales; autorización sigue siendo del backend.
- [ ] Mock llama exclusivamente a API HTTP pública; documentar URL visible desde navegador y demostrar errores sin API.
- [ ] Verificar `docker compose config`, build y `docker compose up --build` en checkout limpio; comprobar liveness/readiness de servicios.
- [ ] Prueba negativa desde mock: sin ruta/DNS/credencial/conectividad a PostgreSQL; prueba positiva mock→API y API→DB.
- [ ] Ejecutar smoke de endpoints/contrato, arranque desde volumen vacío y reinicio; adjuntar comandos/resultados y documentar limpieza.
- [ ] Confirmar ausencia de puertos, secretos o volúmenes que expongan datos más allá del entorno local.

## Decisiones y pendientes de implementación

Los puertos/nombres de variables aquí son valores propuestos de desarrollo local, no contratos externos. Ajustarlos una sola vez en el ticket de bootstrap si la estructura real ya define convenciones. Este documento no afirma que exista compose ni que healthchecks/red hayan sido ejecutados; todas las verificaciones son aceptación futura.