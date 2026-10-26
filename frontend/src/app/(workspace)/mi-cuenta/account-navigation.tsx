"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { FileCheck, Lock, Shield, User } from "lucide-react";
import { cn } from "@/lib/utils";

const items = [
  {
    title: "General",
    href: "/mi-cuenta",
    icon: User,
    description: "Información personal y contacto",
  },
  {
    title: "Seguridad",
    href: "/mi-cuenta/seguridad",
    icon: Lock,
    description: "Contraseña y sesiones",
  },
  {
    title: "Privacidad",
    href: "/mi-cuenta/privacidad",
    icon: Shield,
    description: "Exportación y derechos",
  },
  {
    title: "Verificaciones",
    href: "/mi-cuenta/verificaciones",
    icon: FileCheck,
    description: "Estado de identidad",
  },
];

export function AccountNavigation() {
  const pathname = usePathname();

  return (
    <aside
      aria-label="Navegación de cuenta"
      className="w-full md:w-64 shrink-0 md:border-r md:border-border/60 md:pr-6"
    >
      <nav
        aria-label="Secciones de mi cuenta"
        className="flex flex-row md:flex-col gap-1.5 overflow-x-auto pb-2 md:pb-0"
      >
        {items.map((item) => {
          const isActive = pathname === item.href;
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
              <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
              <div className="flex flex-col min-w-0 text-left">
                <span>{item.title}</span>
                <span
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
