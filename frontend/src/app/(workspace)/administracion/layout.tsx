import type { Metadata } from "next";
import { AdminNavigation } from "@/features/admin/ui/admin-navigation";

export const metadata: Metadata = {
  title: "Panel de Administración | EspaciGo",
  description:
    "Gestión operativa y moderación de publicaciones, KYC, disputas y auditoría.",
  robots: {
    index: false,
    follow: false,
  },
};

export default function AdminLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <main className="container mx-auto px-4 py-8 max-w-7xl space-y-8">
      <div className="border-b border-border/60 pb-5">
        <h1 className="text-3xl font-bold tracking-tight text-foreground">
          Panel de Administración
        </h1>
        <p className="text-muted-foreground mt-2 text-sm sm:text-base">
          Supervisión operativa, moderación de publicaciones, cumplimiento KYC,
          disputas y bitácora de seguridad.
        </p>
      </div>

      <div className="flex flex-col md:flex-row gap-8 items-start min-w-0">
        <AdminNavigation />
        <section
          aria-label="Área operativa"
          className="flex-1 w-full min-w-0 space-y-6"
        >
          {children}
        </section>
      </div>
    </main>
  );
}
