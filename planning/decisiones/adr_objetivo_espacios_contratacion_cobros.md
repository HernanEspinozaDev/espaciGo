# ADR Objetivo de espacios contratación y cobros

**Estado: ratificado por el propietario el 4 de noviembre de 2026.** Es un objetivo de producto y desarrollo transversal para Mis espacios, búsqueda, reservas, contratos, pagos y operación. Prevalece sobre las hipótesis incompatibles de los planes históricos. El avance técnico se registra en la evidencia de cada corte.

## Cuenta, espacio y publicación

- Una cuenta puede ofrecer espacios, buscar, comunicarse y arrendar. Los papeles de arrendador y arrendatario se determinan por recurso y operación. La administración de plataforma conserva capacidades separadas.
- Un espacio es una unidad reservable exclusiva con un titular vigente. Solo el titular puede publicar, administrar y republicar. La verificación de identidad contractual se exige al emitir el contrato. El anuncio se valida por titularidad y por sus propias reglas.
- Conservar las ocho categorías y las unidades CLP hora, día y mes. Un bloque o sesión puntual tiene inicio, fin y precio aceptado. Recurrencia, subdivisión y aforo compartido quedan diferidos.
- La galería admite hasta 30 posiciones por espacio, incluidas cargas reservadas, con orden, portada y control de concurrencia.
- Tras la entrega del arrendatario, el titular confirma desocupación, libera el bloqueo de ese arriendo y puede republicar de inmediato. El contrato queda asociado al historial. Los demás intervalos vigentes se conservan.

## Contrato y firma

- Ambas partes verifican su identidad con RUT y nombre completo según su documento antes de emitir. La representación empresarial, cuando aplique, tiene un expediente separado para la empresa y la acreditación del representante. El contrato conserva la identidad y facultades evaluadas.
- Emitir un documento inmutable con ID único, condiciones, horario, renta y garantía aplicable. Cambiar condiciones requiere nueva emisión y aceptación. El historial conserva documento, referencias, huella y estados.
- Registrar por separado identidad, aceptación de condiciones de cada parte y firma. En hora, bloque o sesión, el recorrido principal es contrato sin firma y SimpleFirma queda en las opciones. Para inmuebles mensuales se recomienda SimpleFirma y se ofrece continuar sin ella. La ruta sin firma muestra «Este contrato se guarda sin firma electrónica de las partes».
- Si ambas partes eligen SimpleFirma, mostrar el desglose y acreditar los aportes antes de habilitar las dos firmas. El costo se divide 50/50 o lo paga íntegro el arrendador si ofreció firma gratis. Un peso impar queda a cargo del arrendador. Solo la nueva emisión que elija SimpleFirma genera nuevas solicitudes de firma. La validación de contratos firmados externamente queda diferida.

## Bloqueo y pagos

- Iniciar el bloqueo cuando ambas identidades estén verificadas y ambas partes acuerden horario y precio. Usar una única autoridad de ocupación y un solo bloqueo activo por reserva. Mostrar el vencimiento antes de cobrar.
- Valores aprobados para el ensayo: 15 minutos para aceptar y pagar la renta sin firma. Con SimpleFirma, 30 minutos para acreditar aportes, 60 minutos desde la acreditación para firmar y 15 minutos para pagar renta desde ambas firmas. Cuando esté habilitada la garantía, abrir 15 minutos adicionales desde el pago de renta. Acotar el plazo total desde el inicio del bloqueo y exigir que el horario comience después de la ventana aplicable. Los reintentos y duplicados conservan el vencimiento.
- La ruta sin firma habilita la renta tras ambas aceptaciones. La ruta SimpleFirma acredita sus cargos y ambas firmas antes de habilitar la renta. Mercado Pago es el proveedor inicial y Split se usa para renta y comisión. La garantía, cuando sea exigible y esté habilitada, se cobra por separado después de acreditar renta. Confirmar la reserva al cumplir condiciones y acreditar todos los cobros exigibles.
- El primer corte admite ofertas sin garantía monetaria. Habilitar ofertas con garantía después de comprobar cobro, devolución total y parcial, plazos, reclamos y responsable de fondos. El contrato refleja la modalidad y los montos aplicables.
- Al vencer, liberar el bloqueo atómicamente. Un pago tardío entra a conciliación y no reactiva la reserva. Registrar por separado aportes, renta, garantía, intentos, pagos, devoluciones y contracargos. Cada devolución se vincula al pago y pagador originales.
- Si vence un aporte antes de abrir la solicitud de firma, devolver los aportes acreditados. Si se abrió y vence sin ambas firmas, el piloto devuelve los aportes y EspaciGo cubre el cargo irrecuperable del proveedor dentro del presupuesto aprobado para el piloto. Con ambas firmas completas y renta vencida, conservar el servicio de firma y su cargo según condiciones informadas. Tratar errores de EspaciGo por una ruta de devolución revisada.

## Traducción a desarrollo

| Corte | Resultado exigido |
| --- | --- |
| Mis espacios | Titular vigente, galería, publicación y documentación privada por ámbito. Incorporar las reglas de oferta y republicación en los contratos del módulo. |
| Reserva sin firma | Identidad, emisión y aceptación, bloqueo, vencimiento y cobro de renta. Ensayar con datos sintéticos, incluido pago tardío. |
| Firma opcional | Desglose, patrocinio, aportes, solicitudes idempotentes, evidencia por firmante y devoluciones. Confirmar facturación de SimpleFirma y soporte de cobros y reversos por pagador antes de activar. |
| Garantía y cierre | Probar garantía por separado, conciliación y devolución. Confirmar entrega y republicación conservando historial. |

Adoptar el modelo lógico normalizado como objetivo. Reutilizar las tablas existentes cuando permitan verificar las invariantes. Determinar los cambios físicos por corte con un mapeo, migraciones incrementales y evidencia. Separar acceso por recurso, auditoría, archivos privados y retención por finalidad. Antes de operar con clientes, ensayar recuperación conjunta de base y objetos, y verificar permisos y referencias restauradas.

La integración de renta conserva Checkout Pro / API de Preferences y la vinculación OAuth del arrendador antes de aceptar un arriendo que genere cobros. La especificación de pagos del 28 de octubre documenta los contratos de proveedor y el alcance anterior. Este ADR fija la secuencia objetivo y las etapas de habilitación de firma y garantía.
