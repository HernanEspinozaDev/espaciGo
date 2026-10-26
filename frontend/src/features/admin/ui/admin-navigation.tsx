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
import { Button } from "@/components/workspace/ui/button";
import { cn } from "@/lib/utils";
import { useAdmin } from "./use-admin";
import type { AdminSection } from "../domain/types";

interface AdminNavItem {
  label: string;
  href: string;
  section: AdminSection;
  icon: typeof BarChart3;
  description: string;
  exact: boolean;
}

const NAV_ITEMS: readonly AdminNavItem[] = [
  {
    label: "Resumen / KPIs",
    href: "/administracion",
    section: "overview",
    icon: BarChart3,
    description: "Métricas y estado general",
    exact: true,
  },
  {
    label: "Verificaciones KYC",
    href: "/administracion/verificaciones",
    section: "verifications",
    icon: UserCheck,
    description: "Validación de identidad",
    exact: false,
  },
  {
    label: "Espacios",
    href: "/administracion/espacios",
    section: "spaces",
    icon: Building2,
    description: "Moderación de catálogo",
    exact: false,
  },
  {
    label: "Disputas y Reclamos",
    href: "/administracion/disputas",
    section: "disputes",
    icon: Scale,
    description: "Gestión de mediación",
    exact: false,
  },
  {
    label: "Cuentas y Roles",
    href: "/administracion/cuentas",
    section: "accounts",
    icon: Users,
    description: "Control de accesos RBAC",
    exact: false,
  },
  {
    label: "Auditoría y Reportes",
    href: "/administracion/auditoria",
    section: "audit",
    icon: FileText,
    description: "Eventos y exportación",
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

  const accessibleItems = NAV_ITEMS.filter((item) => canAccess(item.section));
  const isRoot = pathname === "/administracion";
  const currentItem = accessibleItems.find((item) =>
    item.exact ? pathname === item.href : pathname.startsWith(item.href),
  );

  return (
    <aside
      aria-label="Navegación de administración"
      className="w-full md:w-64 shrink-0 md:border-r md:border-border/60 md:pr-6 space-y-4 max-w-full"
    >
      {/* Breadcrumbs accesibles */}
      <div className="hidden md:block">
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
      </div>

      {/* Badge de rol activo garantizado en la UI */}
      <div className="flex items-center gap-2 p-2.5 rounded-lg bg-muted/40 border border-border/60">
        <Badge
          variant="secondary"
          className="gap-1.5 px-3 py-1 text-xs font-semibold tracking-wide uppercase shadow-none border border-slate-300 dark:border-slate-700"
        >
          <Shield className="size-3.5 text-primary" aria-hidden="true" />
          <span>{formatSingleRole(activeRole)}</span>
        </Badge>
      </div>

      {activeRole !== "administrador" &&
      activeRole !== "administrador_global" ? (
        <aside
          role="region"
          aria-label="Aviso de autenticación administrativa"
          className="flex flex-col gap-3 rounded-xl border border-amber-300 bg-amber-50 p-4 text-amber-900 shadow-sm dark:border-amber-800 dark:bg-amber-950/50 dark:text-amber-200"
        >
          <div className="space-y-0.5">
            <p className="font-semibold text-sm">
              Se requiere autorización administrativa
            </p>
            <p className="text-xs text-amber-800 dark:text-amber-300">
              Para ver métricas y gestionar la plataforma debes iniciar sesión
              con una cuenta de administrador o administrador global.
            </p>
          </div>
          <Button
            asChild
            size="sm"
            className="rounded-full bg-orange-700 text-white hover:bg-orange-800 shrink-0 font-medium w-full"
          >
            <Link href={`/acceso?return_to=${encodeURIComponent(pathname)}`}>
              Iniciar sesión administrativa
            </Link>
          </Button>
        </aside>
      ) : null}

      {/* Navegación vertical en escritorio y horizontal en móvil */}
      <nav
        aria-label="Navegación de secciones de administración"
        className="flex flex-row md:flex-col gap-1.5 overflow-x-auto pb-2 md:pb-0 max-w-full"
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
