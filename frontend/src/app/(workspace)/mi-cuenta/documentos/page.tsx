import type { Metadata } from "next";
import { DocumentVerificationView } from "@/features/document-verification/ui/document-verification-view";

export const metadata: Metadata = {
  title: "Documentos de verificación | Mi Cuenta | EspaciGo",
  description: "Consulta y gestiona tus expedientes documentales privados.",
  robots: { index: false, follow: false },
};

export default function DocumentosPage() {
  return <DocumentVerificationView />;
}
