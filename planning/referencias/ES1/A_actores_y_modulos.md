# Actores del Sistema y Módulos con Requerimientos Funcionales
## Sistema: EspaciGo — Marketplace SaaS B2B2C de Espacios Comerciales

> **Alcance del documento:** este anexo identifica los actores del sistema y organiza el catálogo de requerimientos funcionales en módulos. Los 236 requerimientos funcionales vigentes (RQF-001 a RQF-236) quedan agrupados en 11 módulos, sin duplicar ni renumerar ningún identificador.

**Convención de nomenclatura**
- `RQF-###` = Requerimiento Funcional (catálogo completo en el Anexo B).
- `RNF-###` = Requerimiento No Funcional (catálogo completo en el Anexo C).
- `CU-##` = Caso de Uso (especificación completa en el Anexo D).
- `HU##` = Historia de Usuario (especificación completa en el Anexo E).

**Criterios usados para construir los módulos**
1. Un módulo agrupa requerimientos funcionales que persiguen un mismo objetivo funcional del sistema.
2. Los RF se asignan a **un único módulo**, sin duplicar IDs, y cubren el catálogo completo (185/185).
3. Los módulos se ordenan según el ciclo de vida del negocio (identidad → catálogo → reserva → pago → contrato → operación → post-servicio → gobierno).
4. Los módulos **no son requerimientos**: son agrupaciones que existen para organizar los diagramas y la especificación de casos de uso.
5. Los requerimientos de tipo técnico, de infraestructura o de calidad (cifrado de contraseñas, inmutabilidad de logs, tokenización de tarjetas, escalabilidad) **no están aquí**: pertenecen al catálogo de RNF y **no generan casos de uso**.

## 1. Identificación de Actores

Se consideran actores **únicamente entidades externas** al sistema (personas/roles y sistemas o servicios de terceros). Los componentes internos de la arquitectura (frontend Next.js, API en Go, contenedores, colas, repositorios de datos, controladores) **no son actores**.

### 1.1 Actores Primarios (roles humanos)

| Actor | Alias UML | Descripción | Estado requerido |
|---|---|---|---|
| **Visitante** | `Visitante` | Usuario anónimo, sin cuenta registrada. Puede explorar el catálogo público (búsqueda, mapa, detalle y reseñas) pero no puede interactuar transaccionalmente. | Sin cuenta |
| **Usuario Registrado** | `UsuarioRegistrado` | Posee cuenta con correo verificado, pero aún sin validación de identidad. Puede iniciar sesión, gestionar su perfil, administrar sus datos y solicitar validaciones de identidad. | Correo verificado |
| **Arrendador** | `Arrendador` | Usuario con identidad verificada (KYC o KYB) que publica y administra espacios, gestiona su calendario, aprueba o rechaza solicitudes de reserva, ejecuta check-in/check-out y reclama daños. Corresponde al "Propietario" que mencionan las Historias de Usuario. | KYC/KYB aprobado |
| **Arrendatario** | `Arrendatario` | Usuario con identidad verificada (KYC o KYB) que busca espacios, cotiza, reserva, paga, firma, realiza check-in/check-out, emite reseñas y presenta descargos. | KYC/KYB aprobado |
| **Administrador** | `Administrador` | Operador interno de EspaciGo. Revisa validaciones de identidad fallidas, arbitra disputas, modera reseñas, bloquea cuentas, genera reportes y audita la trazabilidad. Accede al backoffice. | Rol interno de plataforma |

> **Nota de compatibilidad terminológica:** las Historias de Usuario usan "Propietario"; los casos de uso y requerimientos usan "Arrendador". Se mantiene `Arrendador` como nombre oficial del actor y "Propietario" como sinónimo reconocido en las HU.
>
> **Nota de roles no excluyentes:** un mismo usuario puede actuar simultáneamente como Arrendador y como Arrendatario; los roles no son mutuamente excluyentes.

### 1.2 Actores Secundarios (sistemas y servicios externos)

| Actor Externo | Tipo | Rol en el sistema | Módulos donde participa |
|---|---|---|---|
| **Registro Civil** | Servicio externo (API) | Entrega la vigencia de la Cédula de Identidad durante la validación KYC. | M03 |
| **SII** (Servicio de Impuestos Internos) | Servicio externo (API) | Entrega el inicio de actividades y el giro comercial de la empresa durante la validación KYB. | M03 |
| **Mercado Pago** | Servicio externo (pasarela de pagos) | Procesa el pago retenido (Escrow), ejecuta la pre-autorización de garantía, los reembolsos, el cobro de la garantía y la transferencia de fondos (Payout). | M06, M10 |
| **FirmaVirtual** | Servicio externo (firma electrónica) | Recibe el contrato PDF y devuelve los enlaces de firma y las notificaciones de firma completada. | M07 |

