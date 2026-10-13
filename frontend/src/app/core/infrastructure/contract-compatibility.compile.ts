import type * as Projection from '../contracts/models';
import type * as Wire from '../../../generated/api/types.gen';
/** Structural projection compatibility is checked whenever the Go SDK is regenerated. */
type Compatible<A extends B, B> = A;
export type AccountCreatedFromWire = Compatible<Wire.AccountCreated, Projection.AccountCreated>;
export type AccountCreatedToWire = Compatible<Projection.AccountCreated, Wire.AccountCreated>;
export type AvailabilityConfigResponseFromWire = Compatible<
  Wire.AvailabilityConfigResponse,
  Projection.AvailabilityConfigResponse
>;
export type AvailabilityConfigResponseToWire = Compatible<
  Projection.AvailabilityConfigResponse,
  Wire.AvailabilityConfigResponse
>;
export type AvailabilityResultFromWire = Compatible<
  Wire.AvailabilityResult,
  Projection.AvailabilityResult
>;
export type AvailabilityResultToWire = Compatible<
  Projection.AvailabilityResult,
  Wire.AvailabilityResult
>;
export type LocalBookingCancellationPreviewFromWire = Compatible<
  Wire.LocalBookingCancellationPreview,
  Projection.LocalBookingCancellationPreview
>;
export type LocalBookingCancellationPreviewToWire = Compatible<
  Projection.LocalBookingCancellationPreview,
  Wire.LocalBookingCancellationPreview
>;
export type LocalBookingDetailFromWire = Compatible<
  Wire.LocalBookingDetail,
  Projection.LocalBookingDetail
>;
export type LocalBookingDetailToWire = Compatible<
  Projection.LocalBookingDetail,
  Wire.LocalBookingDetail
>;
export type LocalBookingReservationFromWire = Compatible<
  Wire.LocalBookingReservation,
  Projection.LocalBookingReservation
>;
export type LocalBookingReservationToWire = Compatible<
  Projection.LocalBookingReservation,
  Wire.LocalBookingReservation
>;
export type LocalBookingTransitionFromWire = Compatible<
  Wire.LocalBookingTransition,
  Projection.LocalBookingTransition
>;
export type LocalBookingTransitionToWire = Compatible<
  Projection.LocalBookingTransition,
  Wire.LocalBookingTransition
>;
export type LocalRentalOperationFromWire = Compatible<
  Wire.LocalRentalOperation,
  Projection.LocalRentalOperation
>;
export type LocalRentalOperationToWire = Compatible<
  Projection.LocalRentalOperation,
  Wire.LocalRentalOperation
>;
export type LocalRentalOperationInputFromWire = Compatible<
  Wire.LocalRentalOperationInput,
  Projection.LocalRentalOperationInput
>;
export type LocalRentalOperationInputToWire = Compatible<
  Projection.LocalRentalOperationInput,
  Wire.LocalRentalOperationInput
>;
export type LocalRentalOperationListEnvelopeFromWire = Compatible<
  Wire.LocalRentalOperationListEnvelope,
  Projection.LocalRentalOperationListEnvelope
>;
export type LocalRentalOperationListEnvelopeToWire = Compatible<
  Projection.LocalRentalOperationListEnvelope,
  Wire.LocalRentalOperationListEnvelope
>;
export type LocalReservationMessageFromWire = Compatible<
  Wire.LocalReservationMessage,
  Projection.LocalReservationMessage
>;
export type LocalReservationMessageToWire = Compatible<
  Projection.LocalReservationMessage,
  Wire.LocalReservationMessage
>;
export type LocalReservationMessagesPageEnvelopeFromWire = Compatible<
  Wire.LocalReservationMessagesPageEnvelope,
  Projection.LocalReservationMessagesPageEnvelope
>;
export type LocalReservationMessagesPageEnvelopeToWire = Compatible<
  Projection.LocalReservationMessagesPageEnvelope,
  Wire.LocalReservationMessagesPageEnvelope
