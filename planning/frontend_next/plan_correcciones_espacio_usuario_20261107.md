# Plan de correcciones del espacio informado por el usuario

- Fecha de elaboración: 2026-11-07.
- Checkout de trabajo: `/home/nandev/Documentos/projects/espaciGo-wizard-improvements`.
- Rama: `codex/frontend/feature/wizard-02-horario-galeria`.
- HEAD base: `8f0e06e2d9e5f16b429c69f190aa9dcd44148fc6`.
- Espacio mencionado por el usuario: `df900c8a-5486-46ca-bf49-c677e6227a92`; no consultar ni modificar sin autorización y entorno expresos.
- Estado externo: la rama de tarea sigue publicada en `origin/codex/frontend/feature/wizard-02-horario-galeria` (`1c49c042814695f5257feb0262ebac9291622b5f`). Preparé un candidato de integración local en `main`: merge `141d348aa6e4bc7002020741830da899715c277f` y sincronización de cliente OpenAPI `b89fe0f196eb5ff2c3b155bafd4b60e3ea074618`. `origin/main` continúa en `274f37e483a078890e92382a159fa49fd23c2fe8`; no se hizo push ni despliegue. El push a main activa el rollout web automático, pendiente de autorización explícita.
- Evidencia relacionada: `plan_wizard_02_horario_galeria.md`, `evidence/wizard-02-local-gates.md`, `planning/imagenes/plan_img_01_carga_crud.md`, `planning/imagenes/matriz_capacidades_pruebas_cortes.md`, `planning/frontend_next/plan_creacion_espacios_galeria_documentos.md`.

## Objetivo y orden

Resolver por separado los cuatro problemas reportados, cerrando revisión y pruebas locales de cada corte antes de pasar al siguiente. No declarar resuelta una capacidad por copy, mocks o un botón habilitado.

### Fase 1 — Zona horaria del espacio

**Criterio:** crear un espacio permite elegir en un desplegable una zona horaria territorial de Chile con etiqueta legible; el valor IANA elegido se envía al guardar y, al recargar/editar, se lee de vuelta sin ser sobrescrito por el detalle. El editor conserva también zonas IANA históricas no incluidas en las opciones actuales. No se codifican offsets UTC fijos.

**Estado observado:** CERRADA LOCALMENTE. El E2E controlado confirma las cuatro opciones territoriales, selecciona `America/Coyhaique`, comprueba el cuerpo de `PUT /availability`, recarga y valida `GET /weekly-hours` como fuente de rehidratación en el editor. El mock de detalle conserva `America/Santiago` para no satisfacer la aserción por sí mismo. La suite de dominio valida zonas IANA inválidas.

**Gate de cierre:** revisión independiente del delta final aprobada; prueba Playwright enfocada ejecutada el 2026-11-07: 1 passed; suites frontend y diff check pasan según `evidence/wizard-02-local-gates.md`. Evidencia solo UI/mock local, no persistencia de cuenta real. El espacio reportado no se tocó. Cierre local; código publicado solo en la rama de tarea, no en la web.

### Fase 2 — Logo del navbar privado

**Criterio:** el enlace accesible del logo desde Mi Cuenta y las rutas privadas navega a `/`.

**Estado observado:** CERRADA LOCALMENTE. El cambio en `(workspace)/layout.tsx` apunta el logo a `/`; el E2E del flujo de creación comprueba el enlace accesible, hace clic, verifica navegación a `/` y vuelve al editor para continuar la prueba.

**Gate de cierre:** destino y navegación comprobados en el shell privado mediante Playwright local; suite completa final: 146 aprobadas, 63 omitidas, 0 fallidas. Esto no constituye aceptación online ni prueba del producto desplegado. Cierre local; código publicado solo en la rama de tarea, no en la web.

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

