# Entorno para ejecutar la planificación

## Alcance de este documento

El usuario compartió `arquitectura_y_roadmap_del_marketplace.md` para mostrar el entorno de desarrollo disponible donde ejecutará este backlog. **Ese archivo no es una especificación de EspaciGo, no es una fuente de requisitos y no fija la arquitectura del producto.** Se retiró del repositorio para que no induzca a los agentes a mezclar otro contexto con este proyecto.

El adjunto menciona Go, Docker Compose, PostgreSQL local, Terraform y servicios GCP como herramientas/fases del entorno. Se registran solo como contexto de trabajo comunicado por el usuario. La planificación no comprobó las versiones instaladas, credenciales, proyectos cloud, cuotas ni servicios provisionados; la primera tarea técnica debe verificarlos antes de depender de ellos.

El snapshot de la propuesta ES2 incluye una referencia bibliográfica a ese mismo archivo como “documento comparado”. Tras la aclaración del usuario, esa referencia no debe interpretarse como evidencia de que sus decisiones describan EspaciGo ni como validación cruzada de requisitos. El snapshot académico se conserva intacto; prevalecen el contenido propio de ES2 y las instrucciones actuales del usuario.

## Reglas al aplicar herramientas

- La prioridad de ejecución sigue siendo **Base de Datos → Backend → API → pruebas → frontend mock**.
- PostgreSQL local y Compose sirven como entorno de desarrollo; el mock tendrá un contenedor aislado, consumirá el API JSON y nunca se conectará a DB.
- El uso futuro de Terraform/GCP se planifica después de tener slices locales verificables y evidencia para configuración/costos.
- Que una herramienta aparezca en el documento de entorno no la convierte en una decisión del producto. Usar las decisiones consolidadas de ES2 y las instrucciones explícitas actuales.
- El frontend mock se rige por la instrucción del usuario: HTML, CSS, TypeScript compilado, DOM nativo y `fetch`, en su propio contenedor. No adoptar `html/template`/HTMX desde el backend.
- Antes de iniciar una tarjeta de setup, verificar herramientas con comandos/versiones y anotar qué existe; no exponer credenciales en logs, snapshots ni prompts de agente.

## Separación de contextos

| Contexto | Fuente | Uso |
| --- | --- | --- |
| Producto/requisitos EspaciGo | Snapshots en `referencias/ES1/` y `referencias/ES2/` | Autoridad de contenido según `contexto_para_agentes.md`. |
| Herramientas/entorno de desarrollo | Descripción que el usuario compartió, resumida aquí | Orientar verificación local; no decidir funcionalidades ni stack de producción por sí sola. |
| Restricciones vigentes | Solicitudes explícitas del usuario y tarjetas del backlog | Prevalecen si la descripción del entorno sugiere otro flujo (por ejemplo HTMX). |
