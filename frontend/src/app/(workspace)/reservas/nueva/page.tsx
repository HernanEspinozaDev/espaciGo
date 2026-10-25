import { BookingQuoteCard } from "@/features/bookings/ui/booking-quote-card";

interface NuevaReservaPageProps {
  searchParams: Promise<{ space_id?: string; title?: string }>;
}

export default async function NuevaReservaPage({
  searchParams,
}: NuevaReservaPageProps) {
  const params = await searchParams;
  const spaceId = params.space_id ?? "";
  const spaceTitle = params.title ?? undefined;

  return (
    <div className="mx-auto max-w-2xl py-4">
      <BookingQuoteCard spaceId={spaceId} spaceTitle={spaceTitle} />
    </div>
  );
}
