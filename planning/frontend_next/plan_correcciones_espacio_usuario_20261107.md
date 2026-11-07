# Plan de correcciones del espacio informado por el usuario

- Fecha de elaboración: 2026-11-07.
- Checkout de trabajo: `/home/nandev/Documentos/projects/espaciGo-wizard-improvements`.
- Rama: `codex/frontend/feature/wizard-02-horario-galeria`.
- HEAD base: `8f0e06e2d9e5f16b429c69f190aa9dcd44148fc6`.
- Espacio mencionado por el usuario: `df900c8a-5486-46ca-bf49-c677e6227a92`; no consultar ni modificar sin autorización y entorno expresos.
- Estado externo: sin commit, push, integración a `main` ni despliegue. Las comprobaciones de la aplicación online están reservadas al usuario.
- Evidencia relacionada: `plan_wizard_02_horario_galeria.md`, `evidence/wizard-02-local-gates.md`, `planning/imagenes/plan_img_01_carga_crud.md`, `planning/imagenes/matriz_capacidades_pruebas_cortes.md`, `planning/frontend_next/plan_creacion_espacios_galeria_documentos.md`.

## Objetivo y orden

Resolver por separado los cuatro problemas reportados, cerrando revisión y pruebas locales de cada corte antes de pasar al siguiente. No declarar resuelta una capacidad por copy, mocks o un botón habilitado.

### Fase 1 — Zona horaria del espacio

**Criterio:** crear un espacio permite elegir en un desplegable una zona horaria territorial de Chile con etiqueta legible; el valor IANA elegido se envía al guardar y, al recargar/editar, se lee de vuelta sin ser sobrescrito por el detalle. El editor conserva también zonas IANA históricas no incluidas en las opciones actuales. No se codifican offsets UTC fijos.

**Estado observado:** CERRADA LOCALMENTE. El E2E controlado confirma las cuatro opciones territoriales, selecciona `America/Coyhaique`, comprueba el cuerpo de `PUT /availability`, recarga y valida `GET /weekly-hours` como fuente de rehidratación en el editor. El mock de detalle conserva `America/Santiago` para no satisfacer la aserción por sí mismo. La suite de dominio valida zonas IANA inválidas.

**Gate de cierre:** revisión independiente del delta final aprobada; prueba Playwright enfocada ejecutada el 2026-11-07: 1 passed; suites frontend y diff check pasan según `evidence/wizard-02-local-gates.md`. Evidencia solo UI/mock local, no persistencia de cuenta real. El espacio reportado no se tocó. Cierre local; no publicado.

### Fase 2 — Logo del navbar privado

**Criterio:** el enlace accesible del logo desde Mi Cuenta y las rutas privadas navega a `/`.

**Estado observado:** CERRADA LOCALMENTE. El cambio en `(workspace)/layout.tsx` apunta el logo a `/`; el E2E del flujo de creación comprueba el enlace accesible, hace clic, verifica navegación a `/` y vuelve al editor para continuar la prueba.

**Gate de cierre:** destino y navegación comprobados en el shell privado mediante Playwright local; suite completa final: 146 aprobadas, 63 omitidas, 0 fallidas. Esto no constituye aceptación online ni prueba del producto desplegado. Cierre local; no publicado.

### Fase 3 — Agregar imágenes al espacio

**Criterio:** desde el editor el propietario puede seleccionar JPG/PNG/WebP permitidos, recibir un resultado confirmado y ver la imagen rehidratada después de recargar. Si la galería está incierta o no confirmada, el control permanece bloqueado hasta reconciliación autoritativa; no quitar este resguardo para aparentar que la carga funciona.

**Estado observado:** UI y transporte frontend tienen un flujo existente. E2E controlado selecciona un archivo y verifica multipart/resultado con mocks. La matriz IMG-01 declara incompleto IMG-01E: falta evidencia de GCS real, catálogo anónimo, caché/revocación HTTP, carrera de cuota y lifecycle/soft-delete real. La política privada local no autorizó el alcance del ensayo sintético de almacenamiento; no hubo login ni carga real. La prueba mock no demuestra que el espacio del usuario pueda guardar una foto online.

**Secuencia/gates:** reconciliar primero la evidencia/versiones actuales con IMG-01E; identificar un target local aislado de datos sintéticos autorizado, sin sustituirlo por otra base; ejecutar primero gates frontend/BFF y luego los gates de Go/PostgreSQL/storage requeridos por el plan vigente. Cualquier DDL, credenciales, cuenta de ensayo, bucket/IAM/GCP o despliegue requiere el preflight y autorización específicos aplicables. No alterar migraciones aplicadas, recursos retenidos ni datos del usuario. Si el target o autorización no existe, registrar el bloqueo concreto y continuar fases independientes.

### Fase 4 — Calendario de disponibilidad por fechas

**Criterio:** aclarar e implementar la capacidad que el usuario espera, separándola del horario semanal habitual. La UI debe leer y gestionar únicamente intervalos que el contrato del servidor autorice; toda ocupación y disponibilidad final sigue siendo responsabilidad de Go. No inventar reservas, ocupantes, bloqueos ni estado.

