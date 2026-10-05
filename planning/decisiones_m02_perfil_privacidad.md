# M02 — contrato inicial de perfil y derechos

Este corte permite avanzar sobre la base ya integrada de M01. La matriz cubre el contrato local implementable de PRIV-ARCH-01 y PRIV-DB-01; el PR queda en revisión hasta que la persona usuaria acepte la entrega.

| Recurso/campo | Finalidad | Acceso | Conservación / límite |
|---|---|---|---|
| `perfil_usuario.usuario_id` | Vincular perfil a cuenta | Solo titular autenticado | FK `RESTRICT`; sin borrado automático |
| `nombre_visible` | Nombre visible en el producto | Titular consulta/edita; API no expone perfiles ajenos | Sin plazo aprobado; queda pendiente DB02-05 |
| `telefono_normalizado` | Contacto opcional | Solo titular consulta/edita | Sin plazo aprobado; queda pendiente DB02-05 |
| `solicitud_titular.id`, `usuario_id`, `tipo`, `canal`, `estado`, `solicitada_en` | Recibir y seguir solicitudes de acceso/supresión | Titular crea y consulta las propias; no hay interfaz administrativa en este corte | Hecho histórico, FK restrictiva; sin política de retención aprobada |
| referencia de cuenta de cobro | Evitar guardar credencial bancaria en claro | No habilitada en este corte | Requiere decisión de proveedor/token y RUT verificado por M03/KYC |
| foto de perfil | Requisito CU-07 | No implementada | Anexo B no define almacenamiento/objeto ni política; requiere contrato de storage |

## Controles y decisiones pendientes

- Perfil privado por propietario autenticado. El API crea el perfil al recibir el nombre; no deriva nombres desde el correo ni retorna datos de otros usuarios.
- Teléfono opcional validado como exactamente nueve dígitos según CU-07. No se afirma que sea un número telefónico internacional.
- La solicitud de supresión se registra como `en_revision`; no elimina ni desidentifica datos. No existe aún integración para comprobar reservas activas, liquidaciones, pagos o disputas (M06/M10) y DB02-05 no define retención, excepciones, copias o derivados.
- No se habilita cuenta de cobro antes de KYC/RUT verificado. No se guardan números bancarios, tokens de proveedor ni RUT en este corte.
- No hay historial de claves ni avisos durables; DB02-09 permanece pendiente. #123 sigue como limpieza del servidor independiente.

## Trazabilidad

CU-07–09, HU-31/32; RQF-024–037 y RQF-189–191; ES2 Anexo B (`perfil_usuario`, `cuenta_cobro`, `solicitud_titular`); PRIV-ARCH-01, PRIV-DB-01/02, PRIV-BE-01/02, PRIV-API-01, PRIV-TEST-01 y PRIV-MOCK-01. Este documento registra límites del prototipo, no una determinación legal ni cumplimiento demostrado.
