import { describe, expect, it } from 'vitest';
import { backendOperation } from '../contracts/operations';
import { operationPath, operationQuery } from './operation-path';

describe('semantic operation boundary', () => {
  it('encodes resource IDs exclusively in the HTTP mapping', () => {
    expect(
      operationPath(
        backendOperation('verificationEvidence', {
          verificationId: 'case/one',
          evidenceId: 'private?two',
        }),
      ),
    ).toBe('/admin/verifications/case%2Fone/evidence/private%3Ftwo');
  });
  it('maps finance resources without making callers construct HTTP paths', () => {
    expect(
      operationPath(
        backendOperation('reservationFinance', {
          reservationId: 'reservation-a',
          resource: 'claimDefense',
        }),
      ),
    ).toBe('/local/booking-trial/reservations/reservation-a/damage-claim/defense');
    expect(operationQuery({ cursor: 'opaque+cursor', limit: 25, absent: undefined })).toBe(
      '?cursor=opaque%2Bcursor&limit=25',
    );
  });
});
