import type { Metadata } from "next";
import { RegistrationSuccessScreen } from "@/features/auth/ui/registration-success-screen";

export const dynamic = "force-dynamic";
export const metadata: Metadata = {
  title: "Registro exitoso | EspaciGo",
  alternates: { canonical: "/registro-exitoso" },
  robots: { index: false, follow: false },
};

export default function RegistrationSuccessPage() {
  return <RegistrationSuccessScreen />;
}
