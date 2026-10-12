import { By } from '@angular/platform-browser';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { LocalBookingDetail, LocalReservationMessage, LocalReservationMessagesPageEnvelope, LocalReview } from '../../../generated/api/types.gen';
import { IdentityApi } from '../../core/identity/identity-api';
import { IdentitySession } from '../../core/identity/identity-session';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { BookingApi, BookingApiError } from './booking-api';
import { BookingCommunicationApi, BookingCommunicationAttempts } from './booking-communication-api';
import { BookingConversationPanel } from './booking-conversation-panel';

const reservation = (state = 'lista_para_checkin') => ({
  id: 'reservation-r', space_id: 'space-r', renter_id: 'renter-a', host_id: 'host-a', state, time_zone: 'America/Santiago', unread_count: 4,
}) as unknown as LocalBookingDetail;
const message = (sequence: number, body = `message ${sequence}`, author = 'host-a'): LocalReservationMessage => ({
  id: `message-${sequence}`, reservation_id: 'reservation-r', author_id: author, sequence, body, created_at: '2026-10-12T10:00:00Z',
});
const page = (items: LocalReservationMessage[], older_cursor: number | null): LocalReservationMessagesPageEnvelope => ({
  safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL', data: { items, older_cursor },
});
const deferred = <T>() => { let resolve!: (value: T) => void; let reject!: (error: unknown) => void; const promise = new Promise<T>((ok, fail) => { resolve = ok; reject = fail; }); return { promise, resolve, reject }; };
const settle = async (fixture: ReturnType<typeof TestBed.createComponent>) => { for (let i = 0; i < 6; i++) await Promise.resolve(); fixture.detectChanges(); await fixture.whenStable(); fixture.detectChanges(); };
type ComponentAccess = {
  messages(): LocalReservationMessage[]; olderCursor(): number | null; olderLoading(): boolean; loadOlder(): Promise<void>; send(): Promise<void>;
  body: { set(value: string): void; (): string }; reviewComment: { set(value: string): void; (): string };
  writable(): boolean; readThrough(): number; unreadCount(): number; submitReview(): Promise<void>; report(review: LocalReview): Promise<void>;
  loadSpaceReviews(): Promise<void>; spaceReviews(): { items: LocalReview[]; average: number; count: number } | null;
};

function setup(apiOverrides: Record<string, ReturnType<typeof vi.fn>> = {}, state = 'lista_para_checkin', unreadCounts = [4, 0, 0]) {
  const identityApi = {
    login: vi.fn(async (email: string) => ({ account_id: email.startsWith('host') ? 'host-a' : email.startsWith('third') ? 'third-party' : 'renter-a', roles: ['arrendatario'], access_token: email.startsWith('host') ? 'token-host' : email.startsWith('third') ? 'token-third' : 'token-renter', token_type: 'Bearer', expires_at: '2027-01-01T00:00:00Z', restricted_mode: false })),
    session: vi.fn(async (token: string) => ({ account_id: token === 'token-host' ? 'host-a' : token === 'token-third' ? 'third-party' : 'renter-a', roles: ['arrendatario'], restricted_mode: false })),
    logout: vi.fn(async () => undefined),
  };
  const api = {
    messages: vi.fn(async () => page([], null)), sendMessage: vi.fn(async () => ({ safety_notice: 'ENSAYO', data: message(1, 'sent', 'renter-a') })),
    advanceRead: vi.fn(async (_token: string, _id: string, through: number) => ({ data: { read_through_sequence: through }, safety_notice: 'ENSAYO' })),
    reservationReviews: vi.fn(async () => ({ data: { items: [] }, notice: 'ENSAYO' })),
    createReview: vi.fn(async () => ({ data: { id: 'review-r', target_type: 'espacio', rating: 5, state: 'publicada', created_at: '2026-10-12T10:00:00Z' }, notice: 'ENSAYO' })),
    reportReview: vi.fn(async () => ({ data: { id: 'report-r', state: 'pendiente' }, notice: 'ENSAYO' })),
    spaceReviews: vi.fn(async () => ({ data: { items: [], average: 0, count: 0 }, notice: 'ENSAYO' })),
    ownReputation: vi.fn(async () => ({ data: { average: 0, count: 0 }, notice: 'ENSAYO' })),
    ...apiOverrides,
  };
  const bookingApi = { list: vi.fn(async () => ({ safety_notice: 'ENSAYO', data: { items: [{ ...reservation(state), unread_count: unreadCounts.shift() ?? 0 }] } })) };
  TestBed.configureTestingModule({ imports: [BookingConversationPanel], providers: [provideRouter([]), PrivateSessionState,
    { provide: IdentityApi, useValue: identityApi }, { provide: BookingCommunicationApi, useValue: api }, { provide: BookingApi, useValue: bookingApi }] });
  const fixture = TestBed.createComponent(BookingConversationPanel);
  fixture.componentRef.setInput('reservation', reservation(state)); fixture.detectChanges();
  return { fixture, identity: TestBed.inject(IdentitySession), api, bookingApi, panel: () => fixture.componentInstance as unknown as ComponentAccess };
}

