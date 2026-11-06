# Entrega local de Mis espacios — 2026-11-05

## Código probado y alcance

- Frontend: `078eab5257b049291ffb35d21375555a1a3bb03d`, rama `codex/frontend/fix/local-close-02-f6-native-e2e`.
- Backend asociado: `1bbefa19d26c08b388ca113d1a9c70eb0360c090`, rama `codex/backend/feature/local-close-02-espacios-galeria-documentos`.
- Backend entregado con evidencia: `c201a2f4cf58e985f76dd311e5ccf9e099b737cd`, ref remoto leído y coincidente con HEAD local; diff posterior al SHA de código limitado a documentación/evidencia.
- Este recibo es documental; no modifica el código probado. Los HEAD finales de publicación se entregan solo después de verificar los refs remotos. No hay PR, integración a main, despliegue ni aceptación humana implícita.
- La matriz única vive en la rama backend: `planning/frontend_next/plan_creacion_espacios_galeria_documentos.md`. Detalle: `evidence/mis-espacios-tres-pendientes-local-20261105.md`. No se crea otra matriz de plan en este worktree.

## Correcciones y pruebas finales

La fachada elimina lista, selección y blobs si una decisión no queda confirmada, incluso cuando el POST404 puede ser posterior a un commit. No permite operaciones sobre filas anteriores hasta actualización explícita y exitosa. Un refresh ordinario fallido también invalida las filas. No hay repetición automática de decisiones. Confirmado por16pruebas conductuales de fachada y4E2E controlados; el test de incertidumbre cubre POST404trascommit,GET500,revisiónantigua,ambasfilas purgadas,selección/acciones ausentes,unPOST y recuperación con revisión2.

La aprobación no sustituye al propio expediente. La prueba real Go/PostgreSQL/handler reprodujo commit de aprobación, retirada auditada del grant entre commit y readback, HTTP404 y comprobación independiente del estado/eventos persistidos. Se conserva el rechazo de lectura sin devolver metadata ya no autorizada.

Revisiones finales sin bloqueos: `deleg_67ed9314` fachada (16/16 ejecutadas por revisor), `deleg_58769e65` procedencia y criterio404postwrite. ACL/codec/BFF conservan sus aprobaciones anteriores con fuentes verificadas sin cambios. El padre ejecutó y verificó los gates PostgreSQL/nativos por separado.

- **141unitPASS/0SKIP**, TypeScript, ESLint, API-check, formato y build de producciónPASS.
- Suite global sobre el artifact definitivo: **76PASS/52SKIP**. Los opt-in omitidos no se cuentan como integración real.
- Nativo documental repetido después de las correcciones: **1PASS**. Navegador→HTTPS/BFF→Go→PostgreSQL reales; PDF real/SHA/Poppler,scope yselfdenial,retiro/reasignación,approve/revoke con GETposterior,AXEdesktop/mobile.
- Demo documental read-only final: **1PASS**, PDF real, estadoenviada preservado, AXE y sin overflow en1365×900/390×844. Capturas: `reviewer-pending-desktop-20261105.png` y `reviewer-pending-mobile-20261105.png`.
- Galería: corrida nativa `20261105-floor5`1PASS, dos fallbacks1280/Q75 verificados porbytes/SHA/decodificación, rechazo al exceder el piso y descarte explícito solo tras `failed` autoritativo, cola2/alfa96,30/31 y publicación. Se conservaron la evidencia previa de creación/galería/publicación/republicación y el código aprobado de BFF/cola.

## Demo y retención

URL local: `https://127.0.0.1:4203/mi-cuenta/revision-documental`. Doscuentas sintéticas existentes; acceso/materiales exclusivamente en la política e inventario privados ya existentes. No crear cuentas ni resetear contraseñas para revisar.

Backend18082, Nextstandalone4204, TLS4203 yforwardPG15434 se retienen con datos y volúmenes para revisión humana. Destino único: `espacigo_f5_gate_20261101`. Recibos de procesos/fixtures bajo `.local/testing/space-gallery-demo/` en el checkout backend; verificar UID/exe/cwd/destino antes de intervenir. No usar el navegador del usuario ni rutas online.

Se conserva un único grant vigente para el expediente limpio pendiente de la demo; el caso aprobado del ensayo404 y el caso revocado del nativo permanecen como historia. Backup final de DB/storage fuera de tmpfs verificado por lectura completa y SHA, con recibo privado `backup-accepted-*`. Responsable de retención: agente padre/usuario. No retirar el último consumidor de tmpfs ni limpiar volúmenes por patrón.

Límite vigente: daily28173 de ClamAV superó48h. La actualización oficial acotada informó que seguía siendo la última disponible; egress fue retirado. Nuevas inspecciones permanecen bloqueadas correctamente y un borrador sintético negativo se conserva separado. La demo abre archivos realmente inspeccionados/escaneados limpios antes de ese vencimiento; no se falsearon estados ni se relajó la política. No se promete una nueva subida documental limpia hasta disponer de firmas oficiales dentro de la política.

Reserva, firma, cobros, garantía, main, F7/cloud y proveedores/personas reales quedan fuera.
