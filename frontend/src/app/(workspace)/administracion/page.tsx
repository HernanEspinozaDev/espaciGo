import type { Metadata } from "next";
import { AdminOverviewView } from "@/features/admin/ui/admin-overview-view";

export const metadata: Metadata = {
  title: "Resumen y KPIs | Administración | EspaciGo",
  description:
    "Métricas consolidadas, estado de la plataforma y trazabilidad administrativa.",
  robots: {
    index: false,
    follow: false,
  },
};

export default function AdministracionPage() {
  return <AdminOverviewView />;
}
