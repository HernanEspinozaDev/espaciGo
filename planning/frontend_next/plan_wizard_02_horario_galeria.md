# Wizard-02 — zona horaria, navegación, galería y horario

- Fecha: 2026-11-07.
- Base del worktree: `8f0e06e2d9e5f16b429c69f190aa9dcd44148fc6`.
- Rama: `codex/frontend/feature/wizard-02-horario-galeria`.
- Estado: la rama de tarea continúa publicada; integración local normal preparada en el candidato `main` `b89fe0f196eb5ff2c3b155bafd4b60e3ea074618`, sobre `origin/main` `274f37e483a078890e92382a159fa49fd23c2fe8`. Aún no se publicó al ref remoto `main` ni se desplegó. El E2E combinado conserva dos fallas reproducibles también en la base main; IMG-01E y validación online siguen fuera de evidencia.
- Evidencia: [`evidence/wizard-02-local-gates.md`](evidence/wizard-02-local-gates.md).

## Solicitud

Corregir el control de zona horaria del paso 4 para que ofrezca opciones legibles; hacer que el logo de EspaciGo en el espacio privado dirija al inicio público; mejorar la entrada visible del flujo de fotografías; aclarar/permitir el control del horario semanal sin presentarlo como disponibilidad por fecha.

## Alcance implementado

1. Selector de cuatro zonas horarias territoriales de Chile, con nombres legibles: Chile continental, Aysén, Magallanes y Rapa Nui/Isla de Pascua. Los valores de persistencia continúan siendo identificadores IANA; no se codifican offsets UTC. La opción predeterminada para nuevos espacios sigue siendo Chile continental. Al editar, se conserva una zona previamente guardada que no esté en las cuatro opciones.
2. El logo del header privado apunta a `/`, según la instrucción expresa del propietario.
3. Se reemplaza el aviso fijo de «entorno de ensayo» en el panel de galería por instrucciones de formatos y validación. Se conserva el flujo de carga existente y sus bloqueos de seguridad: requiere estado autoritativo de galería y no permite reenvíos automáticos de una operación incierta.
4. El control se rotula «Habilitar horario semanal» y explica que son tramos habituales, no confirmación de disponibilidad por fechas. Para tarifas por día/mes se conserva únicamente la zona horaria; no se emite horario semanal.
5. Las pruebas de UI cubren las cuatro opciones, la navegación del wizard, el envío de `America/Coyhaique` al guardar y su rehidratación tras recargar mediante `GET /weekly-hours` controlado (el mock de detalle permanece en `America/Santiago`), clic y navegación del logo a `/`, el control de subida visible/habilitado en estado confirmado, y que tarifas por día/mes no lean ni guarden horario semanal.

## Fuentes de zona horaria

Se contrastó la tabla oficial IANA tzdb 2026f (`zone1970.tab`, actualizada el 2026-10-28). Chile aparece con cuatro zonas: `America/Santiago`, `America/Coyhaique`, `America/Punta_Arenas` y `Pacific/Easter`. Los identificadores se usan como valores técnicos; las etiquetas visibles son nombres territoriales.

## Límites y criterios pendientes

- El control semanal no es un calendario de fechas ni determina ocupación, bloqueos o posibilidad de reservar. No se añade un calendario de reservas ni se relajan sus gates.
- La galería tiene un flujo de carga en frontend, pero el E2E local usa rutas controladas/mocks. Esta entrega no demuestra upload real a Go/GCS ni persistencia en la cuenta del propietario. IMG-01E sigue incompleto según `planning/imagenes/matriz_capacidades_pruebas_cortes.md`; no declarar fotos reales plenamente aceptadas hasta el recorrido requerido allí.
- No se consultó ni modificó el espacio informado por el usuario; no se crearon espacios, no se alteraron datos, no se ejecutaron migraciones y no se accedió a la aplicación pública/privada online.
- No se modificó backend, OpenAPI, SQL, IAM, Terraform, GCP, el volumen persistente ni la política de acceso de la galería.
- El usuario conserva la prueba funcional online. La integración está solo en worktree local; el push a `main` activa el rollout web automático y espera autorización explícita. No atribuir aceptación de producto a gates locales.

## Calendario manual por fecha — ampliación local del flujo

- El editor incorpora consulta por rango y alta/baja de bloqueos manuales mediante el contrato existente; la zona IANA del espacio define la conversión de fecha/hora local a UTC.
- El E2E controlado verifica conflicto de ocupación `409`, creación, lectura tras recarga, eliminación, fecha «Hasta» vacía sin bloqueo de la UI, zona horaria no configurada y navegación a su configuración. Axe y ausencia de overflow pasan en 1280×800 y 390×844.
- Esto no representa reservas, ocupación confirmada ni disponibilidad final, y solo se probó con BFF mockeado. No se ejecutó una integración API/DB real.

## Salida

Rama de tarea verificada en `origin/codex/frontend/feature/wizard-02-horario-galeria` (`1c49c042814695f5257feb0262ebac9291622b5f`). Candidato local integrado en `main`: merge `141d348aa6e4bc7002020741830da899715c277f`, seguido de sincronización generada OpenAPI `b89fe0f196eb5ff2c3b155bafd4b60e3ea074618`. En el candidato: `api:check` (200 operaciones), typecheck, 480 unitarias aprobadas/1 omitida, lint (0 errores/4 advertencias), Prettier, build y diff check pasaron. Playwright: 144 aprobadas, 63 omitidas, 2 fallidas; ambas fallas también ocurren sobre `origin/main` sin esta rama. Los tests afectados son KYC y jerarquía de encabezados en Mi Cuenta; no se alteraron aquí. La carga real de fotos permanece bloqueada. `origin/main` sigue en `274f37e483a078890e92382a159fa49fd23c2fe8`; sin push, despliegue ni aceptación online.
