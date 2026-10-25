"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Building2, PlusCircle } from "lucide-react";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import { cn } from "@/lib/utils";

export function HostNavigation() {
  const pathname = usePathname();

  const isNuevo = pathname.includes("/espacios/nuevo");
  const isEditar = pathname.includes("/editar");
  const isListado =
    pathname === "/anfitrion/espacios" || pathname === "/anfitrion";

  return (
    <div className="space-y-4">
      {/* Migas de pan accesibles */}
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink href="/">Inicio</BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            {isListado ? (
              <BreadcrumbPage>Panel Anfitrión</BreadcrumbPage>
            ) : (
              <BreadcrumbLink href="/anfitrion/espacios">
                Panel Anfitrión
              </BreadcrumbLink>
            )}
          </BreadcrumbItem>
          {isNuevo && (
            <>
              <BreadcrumbSeparator />
              <BreadcrumbItem>
                <BreadcrumbPage>Publicar Nuevo Espacio</BreadcrumbPage>
              </BreadcrumbItem>
            </>
          )}
          {isEditar && (
            <>
              <BreadcrumbSeparator />
              <BreadcrumbItem>
                <BreadcrumbPage>Editar Espacio</BreadcrumbPage>
              </BreadcrumbItem>
            </>
          )}
        </BreadcrumbList>
      </Breadcrumb>

      {/* Barra de navegación de anfitrión */}
      <nav
        aria-label="Navegación del panel de anfitrión"
        className="border-b border-border flex items-center gap-1 overflow-x-auto pb-px"
      >
        <Link
          href="/anfitrion/espacios"
          aria-current={isListado ? "page" : undefined}
          className={cn(
            "inline-flex items-center gap-2 px-4 py-2.5 text-sm font-medium border-b-2 whitespace-nowrap transition-colors rounded-t-md",
            isListado
              ? "border-primary text-primary bg-muted/40 font-semibold"
              : "border-transparent text-muted-foreground hover:text-foreground hover:border-border",
          )}
        >
          <Building2 className="size-4" aria-hidden="true" />
          <span>Mis Espacios</span>
        </Link>

        <Link
          href="/anfitrion/espacios/nuevo"
          aria-current={isNuevo ? "page" : undefined}
          className={cn(
            "inline-flex items-center gap-2 px-4 py-2.5 text-sm font-medium border-b-2 whitespace-nowrap transition-colors rounded-t-md",
            isNuevo
              ? "border-primary text-primary bg-muted/40 font-semibold"
              : "border-transparent text-muted-foreground hover:text-foreground hover:border-border",
          )}
        >
          <PlusCircle className="size-4" aria-hidden="true" />
          <span>Publicar Espacio</span>
        </Link>
      </nav>
    </div>
  );
}
