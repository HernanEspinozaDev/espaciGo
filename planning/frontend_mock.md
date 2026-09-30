# Frontend mock para validación de módulos

## Propósito y límite

El frontend mock es un **test harness temporal**, no una primera versión del frontend definitivo ni fuente de decisiones visuales/UX. Se crea una tarjeta por módulo funcional al final de su slice, salvo que el tamaño y dependencias aconsejen dividirla. Solo prueba API pública y resultados visibles.

## Tecnología permitida

HTML semántico, CSS mínimo, TypeScript compilado a JavaScript, módulos ES nativos, DOM estándar y `fetch`. Sin frameworks, HTMX por defecto, router complejo, design system, estado global, abstracciones genéricas o generación HTML desde backend. No modificar contratos REST/JSON para atender el mock.

## Aislamiento

```text
Browser → contenedor mock (servidor estático mínimo) → API HTTP pública → PostgreSQL
```

El mock tiene su propio contenedor Docker y red solo hacia el API; no tiene credenciales ni ruta a la DB. CORS y URL de API se configuran por ambiente sin secretos. El compose local futuro coordinará `database`, `backend` y `mock-frontend`; esta planificación no modifica Docker ni levanta servicios.

## Operaciones por tarjeta

Cada ticket enumera operaciones exactas soportadas por el contrato ya implementado. Como mínimo cuando aplique: listar/buscar, leer detalle, crear/actualizar/eliminar si el API lo permite, autenticación requerida, mostrar request/respuesta, errores HTTP y validaciones. No incluir operación no soportada por API ni saltarse roles, permisos, tokens o headers.

## Criterios de aceptación comunes

- Consume únicamente endpoints públicos HTTP/JSON definidos en OpenAPI.
- Corre en contenedor separado y funciona junto a DB/API en entorno local previsto.
- Puede ejecutar las operaciones enumeradas en la tarjeta, mostrar éxito, errores HTTP relevantes y datos retornados.
- Respeta autenticación, autorización, tokens, roles y headers del backend.
- No accede a DB, secretos de proveedor ni storage interno.
- Usa solo tecnologías web básicas enumeradas arriba y no introduce framework frontend.
- El código/función se puede retirar sin cambiar backend ni condicionar la arquitectura futura.

## Decisión diferida

Cuando DB, backend y APIs tengan avance suficiente, se abrirá una decisión independiente para framework, arquitectura, diseño visual, componentes, UX/UI, librerías, estado y navegación definitivos. Ninguna tarjeta de este backlog adelanta esa selección.
