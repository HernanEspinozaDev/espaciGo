import type {
  LocalReservationMessage,
  LocalReservationMessagesPageEnvelope,
  LocalReservationReadCursorEnvelope,
  LocalReviewEnvelope,
  LocalReviewListEnvelope,
  LocalReviewReportEnvelope,
  LocalSpaceReviewsEnvelope,
} from '../../../../core/contracts/models';
export type ReviewInput = { rating: number; comment?: string };
export type ReportReason = 'insultos_acoso' | 'datos_personales' | 'spam' | 'ajeno_experiencia';
export type Attempt = { key: string; payload: string };

export abstract class BookingCommunicationPort {
  abstract messages(
    token: string,
    reservationId: string,
    before?: number,
  ): Promise<LocalReservationMessagesPageEnvelope>;
  abstract sendMessage(
    token: string,
    reservationId: string,
    body: string,
    key: string,
  ): Promise<{ safety_notice: string; data: LocalReservationMessage }>;
  abstract advanceRead(
    token: string,
    reservationId: string,
    through: number,
  ): Promise<LocalReservationReadCursorEnvelope>;
  abstract reservationReviews(
    token: string,
    reservationId: string,
  ): Promise<LocalReviewListEnvelope>;
  abstract createReview(
    token: string,
    reservationId: string,
    input: ReviewInput,
    key: string,
  ): Promise<LocalReviewEnvelope>;
  abstract reportReview(
    token: string,
    reservationId: string,
    reviewId: string,
    reason: ReportReason,
    key: string,
  ): Promise<LocalReviewReportEnvelope>;
  abstract spaceReviews(spaceId: string): Promise<LocalSpaceReviewsEnvelope>;
  abstract ownReputation(token: string): Promise<unknown>;
}