### 1.3 Generalización de actores

La relación de especialización se modela porque un actor especializado **puede ejecutar todo lo que ejecuta el actor general y además funcionalidades propias** (herencia de capacidades):

```text
Visitante  ◁── UsuarioRegistrado ◁── Arrendador
                                 ◁── Arrendatario
Administrador (actor independiente, sin relación de herencia con los anteriores)
```

Justificación de cada generalización:

| Relación | Justificación funcional |
|---|---|
| `UsuarioRegistrado --|> Visitante` | Todo usuario registrado puede hacer lo que hace un visitante (buscar, filtrar, ver detalle, ver reseñas), y además autenticarse y operar con cuenta. |
| `Arrendador --|> UsuarioRegistrado` | El arrendador debe poder iniciar sesión, gestionar perfil, validar identidad y usar el catálogo: ejecuta todo lo del usuario registrado más la gestión de publicaciones y solicitudes. |
| `Arrendatario --|> UsuarioRegistrado` | Mismo razonamiento: hereda capacidades del usuario registrado y agrega reserva, pago, contrato, check-in/out y reseñas. |
| `Administrador` | **No se generaliza.** No es un usuario del marketplace: opera el backoffice con permisos propios (validaciones manuales, arbitraje, moderación, reportes, auditoría) y no ejecuta los flujos comerciales de arrendador ni de arrendatario. |

### 1.4 Exclusiones explícitas de actores (justificación)

| Elemento | Por qué NO es actor |
|---|---|
| **Google BigQuery / Data Warehouse** | Es un repositorio de datos consumido internamente por el sistema para cumplir el RNF-017 (inmutabilidad de logs). No existe un requerimiento funcional de interacción iniciada por una entidad externa con un objetivo observable, por lo que **no genera casos de uso ni actor**. |
| **Registro Civil / SII / Mercado Pago / FirmaVirtual** | **Sí son actores** (sistemas de terceros, fuera del límite del sistema), porque participan en casos de uso iniciados por un actor humano y respaldados por RF explícitos (RQF-044, RQF-051, RQF-114–118, RQF-133–139). |
| **Frontend Next.js, API Go, Docker, Cloud Run, controladores, servicios internos** | Componentes internos de la solución: se describen en la Arquitectura TI, no son actores. |
| **Encriptación de contraseñas (bcrypt)** | Es el `RNF-013` (Seguridad / Confidencialidad). **No es caso de uso ni actor**, porque es una restricción de calidad y no una funcionalidad solicitada por una entidad externa. |

## 2. Módulos del Sistema y Requerimientos Funcionales (236 RF)

> Cada módulo declara los rangos de requerimientos funcionales que agrupa, sus objetivos, sus actores y sus casos de uso; **el texto completo de los 236 requerimientos está en el Anexo B**, que es el catálogo de referencia.

La Figura A.1 presenta los 11 módulos agrupados por etapa del ciclo de vida del negocio y el total de requerimientos funcionales de cada uno.

![Módulos del sistema agrupados por etapa del ciclo de vida del negocio](imagenes/figura-a-etapas-modulos.png){height=15.9cm}

### M01 — Autenticación y Gestión de Cuenta
**RF del módulo:** RQF-001 a RQF-023 (23 RF) + complementarios RQF-186 a RQF-188 (3 RF) + RQF-213 a RQF-218 (6 RF) = **32 RF**
**Objetivo del módulo:** permitir que un visitante se convierta en usuario registrado verificado aceptando los términos y condiciones, y que pueda autenticarse, gestionar su contraseña (cambiarla y recuperarla) y cerrar sesión de forma segura.
**Actores primarios:** Visitante, Usuario Registrado.
**Actores externos:** —
**Casos de uso asociados:** CU-01, CU-02, CU-03, CU-04, CU-05, CU-06, CU-50.

### M02 — Perfil y Privacidad del Usuario
**RF del módulo:** RQF-024 a RQF-037 (14 RF) + complementarios RQF-189 a RQF-191 (3 RF) = **17 RF**
**Objetivo del módulo:** permitir al usuario mantener sus datos personales y de cobro (incluyendo la gestión completa de su cuenta bancaria) y ejercer su derecho de eliminación de datos personales.
**Actores primarios:** Usuario Registrado (heredan Arrendador y Arrendatario).
**Actores externos:** —
**Casos de uso asociados:** CU-07, CU-08, CU-09.

### M03 — Verificación de Identidad (KYC / KYB)
**RF del módulo:** RQF-038 a RQF-059 (22 RF) + complementarios RQF-192 a RQF-194 (3 RF) + RQF-219, RQF-220 (2 RF) = **27 RF**
**Objetivo del módulo:** acreditar la identidad de personas naturales (KYC) y de empresas (KYB) para habilitar la operación transaccional, permitiendo consultar el estado de la validación y resolviendo manualmente los casos que fallan.
**Actores primarios:** Usuario Registrado, Administrador.
**Actores externos:** Registro Civil, SII.
**Casos de uso asociados:** CU-10 (abstracto), CU-11, CU-12, CU-13, CU-14.

