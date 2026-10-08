# Prompt para ejecutar el cierre local de EspaciGo

Este texto es para entregarlo al agente en una ejecución posterior. La creación de este documento no inició implementación ni modificó GitHub.

---

Quiero completar todo el alcance funcional del Backend local de EspaciGo, M01–M11, con Base de Datos, APIs, pruebas y mock por módulo. Te autorizo a desarrollar dentro de ese alcance, siguiendo:

- `planning/plan_cierre_backend_local.md`;
- `planning/backlog_cierre_backend_local.md`;
- `planning/planificardesarrollo.md` para criterios de planificación;
- requisitos, contratos y decisiones ratificadas en `planning/` y `planning/referencias/`.

Esta orden autoriza implementación local posterior a la planificación. La restricción de no implementar del encargo de planificación se aplica a aquella ejecución documental, no convierte en documentales las tareas de desarrollo que autorizo aquí.

## Objetivo y límites

Termina el comportamiento de negocio local completo y comprobable; reutiliza lo aceptado y completa las brechas. No sigas añadiendo funciones de ensayo aisladas mientras existan criterios originales locales pendientes.

**No propongas ni ejecutes pruebas, despliegues o infraestructura GCP durante este trabajo.** Solo después de aceptar LOCAL-1 se evaluará esa fase mediante otra instrucción.

Usa datos sintéticos y fakes explícitos para pagos, devoluciones, verificación, firma y otros terceros. Completa los contratos, estados, persistencia, idempotencia y recuperación que sí pertenecen al Backend. No uses un fake para afirmar cumplimiento jurídico, identidad real, efectos financieros o integración con proveedor. No habilites documentos personales ni cuentas reales.

El frontend sigue siendo solo un mock HTML/CSS/TypeScript compilado, DOM y `fetch`, en su contenedor. Una validación final por módulo, sin diseño ni framework de frontend definitivo y sin acceso directo a DB. Respeta autenticación, autorización y controles vigentes del Backend.

## Primer trabajo

1. Inspecciona estado local, Issues/Project y PRs fusionados sin perder cambios existentes. GitHub es el registro operativo; no uses estados históricos de Hermes como hechos actuales.
2. Ejecuta LOCAL-PLAN-01: concilia todos los criterios RQF/RNF/CU/HU y las 43 entidades con código/evidencia; organiza los paquetes sobre Issues existentes o hijos locales cuando haya mezcla de alcance local/externo. No dupliques todo el backlog ni cierres padres incompletos.
3. Prepara en una sola tabla las decisiones de producto pendientes por grupo, con recomendación, fundamento e impacto. Consulta solo las realmente no ratificadas; bloquea únicamente sus partes dependientes. Las decisiones técnicas rutinarias dentro del alcance aprobado las puedes resolver y documentar.
4. Empieza la primera entrega de código por LOCAL-BOOK-01: consolida los criterios locales existentes de #73/#75/#77, comprueba directamente `23P01`, dos reservas adyacentes realmente persistidas y conflicto público/rollback para solapes. Registra primero la subentrega y dependencias locales; conserva las relaciones generales abiertas.
5. Sigue L1 → L2 → L3 → L4 → L5 conforme al grafo: identidad/privacidad/verificación; publicaciones/búsqueda/reservas generales; contratos/operación/comunicación; disputas/cierre simulado/administración; aceptación integral. Puede avanzar trabajo independiente con dependencias satisfechas.

## Desarrollo y publicación

- Usa **HernanMEC** para commits, pushes y PRs; verifica identidad/acceso sin imprimir tokens. **HernanEspinozaDev** queda para mi aprobación y merge.
- Conserva `espacigo_pgdata`, `.local/secrets` y datos de desarrollo. Migraciones nuevas incrementales; no alteres checksums de las aplicadas ni borres/recrees la DB de desarrollo.
- Usa PostgreSQL desechable para pruebas destructivas. Reutiliza scripts, imágenes y evidencias válidas; limpia solo recursos identificados de pruebas.
- Agrupa DB → Backend → API → pruebas → mock en entregas verticales revisables. Publica commits, push y un PR al finalizar cada entrega; si falta una comprobación, usa Draft con el pendiente concreto. No dejes todo el trabajo solo en archivos locales esperando aprobación.
- Evita PRs exclusivamente para sincronizar cada estado: actualiza matriz/guía en el PR de trabajo o siguiente entrega pertinente.
- Pruebas enfocadas mientras cambias lógica; regresión/integración necesaria al cerrar la entrega y gate completo al cierre final. No repitas suites por documentación. No postergues invariantes críticas de autorización, dinero, concurrencia o pérdida de datos.
- El gate debe fallar si omite integraciones obligatorias. Prueba contratos JSON/OpenAPI y recorridos de mock; no basta parsear YAML o consultar readiness.
- No apruebes ni fusiones tus PRs. Una revisión humana por entrega vertical. No marques Hecho antes de satisfacer criterios, revisión/merge y aceptación correspondiente. No avances dependencias de código aún no integradas; aprovecha trabajo independiente autorizado.
- No añadas hardening cloud ni nuevos requisitos productivos al prototipo. Conserva los controles necesarios de autenticación, permisos, privacidad e integridad.
- Mantén las obligaciones reales/sandbox y la limpieza remota #123 en seguimientos separados. No declares realizadas pruebas ni limpieza de otro host.

## Cierre

Informa avance por hitos aceptados, criterios restantes y decisiones concretas; no por cantidad de PRs ni porcentaje de tablas.

LOCAL-1 requiere los once módulos completos dentro del alcance local ratificado, mock por módulo, reconstrucción y actualización de DB, backup/restore aislado, recuperación/replay, recorridos integrales y pruebas críticas sin omisiones. Las brechas externas quedan trazadas, pero ninguna obligación local se excluye por conveniencia ni se declara resuelta sin evidencia.

Cuando se cumpla, publica la consolidación final para mi aceptación y **detente**. No inicies GCP, proveedores reales ni otra ampliación automáticamente.