describe('BookingConversationPanel with real IdentitySession', () => {
  afterEach(() => { TestBed.resetTestingModule(); vi.restoreAllMocks(); });

  it('discards a renter page that resolves after switching to the host account without remounting', async () => {
    const renterPage = deferred<LocalReservationMessagesPageEnvelope>();
    const test = setup({ messages: vi.fn((token: string) => token === 'token-renter' ? renterPage.promise : Promise.resolve(page([message(2, 'host session content')], null))) });
    const mounted = test.fixture;
    await test.identity.signIn('renter@example.invalid', 'synthetic'); await settle(test.fixture);
    expect(test.api.messages).toHaveBeenCalledWith('token-renter', 'reservation-r');
    await test.identity.signOut(); await test.identity.signIn('host@example.invalid', 'synthetic'); await settle(test.fixture);
    renterPage.resolve(page([message(1, 'private stale renter text')], null)); await settle(test.fixture);
    expect(test.fixture).toBe(mounted);
    expect(test.panel().messages().map((item) => item.body)).toEqual(['host session content']);
    expect(test.fixture.nativeElement.textContent).not.toContain('private stale renter text');
    expect(test.identity.accessToken()).toBe('token-host');
    test.fixture.destroy();
  });

  it('does not expire the host session when an old renter page returns 401 late', async () => {
    const old = deferred<LocalReservationMessagesPageEnvelope>();
    const test = setup({ messages: vi.fn((token: string) => token === 'token-renter' ? old.promise : Promise.resolve(page([message(1, 'host session')], null))) });
    await test.identity.signIn('renter@example.invalid', 'synthetic'); await settle(test.fixture);
    await test.identity.signOut(); await test.identity.signIn('host@example.invalid', 'synthetic'); await settle(test.fixture);
    old.reject(new BookingApiError('expired old token', 401)); await settle(test.fixture);
    expect(test.identity.accessToken()).toBe('token-host');
    expect(test.panel().messages().map((item) => item.body)).toEqual(['host session']);
    test.fixture.destroy();
  });

  it('merges older pages in ascending order and keeps a duplicate from an overlapping cursor only once', async () => {
    const test = setup({ messages: vi.fn().mockResolvedValueOnce(page([message(3), message(4)], 3)).mockResolvedValueOnce(page([message(1), message(2), message(3)], null)) });
    await test.identity.signIn('renter@example.invalid', 'synthetic'); await settle(test.fixture);
    expect(test.panel().olderCursor()).toBe(3);
    await test.panel().loadOlder(); await settle(test.fixture);
    expect(test.panel().messages().map((item) => item.sequence)).toEqual([1, 2, 3, 4]);
    expect(test.panel().olderCursor()).toBeNull();
    test.fixture.destroy();
  });

  it('keeps terminal reservations readable but does not render or send a composer', async () => {
    const test = setup({}, 'cerrada');
    await test.identity.signIn('renter@example.invalid', 'synthetic'); await settle(test.fixture);
    expect(test.panel().writable()).toBe(false);
    expect(test.fixture.nativeElement.textContent).toContain('solo lectura');
    expect(test.fixture.nativeElement.querySelector('#message-body')).toBeNull();
    await test.panel().send();
    expect(test.api.sendMessage).not.toHaveBeenCalled();
    test.fixture.destroy();
  });

  it('renders message bodies as escaped text, not active markup', async () => {
    const hostileText = '<img src=x onerror=alert(1)> <script>unsafe()</script>';
    const test = setup({ messages: vi.fn(async () => page([message(1, hostileText)], null)) });
    await test.identity.signIn('renter@example.invalid', 'synthetic'); await settle(test.fixture);
    expect(test.fixture.nativeElement.querySelector('img, script')).toBeNull();
    expect(test.fixture.nativeElement.textContent).toContain(hostileText);
    test.fixture.destroy();
  });

  it('advances only its own read cursor after the latest message has rendered', async () => {
    const test = setup({ messages: vi.fn(async () => page([message(7, 'visible before marking read')], null)) }, 'lista_para_checkin', [4, 0]);
    await test.identity.signIn('renter@example.invalid', 'synthetic'); await settle(test.fixture);
    await new Promise((resolve) => setTimeout(resolve, 10)); await settle(test.fixture);
    expect(test.fixture.nativeElement.querySelector('[data-sequence="7"]')?.textContent).toContain('visible before marking read');
    expect(test.api.advanceRead).toHaveBeenCalledWith('token-renter', 'reservation-r', 7);
    expect(test.panel().readThrough()).toBe(7);
    expect(test.panel().unreadCount()).toBe(0);
    expect(test.bookingApi.list).toHaveBeenCalledTimes(2);
    test.fixture.destroy();
  });

  it('retries from the real enabled button with the original key and payload after a same-account re-login, without leaking the attempt to another account', async () => {
    const first = deferred<{ safety_notice: string; data: LocalReservationMessage }>();
    const sendMessage = vi.fn().mockReturnValueOnce(first.promise).mockResolvedValueOnce({ safety_notice: 'ENSAYO', data: message(5, 'synthetic original', 'renter-a') });
    const test = setup({ sendMessage });
    await test.identity.signIn('renter@example.invalid', 'synthetic'); await settle(test.fixture);
    const composer = test.fixture.nativeElement.querySelector('#message-body') as HTMLTextAreaElement;
    composer.value = 'synthetic original'; composer.dispatchEvent(new Event('input', { bubbles: true })); test.fixture.detectChanges();
    const sendButton = () => [...test.fixture.nativeElement.querySelectorAll('button')].find((button) => button.textContent.includes('Enviar mensaje') || button.textContent.includes('Reintentar mensaje original')) as HTMLButtonElement;
    expect(sendButton().disabled).toBe(false);
    sendButton().click(); const pending = Promise.resolve(); await Promise.resolve();
    first.reject(new Error('network timeout')); await pending; await settle(test.fixture);
    await test.identity.signOut(); await test.identity.signIn('host@example.invalid', 'synthetic'); await settle(test.fixture);
    expect((test.fixture.nativeElement.querySelector('#message-body') as HTMLTextAreaElement).value).toBe('');
    expect(sendButton().textContent).toContain('Enviar mensaje'); expect(sendButton().disabled).toBe(true);
    await test.identity.signOut(); await test.identity.signIn('renter@example.invalid', 'synthetic'); await settle(test.fixture);
    expect((test.fixture.nativeElement.querySelector('#message-body') as HTMLTextAreaElement).value).toBe('');
    expect(sendButton().textContent).toContain('Reintentar mensaje original'); expect(sendButton().disabled).toBe(false);
    sendButton().click(); await settle(test.fixture);
    expect(sendMessage).toHaveBeenCalledTimes(2);
    expect(sendMessage.mock.calls[0]?.slice(0, 3)).toEqual(sendMessage.mock.calls[1]?.slice(0, 3));
    expect(sendMessage.mock.calls[0]?.[2]).toBe('synthetic original');
    expect(sendMessage.mock.calls[0]?.[3]).toBe(sendMessage.mock.calls[1]?.[3]);
    test.fixture.destroy();
  });

  it.each(['success', 'error'] as const)('keeps pagination usable after a concurrent refresh and stale older-page %s', async (oldResult) => {
    const oldPage = deferred<LocalReservationMessagesPageEnvelope>(); const newPage = deferred<LocalReservationMessagesPageEnvelope>();
    const messages = vi.fn()
      .mockResolvedValueOnce(page([message(5)], 5))
      .mockReturnValueOnce(oldPage.promise)
      .mockResolvedValueOnce(page([message(6)], 6))
      .mockReturnValueOnce(newPage.promise);
    const test = setup({ messages }); await test.identity.signIn('renter@example.invalid', 'synthetic'); await settle(test.fixture);
    const older = () => test.fixture.nativeElement.querySelector('button') as HTMLButtonElement;
    older().click(); await Promise.resolve(); expect(test.panel().olderLoading()).toBe(true);
    const refresh = [...test.fixture.nativeElement.querySelectorAll('button')].find((button) => button.textContent.includes('Actualizar conversación')) as HTMLButtonElement;
    refresh.click(); await settle(test.fixture);
    expect(test.panel().olderLoading()).toBe(false);
    older().click(); await Promise.resolve(); expect(test.panel().olderLoading()).toBe(true);
    if (oldResult === 'success') oldPage.resolve(page([message(1), message(2)], null));
    else oldPage.reject(new Error('old page failed'));
    await settle(test.fixture); expect(test.panel().olderLoading()).toBe(true);
    expect(test.panel().messages().map((item) => item.sequence)).toEqual([6]);
    newPage.resolve(page([message(1), message(2), message(3), message(4), message(5)], null)); await settle(test.fixture);
    expect(test.panel().olderLoading()).toBe(false);
    expect(test.panel().messages().map((item) => item.sequence)).toEqual([1, 2, 3, 4, 5, 6]);
    expect(test.panel().olderCursor()).toBeNull(); test.fixture.destroy();
  });

  it('lets an eligible participant create one immutable review and still renders its comment as text', async () => {
    const comment = '<img src=x onerror=alert(1)> synthetic review';
    let reviews: LocalReview[] = [];
    const test = setup({
      reservationReviews: vi.fn(async () => ({ data: { items: reviews }, notice: 'ENSAYO' })),
      createReview: vi.fn(async () => {
        const saved = { id: 'review-own', target_type: 'espacio', rating: 5, comment, state: 'publicada', created_at: '2026-10-12T10:00:00Z' } as LocalReview;
        reviews = [saved]; return { data: saved, notice: 'ENSAYO' };
      }),
    }, 'finalizada');
    await test.identity.signIn('renter@example.invalid', 'synthetic'); await settle(test.fixture);
    test.panel().reviewComment.set(comment); await test.panel().submitReview(); await settle(test.fixture);
    expect(test.api.createReview).toHaveBeenCalledWith('token-renter', 'reservation-r', { rating: 5, comment }, expect.any(String));
    expect(test.fixture.nativeElement.querySelector('img')).toBeNull();
    expect(test.fixture.nativeElement.textContent).toContain(comment);
    expect(test.fixture.nativeElement.querySelector('#review-comment')).toBeNull();
    test.fixture.destroy();
  });

  it('refreshes the Go public space summary after publishing a review and discards an earlier read', async () => {
    const oldPublicRead = deferred<{ data: { items: LocalReview[]; average: number; count: number }; notice: string }>();
    let reservationReviews: LocalReview[] = [];
    const test = setup({
      spaceReviews: vi.fn().mockReturnValueOnce(oldPublicRead.promise).mockResolvedValueOnce({ data: { items: [], average: 4, count: 1 }, notice: 'ENSAYO' }),
      createReview: vi.fn(async () => {
        const saved = { id: 'public-review', target_type: 'espacio', rating: 4, comment: 'review now visible', state: 'publicada', created_at: '2026-10-12T10:00:00Z' } as LocalReview;
        reservationReviews = [saved]; return { data: saved, notice: 'ENSAYO' };
      }),
      reservationReviews: vi.fn(async () => ({ data: { items: reservationReviews }, notice: 'ENSAYO' })),
    }, 'finalizada');
    await test.identity.signIn('renter@example.invalid', 'synthetic'); await settle(test.fixture);
    expect(test.api.spaceReviews).toHaveBeenCalledTimes(1);
    test.panel().reviewComment.set('review now visible'); await test.panel().submitReview(); await settle(test.fixture);
    expect(test.api.createReview).toHaveBeenCalledTimes(1);
    expect(test.api.spaceReviews).toHaveBeenCalledTimes(2);
    expect(test.panel().spaceReviews()).toMatchObject({ count: 1, average: 4 });
    oldPublicRead.resolve({ data: { items: [], average: 0, count: 0 }, notice: 'ENSAYO' }); await settle(test.fixture);
    expect(test.panel().spaceReviews()).toMatchObject({ count: 1, average: 4 });
    expect(test.fixture.nativeElement.textContent).toContain('Promedio 4/5 · 1 reseña(s).');
    test.fixture.destroy();
  });

  it('exposes report controls only to the host for a review of the host space', async () => {
    const review = { id: 'review-space', target_type: 'espacio', rating: 1, comment: 'synthetic report target', state: 'publicada', created_at: '2026-10-12T10:00:00Z' } as LocalReview;
    const test = setup({ reservationReviews: vi.fn(async () => ({ data: { items: [review] }, notice: 'ENSAYO' })) }, 'finalizada');
    await test.identity.signIn('host@example.invalid', 'synthetic'); await settle(test.fixture);
    expect(test.fixture.nativeElement.textContent).toContain('Reportar reseña');
    await test.panel().report(review); await settle(test.fixture);
    expect(test.api.reportReview).toHaveBeenCalledWith('token-host', 'reservation-r', 'review-space', 'insultos_acoso', expect.any(String));
    expect(test.fixture.nativeElement.textContent).toContain('Reporte pendiente');
    test.fixture.destroy();
  });
});
