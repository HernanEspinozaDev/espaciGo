# Auditoría Técnica Frontend: Mi Cuenta, Mis Espacios, Editores Dinámicos y Galería Real

**Fecha:** 27 de Octubre de 2026  
**Repositorio Principal:** `/home/nandev/Documentos/projects/espaciGo` (`main` limpio en commit `3956d2f`)  
**Referencia Funcional Legada:** `/home/nandev/Documentos/projects/Proyectotitulo/espaciGo` (Angular en reposo, cambios de usuario preservados intactos)  
**Alcance:** Auditoría exclusiva de solo lectura, análisis de brechas e ingeniería de integración para el plan de creación de espacios y galería multimedia real. Sin modificaciones de código Go, SQL, esquemas ni despliegues GCP.

> Evidencia histórica del árbol `main` en `3956d2f`, no estado vigente del checkout ni del frontend actual. Para estado y pruebas actuales, consultar `plan_creacion_espacios_galeria_documentos.md` §8 (reconciliación 2026-10-29).

---

## 1. Estado Actual de Rutas y Navegación del Workspace

### 1.1. Rutas de Espacios (`/anfitrion`)
- **`/anfitrion` / `/anfitrion/espacios`:**
  - *Componente:* [`HostListingsList`](file:///home/nandev/Documentos/projects/espaciGo/frontend/src/features/host-listings/ui/host-listings-list.tsx).
  - *Estado:* Implementado y montado. Incluye filtros de estado (`all`, `borrador`, `activa`, `oculta`), paginación en cliente (`PAGE_SIZE = 6`), tarjeta por espacio con métricas básicas (superficie, capacidad, tarifa base CLP), y acciones rápidas: Publicar (`handlePublish`), Ocultar/Archivar (`handleArchive`), y Eliminar (`handleDelete`).
  - *Estado vacío:* Mensaje informativo con botón de llamado a la acción para crear primer espacio.
- **`/anfitrion/espacios/nuevo`:**
  - *Componente:* [`ListingWizard`](file:///home/nandev/Documentos/projects/espaciGo/frontend/src/features/host-listings/ui/listing-wizard.tsx).
  - *Estado:* Asistente en 5 pasos (1. Tipo y Resumen, 2. Ubicación y Atributos Dinámicos, 3. Tarifas y Reglas, 4. Horarios y Disponibilidad semanal, 5. Confirmación).
  - *Brecha Crítica (Draft Resume):* No admite reanudar un borrador existente vía parámetro (`?draft_id=...` o resume-flow). La creación del borrador (`createDraft`) solo se dispara al final del asistente o paso a paso sin persistencia de estado si el usuario abandona la sesión.
- **`/anfitrion/espacios/[id]/editar`:**
  - *Componente:* [`ListingEditor`](file:///home/nandev/Documentos/projects/espaciGo/frontend/src/features/host-listings/ui/listing-editor.tsx).
  - *Estado:* Editor por pestañas (`info`, `pricing`, `schedule`, `photos`). Permite edición granular con optimismo y control de concurrencia (`If-Match`).
- **`/anfitrion/espacios/[id]`:**
  - *Ruta:* Existe página de detalle pero requiere alineación con la visualización del estado del borrador frente al espacio activo.

### 1.2. Módulo Mi Cuenta (`/mi-cuenta`) y Enlace a Mis Espacios
- *Componentes actuales:* [`ProfileCard`](file:///home/nandev/Documentos/projects/espaciGo/frontend/src/features/account/ui/profile-card.tsx), [`SyntheticPhotoCard`](file:///home/nandev/Documentos/projects/espaciGo/frontend/src/features/account/ui/synthetic-photo-card.tsx), [`SyntheticPayoutCard`](file:///home/nandev/Documentos/projects/espaciGo/frontend/src/features/account/ui/synthetic-payout-card.tsx).
- *Navegación de Cuenta:* [`AccountNavigation`](file:///home/nandev/Documentos/projects/espaciGo/frontend/src/app/(workspace)/mi-cuenta/account-navigation.tsx) contiene enlaces a General (`/mi-cuenta`), Seguridad (`/mi-cuenta/seguridad`), Privacidad (`/mi-cuenta/privacidad`) y Verificaciones (`/mi-cuenta/verificaciones`).
- *Hallazgo y Brecha:* **No existe ningún enlace ni tarjeta hacia "Mis Espacios" ni hacia el panel de anfitrión dentro de `/mi-cuenta`**. Un usuario autenticado no tiene visibilidad de sus propiedades desde el área de cuenta personal, ni llamada a la acción para convertirse en anfitrión si cumple los requisitos.

### 1.3. Lista de Permitidos de Redirección (`return-target.ts`)
- Inspección de [`return-target.ts`](file:///home/nandev/Documentos/projects/espaciGo/frontend/src/features/auth/domain/return-target.ts#L42-L55):
  - Rutas fijas permitidas: `/anfitrion`, `/anfitrion/espacios`, `/anfitrion/espacios/nuevo`, `/anfitrion/reservas`.
  - Expresiones dinámicas permitidas: `/anfitrion/espacios/:id/editar` y `/anfitrion/reservas/:id`.
  - *Brecha:* `/anfitrion/espacios/:id` (vista de detalle/resumen no editor) no está permitida dinámicamente y revertiría al fallback `/` o `/dashboard`.

---

## 2. Autenticación, Roles y Reglas de Negocio en Backend Go

### 2.1. Política de Roles y Elegibilidad de Anfitrión
- **Corrección contrastada del padre:** V74 no otorga ningún rol comercial al registrarse; el primer borrador concede `arrendador` en Go/DB. No asumir un rol inicial `arrendatario` ni exigir `arrendador` en la UI antes de poder crear el primer borrador. Tras crearlo, revalidar sesión/capacidades sin conceder roles desde React.
- **Autorización Go:**
  - Crear borrador (`POST /spaces`): Exige usuario autenticado con sesión activa (`principal.AccountID`). El propietario se asigna automáticamente de la sesión; el espacio nace en estado `borrador`.
  - Publicar (`PUT /spaces/{spaceId}/publication`): Exige rol `arrendador` (`identity.RoleLandlord`). Además, el backend Go valida que la cuenta posea verificación KYC efectiva y zona horaria IANA configurada (de lo contrario retorna `time_zone_required` o `verification_required`).
  - Contenido comercial (`PUT /spaces/{spaceId}/publication-content`): Exige rol `arrendador` y titularidad.

### 2.2. Esquemas Dinámicos de Categorías y Atributos (Fase 7)
- **API Endpoints:**
  - `GET /spaces/categories`: Lista taxonomía activa (código, nombre).
  - `GET /spaces/categories/{categoryCode}/attributes/{version}`: Entrega la definición versionada de atributos (`attributes`: código, label, tipo, opciones, required, order, unit, minimum, maximum, step).
- **Validación Backend Go:**
  - `spaces.Service` valida estrictamente contra la versión de esquema (`attribute_schema_version`). Tipos admitidos: `boolean`, `integer`, `number`, `enum`, `enum_list`.
  - **Fail-Closed vs Fallback:** Si una categoría no existe o su versión de esquema no es reconocida por el backend, responde error `422 validation_error` (fail-closed). No se admite fallback libre de JSON arbitrario en la carga productiva porque corrompe la indexación en catálogo.

### 2.3. Reglas Financieras (CLP) y Horarias
- **Corrección contrastada del padre:** el precio base del borrador/contenido debe ser entero **mayor que 5000** (`internal/spaces/domain.go:141,199`), no solo positivo. Las tarifas adicionales tienen su contrato propio y no deben confundirse con ese mínimo. Título máximo 70 caracteres, descripción mínima 100, normas máximo 250 y dirección máximo 500; área entre 0.01 y 99999999.99 con dos decimales.
- **HTML5 Step Mismatch:** Para precio base usar `min={5001}`, `step={1}` y entero; no imponer saltos de 100 que rechacen importes válidos para Go. Alinear por operación, no mediante un mínimo genérico compartido.
- **Zonas Horarias:** Validación estricta con IANA (por defecto `America/Santiago`). Días normalizados ISO `1..7` (Lunes a Domingo).

---
## 3. Estado de la Galería Multimedia: Ensayos Sintéticos vs Galería Real

### 3.1. Estado Actual en Frontend (`host-listings-openapi-adapter.ts` y `listing-editor.tsx`)
- La pestaña de fotos en [`ListingEditor`](file:///home/nandev/Documentos/projects/espaciGo/frontend/src/features/host-listings/ui/listing-editor.tsx#L1340-L1405) muestra un cartel permanente de "Entorno de ensayo de galería privada" y únicamente invoca `handleUploadSyntheticPhoto`.
- `uploadPhoto` llama a `POST /spaces/{spaceId}/gallery` sin cuerpo, lo cual dispara la generación de un PNG sintético en el backend (`synthetic-png-v1`).

### 3.2. Realidad del Backend Go para Multimedia Real
- El backend Go (`internal/gallery/transport/http/handler.go` y `internal/media/`):
  - **Subida Real:** Soporta `POST /spaces/{spaceId}/gallery` con `Content-Type: multipart/form-data` y campo de archivo `image`.
  - **Límites de Entrada:** Tamaño máximo `MaxInputBytes = 10 MB` (`10 << 20`). Dimensiones máximas `MaxPixels = 25.000.000 px`, lado máximo `12.000 px`.
  - **Formatos Permitidos:** JPEG, PNG y WebP estático. Rechaza terminantemente WebP animado, PNG animado (APNG) y formatos desconocidos (`media.ErrNotSupported`).
  - **Procesamiento:** Decodifica, normaliza la orientación EXIF en JPEG, elimina todos los metadatos y re-codifica únicamente píxeles en WebP: variante `display` (1600px) y variante `thumbnail` (480px).
  - **Límite de Galería:** `MaxPhotosPerSpace = 10` (en `internal/gallery/domain.go` y `internal/media/domain.go`).
    - *Nota de Diseño:* En caso de requerir cuota ampliada a 30 fotos por espacio, el frontend debe manejar el slot físico temporal 31 durante el reemplazo atómico antes de purgar la foto anterior.
  - **Corrección de ruta por el padre:** las variantes privadas están montadas en `/api/v1/spaces/{spaceId}/gallery/{photoId}/variants/{variant}` (`cmd/api/main.go:339`), no `/photos/`. Se descargan mediante BFF `same-origin`, sin Bearer de navegador; registrar/revocar sus Blob URLs. El adaptador ya construye FormData cuando recibe `input.image`; falta conectarlo a UI y conservar/validar `variants` en el mapper (`host-listings-openapi-adapter.ts:130-154,577-611`).

---

## 4. Estructura de la Sección Documentos y Privacidad

- **Descubrimiento en Código Go:**
  - `internal/identity/consent_documents.go`: Gestiona documentos públicos inmutables de consentimiento (`terms.json`, `privacy.json`) con estructura de secciones `{ heading, body }` y verificación de hash SHA-256.
  - `internal/adapters/postgres/documents/local.go`: Gestiona la tabla `documento_privado_sintetico_local` dedicada exclusivamente a los PDFs de contratos digitales bilaterales de reservas (`application/pdf`, cifrado `aes-256-gcm`, máximo 2 MB).
  - **Límite Estricto:** No existe en el backend Go ningún sistema de recolección de documentos arbitrarios ni KYC libre en el flujo de espacios. La verificación se realiza exclusivamente contra el motor de elegibilidad KYC sintético (`/verificaciones`). No inventar subidas de documentos no soportadas.

---

## 5. Referencia Funcional del Legado Angular (`Proyectotitulo`)

- Ubicación: [`hosting-spaces-page.ts`](file:///home/nandev/Documentos/projects/Proyectotitulo/espaciGo/frontend/src/app/features/hosting/hosting-spaces-page.ts) y [`hosting-spaces-facade.ts`](file:///home/nandev/Documentos/projects/Proyectotitulo/espaciGo/frontend/src/app/features/hosting/application/hosting-spaces-facade.ts).
- **Lecciones y Patrones Útiles a Adaptar:**
  - Manejo de formulario monolítico con reactividad ante cambio de categoría (`categoryChanged`) que reinicializa los controles dinámicos de atributos.
  - Selectores en cascada para Región y Comuna según catálogo SUBDERE DPA.
  - Manejo de reordenamiento de fotos y designación de portada.
- **Lo que NO debe copiarse ni resucitarse:**
  - Código de componentes Angular, decoradores `@Component`, `@Injectable`, `FormRecord`, pipes ni archivos `.scss` heredados.
  - La UI de Next.js standalone se basa estrictamente en Tailwind CSS v4, componentes accesibles de `@/components/workspace/ui` y React Hooks estructurados con la arquitectura Hexagonal (Puerto / Adaptador / Fachada / Hook).

---

## 6. Matriz de Componentes y Funcionalidades: Reutilizar / Adaptar / Falta

| Área / Componente | Estado Actual en Next | Acción Requerida | Detalle Técnico y Criterios de Aceptación |
| :--- | :--- | :--- | :--- |
| **Tarjeta "Mis Espacios" en Mi Cuenta** | Inexistente en `/mi-cuenta` | **FALTA** | Crear `MySpacesSummaryCard` en `/mi-cuenta` con resumen de espacios (conteo de borradores/activos), botón hacia `/anfitrion/espacios` y CTA "Publicar nuevo espacio". |
| **Navegación Mi Cuenta (`AccountNavigation`)** | 4 enlaces sin anfitrión | **ADAPTAR** | Agregar enlace secundario o indicador condicional al Panel Anfitrión (`/anfitrion/espacios`) respetando accesibilidad y jerarquía visual. |
| **Lista de Espacios (`HostListingsList`)** | Implementado completo | **REUTILIZAR** | Preservar filtros, paginación, tarjetas de espacio y acciones directas. Conectar badges de estado y links al editor. |
| **Creación Asistida (`ListingWizard`)** | 5 pasos implementados | **ADAPTAR** | Incorporar reanudación de borrador (`?draft_id=...`). Reemplazar creación tardía por autoguardado explícito con botón "Guardar borrador y continuar". |
| **Renderer de Atributos Dinámicos** | Implementado en wizard y editor | **REUTILIZAR / ADAPTAR** | Validar contra esquema `CategoryProfileSchema`. Enforce fail-closed para esquemas desconocidos con mensaje amigable sin pantalla blanca. |
| **Inputs Numéricos CLP** | Funcionales | **ADAPTAR** | Asegurar atributo `step={100}` o entero y validación `Number.isSafeInteger(val) && val >= 1` para erradicar `stepMismatch` de HTML5. |
| **Editor de Horarios y Disponibilidad** | Implementado en pestaña `schedule` | **REUTILIZAR** | Validar zona IANA (`America/Santiago`), franjas no solapadas y días `1..7`. Enviar `If-Match`. |
| **Galería: Subida de Archivos Reales** | Solo botón sintético en UI | **FALTA** | Implementar selector de archivos con arrastrar y soltar (`drag & drop`), validación de MIME (`image/jpeg`, `image/png`, `image/webp`), tamaño `<= 10MB`, y envío `multipart/form-data` con `Idempotency-Key`. |
| **Galería: Previsualización de Imágenes** | Muestra datos planos o SVG | **ADAPTAR** | Conectar cargador de variantes WebP privadas mediante BFF (`same-origin`), gestión con `PrivateBlobUrlRegistry` y revocación en `onDestroy` / cambio de sesión. |
| **Galería: Cuotas y Concurrencia** | Básico (10 fotos) | **ADAPTAR** | Soporte de ordenamiento posicional, designación de portada (`/spaces/{id}/cover`), eliminación y control de cuota máxima. |
| **Control de Concurrencia (`If-Match`)** | Cabecera opcional en adapter, sin prueba de enforcement Go para contenido | **CERRAR BRECHA** | Definir versión de recurso/ETag y CAS real en Go/DB/OpenAPI donde falte; no confundir `attribute_schema_version` con versión de edición ni asumir que la cabecera actual produce 409. |
| **Aislamiento Multi-Tenant y Sesión** | `authFacade` inyectado | **REUTILIZAR** | Al detectar cambio de usuario (A -> B) o error 401/403, purgar inmediatamente borradores locales, cachés de esquemas y URLs de Blob privadas. |

---

## 7. Plan de Integración y Matriz de Pruebas de Verificación

### 7.1. Criterios de Aceptación Técnicos
1. **Persistencia y Resumen de Borradores:**
   - El primer guardado exige los campos comunes y atributos requeridos que hoy valida Go, no únicamente título/categoría: no inventar valores para satisfacer NOT NULL/checks. Tras ese guardado, salir y reanudar por ID desde Mis espacios. Un borrador parcial anterior exigiría un contrato/schema adicional expresamente diseñado, fuera de la integración mínima.
   - La estrategia predeterminada es guardado explícito (`ExplicitSave`) con indicador de estado (Guardado / Guardando / Sin guardar), evitando conflictos de mutaciones ciegas en segundo plano.
2. **Galería Multimedia Real:**
   - Subida real de archivos JPEG, PNG y WebP estático hasta 10 MB mediante `multipart/form-data` con cabecera `Idempotency-Key` (UUIDv4).
   - Rechazo inmediato en cliente y backend de archivos mayores a 10 MB, tipos MIME no permitidos o WebP animado.
   - Creación de Blob URLs seguras mediante `PrivateBlobUrlRegistry` y revocación inmediata al eliminar foto, cambiar de portada o desmontar la vista.
3. **Cero Regresión en Accesibilidad y Diseño:**
   - Cero scroll horizontal (`scrollWidth <= clientWidth`) en resoluciones móviles (390x844) y desktop (1365x900).
   - Preservación estricta de la paleta institucional Tailwind v4 (`brand-palette.css`) sin estilos CSS directos ni resucitación de SCSS de Angular.

### 7.2. Matriz de Pruebas Automatizables
- **Test Case 1 (Cross-Tenant Isolation):** El Usuario A crea un borrador y sube una foto; al cerrar sesión o conmutar al Usuario B, la caché, el registro de Blob URLs y el estado del formulario se purgan a cero sin fugas en memoria.
- **Test Case 2 (Gallery Limits & MIME Validation):** Intentar subir archivo 11 (o 31 en cuota extendida) -> Go responde 422/400 limit reached. Subir PDF o GIF -> Rechazo con mensaje de validación amigable.
- **Test Case 3 (CLP Integer Step & Currency):** Entrada de valor decimal (`15000.50`) -> Bloqueado en input y validación de esquema; solo enteros positivos permitidos.
- **Test Case 4 (Optimistic Concurrency):** Dos pestañas editando el mismo espacio -> La segunda en enviar con versión desactualizada recibe `409 Conflict` (o error de `If-Match`) y solicita recarga sin sobrescritura ciega.
- **Test Case 5 (Fail-Closed on Unknown Category):** Solicitud de esquema para categoría inexistente -> Bloqueo fail-closed controlado con mensaje amigable, sin cuelgues ni renderizado corrupto.
