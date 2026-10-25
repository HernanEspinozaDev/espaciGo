import type { Metadata } from "next";
import { PrivacyCard } from "@/features/account/ui/privacy-card";

export const metadata: Metadata = {
  title: "Privacidad de Datos | Mi Cuenta | EspaciGo",
  description: "Gestión de derechos ARCO y exportación de datos personales.",
  robots: {
    index: false,
    follow: false,
  },
};

export default function PrivacidadPage() {
  return <PrivacyCard />;
}
