import type { BackendOperation, ReservationFinanceResource } from '../contracts/operations';
const resources: Record<ReservationFinanceResource, string> = {
  incident: '/disputes',
  claim: '/damage-claim',
  claimDefense: '/damage-claim/defense',
  defenseEvidence: '/damage-claim/defense/evidence',
  guarantee: '/guarantee',
  settlement: '/settlement',
  receipt: '/receipt',
  guaranteeReconcile: '/guarantee-reconcile',
  settlementReconcile: '/settlement-reconcile',
};
const paths: Record<BackendOperation['kind'], string> = {
  verificationEvidenceList: '/admin/verifications/{verificationId}/evidence',
  verificationEvidence: '/admin/verifications/{verificationId}/evidence/{evidenceId}',
  lookupAccount: '/admin/local/accounts/{accountId}',
  reservationSummary: '/admin/local/reservations/{reservationId}',
  publishedSpaceSummary: '/admin/local/published-spaces/{spaceId}',
  purgeRetention: '/privacy/retention/purge',
  suppressionAction: '/privacy/suppression-requests/{requestId}/{action}',
  administrativeReport: '/admin/local/reports/{report}',
  auditExport: '/admin/local/audit-events/export',
  rejectedVerifications: '/admin/verifications/failed',
  pendingVerifications: '/admin/verifications',
  suppressionRequests: '/privacy/suppression-requests',
  reviewReports: '/admin/local/review-reports',
  localNotices: '/admin/local/notices',
  credentialNotices: '/admin/credential-notices',
  accounts: '/admin/local/accounts',
  publishedSpaces: '/admin/local/published-spaces',
  reservations: '/admin/local/reservations',
  auditEvents: '/admin/local/audit-events',
  reservationsReport: '/admin/local/reports/reservations',
  reviewVerification: '/admin/verifications/{verificationId}/review',
  revokeVerification: '/admin/verifications/{verificationId}/revoke',
  reviewSuppression: '/privacy/suppression-requests/{requestId}/review',
  executeSuppression: '/privacy/suppression-requests/{requestId}/execute',
  moderateReviewReport: '/admin/local/review-reports/{reviewReportId}/{action}',
  reopenLocalNotice: '/admin/local/notices/{noticeId}/reopen',
  reopenCredentialNotice: '/admin/credential-notices/{noticeId}/reopen',
  blockAccount: '/admin/local/accounts/{accountId}/block',
  unblockAccount: '/admin/local/accounts/{accountId}/unblock',
  openDisputes: '/admin/disputes',
  damageClaims: '/admin/local/damage-claims',
  reservationGuarantee: '/local/booking-trial/reservations/{reservationId}/guarantee',
  reservationSettlement: '/local/booking-trial/reservations/{reservationId}/settlement',
  reservationReceipt: '/local/booking-trial/reservations/{reservationId}/receipt',
  decideReservationFinance: '/local/booking-trial/reservations/{reservationId}/financial-decision',
  closeDispute: '/admin/disputes/{disputeId}/close',
  claimEvidence: '/admin/local/damage-claims/{claimId}/evidence/{evidenceId}',
  reservationEvidence: '/local/booking-trial/reservations/{reservationId}/evidence/{evidenceId}',
  resolveClaim: '/admin/local/damage-claims/{claimId}/resolution',
  reservationFinance: '/local/booking-trial/reservations/{reservationId}{resource}',
};
/** The only mapping from application resource names to REST routes. */
export function operationPath(operation: BackendOperation): string {
  const values = operation.params as Record<string, string>;
  return paths[operation.kind].replace(/\{(\w+)\}/g, (_, key: string) => {
    if (operation.kind === 'reservationFinance' && key === 'resource')
      return resources[values[key] as ReservationFinanceResource];
    return encodeURIComponent(values[key] ?? '');
  });
}
export function operationQuery(query?: Record<string, string | number | undefined>): string {
  const values = Object.entries(query ?? {}).filter(
    (entry): entry is [string, string | number] => entry[1] !== undefined,
  );
  return values.length
    ? '?' + new URLSearchParams(values.map(([key, value]) => [key, String(value)])).toString()
    : '';
}
