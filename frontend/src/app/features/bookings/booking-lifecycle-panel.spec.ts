import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { LocalBookingDetail, LocalRentalOperation, LocalSyntheticContract } from '../../../generated/api/types.gen';
import { IdentityApi } from '../../core/identity/identity-api';
import { IdentitySession } from '../../core/identity/identity-session';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { BookingLifecycleApi, BookingLifecycleAttempts } from './booking-lifecycle-api';
import { BookingLifecyclePanel } from './booking-lifecycle-panel';

const reservation = (state: string, renter = 'renter-a', host = 'host-a') => ({
  id: 'reservation-a', renter_id: renter, host_id: host, state, time_zone: 'America/Santiago',
}) as unknown as LocalBookingDetail;
const login = (account_id: string, access_token: string) => ({ account_id, roles: ['arrendatario'], access_token, token_type: 'Bearer', expires_at: '2027-01-01T00:00:00Z', restricted_mode: false });
const envelope = <T>(data: T) => ({ data, notice: 'ENSAYO LOCAL — SIN VALIDEZ JURÍDICA' });
const deferred = <T>() => { let resolve!: (value: T) => void; const promise = new Promise<T>((done) => { resolve = done; }); return { promise, resolve }; };
const contract = (signer = 'renter-a', state: 'pendiente' | 'firmada' = 'pendiente') => ({
  id: 'contract-a', reservation_id: 'reservation-a', version: 1, state: 'generado', snapshot: '{}', sha256: 'a'.repeat(64),
  signatures: [{ signer_id: signer, role: signer === 'host-a' ? 'anfitrion' : 'arrendatario', state, updated_at: '2026-10-12T00:00:00Z' }], history: [],
}) as unknown as LocalSyntheticContract;
const operation = (): LocalRentalOperation => ({
  id: 'operation-a', reservation_id: 'reservation-a', kind: 'checkin', actor_id: 'renter-a', occurred_at: '2026-10-12T00:00:00Z', time_zone: 'America/Santiago',
  synthetic_location: { source: 'synthetic-fixture-v1', location_code: 'santiago-demo-center-v1', latitude: -33.456, longitude: -70.6693 },
  result: 'registrada', sequence: 1, evidence: [{ id: 'evidence-a', fixture_code: 'synthetic-png-v1', mime_type: 'image/png', sha256: 'b'.repeat(64), size_bytes: 100, created_at: '2026-10-12T00:00:00Z' }],
});

function setup(account = 'renter-a', apiOverrides: Record<string, ReturnType<typeof vi.fn>> = {}) {
  const identityApi = {
    login: vi.fn(async (email: string) => email.startsWith('host') ? login('host-a', 'token-host') : email.startsWith('third') ? login('third-party', 'token-third') : login('renter-a', 'token-renter')),
    session: vi.fn(async (token: string) => ({ account_id: token === 'token-host' ? 'host-a' : token === 'token-third' ? 'third-party' : 'renter-a', roles: ['arrendatario'], restricted_mode: false })),
    logout: vi.fn(async () => undefined),
  };
  const api = {
    createOrGetContract: vi.fn(async () => envelope(contract())), getContract: vi.fn(), signContract: vi.fn(async () => envelope({ ...contract('renter-a', 'firmada'), state: 'firma_parcial' })), rejectContract: vi.fn(), contractPdf: vi.fn(async () => new Blob(['%PDF synthetic'], { type: 'application/pdf' })),
    operations: vi.fn(async () => ({ data: { items: [] }, notice: 'ENSAYO LOCAL — EVIDENCIA SINTÉTICA; SIN VALIDEZ PROBATORIA REAL' })),
    record: vi.fn(async () => envelope(operation())), evidence: vi.fn(async () => new Blob(['PNG synthetic'], { type: 'image/png' })),
    ...apiOverrides,
  };
  TestBed.configureTestingModule({ imports: [BookingLifecyclePanel], providers: [provideRouter([]), PrivateSessionState, { provide: IdentityApi, useValue: identityApi }, { provide: BookingLifecycleApi, useValue: api }] });
  const fixture = TestBed.createComponent(BookingLifecyclePanel);
  fixture.componentRef.setInput('reservation', reservation('aprobada_host'));
  fixture.detectChanges();
  return { fixture, identity: TestBed.inject(IdentitySession), api, attempts: TestBed.inject(BookingLifecycleAttempts), account };
}
type View = {
  openContract(): Promise<void>; downloadPdf(): Promise<void>; sign(): Promise<void>; reject(): Promise<void>; record(action: 'check-in' | 'check-out' | 'reception'): Promise<void>; loadEvidence(id: string): Promise<void>;
  contract(): LocalSyntheticContract | null; operations(): LocalRentalOperation[]; evidenceUrls(): Record<string, string>; error(): string; canOpenContract(): boolean; canCheckIn(): boolean;
};
const view = (fixture: ReturnType<typeof TestBed.createComponent>) => fixture.componentInstance as unknown as View;
async function settle(fixture: ReturnType<typeof TestBed.createComponent>) { await Promise.resolve(); await Promise.resolve(); fixture.detectChanges(); await fixture.whenStable(); fixture.detectChanges(); }

