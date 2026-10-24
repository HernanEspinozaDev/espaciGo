import Link from "next/link";
import { Brand } from "@/components/brand/brand";

const publicLinks = [
  { href: "#espacios", label: "Espacios" },
  { href: "#como-funciona", label: "Cómo funciona" },
];

function PublicNavigation({ mobile = false }: { mobile?: boolean }) {
  return (
    <nav
      aria-label={mobile ? "Navegación móvil" : "Navegación principal"}
      className={
        mobile ? "public-nav-list public-nav-list--mobile" : "public-nav-list"
      }
    >
      {publicLinks.map((item) => (
        <Link className="nav-link" href={item.href} key={item.href}>
          {item.label}
        </Link>
      ))}
      {mobile && (
        <Link className="nav-link" href="/design-system">
          Sistema visual
        </Link>
      )}
    </nav>
  );
}

export function PublicHeader() {
  return (
    <>
      <p className="announcement">
        Una buena idea siempre merece un buen lugar.
      </p>
      <header className="public-header">
        <div className="public-header__inner page-wrap">
          <Brand />
          <div className="public-header__nav">
            <PublicNavigation />
          </div>
          <div className="public-header__actions">
            <details className="mobile-menu">
              <summary className="button button--secondary mobile-menu__trigger">
                Menú
              </summary>
              <PublicNavigation mobile />
            </details>
            <Link
              className="button button--primary header-demo-link"
              href="/panel"
            >
              Panel demo
            </Link>
          </div>
        </div>
      </header>
    </>
  );
}

export function PublicFooter() {
  return (
    <footer className="public-footer">
      <div className="page-wrap public-footer__inner">
        <Brand compact />
        <p>EspaciGo · maqueta visual local, sin conexión a servicios.</p>
        <Link className="footer-link" href="/design-system">
          Componentes y estados
        </Link>
      </div>
    </footer>
  );
}
