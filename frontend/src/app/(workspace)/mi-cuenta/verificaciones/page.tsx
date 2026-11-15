import type { Metadata } from "next";
import { VerificationsView } from "@/features/verifications/ui/verifications-view";

export const metadata: Metadata = {
  title: "Verificaciones | Mi Cuenta",
  description:
    "Gestiona tu declaración de identidad y consulta verificaciones de usuario y espacios.",
  robots: {
    index: false,
    follow: false,
  },
};

export default function VerificacionesPage() {
  return <VerificationsView />;
}
