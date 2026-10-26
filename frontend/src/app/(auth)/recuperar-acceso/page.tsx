import type { Metadata } from "next";
import { RecoveryScreen } from "@/features/auth/ui/action-screens";

export const dynamic = "force-dynamic";
export const metadata: Metadata = {
  title: "Recuperar acceso",
  alternates: { canonical: "/recuperar-acceso" },
  robots: { index: false, follow: false },
};

export default function RecoveryPage() {
  return <RecoveryScreen />;
}
