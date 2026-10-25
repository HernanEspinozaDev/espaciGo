import type { Metadata } from "next";
import { AccountNavigation } from "./account-navigation";

export const metadata: Metadata = {
  title: "Mi Cuenta | EspaciGo",
  description:
    "Configuración de cuenta, perfil, privacidad, seguridad y verificaciones.",
  robots: {
    index: false,
    follow: false,
  },
};

export default function MiCuentaLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <main className="container mx-auto px-4 py-8 max-w-5xl space-y-8">
      <div>
        <h1 className="text-3xl font-bold tracking-tight text-foreground">
          Mi Cuenta
        </h1>
        <p className="text-sm text-muted-foreground mt-1">
          Gestiona tu perfil público, opciones de seguridad, privacidad de datos
          y certificaciones de identidad.
        </p>
      </div>

      <AccountNavigation />

      <div>{children}</div>
    </main>
  );
}
