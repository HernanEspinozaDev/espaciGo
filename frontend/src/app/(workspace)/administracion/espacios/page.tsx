import type { Metadata } from "next";
import { AdminSpacesView } from "@/features/admin/ui/admin-spaces-view";

export const metadata: Metadata = {
  title: "Moderación de Espacios | Administración | EspaciGo",
  description:
    "Control de visibilidad, retiro y suspensión de publicaciones en catálogo.",
  robots: {
    index: false,
    follow: false,
  },
};

export default function AdministracionEspaciosPage() {
  return <AdminSpacesView />;
}
