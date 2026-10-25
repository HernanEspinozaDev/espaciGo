import { ListingEditor } from "@/features/host-listings/ui";

interface PageProps {
  params: Promise<{ id: string }>;
}

export default async function EditarEspacioPage({ params }: PageProps) {
  const { id } = await params;
  return <ListingEditor listingId={id} />;
}
