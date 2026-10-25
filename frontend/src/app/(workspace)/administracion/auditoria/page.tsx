import type { Metadata } from "next";
import { AdminAuditView } from "@/features/admin/ui/admin-audit-view";

export const metadata: Metadata = {
  title: "Auditoría y Trazabilidad | Administración | EspaciGo",
  description:
    "Registro cronológico de eventos administrativos y exportación de bitácora del sistema.",
  robots: {
    index: false,
    follow: false,
  },
};

export default function AdministracionAuditoriaPage() {
  return <AdminAuditView />;
}
