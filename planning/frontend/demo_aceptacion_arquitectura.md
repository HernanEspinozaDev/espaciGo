# Recorrido de aceptación FE-14 tras la adecuación arquitectónica

Estado: **preparado para ejecutar sobre el commit aprobado de la adecuación; no es una aceptación del usuario**. La retirada del mock (FE-15) permanece pendiente. Este recorrido complementa las evidencias históricas, no las sustituye por afirmaciones sobre código que no se haya ejecutado.

## 1. Revisión previa

1. Revisar `revision_arquitectura.md` y las decisiones originales: features, puertos, fachadas, infraestructura, contratos puros y composición DI.
2. Aprobar y fusionar el PR de adecuación cuando sus comprobaciones y revisión sean satisfactorias.
3. Registrar el SHA de `main` que se va a demostrar. No mezclar configuración privada ni documentos locales ajenos.

## 2. Gate reproducible

Desde la raíz del checkout aprobado, con Node/npm fijados en `frontend/package.json`:

```bash
npm --prefix frontend ci
npm --prefix frontend run lint
npm --prefix frontend run api:check
npm --prefix frontend test -- --watch=false
npm --prefix frontend run build
npm --prefix frontend audit --omit=dev
bash scripts/test-frontend-docker.sh
bash scripts/test-frontend-e2e.sh
```

Ejecutar Docker y E2E **en secuencia**. Los scripts generan proyecto, redes, base, cuentas y volumen desechables y los retiran al terminar. No ejecutar Compose por defecto para preparar la demo. El E2E genera capturas `fe14_*.jpg/json`: ejecutarlo en una copia del commit si se quieren conservar intactas las evidencias históricas; identificar las nuevas con el SHA probado.

El gate incluye una creación/edición de borrador desde el formulario Angular real contra Go, además de aislamiento del runner, roles, descargas privadas y recuperación de una sesión durante una caída real de Go. No cubre por sí solo todas las acciones del producto.

## 3. Demo acompañada

Desarrollo prepara un stack aislado del mismo commit aprobado, con cuentas sintéticas y un inventario de estados confirmados por Go. Si faltan cuentas o contenido, crearlos en esa base desechable. Guardar credenciales solo localmente en `.local/testing/`, excluidas de Git y con permisos `0600`. La demo manual necesita datos representativos adicionales a las cuatro cuentas del E2E.

| Recorrido | Comprobación del usuario |
| --- | --- |
| Portada/categorías/acceso | Móvil 390×844 y escritorio 1365×900; menú con teclado, Escape, foco y ausencia de desbordamiento. |
| Sesión y permisos | Login visual, rutas por rol, tercero rechazado, logout y desaparición de datos privados. |
| Catálogo/oferta/calendario | Búsqueda y detalle; crear/editar borrador; atributos por categoría y horario por zona IANA. No afirmar publicación si Go no confirma elegibilidad. |
| Cuenta/KYC | Perfil, evidencia sintética privada, estados confirmados, exportaciones JSON/ZIP y descarga real. |
| Reserva/contrato/operación | Solicitud y pago fake; decisión, firmas, PDF, check-in/out y recepción. |
| Conversación/disputa/finanzas | Mensajes y estados terminales; reclamo/evidencia; decisiones, liquidación y comprobante cuando el fixture cumpla las condiciones de Go. |
| Administración | Capacidades verificadas, colas, motivos y reintentos, bloqueo/supresión/moderación y auditoría. |
| Caída de Go | Misma pestaña y sesión; error real, vista privada sin datos obsoletos y recuperación con HTTP autenticado confirmado. |

Registrar para cada recorrido ruta, rol, estado inicial/final, respuesta Go, resultado y límite. Conservar capturas y dimensiones sin credenciales. No forzar condiciones comerciales ni presentar como realizado un flujo que no pudo ejecutarse. El entorno local y sus pagos son sintéticos; Cloud Run, staging y proveedores reales requieren su propio alcance.

## 4. Decisión final

Aceptar FE-14 solo después de revisar la arquitectura aprobada y la demo del mismo commit. Una aprobación técnica del PR no reemplaza esa decisión. Si aparece una brecha, registrarla y corregirla antes de retirar el mock.

Comentario que puede usar el usuario después de la revisión y demo satisfactorias:

> Acepto explícitamente FE-14 sobre el commit demostrado y la adecuación arquitectónica revisada. Continúa con FE-15 según el plan, conserva Angular/Go y elimina el mock únicamente dentro del alcance acordado. Publica el PR con HernanMEC y solicita revisión a HernanEspinozaDev; verifica ambas identidades antes de publicar.
