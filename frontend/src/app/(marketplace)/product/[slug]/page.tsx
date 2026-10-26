import { redirect } from "next/navigation";
import { notFound } from "next/navigation";
import { isPublicUuid } from "@/features/public-catalog/domain/media";

export default async function LegacyProductPage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await params;
  if (!isPublicUuid(slug)) notFound();
  redirect(`/espacios/${encodeURIComponent(slug)}`);
}