**Inventario inicial (antes de esta implementación):** OpenAPI define `GET/POST /spaces/{spaceId}/availability/blocks`, `DELETE /spaces/{spaceId}/availability/blocks/{blockId}` y rutas de zona horaria; el handler Go implementa lectura/creación/borrado de bloqueos manuales. Ese contrato permitió integrar el feature Next descrito abajo; no implica disponibilidad derivada de reservas.

**Estado observado:** IMPLEMENTADA Y PROBADA LOCALMENTE como administración de bloqueos manuales, no como calendario de reservas ni disponibilidad confirmada. El adaptador tipado, facade/port y editor permiten listar por rango, crear, eliminar y convertir hora local usando la zona IANA guardada; se rechazan rangos inválidos y horas DST ambiguas/inexistentes. El E2E controlado comprueba conflicto `409`, creación, listado tras recarga, eliminación, fecha «Hasta» vacía sin petición ni estado de carga atascado, falta de zona horaria (sin petición de listado) y navegación para configurarla. Axe y desbordamiento horizontal pasan en 1280×800 y 390×844.

**Límites/gates:** solo rutas BFF mockeadas y datos sintéticos; no es prueba de API Go/DB real, reservas, ocupación confirmada ni disponibilidad final. Revisión independiente del delta y del ajuste de rango completada; sin hallazgos pendientes. Si se requiere disponibilidad derivada de reservas, usar el contrato y corte de reservas correspondiente; no ampliar esta interfaz por inferencia.

### Fase 5 — Publicación y aceptación

Cada corte terminado puede publicarse solo por la ruta autorizada en su rama de tarea, tras revisar los archivos exactos, autoría, hooks, pruebas y evidencia; un push a la rama no hace visible el cambio en la web. Integrar a `main` y desplegar son gates separados: las reglas actuales exigen completar el plan y sus revisiones antes de integración/despliegue, y el usuario conserva la validación funcional online. No realizar despliegue intermedio si contradice esos gates. Antes de afirmar publicación web, obtener evidencia del release exacto y de la revisión que sirve tráfico; no usar navegador/cuenta del propietario.

## Registro de avance

- 2026-11-07: iniciada la ejecución secuencial. Fase 1 ejecutada y cerrada localmente tras correr `npx --yes pnpm@12.15.0 exec playwright test tests/phase7-host-listings.spec.ts --grep 'asistente en 5 pasos'`: 1 passed. El recorrido verifica selección territorial, guardado IANA y rehidratación tras recarga. En el mismo recorrido se verificó el `href="/"` del logo desde el editor privado; Fase 2 cerrada localmente. No se desplegó ni se consultó la aplicación online.
- 2026-11-07: preflight de Fase 3, solo conectividad local: `127.0.0.1:18086/health/ready` respondió HTTP 200, Next en `https://127.0.0.1:4206/` respondió HTTP 200 y PostgreSQL en loopback:15436 aceptó conexiones. No se consultaron filas, no hubo login ni mutaciones. Esto no acredita por sí solo la identidad completa de DB ni prueba carga/persistencia de imágenes; el ensayo IMG-01E sigue condicionado a su política y gates, y GCS real no se ha verificado.
- 2026-11-07: fases 1 y 2 tienen cambios locales sin commit en esta rama. La suite frontend controlada pasó según `evidence/wizard-02-local-gates.md`; revisión independiente del flujo horario señaló inicialmente una aserción de rehidratación insuficiente, que se reforzó para comprobar `GET /weekly-hours` tras recarga. Ese delta fue revisado de forma independiente y aprobado. No hay publicación remota ni despliegue.
- 2026-11-07: se confirmó en código/OpenAPI que hay contrato backend para bloqueos manuales por fecha; se implementó su consumo Next local en la interfaz de anfitrión. Revisión independiente detectó fallo al refrescar con fecha «Hasta» vacía; la regresión Playwright fue RED, se corrigió el cálculo de rango para validar fechas sin convertir inválidos a ISO, y la prueba focal pasó (1/1). Re-revisión independiente aprobó el arreglo y su cobertura. La última corrida combinada pasó: 146 aprobadas, 63 omitidas, 0 fallidas. Gates adicionales: `api:check` (197 operaciones), typecheck, unitarias (480 aprobadas, 1 omitida), lint (0 errores, 4 advertencias), formato, build y `git diff --check` aprobados. No se inspeccionó la aplicación online ni se accedió al espacio informado.

## Siguiente paso

Siguiente: mantener IMG-01E bloqueada hasta que exista un alcance/target sintético autorizado conforme a la política privada; no hacer login ni carga real mientras falte. El calendario quedó implementado solo para bloqueos manuales con mocks; integración Go/DB y cualquier disponibilidad basada en reservas permanecen fuera de evidencia. Cualquier push será solo a la rama de tarea después de verificar identidad y hooks; no integrar a `main` ni desplegar, y no tocar el espacio reportado.
