import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { StatusPanel } from "@/components/ui/status-panel";

const examples = [
  { label: "Publicación de ejemplo", value: "Vista de muestra" },
  { label: "Fuente", value: "Contenido local" },
  { label: "Conexión", value: "Desactivada" },
];

const requests = [
  {
    title: "Sala de ideas",
    detail: "Solicitud ilustrativa · hoy",
    state: "Pendiente",
  },
  {
    title: "Taller creativo",
    detail: "Solicitud ilustrativa · ayer",
    state: "En revisión",
  },
  {
    title: "Estudio abierto",
    detail: "Solicitud ilustrativa · esta semana",
    state: "Borrador",
  },
];

export default function WorkspaceOverview() {
  return (
    <main className="workspace-page" id="main-content" tabIndex={-1}>
      <header className="workspace-page-heading">
        <div>
          <p className="eyebrow">Resumen del espacio de trabajo</p>
          <h1>Hola, bienvenida.</h1>
          <p className="workspace-page-heading__description">
            Una vista de muestra para presentar el shell de administración.
          </p>
        </div>
        <Badge tone="warning">Datos sintéticos</Badge>
      </header>

      <StatusPanel
        className="workspace-notice"
        kind="restricted"
        title="Modo de demostración"
      >
        No hay sesión, API ni datos persistidos conectados en esta fase.
      </StatusPanel>

      <section aria-label="Indicadores ilustrativos" className="metric-grid">
        {examples.map((item, index) => (
          <Card className="metric-card" density="admin" key={item.label}>
            <p>{item.label}</p>
            <strong>{item.value}</strong>
            <span aria-hidden="true" className="metric-card__index">
              0{index + 1}
            </span>
          </Card>
        ))}
      </section>

      <section
        aria-labelledby="spaces-panel-title"
        className="workspace-section"
        id="espacios"
      >
        <div className="workspace-section__heading">
          <div>
            <p className="eyebrow">Contenido demostrativo</p>
            <h2 id="spaces-panel-title">Tus espacios</h2>
          </div>
          <span className="section-count">Sin datos reales</span>
        </div>
        <Card className="workspace-empty" density="admin">
          <StatusPanel kind="empty" title="Todavía no hay espacios conectados">
            Este panel es una presentación visual; no crea ni modifica
            publicaciones.
          </StatusPanel>
        </Card>
      </section>

      <section
        aria-labelledby="requests-title"
        className="workspace-section"
        id="solicitudes"
      >
        <div className="workspace-section__heading">
          <div>
            <p className="eyebrow">Actividad visual</p>
            <h2 id="requests-title">Solicitudes recientes</h2>
          </div>
          <span className="section-count">Ejemplos sin acción</span>
        </div>
        <ul className="request-list">
          {requests.map((request) => (
            <li className="request-row" key={request.title}>
              <span aria-hidden="true" className="request-row__marker" />
              <div className="request-row__copy">
                <h3>{request.title}</h3>
                <p>{request.detail}</p>
              </div>
              <Badge tone="neutral">{request.state}</Badge>
              <span className="request-row__disclaimer">Solo muestra</span>
            </li>
          ))}
        </ul>
      </section>
    </main>
  );
}
