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
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { useAdmin } from "./use-admin";
import type { AdminSection } from "../domain/types";

interface AdminNavItem {
  label: string;
  href: string;
  section: AdminSection;
  icon: React.ComponentType<{
    className?: string;
    "aria-hidden"?: boolean | "true" | "false";
  }>;
  exact: boolean;
}

const NAV_ITEMS: readonly AdminNavItem[] = [
  {
    label: "Resumen / KPIs",
    href: "/administracion",
    section: "overview",
    icon: BarChart3,
    exact: true,
  },
  {
    label: "Verificaciones KYC",
    href: "/administracion/verificaciones",
    section: "verifications",
    icon: UserCheck,
    exact: false,
  },
  {
    label: "Espacios",
    href: "/administracion/espacios",
    section: "spaces",
    icon: Building2,
    exact: false,
  },
  {
    label: "Disputas y Reclamos",
    href: "/administracion/disputas",
    section: "disputes",
    icon: Scale,
    exact: false,
  },
  {
    label: "Cuentas y Roles",
    href: "/administracion/cuentas",
    section: "accounts",
    icon: Users,
    exact: false,
  },
  {
    label: "Auditoría y Reportes",
    href: "/administracion/auditoria",
    section: "audit",
    icon: FileText,
    exact: false,
  },
];

function formatSingleRole(role: string | null): string {
  switch (role) {
    case "administrador_global":
      return "Administrador Global";
    case "administrador":
      return "Administrador";
    case "arrendador":
      return "Arrendador";
    case "arrendatario":
      return "Arrendatario";
    default:
      return "Sesión Restringida";
  }
}

export function AdminNavigation() {
  const pathname = usePathname();
  const { canAccess, activeRole } = useAdmin();

  // Visible nav items filtered by capabilities
  const accessibleItems = NAV_ITEMS.filter((item) => canAccess(item.section));

  // Active section computation for breadcrumb and header
  const currentItem = accessibleItems.find((item) =>
    item.exact ? pathname === item.href : pathname.startsWith(item.href),
  );
  const isRoot = pathname === "/administracion";

  return (
    <div className="space-y-4">
      {/* Breadcrumbs accesibles */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <Breadcrumb>
          <BreadcrumbList>
            <BreadcrumbItem>
              <BreadcrumbLink href="/">Inicio</BreadcrumbLink>
            </BreadcrumbItem>
            <BreadcrumbSeparator />
            <BreadcrumbItem>
              {isRoot ? (
                <BreadcrumbPage>Administración</BreadcrumbPage>
              ) : (
                <BreadcrumbLink href="/administracion">
                  Administración
                </BreadcrumbLink>
              )}
            </BreadcrumbItem>
            {!isRoot && currentItem && (
              <>
                <BreadcrumbSeparator />
                <BreadcrumbItem>
                  <BreadcrumbPage>{currentItem.label}</BreadcrumbPage>
                </BreadcrumbItem>
              </>
            )}
          </BreadcrumbList>
        </Breadcrumb>

        {/* Solo un rol en UI por sesión */}
        <div className="flex items-center gap-2">
          <Badge
            variant="secondary"
            className="gap-1.5 px-3 py-1 text-xs font-semibold tracking-wide uppercase shadow-none border border-slate-300 dark:border-slate-700"
          >
            <Shield className="size-3.5 text-primary" aria-hidden="true" />
            <span>{formatSingleRole(activeRole)}</span>
          </Badge>
        </div>
      </div>

      {/* Pestañas de acceso por capacidades */}
      <nav
        aria-label="Navegación de secciones de administración"
        className="border-b border-border flex items-center gap-1 overflow-x-auto pb-px"
      >
        {accessibleItems.map((item) => {
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
                "inline-flex items-center gap-2 px-4 py-2.5 text-sm font-medium border-b-2 whitespace-nowrap transition-colors rounded-t-md",
                isActive
                  ? "border-primary text-primary bg-muted/40 font-semibold"
                  : "border-transparent text-muted-foreground hover:text-foreground hover:border-border",
              )}
            >
              <Icon className="size-4" aria-hidden="true" />
              <span>{item.label}</span>
            </Link>
          );
        })}
      </nav>
    </div>
  );
}
