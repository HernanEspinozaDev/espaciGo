# Cierre del flujo de creación y gestión de espacios

- Corte: 2026-11-10.
- Plan detallado de capacidades: `planning/frontend_next/plan_creacion_espacios_galeria_documentos.md` (sus tablas históricas se subordinan a este estado reconciliado y a la evidencia fechada).
- Rama de implementación: `codex/frontend/fix/space-01-create-postcommit-recovery`.
- Base funcional inspeccionada: `5830c9ce2dc61d4df13a4139da984b4cafd6d9ba`.
- Primer corte de implementación: `a5aea39bac4341b415a642297e3c4d375bacd27b`, publicado en la rama anterior con el mismo SHA y worktree limpio. No integrado a `main`, no desplegado.

## Objetivo y límites

Cerrar desde la interfaz local de anfitrión el recorrido: Mis espacios → crear borrador → completar/guardar → reabrir/editar → horario y bloqueos manuales → galería → publicar → consultar el anuncio. Go conserva autorización, validaciones y persistencia.

Fuera de este plan: KYC como requisito para crear/publicar, reservas y ocupación derivada de reservas, contratos, firma, cobros, garantías, administración global, F7 y operaciones de Cloud SQL. No convertir horario semanal ni bloqueos manuales en disponibilidad reservable. La navegación online del producto corresponde al usuario.

## Estado por orden

| Orden | Alcance                                                                                                                                     | Estado al 2026-11-10                                                                                                                                                                                                                                                 |
| ----- | ------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 0     | Baseline por criterio: pantalla/ruta, contrato OpenAPI, handler, persistencia, pruebas y nivel de evidencia.                                | **Completo como inventario estático** en `evidence/espacios-baseline-trazabilidad-20261110.md`. La matriz distingue pruebas mock, evidencia local histórica y fuente actual; no afirma integración del HEAD actual.                                                  |
| 1     | Wizard actual sobre target sintético aislado: sesión, perfil dinámico, validación, guardado, reanudación tras reload y readback persistido. | **Parcial.** El primer bugfix de recuperación parcial está publicado en la rama; su regresión y gates son controlados. Falta un recorrido integrado del árbol actual con readback. No repetir el alta manual del usuario ni recursos mutados por ensayos anteriores. |
| 2     | Zona IANA, horario semanal y bloqueos manuales; CAS, rango, refresh, reload y zona/DST.                                                     | **Pendiente de verificación contra el árbol actual.** Las pruebas nativas de SHA anterior son evidencia histórica; no repetir cambios persistidos.                                                                                                                   |
| 3     | Galería: carga/readback, 30/31, portada, orden, reemplazo, borrado, ownership e incertidumbre en el target elegido.                         | **Pendiente de verificación contra el árbol actual.** Hay pruebas/evidencia histórica y tests controlados, no asumir que alcanzan el UI actual ni recrear fotos.                                                                                                     |
| 4     | Publicación/consulta pública, elegibilidad, CAS y visibilidad/privacidad.                                                                   | **Parcial por fuente; gate nativo del UI actual pendiente.** Se detectó que el listado/editor no ofrece un CTA directo a `/espacios/{id}` aunque la página pública y las operaciones backend existen. No implementar antes del orden 1–3.                            |
| 5     | Una sola corrida final del artefacto reunido: API/OpenAPI, Go, unitarias, tipos/lint/formato, build, Playwright/Axe y recorrido integrado.  | **No iniciada.** Ejecutarla una vez cuando las brechas y target estén reunidos/autorizados.                                                                                                                                                                          |
| 6     | Integración/entrega y aceptación.                                                                                                           | **Separada.** La publicación de rama no es integración a `main`, despliegue ni aceptación online. Cualquier Cloud SQL/lock/permiso/reintento requiere autorización y procedimiento propios.                                                                          |

Secuencia: `0 → 1 → 2 → 3 → 4 → 5`; la entrega/aceptación permanece separada. No saltar a un orden posterior por cobertura incidental de una suite.

## Primer corte publicado

`a5aea39bac4341b415a642297e3c4d375bacd27b` recuerda el ID recibido de `POST /spaces` si falla configuración posterior, ofrece alerta accesible y recuperación al editor (o a Mis espacios si cambió el ámbito de cuenta), y bloquea un segundo POST en esa instancia del wizard. Añade una regresión que fuerza 503 en `PUT /weekly-hours` después de crear.

Verificación local recogida en `evidence/space-create-partial-recovery-20261110.md`: RED/GREEN controlada; el archivo original tuvo 2 PASS y la corrida combinada con la nueva regresión de cambio de cuenta tuvo 3 PASS; build, typecheck, formato y unitarias PASS; lint sin errores y 4 warnings ajenos al cambio; 1 skip opt-in en unitarias. No demuestra Go/PostgreSQL, storage ni comportamiento online. La pérdida de respuesta del POST de alta continúa sin reconciliación contractual; no se añadió `Idempotency-Key` no soportado.

El seguimiento de aislamiento conserva fuera de `PrivateSessionBoundary` solo una marca booleana en memoria, nunca el ID del borrador. Tras un cambio de cuenta/scope, el wizard remountado muestra un aviso genérico con destino a Mis espacios y sigue bloqueando el POST. Un coordinador en el layout compartido limpia la marca al salir de `/anfitrion/espacios/nuevo`; la regresión controlada verifica navegación efectiva, ausencia de enlace al editor anterior y limpieza al volver después de salir del flujo. No implica persistencia tras recarga ni cierra el orden 1.

El SHA `a5aea39bac4341b415a642297e3c4d375bacd27b` fue verificado en la rama remota `codex/frontend/fix/space-01-create-postcommit-recovery`. El hook permaneció habilitado. No se creó PR, no se empujó `main` ni se declaró rollout.

## Reglas de ejecución

- No visitar la aplicación pública/demo, no usar navegador/CUA del usuario ni credenciales reales; conservar la validación online para el usuario.
- No repetir alta, publicación, guardado horario o carga ya persistidos. No crear/modificar cuentas o fixtures sin target y autorización explícitos.
- Antes de una prueba integrada, verificar el target local aislado seleccionado y su compatibilidad con política de cuentas; no sustituirlo por una base persistente ni por datos del usuario. Si falta ese acceso, detener solo el gate integrado y continuar el análisis/cobertura no mutante.
- No cambiar bytes/ledger/versiones de migraciones aplicadas o intentadas; no tocar KYC, Cloud SQL, roles ni permisos en este plan. La matriz documenta esquema existente; no propone DDL.
- Distinguir mocks, tests de código, DB local, browser→BFF→Go/storage y release. Un nivel no sustituye otro.
- Mantener cuentas, sesiones, recursos y demos de ensayos anteriores; ningún resultado de pruebas automáticas inventa aceptación humana.

## Siguiente paso

El orden 0 ya está registrado. Para completar orden 1 falta fijar un target local sintético aislado y autorización de uso/creación de la identidad de ensayo que requiera la ruta visible. En este corte no se eligió ni se arrancó ese target y no se leyó una política privada. Una vez resuelta esa precondición, ejecutar solo una creación nueva en ese target, con recibo previo y readback independiente; nunca usarla para reejecutar una mutación incierta.