describe('BookingLifecyclePanel with real IdentitySession', () => {
  afterEach(() => { vi.restoreAllMocks(); TestBed.resetTestingModule(); });

  it('lets an authenticated participant generate and sign the Go contract', async () => {
    const { fixture, identity, api } = setup();
    await identity.signIn('renter@example.invalid', 'synthetic'); await settle(fixture);
    const panel = view(fixture); await panel.openContract(); await settle(fixture);
    expect(api.createOrGetContract).toHaveBeenCalledWith('token-renter', 'reservation-a');
    expect(panel.contract()?.id).toBe('contract-a');
    await panel.sign(); await settle(fixture);
    expect(api.signContract).toHaveBeenCalledWith('token-renter', 'contract-a');
    expect(panel.contract()?.signatures[0]?.state).toBe('firmada');
    expect(fixture.nativeElement.textContent).toContain('Firma parcial');
    fixture.destroy();
  });

  it('does not offer a contract to a third party even if that person has an account role', async () => {
    const { fixture, identity, api } = setup();
    await identity.signIn('third@example.invalid', 'synthetic'); await settle(fixture);
    expect(view(fixture).canOpenContract()).toBe(false);
    expect(fixture.nativeElement.textContent).not.toContain('Generar o consultar contrato');
    expect(api.createOrGetContract).not.toHaveBeenCalled();
    fixture.destroy();
  });

  it('supports the host participant and uses the Go-confirmed reception state', async () => {
    const contractWithTwoSignatures = { ...contract('host-a'), signatures: [
      { signer_id: 'renter-a', role: 'arrendatario', state: 'pendiente', updated_at: '2026-10-12T00:00:00Z' },
      { signer_id: 'host-a', role: 'anfitrion', state: 'pendiente', updated_at: '2026-10-12T00:00:00Z' },
    ] } as unknown as LocalSyntheticContract;
    const { fixture, identity, api } = setup('host-a', {
      createOrGetContract: vi.fn(async () => envelope(contractWithTwoSignatures)),
      signContract: vi.fn(async () => envelope({ ...contractWithTwoSignatures, state: 'firma_parcial', signatures: [...contractWithTwoSignatures.signatures.slice(0, 1), { ...contractWithTwoSignatures.signatures[1], state: 'firmada' }] })),
    });
    await identity.signIn('host@example.invalid', 'synthetic'); await settle(fixture);
    const panel = view(fixture); await panel.openContract(); await settle(fixture); await panel.sign(); await settle(fixture);
    expect(api.signContract).toHaveBeenCalledWith('token-host', 'contract-a');
    fixture.componentRef.setInput('reservation', reservation('finalizada')); fixture.detectChanges(); await settle(fixture);
    expect((panel as unknown as { canReceive(): boolean }).canReceive()).toBe(true);
    await panel.record('reception'); await settle(fixture);
    expect(api.record).toHaveBeenCalledWith('token-host', 'reservation-a', 'reception', { observations: '' }, expect.any(String));
    fixture.destroy();
  });

  it('discards an old operations response after logout/login and revokes private evidence URLs', async () => {
    const oldRead = deferred<{ data: { items: LocalRentalOperation[] }; notice: string }>();
    const apiOverrides = { operations: vi.fn(() => oldRead.promise) };
    const { fixture, identity, api } = setup('renter-a', apiOverrides);
    const createUrl = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:private-evidence');
    const revokeUrl = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined);
    await identity.signIn('renter@example.invalid', 'synthetic'); await settle(fixture);
    expect(api.operations).toHaveBeenCalledTimes(1);
    await identity.signOut(); await identity.signIn('third@example.invalid', 'synthetic'); await settle(fixture);
    oldRead.resolve({ data: { items: [operation()] }, notice: 'private' }); await settle(fixture);
    expect(view(fixture).operations()).toEqual([]);
    expect(api.evidence).not.toHaveBeenCalled();

    await identity.signOut(); await identity.signIn('renter@example.invalid', 'synthetic'); await settle(fixture);
    (api.operations as ReturnType<typeof vi.fn>).mockResolvedValue({ data: { items: [operation()] }, notice: 'private' });
    await (view(fixture) as unknown as { loadOperations(): Promise<void> }).loadOperations(); await settle(fixture);
    await view(fixture).loadEvidence('evidence-a'); await settle(fixture);
    expect(createUrl).toHaveBeenCalled();
    expect(view(fixture).evidenceUrls()['evidence-a']).toBe('blob:private-evidence');
    await identity.signOut(); await settle(fixture);
    expect(revokeUrl).toHaveBeenCalledWith('blob:private-evidence');
    expect(view(fixture).evidenceUrls()).toEqual({});
    fixture.destroy();
  });

  it('revokes a private contract PDF URL when the participant session ends', async () => {
    const signedContract = { ...contract('renter-a', 'firmada'), state: 'firmado', signatures: [
      { signer_id: 'renter-a', role: 'arrendatario', state: 'firmada', updated_at: '2026-10-12T00:00:00Z' },
      { signer_id: 'host-a', role: 'anfitrion', state: 'firmada', updated_at: '2026-10-12T00:00:00Z' },
    ] } as unknown as LocalSyntheticContract;
    const { fixture, identity, api } = setup('renter-a', {
      createOrGetContract: vi.fn(async () => envelope(signedContract)),
    });
    const createUrl = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:private-contract');
    const revokeUrl = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined);
    await identity.signIn('renter@example.invalid', 'synthetic'); await settle(fixture);
    await view(fixture).openContract(); await settle(fixture);
    await view(fixture).downloadPdf(); await settle(fixture);
    expect(api.contractPdf).toHaveBeenCalledWith('token-renter', 'contract-a');
    expect(createUrl).toHaveBeenCalled();
    await identity.signOut(); await settle(fixture);
    expect(revokeUrl).toHaveBeenCalledWith('blob:private-contract');
    fixture.destroy();
  });

  it('keeps an uncertain operation key and payload, and rejects a changed payload until Go confirms it', async () => {
    const { fixture, identity, api, attempts } = setup();
    (api.record as ReturnType<typeof vi.fn>).mockRejectedValueOnce(new Error('network lost'));
    await identity.signIn('renter@example.invalid', 'synthetic'); await settle(fixture);
    fixture.componentRef.setInput('reservation', reservation('en_curso')); fixture.detectChanges(); await settle(fixture);
    const panel = view(fixture); await panel.record('check-out'); await settle(fixture);
    const first = attempts.find('renter-a', 'reservation-a', 'checkout');
    expect(first?.payload).toBe('{"comments":""}');
    expect(panel.error()).toContain('incierta');
    await identity.signOut(); await identity.signIn('renter@example.invalid', 'synthetic'); await settle(fixture);
    fixture.componentRef.setInput('reservation', reservation('en_curso')); fixture.detectChanges(); await settle(fixture);
    await panel.record('check-out'); await settle(fixture);
    expect((api.record as ReturnType<typeof vi.fn>).mock.calls[1]?.[4]).toBe(first?.key);
    fixture.destroy();
  });
});
