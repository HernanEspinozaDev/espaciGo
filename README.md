# espaciGo 🏢

**Plataforma Cloud (SaaS) para la Gestión, Arriendo Flexible y Legalización de Espacios Comerciales e Inmobiliarios.**

espaciGo es un Marketplace y software de gestión diseñado para rentabilizar infraestructura comercial ociosa (bodegas, oficinas, espacios para eventos) en Chile. Su objetivo es resolver la ineficiencia operativa, la alta fricción legal y la vulnerabilidad de datos personales y financieros en el mercado de arriendos.

## 🚀 Propuesta de Valor

Flexibilización del arriendo de espacios eliminando la burocracia presencial, garantizando:
- **Seguridad jurídica:** Contratos dinámicos con firma notarial remota en estricto cumplimiento de la Ley 21.461 ("Devuélveme mi Casa").
- **Seguridad financiera:** Retención de pagos (Escrow) mediante Mercado Pago para proteger a ambas partes.
- **Privacidad absoluta (Privacy by Design):** Resguardo y cifrado de documentos sensibles cumpliendo con la nueva Ley de Protección de Datos Personales (Ley 21.719).

## 💻 Arquitectura y Stack Tecnológico

El proyecto opera 100% online mediante una arquitectura **Cloud-Native** orientada a microservicios y basada en contenedores.

- **Frontend:** Next.js (React) para un renderizado rápido desde el servidor (SSR) y un excelente SEO.
- **Backend Core:** Golang (Go) para manejar alta concurrencia mediante Goroutines y orquestar las reglas de negocio de manera eficiente.
- **Base de Datos Operativa (OLTP):** PostgreSQL para asegurar transacciones financieras atómicas (ACID).
- **Almacenamiento Analítico (OLAP):** Google BigQuery para el registro inmutable de logs de auditoría asíncronos.
- **Almacenamiento de Archivos:** Google Cloud Storage para resguardar contratos en PDF y documentos de identidad cifrados.
- **Infraestructura y Despliegue:** Docker, Google Cloud Platform (Cloud Run, Cloud SQL), e integración continua (CI/CD) con GitHub Actions.
- **Microservicio de Contratos Dinámicos:** Generación aislada de contratos PDF (usando Gotenberg/Python) con editores tipo TipTap/Slate.js.
- **Integraciones (APIs):** Mercado Pago (Split Payments) y FirmaVirtual (Notaría API).

## ✨ Funcionalidades Principales

1. **Gestión Dinámica de Contratos:** Generación automatizada, edición colaborativa (arrendador/arrendatario) y renderizado a un PDF inmutable para firma.
2. **Sistema Escrow (Retención de Pagos):** El dinero queda retenido en la plataforma y solo se libera al propietario cuando se cumplen las condiciones de arriendo.
3. **Validación KYC/KYB:** Verificación de identidad y giro comercial (SII) antes de publicar o reservar espacios.
4. **Check-in y Registro Fotográfico:** Respaldo del estado del espacio al inicio y fin de la reserva para la gestión de las garantías.
5. **Auditoría y Derecho al Olvido:** Trazabilidad inmutable de los eventos críticos en el sistema y capacidad del usuario para solicitar la eliminación de su data.

## 👥 Equipo de Trabajo

- **Hernán Espinoza:** Arquitecto Cloud y Backend. (Infraestructura GCP, Docker, CI/CD, PostgreSQL, BigQuery).
- **Erick Silva:** Especialista en Integraciones y Reglas de Negocio. (Consumo de APIs: Mercado Pago, FirmaVirtual, validación tributaria).
- **Anita Marchant:** Desarrolladora Frontend. (Next.js, UI/UX, dashboards, integración de editor de contratos).

---
*Proyecto de Título (TIH184) - INACAP - Septiembre 2026*

## Desarrollo y planificación vigentes

Este README conserva la descripción académica original. La autoridad actual para requisitos, arquitectura, orden de capas y ejecución está en [`planning/README.md`](planning/README.md) y [`planning/contexto_para_agentes.md`](planning/contexto_para_agentes.md). El backlog operativo se migró a GitHub Issues y al Project `EspaciGo — Desarrollo`; consulta [`planning/migracion_github_projects.md`](planning/migracion_github_projects.md) y [`planning/ejecucion_con_github_projects.md`](planning/ejecucion_con_github_projects.md). Este puntero no ratifica ni amplía requisitos del producto.
