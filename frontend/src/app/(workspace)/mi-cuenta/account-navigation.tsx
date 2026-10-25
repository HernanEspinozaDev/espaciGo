"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { FileCheck, Lock, Shield, User } from "lucide-react";
import { cn } from "@/lib/utils";

const NAV_ITEMS = [
  {
    label: "General",
    href: "/mi-cuenta",
    icon: User,
    exact: true,
  },
  {
    label: "Privacidad",
    href: "/mi-cuenta/privacidad",
    icon: Shield,
    exact: false,
  },
  {
    label: "Seguridad",
    href: "/mi-cuenta/seguridad",
    icon: Lock,
    exact: false,
  },
  {
    label: "Verificaciones",
    href: "/mi-cuenta/verificaciones",
    icon: FileCheck,
    exact: false,
  },
];

export function AccountNavigation() {
  const pathname = usePathname();

  return (
    <nav
      aria-label="Navegación de cuenta"
      className="border-b border-border flex items-center gap-1 overflow-x-auto pb-px"
    >
      {NAV_ITEMS.map((item) => {
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
  );
}