>;
export type LocalReservationReadCursorEnvelopeFromWire = Compatible<
  Wire.LocalReservationReadCursorEnvelope,
  Projection.LocalReservationReadCursorEnvelope
>;
export type LocalReservationReadCursorEnvelopeToWire = Compatible<
  Projection.LocalReservationReadCursorEnvelope,
  Wire.LocalReservationReadCursorEnvelope
>;
export type LocalReviewFromWire = Compatible<Wire.LocalReview, Projection.LocalReview>;
export type LocalReviewToWire = Compatible<Projection.LocalReview, Wire.LocalReview>;
export type LocalReviewEnvelopeFromWire = Compatible<
  Wire.LocalReviewEnvelope,
  Projection.LocalReviewEnvelope
>;
export type LocalReviewEnvelopeToWire = Compatible<
  Projection.LocalReviewEnvelope,
  Wire.LocalReviewEnvelope
>;
export type LocalReviewListEnvelopeFromWire = Compatible<
  Wire.LocalReviewListEnvelope,
  Projection.LocalReviewListEnvelope
>;
export type LocalReviewListEnvelopeToWire = Compatible<
  Projection.LocalReviewListEnvelope,
  Wire.LocalReviewListEnvelope
>;
export type LocalReviewReportFromWire = Compatible<
  Wire.LocalReviewReport,
  Projection.LocalReviewReport
>;
export type LocalReviewReportToWire = Compatible<
  Projection.LocalReviewReport,
  Wire.LocalReviewReport
>;
export type LocalReviewReportEnvelopeFromWire = Compatible<
  Wire.LocalReviewReportEnvelope,
  Projection.LocalReviewReportEnvelope
>;
export type LocalReviewReportEnvelopeToWire = Compatible<
  Projection.LocalReviewReportEnvelope,
  Wire.LocalReviewReportEnvelope
>;
export type LocalSpaceReviewsEnvelopeFromWire = Compatible<
  Wire.LocalSpaceReviewsEnvelope,
  Projection.LocalSpaceReviewsEnvelope
>;
export type LocalSpaceReviewsEnvelopeToWire = Compatible<
  Projection.LocalSpaceReviewsEnvelope,
  Wire.LocalSpaceReviewsEnvelope
>;
export type LocalSyntheticContractFromWire = Compatible<
  Wire.LocalSyntheticContract,
  Projection.LocalSyntheticContract
>;
export type LocalSyntheticContractToWire = Compatible<
  Projection.LocalSyntheticContract,
  Wire.LocalSyntheticContract
>;
export type LocalWeeklyDayFromWire = Compatible<Wire.LocalWeeklyDay, Projection.LocalWeeklyDay>;
export type LocalWeeklyDayToWire = Compatible<Projection.LocalWeeklyDay, Wire.LocalWeeklyDay>;
export type LocalWeeklyHoursFromWire = Compatible<
  Wire.LocalWeeklyHours,
  Projection.LocalWeeklyHours
>;
export type LocalWeeklyHoursToWire = Compatible<Projection.LocalWeeklyHours, Wire.LocalWeeklyHours>;
export type LocalWeeklyHoursEnvelopeFromWire = Compatible<
  Wire.LocalWeeklyHoursEnvelope,
  Projection.LocalWeeklyHoursEnvelope
>;
export type LocalWeeklyHoursEnvelopeToWire = Compatible<
  Projection.LocalWeeklyHoursEnvelope,
  Wire.LocalWeeklyHoursEnvelope
>;
export type LocalWeeklyPeriodFromWire = Compatible<
  Wire.LocalWeeklyPeriod,
  Projection.LocalWeeklyPeriod
>;
export type LocalWeeklyPeriodToWire = Compatible<
  Projection.LocalWeeklyPeriod,
  Wire.LocalWeeklyPeriod