Cada corte terminado puede publicarse solo por la ruta autorizada en su rama de tarea, tras revisar los archivos exactos, autoría, hooks, pruebas y evidencia; un push a la rama no hace visible el cambio en la web. A petición expresa del usuario preparé una integración local normal, sin PR ni reescritura, en `main`; no actualicé el ref remoto. El trigger `espacigo-main-rollout` está activo para `^main$`; como el delta de código es frontend, el pipeline clasifica `web` y despliega los servicios público y privado. El push permanece pendiente de autorización explícita para ese efecto. El usuario conserva la validación funcional online.

## Registro de avance

- 2026-11-07: iniciada la ejecución secuencial. Fase 1 ejecutada y cerrada localmente tras correr `npx --yes pnpm@12.15.0 exec playwright test tests/phase7-host-listings.spec.ts --grep 'asistente en 5 pasos'`: 1 passed. El recorrido verifica selección territorial, guardado IANA y rehidratación tras recarga. En el mismo recorrido se verificó el `href="/"` del logo desde el editor privado; Fase 2 cerrada localmente. No se desplegó ni se consultó la aplicación online.
- 2026-11-07: preflight de Fase 3, solo conectividad local: `127.0.0.1:18086/health/ready` respondió HTTP 200, Next en `https://127.0.0.1:4206/` respondió HTTP 200 y PostgreSQL en loopback:15436 aceptó conexiones. No se consultaron filas, no hubo login ni mutaciones. Esto no acredita por sí solo la identidad completa de DB ni prueba carga/persistencia de imágenes; el ensayo IMG-01E sigue condicionado a su política y gates, y GCS real no se ha verificado.
- 2026-11-07: fases 1 y 2 se cerraron localmente; la rehidratación se comprobó mediante `GET /weekly-hours` tras recarga y su revisión independiente fue aprobada. El corte se publicó en la rama de tarea y luego se integró localmente en el candidato `main` descrito abajo; `origin/main` no se actualizó.
- 2026-11-07: se confirmó en código/OpenAPI que hay contrato backend para bloqueos manuales por fecha; se implementó su consumo Next local en la interfaz de anfitrión. Revisión independiente detectó fallo al refrescar con fecha «Hasta» vacía; la regresión Playwright fue RED, se corrigió el cálculo de rango para validar fechas sin convertir inválidos a ISO, y la prueba focal pasó (1/1). Re-revisión independiente aprobó el arreglo y su cobertura. La última corrida combinada pasó: 146 aprobadas, 63 omitidas, 0 fallidas. Gates adicionales: `api:check` (197 operaciones), typecheck, unitarias (480 aprobadas, 1 omitida), lint (0 errores, 4 advertencias), formato, build y `git diff --check` aprobados. No se inspeccionó la aplicación online ni se accedió al espacio informado.
- 2026-11-07: reconciliación contra `origin/main` (`274f37e`): la base compartida avanzó 10 commits desde la base de tarea; la rama aporta dos commits. `git merge-tree` y el merge normal no reportaron conflictos. `api:check` detectó que el cliente generado estaba atrasado respecto de tres operaciones Didit ya presentes en OpenAPI de main; se regeneraron cliente, allowlist e inventario (200 operaciones), y el check pasó. En el candidato integrado pasaron API check, typecheck, unitarias (480/1 omitida), lint (0 errores/4 advertencias), Prettier, build y diff check. Playwright completo: 144 aprobadas, 63 omitidas y 2 fallidas; las mismas dos fallas se reprodujeron sobre `origin/main` sin la rama de tarea: expectativa antigua de KYC y dos `h1` en la página de verificaciones. No son archivos modificados por esta rama y quedan pendientes, no se corrigieron dentro del corte. El candidato local está en `b89fe0f`; `origin/main` sigue intacta y el rollout no se inició.

## Siguiente paso

Siguiente: mantener IMG-01E bloqueada hasta que exista un alcance/target sintético autorizado; no hacer login ni carga real mientras falte. El calendario solo prueba bloqueos manuales con mocks. Antes del push a main, el propietario debe confirmar que autoriza el rollout web automático asociado. Mantener documentadas las dos fallas preexistentes de E2E y no modificar el espacio reportado.
