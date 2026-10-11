# EspaciGo Angular

Frontend independiente del backend Go y del mock de transición. Este primer corte cubre FE-00 y FE-01: contrato generado, shell responsivo, páginas informativas de categorías y una comprobación real de readiness. No implementa aún cuentas, catálogo, reservas ni otras capacidades marcadas para cortes posteriores.

## Herramientas fijadas

- Node `24.21.0` (`.nvmrc`; Angular 22 requiere Node `^22.22.3 || ^24.15.0 || >=26.0.0`).
- Angular framework, CLI, build y compiler-cli `22.2.2`.
- TypeScript `6.0.3` (rango Angular `>=6.0.0 <6.1.0`).
- npm `12.1.0`, RxJS `7.8.2`, `@hey-api/openapi-ts` `0.99.0`.

## Contrato y cliente

El contrato canónico es `../planning/openapi.yaml` (OpenAPI 3.1.0). El cliente Angular se regenera sin editar archivos generados:

```sh
npm ci
npm run api:generate
npm run api:check
```

Se comprobó el generador `typescript-angular` de OpenAPI Generator, pero el ejecutor estándar requiere Java, no instalado en el entorno de desarrollo. El generador fijado Hey API 0.99.0 reconoce OpenAPI 3.1. La primera ejecución detectó una referencia inexistente `#/components/parameters/IdempotencyKey`; se añadió el parámetro reutilizable a la especificación y la generación produjo el SDK Angular completo. `api:check` regenera el cliente y compila el TypeScript de la aplicación. El output incluye clientes tipados para login, perfil, catálogo autenticado y archivo ZIP privado.

El SDK conserva `int64` JSON como `number`; por ello, no se deben presentar o usar como valores exactos importes fuera del rango seguro de JavaScript. FE-00 deja esta limitación explícita para resolver el decodificador de números JSON sin pérdida antes de los flujos financieros. El corte actual no consume ni transforma importes.

El SDK conserva el contrato Bearer, pero todavía no hay sesión ni interceptores de autenticación en FE-01. Los archivos binarios están tipados como `Blob | File`; su descarga autenticada, manejo de URL temporal y aislamiento entre sesiones quedan para el corte de privacidad que consume esas operaciones.

## Servidor local

`GET /healthz` comprueba el proceso Angular. La aplicación consulta el endpoint real `GET /health/ready` del backend Go mediante `API_PUBLIC_BASE_URL`; nunca muestra una disponibilidad sintética. En ejecución Docker esta URL es configuración pública del navegador y no contiene credenciales. El origen Angular local se agrega a la lista CORS de Compose, sin cambiar rutas, autenticación ni autorización del backend.

Para añadir Angular al stack local y conservar el mock:

```sh
docker compose -f compose.yaml -f compose.frontend.yaml up --build angular-frontend
```

El frontend se publica en `http://localhost:4200` (override `LOCAL_ANGULAR_PORT`). Puede ejecutarse independientemente con `npm start`; la configuración de desarrollo apunta a `http://localhost:8080/api/v1`.

## Alcance visible

`/`, `/categorias` y `/como-funciona` contienen solo información del producto y son prerenderizadas. `/explorar` y `/acceso` muestran claramente que el catálogo requiere sesión (FE-03) y que la identidad llegará en FE-02. No se muestran fichas, disponibilidad, precios ni acciones no respaldadas por una sesión o por el contrato actual.
