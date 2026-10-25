import type { Metadata } from "next";
import { AdminNavigation } from "@/features/admin/ui/admin-navigation";

export const metadata: Metadata = {
  title: "Panel de Administración | EspaciGo",
  description:
    "Gestión de identidades, moderación de espacios, arbitraje de disputas y auditoría del sistema.",
  robots: {
    index: false,
    follow: false,
  },
};

export default function AdministracionLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <main className="container mx-auto px-4 py-8 max-w-5xl space-y-6">
      <div className="space-y-1">
        <h1 className="text-3xl font-bold tracking-tight text-foreground">
          Panel de Administración
        </h1>
        <p className="text-sm text-muted-foreground">
          Consola operativa y de gobernanza para gestión de cuentas,
          publicaciones y cumplimiento normativo.
        </p>
      </div>

      <AdminNavigation />
      <section aria-label="Contenido de la sección administrativa">
        {children}
      </section>
    </main>
  );
}
