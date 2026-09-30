# Referencias de contexto incluidas en el repositorio

Estos archivos son snapshots para que Hermes y otros agentes tengan acceso a los requisitos sin leer el repositorio académico vecino. ES1 permanece congelada; copiar aquí sus anexos no la modifica. La versión original indicada en la tabla sigue siendo la procedencia académica.

Los hashes SHA-256 y la comprobación de igualdad byte a byte al copiar están en [manifiesto](manifesto_snapshots.md).

`contexto_para_agentes.md` y `ejecucion_con_hermes.md` explican qué fuente prevalece, cómo cargar contexto y cómo ejecutar tarjetas.

## ES1 congelada

| Snapshot | Fuente original |
| --- | --- |
| `ES1/A_actores_y_modulos.md` | `Informes/ES1PT/docx/anexos/A_actores_y_modulos.md` |
| `ES1/B_requerimientos_funcionales.md` | `Informes/ES1PT/docx/anexos/B_requerimientos_funcionales.md` |
| `ES1/C_requerimientos_no_funcionales.md` | `Informes/ES1PT/docx/anexos/C_requerimientos_no_funcionales.md` |
| `ES1/D_casos_de_uso.md` | `Informes/ES1PT/docx/anexos/D_casos_de_uso.md` |
| `ES1/E_historias_de_usuario.md` | `Informes/ES1PT/docx/anexos/E_historias_de_usuario.md` |

## ES2 vigente al corte de planificación

| Snapshot | Fuente original |
| --- | --- |
| `ES2/propuesta_backend_final.md` | `Informes/ES2PT/investigacion/propuesta_backend_final.md` v1.2, 29-09-2026 |
| `ES2/B_diccionario_datos.md` | `Informes/ES2PT/anexos/B_diccionario_datos.md` v2.0, 29-09-2026 |
| `ES2/C_casos_de_prueba.md` | `Informes/ES2PT/anexos/C_casos_de_prueba.md` |
| `ES2/03_00_arquitectura.md` | `Informes/ES2PT/secciones/03_00_arquitectura.md` |
| `ES2/03_03_componentes.md` | `Informes/ES2PT/secciones/03_03_componentes.md` |
| `ES2/03_04_datos.md` | `Informes/ES2PT/secciones/03_04_datos.md` |
| `ES2/03_06_infraestructura.md` | `Informes/ES2PT/secciones/03_06_infraestructura.md` |
| `ES2/contexto.md` | `Informes/ES2PT/contexto.md` |
| `ES2/pendientes.md` | `Informes/ES2PT/pendientes.md` |
| `ES2/matriz_trazabilidad_es2.md` | `Informes/ES2PT/investigacion/matriz_trazabilidad_es2.md` |

## Entorno de desarrollo

El contexto de herramientas compartido por el usuario está resumido en `../entorno_de_desarrollo.md`; no se replica entre las referencias de requisitos porque no es especificación del producto.

## Mantenimiento

- ES1 snapshots no se editan. Si se necesita una interpretación, añadir nota separada con ID de requisito.
- ES2 es evolutiva: cuando los originales cambien, sincronizar el snapshot y actualizar el corte/hash, dejando diferencias y tickets afectados.
- El backlog debe citar IDs exactos; el snapshot permite abrir la ficha completa. Un hallazgo externo no actualiza silenciosamente requisito o arquitectura.
