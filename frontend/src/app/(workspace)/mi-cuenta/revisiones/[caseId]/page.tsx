import { ReviewerCaseDetailView } from "@/features/reviewer-queue/ui/reviewer-case-detail-view";

type PageProps = { params: Promise<{ caseId: string }> };

export default async function ReviewerCasePage({ params }: PageProps) {
  const { caseId } = await params;
  return <ReviewerCaseDetailView caseId={caseId} />;
}
