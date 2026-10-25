import type { Metadata } from "next";
import { SecurityCard } from "@/features/account/ui/security-card";

export const metadata: Metadata = {
  title: "Seguridad de Cuenta | Mi Cuenta | EspaciGo",
  description:
    "Actualización de contraseña y estado de seguridad de la sesión.",
  robots: {
    index: false,
    follow: false,
  },
};

export default function SeguridadPage() {
  return <SecurityCard />;
}
