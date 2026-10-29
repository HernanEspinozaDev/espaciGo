# F3 — errores de validación del borrador en la interfaz

Fecha: 2026-10-28

## Cambio

Cuando el editor recibe un `422 validation_error` con `error.details` del contrato Go, convierte los campos comunes permitidos del borrador y los atributos presentes en el esquema de categoría cargado a mensajes breves en español junto al control correspondiente. Los controles soportados reciben `aria-invalid` y `aria-describedby` enlaza el texto de error; al corregir el valor se limpia el error. Los errores no reconocidos siguen usando el aviso general fijo; no se muestran claves/códigos desconocidos ni valores enviados por el usuario.

## Evidencia

- Playwright Chromium local, test del editor `navegación por pestañas`: el caso RED devolvió un 422 sintético Go-shaped para `title/max_length`; falló porque el input permanecía `aria-invalid=false`.
- El mismo E2E ahora verifica errores `location_pair` asociados a Región y Comuna junto con un atributo fuera del esquema (detalle mixto, con aviso general seguro), además de un error con código no reconocido para `wifi` (aunque el campo exista en esquema) combinado con un atributo desconocido: ambos detalles no producen errores inline y la respuesta sólo conserva el aviso fijo, sin mostrar claves/códigos. También prueba una respuesta sólo con detalles desconocidos, la asociación de `title/max_length` mediante `aria-describedby` y la limpieza al corregir.
- Para atributos dinámicos, `attributes.<code>/invalid_value` solo se presenta inline cuando el código está en el esquema cargado; el E2E cubre los cuatro tipos de control renderizados (booleano, numérico, enum y enum_list), con `aria-invalid`, `aria-describedby`, mensaje y limpieza al corregir. También verifica en el payload mock los valores editados. No demuestra que Go entregue esos 422 ni persistencia.
- Gates del frontend (corrida del 2026-10-28): API check (183 operaciones), format, typecheck, lint, 260/260 unit tests, build y `@phase7` (22 passed, 1 opt-in native skipped).
- Los 422 son mocks con la forma del contrato; no demuestran browser→BFF→Go ni persistencia real.

## Límites

El mapeo UI de esta rebanada cubre campos comunes del borrador y atributos que pertenecen al esquema actualmente cargado. Los errores de atributos/campos desconocidos conservan el aviso general seguro; esta prueba no recorre cada tipo de atributo ni las validaciones de precio/horario del servidor en sus operaciones separadas. F3 continúa parcial hasta probar el recorrido persistido con Go/PostgreSQL y rehidratación real.
