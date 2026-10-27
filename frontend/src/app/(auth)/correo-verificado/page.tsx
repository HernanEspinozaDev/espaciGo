import type { Metadata } from "next";
import { EmailVerifiedScreen } from "@/features/auth/ui/email-verified-screen";

export const dynamic = "force-dynamic";
export const metadata: Metadata = {
  title: "Correo verificado",
  alternates: { canonical: "/correo-verificado" },
  robots: { index: false, follow: false },
};

export default function EmailVerifiedPage() {
  return <EmailVerifiedScreen />;
}
