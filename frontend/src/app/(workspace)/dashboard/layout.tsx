"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import {
  Activity,
  LayoutDashboard,
  Menu,
  Settings,
  Shield,
  User,
} from "lucide-react";
import { Button } from "@/components/workspace/ui/button";

const navItems = [
  { href: "/dashboard", icon: LayoutDashboard, label: "Resumen" },
  { href: "/dashboard/general", icon: Settings, label: "General" },
  { href: "/dashboard/activity", icon: Activity, label: "Actividad" },
  { href: "/dashboard/security", icon: Shield, label: "Seguridad" },
  { href: "/mi-cuenta", icon: User, label: "Mi Cuenta" },
];

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);

  return (
    <div className="relative mx-auto flex min-h-[calc(100dvh-112px)] w-full max-w-7xl flex-col">
      <div className="flex items-center justify-between border-b border-gray-200 bg-white p-4 lg:hidden">
        <span className="font-medium text-gray-900">Espacio de trabajo</span>
        <Button
          type="button"
          variant="ghost"
          aria-expanded={isSidebarOpen}
          aria-controls="workspace-sidebar"
          onClick={() => setIsSidebarOpen((open) => !open)}
        >
          <Menu className="h-6 w-6" aria-hidden="true" />
          <span className="sr-only">
            {isSidebarOpen ? "Cerrar navegación" : "Abrir navegación"}
          </span>
        </Button>
      </div>
      <div className="flex flex-1">
        <aside
          id="workspace-sidebar"
          className={`${isSidebarOpen ? "block" : "hidden"} absolute inset-x-0 top-[57px] z-30 border-b border-gray-200 bg-white lg:relative lg:inset-auto lg:block lg:w-64 lg:shrink-0 lg:border-b-0 lg:border-r lg:bg-gray-50`}
        >
          <nav
            aria-label="Navegación del panel"
            className="flex flex-col gap-1 p-4"
          >
            {navItems.map((item) => {
              const active = pathname === item.href;
              return (
                <Button
                  key={item.href}
                  asChild
                  variant={active ? "secondary" : "ghost"}
                  className={`my-1 w-full justify-start shadow-none ${active ? "bg-gray-100" : ""}`}
                >
                  <Link
                    href={item.href}
                    aria-current={active ? "page" : undefined}
                    onClick={() => setIsSidebarOpen(false)}
                  >
                    <item.icon className="h-4 w-4" aria-hidden="true" />
                    {item.label}
                  </Link>
                </Button>
              );
            })}
          </nav>
        </aside>
        <main className="min-w-0 flex-1 overflow-x-hidden">{children}</main>
      </div>
    </div>
  );
}
