import type { Metadata } from "next";
import { VerificationsView } from "@/features/verifications/ui/verifications-view";

export const metadata: Metadata = {
  title: "Verificaciones KYC/KYB | Mi Cuenta | EspaciGo",
  description:
    "Elegibilidad, historial y evidencias de verificación de identidad.",
  robots: {
    index: false,
    follow: false,
  },
};

export default function VerificacionesPage() {
  return <VerificationsView />;
}
