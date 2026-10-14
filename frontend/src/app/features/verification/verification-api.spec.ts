import { provideHttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { M03Local } from '../../../generated/api/sdk.gen';
import { VerificationApi, VerificationApiError } from './infrastructure/verification-api';

describe('VerificationApi.deleteEvidence', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('calls only the owner DELETE operation with the current bearer and returns on 204', async () => {
    const deleteOwnSyntheticEvidence = vi.fn().mockResolvedValue({
      data: undefined,
      error: undefined,
      response: { status: 204 },
    });
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(),
        VerificationApi,
        { provide: M03Local, useValue: { deleteOwnSyntheticEvidence } },
      ],
    });
    await TestBed.inject(VerificationApi).deleteEvidence('bearer-memory-only', 'case/1', 'evidence?2');
    expect(deleteOwnSyntheticEvidence).toHaveBeenCalledWith({
      path: { id: 'case/1', evidenceId: 'evidence?2' },
      headers: { Authorization: 'Bearer bearer-memory-only' },
    });
  });

  it.each([401, 403, 404])('preserves Go’s DELETE %i status for the facade', async (status) => {
    const deleteOwnSyntheticEvidence = vi.fn().mockResolvedValue({
      data: undefined,
      error: { message: 'private response' },
      response: { status },
    });
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(),
        VerificationApi,
        { provide: M03Local, useValue: { deleteOwnSyntheticEvidence } },
      ],
    });
    await expect(
      TestBed.inject(VerificationApi).deleteEvidence('token', 'case', 'evidence'),
    ).rejects.toMatchObject({ name: 'VerificationApiError', status } satisfies Partial<VerificationApiError>);
  });
});
