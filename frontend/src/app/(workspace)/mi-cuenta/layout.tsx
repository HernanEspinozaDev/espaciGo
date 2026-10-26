import type { Metadata } from "next";
import { AccountNavigation } from "./account-navigation";

export const metadata: Metadata = {
  title: "Mi Cuenta | EspaciGo",
  description:
    "Administración de perfil de usuario, seguridad, preferencias y verificaciones en EspaciGo.",
};

export default function MiCuentaLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <main className="container mx-auto px-4 py-8 max-w-6xl space-y-8">
      <div className="border-b border-border/60 pb-5">
        <h1 className="text-3xl font-bold tracking-tight text-foreground">
          Mi Cuenta
        </h1>
        <p className="text-muted-foreground mt-2 text-sm sm:text-base">
          Administra tu perfil, configuración de seguridad, privacidad y estado
          de verificaciones.
        </p>
      </div>

      <div className="flex flex-col md:flex-row gap-8 items-start min-w-0">
        <AccountNavigation />
        <section
          aria-label="Contenido de la sección de cuenta"
          className="flex-1 w-full min-w-0 space-y-6"
        >
          {children}
        </section>
      </div>
    </main>
  );
}