### M04 — Gestión de Publicaciones
**RF del módulo:** RQF-060 a RQF-093 (34 RF) + complementarios RQF-195 a RQF-198 (4 RF) + RQF-221 a RQF-226 (6 RF) = **44 RF**
**Objetivo del módulo:** que el arrendador verificado registre, publique, ilustre, calendarice y mantenga vigente y actualizada la oferta de espacios del marketplace.
**Actores primarios:** Arrendador (la consulta pública del catálogo se modela en M05).
**Actores externos:** —
**Casos de uso asociados:** CU-15, CU-16, CU-17, CU-18.

### M05 — Búsqueda y Cotización de Espacios
**RF del módulo:** RQF-094 a RQF-107 (14 RF) = **14 RF**
**Objetivo del módulo:** que cualquier visitante encuentre espacios disponibles según ubicación, precio, tipo y fechas, y que el arrendatario conozca el desglose del costo antes de reservar.
**Actores primarios:** Visitante, Arrendatario (el Usuario Registrado hereda la capacidad de búsqueda).
**Actores externos:** —
**Casos de uso asociados:** CU-19, CU-20, CU-21.

### M06 — Reservas y Pagos (Escrow)
**RF del módulo:** RQF-108 a RQF-129 (22 RF) + complementarios RQF-199 a RQF-201 (3 RF) + RQF-227 a RQF-231 (5 RF) = **30 RF**
**Objetivo del módulo:** registrar la reserva sin colisiones, permitir el seguimiento y la cancelación de las reservas del usuario, cobrar el pago retenido en custodia (Escrow), bloquear la garantía y obtener la aprobación o el rechazo del arrendador.
**Actores primarios:** Arrendatario, Arrendador.
**Actores externos:** Mercado Pago.
**Casos de uso asociados:** CU-22, CU-23, CU-24, CU-25, CU-26, CU-27, CU-28, CU-47, CU-51.

### M07 — Contratos y Firma Electrónica
**RF del módulo:** RQF-130 a RQF-142 (13 RF) + complementario RQF-202 (1 RF) = **14 RF**
**Objetivo del módulo:** generar el contrato con los datos legales de las partes y del inmueble, gestionarlo ante el proveedor de firma electrónica (incluyendo el rechazo de firma) y resguardar el documento firmado.
**Actores primarios:** Arrendador, Arrendatario.
**Actores externos:** FirmaVirtual.
**Casos de uso asociados:** CU-29, CU-30, CU-31, CU-32.

### M08 — Check-in y Check-out
**RF del módulo:** RQF-143 a RQF-152 (10 RF) + complementarios RQF-203 a RQF-206 (4 RF) = **14 RF**
**Objetivo del módulo:** documentar con evidencia fotográfica obligatoria, fecha y ubicación la entrega y la devolución del espacio, incluyendo la confirmación de recepción del arrendador, y habilitar el estado operativo de la reserva.
**Actores primarios:** Arrendatario, Arrendador.
**Actores externos:** —
**Casos de uso asociados:** CU-33, CU-34, CU-48.

### M09 — Comunicación y Reputación
**RF del módulo:** RQF-153 a RQF-158 (6 RF) + complementario RQF-207 (1 RF) + RQF-232 a RQF-235 (4 RF) = **11 RF**
**Objetivo del módulo:** permitir la calificación y reseña de los espacios, el reporte de reseñas que incumplen las reglas y la comunicación trazable entre las partes durante la reserva.
**Actores primarios:** Arrendatario, Arrendador, Visitante (consulta de reseñas).
**Actores externos:** —
**Casos de uso asociados:** CU-35, CU-36, CU-37, CU-38, CU-49.

### M10 — Disputas, Payout y Facturación
**RF del módulo:** RQF-159 a RQF-177 (19 RF) + complementarios RQF-208 a RQF-211 (4 RF) = **23 RF**
**Objetivo del módulo:** resolver el cierre económico del servicio: liberar los fondos al arrendador, liberar la garantía en el flujo sin reclamos o ejecutarla según el fallo, resolver reclamos por daños y emitir y enviar la boleta de la comisión.
**Actores primarios:** Arrendador, Arrendatario, Administrador.
**Actores externos:** Mercado Pago.
**Casos de uso asociados:** CU-39, CU-40, CU-41, CU-42.

