import type {
  SyntheticEvidence,
  VerificationCase,
  VerificationEligibilityList,
  VerificationHistoryList,
  VerificationList,
} from '../../../../core/contracts/models';

export class VerificationApiError extends Error {
  constructor(readonly status?: number) {
    super(
      status === 404
        ? 'El caso o la evidencia ya no están disponibles.'
        : 'Go no pudo completar la solicitud.',
    );
    this.name = 'VerificationApiError';
  }
}
export function verificationStatus(error: unknown): number | undefined {
  return (error as { status?: number })?.status;
}
export abstract class VerificationPort {
  abstract list(token: string): Promise<VerificationList>;
  abstract eligibility(token: string): Promise<VerificationEligibilityList>;
  abstract detail(token: string, id: string): Promise<VerificationCase>;
  abstract history(token: string, id: string): Promise<VerificationHistoryList>;
  abstract evidence(token: string, id: string): Promise<SyntheticEvidence[]>;
  abstract create(token: string, type: 'kyc' | 'kyb', key: string): Promise<VerificationCase>;
  abstract retry(
    token: string,
    id: string,
    correction_code:
      | 'fixture_vigente_actualizado'
      | 'antecedentes_fixture_actualizados'
      | 'inicio_actividades_fixture_actualizadas',
    key: string,
  ): Promise<VerificationCase>;
  abstract generateEvidence(token: string, id: string, key: string): Promise<SyntheticEvidence>;
  abstract content(token: string, id: string, evidenceId: string): Promise<Blob>;
  abstract deleteEvidence(token: string, id: string, evidenceId: string): Promise<void>;
}
