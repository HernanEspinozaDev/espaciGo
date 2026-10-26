import type { Metadata } from "next";
import { AdminDisputesView } from "@/features/admin/ui/admin-disputes-view";

export const metadata: Metadata = {
  title: "Disputas y Reclamos | Administración | EspaciGo",
  description:
    "Gestión de incidentes, arbitraje de daños y retención de depósitos de garantía.",
  robots: {
    index: false,
    follow: false,
  },
};

export default function AdministracionDisputasPage() {
  return <AdminDisputesView />;
}