>;
export type LoginResponseFromWire = Compatible<Wire.LoginResponse, Projection.LoginResponse>;
export type LoginResponseToWire = Compatible<Projection.LoginResponse, Wire.LoginResponse>;
export type ManualBlockFromWire = Compatible<Wire.ManualBlock, Projection.ManualBlock>;
export type ManualBlockToWire = Compatible<Projection.ManualBlock, Wire.ManualBlock>;
export type ManualBlockListFromWire = Compatible<Wire.ManualBlockList, Projection.ManualBlockList>;
export type ManualBlockListToWire = Compatible<Projection.ManualBlockList, Wire.ManualBlockList>;
export type OwnIdentityExportFromWire = Compatible<
  Wire.OwnIdentityExport,
  Projection.OwnIdentityExport
>;
export type OwnIdentityExportToWire = Compatible<
  Projection.OwnIdentityExport,
  Wire.OwnIdentityExport
>;
export type PriceSimulationFromWire = Compatible<Wire.PriceSimulation, Projection.PriceSimulation>;
export type PriceSimulationToWire = Compatible<Projection.PriceSimulation, Wire.PriceSimulation>;
export type PriceSimulationInputFromWire = Compatible<
  Wire.PriceSimulationInput,
  Projection.PriceSimulationInput
>;
export type PriceSimulationInputToWire = Compatible<
  Projection.PriceSimulationInput,
  Wire.PriceSimulationInput
>;
export type ProfileFromWire = Compatible<Wire.Profile, Projection.Profile>;
export type ProfileToWire = Compatible<Projection.Profile, Wire.Profile>;
export type ProfileUpdateFromWire = Compatible<Wire.ProfileUpdate, Projection.ProfileUpdate>;
export type ProfileUpdateToWire = Compatible<Projection.ProfileUpdate, Wire.ProfileUpdate>;
export type PublishedSpaceContentInputFromWire = Compatible<
  Wire.PublishedSpaceContentInput,
  Projection.PublishedSpaceContentInput
>;
export type PublishedSpaceContentInputToWire = Compatible<
  Projection.PublishedSpaceContentInput,
  Wire.PublishedSpaceContentInput
>;
export type RateFromWire = Compatible<Wire.Rate, Projection.Rate>;
export type RateToWire = Compatible<Projection.Rate, Wire.Rate>;
export type RateInputFromWire = Compatible<Wire.RateInput, Projection.RateInput>;
export type RateInputToWire = Compatible<Projection.RateInput, Wire.RateInput>;
export type RightsRequestFromWire = Compatible<Wire.RightsRequest, Projection.RightsRequest>;
export type RightsRequestToWire = Compatible<Projection.RightsRequest, Wire.RightsRequest>;
export type RightsRequestListFromWire = Compatible<
  Wire.RightsRequestList,
  Projection.RightsRequestList
>;
export type RightsRequestListToWire = Compatible<
  Projection.RightsRequestList,
  Wire.RightsRequestList
>;
export type SessionResponseFromWire = Compatible<Wire.SessionResponse, Projection.SessionResponse>;
export type SessionResponseToWire = Compatible<Projection.SessionResponse, Wire.SessionResponse>;
export type SpaceAttributeProfileFromWire = Compatible<
  Wire.SpaceAttributeProfile,
  Projection.SpaceAttributeProfile
>;
export type SpaceAttributeProfileToWire = Compatible<
  Projection.SpaceAttributeProfile,
  Wire.SpaceAttributeProfile
>;
export type SpaceCategoryFromWire = Compatible<Wire.SpaceCategory, Projection.SpaceCategory>;
export type SpaceCategoryToWire = Compatible<Projection.SpaceCategory, Wire.SpaceCategory>;
export type SpaceCategoryListFromWire = Compatible<
  Wire.SpaceCategoryList,
  Projection.SpaceCategoryList
>;
export type SpaceCategoryListToWire = Compatible<
  Projection.SpaceCategoryList,
  Wire.SpaceCategoryList
>;
export type SpaceDeletionReceiptFromWire = Compatible<
  Wire.SpaceDeletionReceipt,
  Projection.SpaceDeletionReceipt
>;
export type SpaceDeletionReceiptToWire = Compatible<
  Projection.SpaceDeletionReceipt,
  Wire.SpaceDeletionReceipt