### M11 — Administración y Auditoría
**RF del módulo:** RQF-178 a RQF-185 (8 RF) + complementarios RQF-212 y RQF-236 (2 RF) = **10 RF**
**Objetivo del módulo:** entregar al administrador las herramientas de gobierno de la plataforma: búsqueda y bloqueo de cuentas, moderación, reportes y consulta/exportación de la trazabilidad.
**Actores primarios:** Administrador.
**Actores externos:** — (la inmutabilidad del repositorio de logs corresponde al RNF-017, no a un actor).
**Casos de uso asociados:** CU-43, CU-44, CU-45, CU-46, CU-52.

## 3. Resumen: Actores por Módulo

La Figura A.2 relaciona cada actor primario con los módulos en los que participa, y la Figura A.3 hace lo propio con los actores secundarios.

![Actores primarios y módulos en los que participan](imagenes/figura-a-actores-primarios.png){height=15.8cm}

![Actores secundarios y módulos en los que participan](imagenes/figura-a-actores-secundarios.png){height=9.8cm}

| Módulo | RF base | RF complementarios | Total RF | Actores Primarios | Actores Externos | Casos de Uso |
|---|---|---|---|---|---|---|
| M01 — Autenticación y Gestión de Cuenta | 001–023 | 186–188, 213–218 | 32 | Visitante, Usuario Registrado | — | CU-01 … CU-06, CU-50 |
| M02 — Perfil y Privacidad del Usuario | 024–037 | 189–191 | 17 | Usuario Registrado | — | CU-07 … CU-09 |
| M03 — Verificación de Identidad (KYC/KYB) | 038–059 | 192–194, 219–220 | 27 | Usuario Registrado, Administrador | Registro Civil, SII | CU-10 … CU-14 |
| M04 — Gestión de Publicaciones | 060–093 | 195–198, 221–226 | 44 | Arrendador | — | CU-15 … CU-18 |
| M05 — Búsqueda y Cotización de Espacios | 094–107 | — | 14 | Visitante, Arrendatario | — | CU-19 … CU-21 |
| M06 — Reservas y Pagos (Escrow) | 108–129 | 199–201, 227–231 | 30 | Arrendatario, Arrendador | Mercado Pago | CU-22 … CU-28, CU-47, CU-51 |
| M07 — Contratos y Firma Electrónica | 130–142 | 202 | 14 | Arrendador, Arrendatario | FirmaVirtual | CU-29 … CU-32 |
| M08 — Check-in y Check-out | 143–152 | 203–206 | 14 | Arrendatario, Arrendador | — | CU-33, CU-34, CU-48 |
| M09 — Comunicación y Reputación | 153–158 | 207, 232–235 | 11 | Arrendatario, Arrendador, Visitante | — | CU-35 … CU-38, CU-49 |
| M10 — Disputas, Payout y Facturación | 159–177 | 208–211 | 23 | Arrendador, Arrendatario, Administrador | Mercado Pago | CU-39 … CU-42 |
| M11 — Administración y Auditoría | 178–185 | 212, 236 | 10 | Administrador | — | CU-43 … CU-46, CU-52 |
| **Total** | **185** | **51** | **236** | — | — | **52** |

## 4. Estadísticas del Catálogo

| Métrica | Valor |
|---|---|
| Total de Requerimientos Funcionales | **236** (185 base, RQF-001 – RQF-185, más 51 complementarios, RQF-186 – RQF-236), sin duplicados |
| Total de Módulos funcionales | **11** |
| Total de Requerimientos No Funcionales | **43** (catálogo completo en el Anexo C, clasificados en 11 categorías con al menos 3 requerimientos cada una) |
| Total de Casos de Uso | **52** (CU-01 – CU-52, especificados en el Anexo D) |
| Actores Primarios | **5** (Visitante, Usuario Registrado, Arrendador, Arrendatario, Administrador) |
| Actores Secundarios (sistemas externos) | **4** (Registro Civil, SII, Mercado Pago, FirmaVirtual) |
| Módulo con más RF | M04 — Gestión de Publicaciones (44 RF) |
| Módulo con menos RF | M11 — Administración y Auditoría (10 RF) |
| Promedio de RF por módulo | 21,5 |

**Distribución de RF por módulo**

```text
M01 Autenticación y Cuenta      ████████████████████████████████ 32
M02 Perfil y Privacidad         █████████████████ 17
M03 Identidad KYC/KYB           ███████████████████████████ 27
M04 Publicaciones               ████████████████████████████████████████████ 44
M05 Búsqueda y Cotización       ██████████████ 14
M06 Reservas y Pagos            ██████████████████████████████ 30
M07 Contratos y Firma           ██████████████ 14
M08 Check-in y Check-out        ██████████████ 14
M09 Comunicación y Reputación   ███████████ 11
M10 Disputas, Payout y Boleta   ███████████████████████ 23
M11 Administración y Auditoría  ██████████ 10
                                                     Total: 236
```
