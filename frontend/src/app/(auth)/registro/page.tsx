import type { Metadata } from "next";
import { RegistrationScreen } from "@/features/auth/ui/registration-screen";

export const dynamic = "force-dynamic";
export const metadata: Metadata = {
  title: "Crear cuenta",
  alternates: { canonical: "/registro" },
  robots: { index: false, follow: false },
};

export default function RegistrationPage() {
  return <RegistrationScreen />;
}
