"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  BarChart3,
  Building2,
  FileText,
  Scale,
  Shield,
  UserCheck,
  Users,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useAdminAuth } from "./use-admin-auth";

interface NavItem {
  label: string;
  href: string;
  icon: typeof BarChart3;
  description: string;
  capability?: string;
  exact?: boolean;
}

const NAV_ITEMS: NavItem[] = [
  {
    label: "Resumen / KPIs",
    href: "/administracion",
    icon: BarChart3,
    description: "Métricas y estado general",
    capability: "kpi.read",
    exact: true,
  },
  {
    label: "Verificaciones KYC/KYB",
    href: "/administracion/verificaciones",
    icon: UserCheck,
    description: "Validación de identidad",
    capability: "verifications.review",
  },
  {
    label: "Espacios y Publicaciones",
    href: "/administracion/espacios",
    icon: Building2,
    description: "Moderación de catálogo",
    capability: "publications.review",
  },
  {
    label: "Disputas",
    href: "/administracion/disputas",
    icon: Scale,
    description: "Gestión de mediación",
    capability: "disputes.manage",
  },
  {
    label: "Cuentas y Roles",
    href: "/administracion/cuentas",
    icon: Users,
    description: "Control de accesos RBAC",
    capability: "users.roles.manage",
  },
  {
    label: "Bitácora de Auditoría",
    href: "/administracion/auditoria",
    icon: FileText,
    description: "Eventos y exportación",
    capability: "audit.read",
  },
];

export function AdminNavigation() {
  const pathname = usePathname();
  const { can, user, activeRole } = useAdminAuth();

  const visibleItems = NAV_ITEMS.filter(
    (item) => !item.capability || can(item.capability),
  );

  return (
    <aside
      aria-label="Navegación de administración"
      className="w-full md:w-64 shrink-0 md:border-r md:border-border/60 md:pr-6 space-y-4 max-w-full"
    >
      {/* Estado de sesión administrativa activa */}
      <div className="p-3.5 rounded-lg bg-muted/40 border border-border/60 space-y-1.5">
        <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          <Shield
            className="size-3.5 text-primary shrink-0"
            aria-hidden="true"
          />
          <span>Sesión administrativa activa:</span>
        </div>
        <div className="text-sm font-medium text-foreground truncate">
          {user?.email ?? "Usuario no autenticado"}
        </div>
        {activeRole && (
          <div className="pt-1">
            <span className="inline-block rounded bg-primary/10 px-2 py-0.5 text-xs font-semibold text-primary uppercase tracking-wide">
              {activeRole.replace(/_/g, " ")}
            </span>
          </div>
        )}
      </div>

      {/* Navegación vertical en escritorio y horizontal en móvil */}
      <nav
        aria-label="Secciones de administración"
        className="flex flex-row md:flex-col gap-1.5 overflow-x-auto pb-2 md:pb-0 max-w-full"
      >
        {visibleItems.map((item) => {
          const isActive = item.exact
            ? pathname === item.href
            : pathname.startsWith(item.href);
          const Icon = item.icon;

          return (
            <Link
              key={item.href}
              href={item.href}
              aria-current={isActive ? "page" : undefined}
              className={cn(
                "flex items-center gap-3 rounded-lg px-3.5 py-2.5 text-sm font-medium transition-colors whitespace-nowrap md:whitespace-normal",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                isActive
                  ? "bg-primary/10 text-primary font-semibold border-b-2 md:border-b-0 md:border-l-2 border-primary"
                  : "text-muted-foreground hover:bg-muted/60 hover:text-foreground",
              )}
            >
              <Icon className="size-4 shrink-0" aria-hidden="true" />
              <div className="flex flex-col min-w-0 text-left">
                <span>{item.label}</span>
                <span
                  aria-hidden="true"
                  className={cn(
                    "text-xs font-normal hidden lg:block truncate",
                    isActive ? "text-foreground" : "text-muted-foreground",
                  )}
                >
                  {item.description}
                </span>
              </div>
            </Link>
          );
        })}
      </nav>
    </aside>
  );
}
