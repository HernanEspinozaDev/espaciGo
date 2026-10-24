import type { ReactNode } from "react";
import Link from "next/link";
import { Brand } from "@/components/brand/brand";

const workspaceLinks = [
  { href: "/panel", label: "Resumen", icon: "01" },
  { href: "/panel#espacios", label: "Mis espacios", icon: "02" },
  { href: "/panel#solicitudes", label: "Solicitudes", icon: "03" },
  { href: "/design-system", label: "Sistema visual", icon: "04" },
];

function WorkspaceNavigation({ mobile = false }: { mobile?: boolean }) {
  return (
    <nav
      aria-label={
        mobile ? "Navegación móvil del panel" : "Navegación del panel"
      }
      className={
        mobile ? "workspace-nav workspace-nav--mobile" : "workspace-nav"
      }
    >
      <p className="workspace-nav__label">Espacio de trabajo</p>
      {workspaceLinks.map((item, index) => (
        <Link
          aria-current={index === 0 ? "page" : undefined}
          className={`workspace-nav__link${index === 0 ? " is-current" : ""}`}
          href={item.href}
          key={item.href}
        >
          <span aria-hidden="true" className="workspace-nav__icon">
            {item.icon}
          </span>
          <span>{item.label}</span>
        </Link>
      ))}
    </nav>
  );
}

export function WorkspaceShell({ children }: { children: ReactNode }) {
  return (
    <div className="workspace-shell" data-density="admin">
      <header className="workspace-topbar">
        <Brand href="/panel" />
        <span className="workspace-topbar__context">
          Vista de administración
        </span>
        <div className="workspace-topbar__actions">
          <Link className="button button--quiet workspace-return" href="/">
            Ir al marketplace
          </Link>
          <details className="workspace-mobile-menu">
            <summary className="button button--secondary mobile-menu__trigger">
              Menú del panel
            </summary>
            <WorkspaceNavigation mobile />
          </details>
        </div>
      </header>
      <div className="workspace-layout">
        <aside aria-label="Panel de navegación" className="workspace-sidebar">
          <WorkspaceNavigation />
          <p className="workspace-sidebar__note">
            Interfaz demostrativa. No representa una cuenta real.
          </p>
        </aside>
        <div className="workspace-content">{children}</div>
      </div>
    </div>
  );
}
