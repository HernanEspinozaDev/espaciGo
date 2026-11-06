import type { Metadata } from "next";
import { DocumentReviewView } from "@/features/document-review/ui/document-review-view";
export const metadata: Metadata = {
  title: "Revisión documental | EspaciGo",
  description: "Revisión privada de expedientes documentales asignados.",
  robots: { index: false, follow: false },
};
export default function RevisionDocumentalPage() {
  return <DocumentReviewView />;
}
