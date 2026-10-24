import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { StatusPanel } from "@/components/ui/status-panel";
import { TextField } from "@/components/ui/text-field";

const stateExamples = [
  {
    kind: "loading",
    title: "Cargando espacios",
    text: "Estamos preparando la vista de ejemplo.",
  },
  {
    kind: "empty",
    title: "Aún no hay resultados",
    text: "Prueba más adelante con otra selección.",
  },
  {
    kind: "error",
    title: "No pudimos completar la vista",
    text: "Este estado no se conecta a un servicio.",
  },
  {
    kind: "success",
    title: "Cambios guardados",
    text: "Ejemplo visual; no se guardó ningún dato.",
  },
  {
    kind: "denied",
    title: "Acceso no disponible",
    text: "Las capacidades se conectarán en otra fase.",
  },
  {
    kind: "restricted",
    title: "Vista restringida",
    text: "El estado es ilustrativo y no representa una cuenta.",
  },
  {
    kind: "offline",
    title: "Sin conexión al servicio",
    text: "La maqueta funciona con contenido local.",
  },
] as const;

const swatches = [
  { token: "Marca", name: "--color-brand", value: "#8f3c2b" },
  { token: "Lienzo", name: "--color-canvas", value: "#f6f4ee" },
  { token: "Superficie", name: "--color-surface", value: "#ffffff" },
  { token: "Texto", name: "--color-ink", value: "#252923" },
  { token: "Borde", name: "--color-line", value: "#ddd9cf" },
  { token: "Éxito", name: "--color-success", value: "#155f3c" },
  { token: "Aviso", name: "--color-warning", value: "#785300" },
  { token: "Error", name: "--color-error", value: "#9c2d25" },
];

export default function DesignSystemPage() {
  return (
    <main className="design-page page-wrap" id="main-content" tabIndex={-1}>
      <header className="design-page__heading">
        <Badge tone="info">Demo local · sin API</Badge>
        <p className="eyebrow">Fase 2 · lenguaje visual compartido</p>
        <h1>Una marca, dos formas de trabajar.</h1>
        <p>
          Tokens y componentes comunes para la exploración pública y la gestión
          de espacios. Toda la información de esta página es ilustrativa.
        </p>
      </header>

      <section aria-labelledby="tokens-title" className="design-section">
        <div className="section-heading section-heading--compact">
          <div>
            <p className="eyebrow">Base visual</p>
            <h2 id="tokens-title">Color y superficies</h2>
          </div>
          <p>Contraste comprobado por pares de texto y fondo.</p>
        </div>
        <div className="swatch-grid">
          {swatches.map((swatch) => (
            <Card className="swatch-card" key={swatch.name}>
              <span
                aria-hidden="true"
                className="swatch-card__color"
                style={{ backgroundColor: `var(${swatch.name})` }}
              />
              <div>
                <strong>{swatch.token}</strong>
                <code>{swatch.name}</code>
                <span>{swatch.value}</span>
              </div>
            </Card>
          ))}
        </div>
        <Card className="token-notes">
          <p>
            <strong>Escala:</strong> spacing 4 px–96 px · radios 8 px–28 px ·
            elevación sutil en superficies.
          </p>
          <p>
            <strong>Tipografía:</strong> sistema sans para producto y serif
            editorial para titulares públicos.
          </p>
          <p>
            <strong>Densidades:</strong> marketplace amplio; panel
            administrativo compacto sin reducir el tamaño mínimo de controles.
          </p>
        </Card>
      </section>

      <section aria-labelledby="components-title" className="design-section">
        <div className="section-heading section-heading--compact">
          <div>
            <p className="eyebrow">Piezas comunes</p>
            <h2 id="components-title">Componentes base</h2>
          </div>
        </div>
        <div className="component-grid">
          <Card className="component-card">
            <h3>Acciones</h3>
            <div className="component-row">
              <Button type="button">Acción principal</Button>
              <Button type="button" variant="secondary">
                Secundaria
              </Button>
              <Button type="button" variant="quiet">
                Discreta
              </Button>
            </div>
          </Card>
          <Card className="component-card">
            <h3>Etiquetas</h3>
            <div className="component-row component-row--wrap">
              <Badge tone="success">Disponible</Badge>
              <Badge tone="warning">En revisión</Badge>
              <Badge tone="error">No disponible</Badge>
              <Badge tone="info">Informativo</Badge>
            </div>
          </Card>
          <Card className="component-card">
            <h3>Campo con etiqueta persistente</h3>
            <TextField
              id="demo-zone"
              label="Zona de referencia"
              placeholder="Ej. Ñuñoa"
              hint="Campo ilustrativo; no envía información."
            />
          </Card>
        </div>
      </section>

      <section aria-labelledby="states-title" className="design-section">
        <div className="section-heading section-heading--compact">
          <div>
            <p className="eyebrow">Feedback coherente</p>
            <h2 id="states-title">Estados comunes</h2>
          </div>
        </div>
        <div className="state-grid">
          {stateExamples.map((state) => (
            <StatusPanel key={state.kind} kind={state.kind} title={state.title}>
              {state.text}
            </StatusPanel>
          ))}
        </div>
      </section>

      <section aria-labelledby="density-title" className="design-section">
        <div className="section-heading section-heading--compact">
          <div>
            <p className="eyebrow">Adaptación por contexto</p>
            <h2 id="density-title">Densidades de interfaz</h2>
          </div>
        </div>
        <div className="density-grid">
          <Card className="density-card density-card--public">
            <span className="eyebrow">Pública</span>
            <h3>Descubrir sin prisa</h3>
            <p>Más espacio entre bloques, copy editorial y foco en explorar.</p>
          </Card>
          <Card className="density-card density-card--admin" density="admin">
            <span className="eyebrow">Administrativa</span>
            <h3>Información a mano</h3>
            <p>
              Jerarquía compacta, navegación persistente y controles táctiles
              cómodos.
            </p>
          </Card>
        </div>
      </section>
    </main>
  );
}
