import type { Metadata } from "next";
import { LegalDocument } from "@/features/public-catalog/ui/legal-document";

export const dynamic = "force-dynamic";
export const metadata: Metadata = {
  title: "Política de privacidad",
  description: "Documento de privacidad entregado por la API de EspaciGo.",
  alternates: { canonical: "/politica-de-privacidad" },
  robots: { index: false, follow: true },
};

export default function PrivacyPage() {
  return <LegalDocument type="privacidad" />;
}
