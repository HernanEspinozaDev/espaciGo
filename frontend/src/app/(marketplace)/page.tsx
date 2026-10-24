import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";

const spaces = [
  {
    name: "Sala clara para reunirse",
    place: "Ñuñoa · Santiago",
    kind: "Reuniones",
    art: "space-art--sunroom",
    initials: "01",
  },
  {
    name: "Taller abierto y flexible",
    place: "Barrio Italia · Santiago",
    kind: "Talleres",
    art: "space-art--workshop",
    initials: "02",
  },
  {
    name: "Estudio para crear en calma",
    place: "Providencia · Santiago",
    kind: "Producción",
    art: "space-art--studio",
    initials: "03",
  },
];

const categories = ["Reuniones", "Talleres", "Producción", "Celebraciones"];

export default function MarketplaceHome() {
  return (
    <main className="marketplace-main" id="main-content" tabIndex={-1}>
      <section aria-labelledby="hero-title" className="hero-section page-wrap">
        <div className="hero-copy">
          <Badge className="demo-badge" tone="info">
            Exploración visual · datos de ejemplo
          </Badge>
          <p className="eyebrow">Un lugar para cada gran idea</p>
          <h1 id="hero-title">
            Encuentra el espacio que <em>te inspira.</em>
          </h1>
          <p className="hero-lede">
            Salas, talleres y rincones con personalidad para reunir a tu equipo,
            crear algo nuevo o celebrar un momento especial.
          </p>
          <div className="hero-actions">
            <Link className="button button--primary" href="#espacios">
              Explorar espacios
            </Link>
            <Link className="text-link" href="#como-funciona">
              Cómo funciona <span aria-hidden="true">↗</span>
            </Link>
          </div>
          <p className="hero-note">
            Esta vista es una maqueta local: no muestra anuncios ni
            disponibilidad reales.
          </p>
        </div>
        <div aria-hidden="true" className="hero-art">
          <div className="hero-art__sun" />
          <div className="hero-art__frame">
            <div className="hero-art__window">
              <span />
              <span />
              <span />
            </div>
            <div className="hero-art__table" />
            <div className="hero-art__plant">
              <i />
              <i />
              <i />
            </div>
          </div>
          <div className="hero-art__caption">
            <span className="hero-art__dot" />
            Espacios con intención
          </div>
        </div>
      </section>

      <section aria-labelledby="categories-title" className="category-section">
        <div className="page-wrap">
          <div className="section-heading section-heading--compact">
            <div>
              <p className="eyebrow">Ideas para empezar</p>
              <h2 id="categories-title">¿Qué tienes en mente?</h2>
            </div>
            <p>Una selección ilustrativa para recorrer el diseño.</p>
          </div>
          <nav aria-label="Categorías de ejemplo" className="category-list">
            {categories.map((category, index) => (
              <Link className="category-chip" href="#espacios" key={category}>
                <span aria-hidden="true" className="category-chip__number">
                  0{index + 1}
                </span>
                {category}
                <span aria-hidden="true" className="category-chip__arrow">
                  ↗
                </span>
              </Link>
            ))}
          </nav>
        </div>
      </section>

      <section
        aria-labelledby="spaces-title"
        className="spaces-section page-wrap"
        id="espacios"
      >
        <div className="section-heading">
          <div>
            <p className="eyebrow">Selección de muestra</p>
            <h2 id="spaces-title">Lugares que abren posibilidades</h2>
          </div>
          <span className="section-count">03 espacios ilustrativos</span>
        </div>
        <div className="space-grid">
          {spaces.map((space) => (
            <Card className="space-card" key={space.initials}>
              <div aria-hidden="true" className={`space-art ${space.art}`}>
                <span className="space-art__index">{space.initials}</span>
                <span className="space-art__shape" />
                <span className="space-art__shape space-art__shape--small" />
              </div>
              <div className="space-card__content">
                <div className="space-card__meta">
                  <Badge tone="neutral">{space.kind}</Badge>
                  <span>{space.place}</span>
                </div>
                <h3>{space.name}</h3>
                <p>Un espacio de ejemplo para esta demostración visual.</p>
              </div>
            </Card>
          ))}
        </div>
      </section>

      <section
        aria-labelledby="how-title"
        className="how-section"
        id="como-funciona"
      >
        <div className="page-wrap how-section__inner">
          <div>
            <p className="eyebrow">Sencillo desde el primer paso</p>
            <h2 id="how-title">Un buen espacio cambia la energía.</h2>
          </div>
          <ol className="steps-list">
            <li>
              <span>01</span>
              <div>
                <h3>Descubre</h3>
                <p>Explora opciones pensadas para distintos momentos.</p>
              </div>
            </li>
            <li>
              <span>02</span>
              <div>
                <h3>Compara</h3>
                <p>Revisa detalles claros antes de elegir.</p>
              </div>
            </li>
            <li>
              <span>03</span>
              <div>
                <h3>Encuentra tu lugar</h3>
                <p>La experiencia real se incorporará en fases posteriores.</p>
              </div>
            </li>
          </ol>
        </div>
      </section>
    </main>
  );
}
