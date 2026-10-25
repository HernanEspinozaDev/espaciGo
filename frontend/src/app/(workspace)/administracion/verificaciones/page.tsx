import type { Metadata } from "next";
import { AdminVerificationsView } from "@/features/admin/ui/admin-verifications-view";

export const metadata: Metadata = {
  title: "Verificaciones KYC/KYB | Administración | EspaciGo",
  description:
    "Bandeja de moderación y aprobación de identidad sintética y antecedentes.",
  robots: {
    index: false,
    follow: false,
  },
};

export default function AdministracionVerificacionesPage() {
  return <AdminVerificationsView />;
}