>;
export type SpaceDraftFromWire = Compatible<Wire.SpaceDraft, Projection.SpaceDraft>;
export type SpaceDraftToWire = Compatible<Projection.SpaceDraft, Wire.SpaceDraft>;
export type SpaceDraftInputFromWire = Compatible<Wire.SpaceDraftInput, Projection.SpaceDraftInput>;
export type SpaceDraftInputToWire = Compatible<Projection.SpaceDraftInput, Wire.SpaceDraftInput>;
export type SpaceDraftListFromWire = Compatible<Wire.SpaceDraftList, Projection.SpaceDraftList>;
export type SpaceDraftListToWire = Compatible<Projection.SpaceDraftList, Wire.SpaceDraftList>;
export type SpacePublicationInputFromWire = Compatible<
  Wire.SpacePublicationInput,
  Projection.SpacePublicationInput
>;
export type SpacePublicationInputToWire = Compatible<
  Projection.SpacePublicationInput,
  Wire.SpacePublicationInput
>;
export type SyntheticEvidenceFromWire = Compatible<
  Wire.SyntheticEvidence,
  Projection.SyntheticEvidence
>;
export type SyntheticEvidenceToWire = Compatible<
  Projection.SyntheticEvidence,
  Wire.SyntheticEvidence
>;
export type SyntheticEvidenceListFromWire = Compatible<
  Wire.SyntheticEvidenceList,
  Projection.SyntheticEvidenceList
>;
export type SyntheticEvidenceListToWire = Compatible<
  Projection.SyntheticEvidenceList,
  Wire.SyntheticEvidenceList
>;
export type SyntheticSpaceCoverFromWire = Compatible<
  Wire.SyntheticSpaceCover,
  Projection.SyntheticSpaceCover
>;
export type SyntheticSpaceCoverToWire = Compatible<
  Projection.SyntheticSpaceCover,
  Wire.SyntheticSpaceCover
>;
export type SyntheticSpaceGalleryPhotoFromWire = Compatible<
  Wire.SyntheticSpaceGalleryPhoto,
  Projection.SyntheticSpaceGalleryPhoto
>;
export type SyntheticSpaceGalleryPhotoToWire = Compatible<
  Projection.SyntheticSpaceGalleryPhoto,
  Wire.SyntheticSpaceGalleryPhoto
>;
export type SyntheticSpaceGalleryRemovalFromWire = Compatible<
  Wire.SyntheticSpaceGalleryRemoval,
  Projection.SyntheticSpaceGalleryRemoval
>;
export type SyntheticSpaceGalleryRemovalToWire = Compatible<
  Projection.SyntheticSpaceGalleryRemoval,
  Wire.SyntheticSpaceGalleryRemoval
>;
export type TermsResponseFromWire = Compatible<Wire.TermsResponse, Projection.TermsResponse>;
export type TermsResponseToWire = Compatible<Projection.TermsResponse, Wire.TermsResponse>;
export type UtcTimestampFromWire = Compatible<Wire.UtcTimestamp, Projection.UtcTimestamp>;
export type UtcTimestampToWire = Compatible<Projection.UtcTimestamp, Wire.UtcTimestamp>;
export type VerificationCaseFromWire = Compatible<
  Wire.VerificationCase,
  Projection.VerificationCase
>;
export type VerificationCaseToWire = Compatible<Projection.VerificationCase, Wire.VerificationCase>;
export type VerificationEligibilityListFromWire = Compatible<
  Wire.VerificationEligibilityList,
  Projection.VerificationEligibilityList
>;
export type VerificationEligibilityListToWire = Compatible<
  Projection.VerificationEligibilityList,
  Wire.VerificationEligibilityList
>;
export type VerificationHistoryListFromWire = Compatible<
  Wire.VerificationHistoryList,
  Projection.VerificationHistoryList
>;
export type VerificationHistoryListToWire = Compatible<
  Projection.VerificationHistoryList,
  Wire.VerificationHistoryList
>;
export type VerificationListFromWire = Compatible<
  Wire.VerificationList,
  Projection.VerificationList
>;
export type VerificationListToWire = Compatible<Projection.VerificationList, Wire.VerificationList>;
