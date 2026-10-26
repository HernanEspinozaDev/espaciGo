import type { Metadata } from "next";
import { LegalDocument } from "@/features/public-catalog/ui/legal-document";

export const dynamic = "force-dynamic";
export const metadata: Metadata = {
  title: "Términos y condiciones",
  description: "Documento legal entregado por la API de EspaciGo.",
  alternates: { canonical: "/terminos-y-condiciones" },
  robots: { index: false, follow: true },
};

export default function TermsPage() {
  return <LegalDocument type="terminos" />;
}
