import type { Metadata } from "next";
import { VerificationScreen } from "@/features/auth/ui/action-screens";

export const dynamic = "force-dynamic";
export const metadata: Metadata = {
  title: "Verificar correo",
  alternates: { canonical: "/verificar-correo" },
  robots: { index: false, follow: false },
};

export default function VerificationPage() {
  return <VerificationScreen />;
}
