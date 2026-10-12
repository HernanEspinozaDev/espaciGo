/** Semantic resources/commands used by admin and finance application ports. No URLs or HTTP dependencies. */
export type ReservationFinanceResource =
  | 'incident'
  | 'claim'
  | 'claimDefense'
  | 'defenseEvidence'
  | 'guarantee'
  | 'settlement'
  | 'receipt'
  | 'guaranteeReconcile'
  | 'settlementReconcile';
export interface OperationParameters {
  verificationEvidenceList: { verificationId: string };
  verificationEvidence: { verificationId: string; evidenceId: string };
  lookupAccount: { accountId: string };
  reservationSummary: { reservationId: string };
  publishedSpaceSummary: { spaceId: string };
  purgeRetention: {};
  suppressionAction: { requestId: string; action: string };
  administrativeReport: { report: string };
  auditExport: {};
  rejectedVerifications: {};
  pendingVerifications: {};
  suppressionRequests: {};
  reviewReports: {};
  localNotices: {};
  credentialNotices: {};
  accounts: {};
  publishedSpaces: {};
  reservations: {};
  auditEvents: {};
  reservationsReport: {};
  reviewVerification: { verificationId: string };
  revokeVerification: { verificationId: string };
  reviewSuppression: { requestId: string };
  executeSuppression: { requestId: string };
  moderateReviewReport: { reviewReportId: string; action: string };
  reopenLocalNotice: { noticeId: string };
  reopenCredentialNotice: { noticeId: string };
  blockAccount: { accountId: string };
  unblockAccount: { accountId: string };
  openDisputes: {};
  damageClaims: {};
  reservationGuarantee: { reservationId: string };
  reservationSettlement: { reservationId: string };
  reservationReceipt: { reservationId: string };
  decideReservationFinance: { reservationId: string };
  closeDispute: { disputeId: string };
  claimEvidence: { claimId: string; evidenceId: string };
  reservationEvidence: { reservationId: string; evidenceId: string };
  resolveClaim: { claimId: string };
  reservationFinance: { reservationId: string; resource: ReservationFinanceResource };
}
export type OperationKind = keyof OperationParameters;
export type BackendOperation = {
  [K in OperationKind]: { kind: K; params: OperationParameters[K] };
}[OperationKind];
export function backendOperation<K extends OperationKind>(
  kind: K,
  params: OperationParameters[K] = {} as OperationParameters[K],
): BackendOperation {
  return { kind, params } as BackendOperation;
}
