import type { Metadata } from "next";
import { AccessScreen } from "@/features/auth/ui/access-screen";

export const dynamic = "force-dynamic";
export const metadata: Metadata = {
  title: "Acceso",
  alternates: { canonical: "/acceso" },
  robots: { index: false, follow: false },
};

export default function LegacyLoginPage() {
  return <AccessScreen />;
}
