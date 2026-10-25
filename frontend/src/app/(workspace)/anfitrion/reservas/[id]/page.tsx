import { notFound } from "next/navigation";
import { ReservationLifecyclePanel } from "@/features/bookings/ui/reservation-lifecycle-panel";
import { BookingConversationPanel } from "@/features/bookings/ui/booking-conversation-panel";
import { BookingReviewsPanel } from "@/features/bookings/ui/booking-reviews-panel";
import { BookingDisputesPanel } from "@/features/bookings/ui/booking-disputes-panel";

interface PageProps {
  params: Promise<{ id: string }>;
}

export default async function HostReservationDetailPage({ params }: PageProps) {
  const { id } = await params;

  if (!id) {
    notFound();
  }

  return (
    <div className="space-y-10">
      <ReservationLifecyclePanel reservationId={id} role="anfitrion" />
      <div className="grid grid-cols-1 gap-8 lg:grid-cols-2">
        <BookingConversationPanel reservationId={id} />
        <BookingReviewsPanel reservationId={id} />
      </div>
      <BookingDisputesPanel reservationId={id} />
    </div>
  );
}
