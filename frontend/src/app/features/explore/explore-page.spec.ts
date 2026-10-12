import { provideHttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, provideRouter } from '@angular/router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { IdentityApi } from '../../core/identity/identity-api';
import { IdentitySession } from '../../core/identity/identity-session';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { ExploreApi } from './explore-api';
import { ExplorePage } from './explore-page';

const accountA = { account_id: 'account-a', roles: ['arrendatario'], restricted_mode: false };
const accountB = { account_id: 'account-b', roles: ['arrendatario'], restricted_mode: false };
const login = (account_id: string, access_token: string) => ({
  account_id,
  roles: ['arrendatario'],
  access_token,
  token_type: 'Bearer',
  expires_at: '2027-01-01T00:00:00Z',
  restricted_mode: false,
});

describe('ExplorePage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('discards an A catalog response after logout and login as B', async () => {
    let finishSearch!: (result: unknown) => void;
    const pending = new Promise((resolve) => (finishSearch = resolve));
    const explore = {
      categories: vi.fn().mockResolvedValue({ items: [] }),
      search: vi.fn(() => pending),
      profile: vi.fn(),
    };
    TestBed.configureTestingModule({
      imports: [ExplorePage],
      providers: [
        provideHttpClient(),
        provideRouter([]),
        PrivateSessionState,
        { provide: ActivatedRoute, useValue: { snapshot: { data: {} } } },
        {
          provide: IdentityApi,
          useValue: {
            login: vi
              .fn()
              .mockResolvedValueOnce(login('account-a', 'bearer-a'))
              .mockResolvedValueOnce(login('account-b', 'bearer-b')),
            session: vi.fn().mockResolvedValueOnce(accountA).mockResolvedValueOnce(accountB),
            logout: vi.fn().mockResolvedValue(undefined),
          },
        },
        { provide: ExploreApi, useValue: explore },
      ],
    });
    const fixture = TestBed.createComponent(ExplorePage);
    fixture.detectChanges();
    const identity = TestBed.inject(IdentitySession);
    await identity.signIn('a@example.invalid', 'Synthetic#123');
    const page = fixture.componentInstance as unknown as {
      items: () => unknown[];
      search(): Promise<void>;
    };
    const request = page.search();
    await identity.signOut();
    await identity.signIn('b@example.invalid', 'Synthetic#456');
    finishSearch({
      safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL',
      data: { items: [{ space_id: 'space-from-a' }], next_cursor: 'cursor-a' },
    });
    await request;

    expect(identity.session()?.account_id).toBe('account-b');
    expect(identity.accessToken()).toBe('bearer-b');
    expect(page.items()).toEqual([]);
    fixture.destroy();
  });

  it('starts a fresh search without reusing a cursor after clearing filters', async () => {
    const explore = {
      categories: vi.fn().mockResolvedValue({ items: [] }),
      search: vi
        .fn()
        .mockResolvedValueOnce({
          safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL',
          data: { items: [], next_cursor: 'old-cursor' },
        })
        .mockResolvedValueOnce({
          safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL',
          data: { items: [], next_cursor: 'new-cursor' },
        }),
      profile: vi.fn(),
    };
    TestBed.configureTestingModule({
      imports: [ExplorePage],
      providers: [
        provideHttpClient(),
        provideRouter([]),
        PrivateSessionState,
        { provide: ActivatedRoute, useValue: { snapshot: { data: {} } } },
        {
          provide: IdentityApi,
          useValue: {
            login: vi.fn().mockResolvedValue(login('account-a', 'bearer-a')),
            session: vi.fn().mockResolvedValue(accountA),
            logout: vi.fn(),
          },
        },
        { provide: ExploreApi, useValue: explore },
      ],
    });
    const fixture = TestBed.createComponent(ExplorePage);
    fixture.detectChanges();
    const identity = TestBed.inject(IdentitySession);
    await identity.signIn('a@example.invalid', 'Synthetic#123');
    const page = fixture.componentInstance as unknown as {
      search(reset?: boolean): Promise<void>;
      clearFilters(): void;
      nextCursor(): string | undefined;
    };
    await page.search();
    expect(page.nextCursor()).toBe('old-cursor');
    page.clearFilters();
    await vi.waitFor(() => expect(explore.search).toHaveBeenCalledTimes(2));
    expect(explore.search.mock.calls[1][1].cursor).toBeUndefined();
    fixture.destroy();
  });

  it('discards an in-flight result and resets the cursor when criteria change', async () => {
    let finishSearch!: (result: unknown) => void;
    const pending = new Promise((resolve) => (finishSearch = resolve));
    const explore = {
      categories: vi.fn().mockResolvedValue({ items: [] }),
      search: vi
        .fn()
        .mockResolvedValueOnce({ safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL', data: { items: [], next_cursor: 'cursor-old' } })
        .mockImplementationOnce(() => pending),
      profile: vi.fn(),
    };
    TestBed.configureTestingModule({
      imports: [ExplorePage],
      providers: [
        provideHttpClient(), provideRouter([]), PrivateSessionState,
        { provide: ActivatedRoute, useValue: { snapshot: { data: {} } } },
        { provide: IdentityApi, useValue: {
          login: vi.fn().mockResolvedValue(login('account-a', 'bearer-a')),
          session: vi.fn().mockResolvedValue(accountA), logout: vi.fn(),
        } },
        { provide: ExploreApi, useValue: explore },
      ],
    });
    const fixture = TestBed.createComponent(ExplorePage);
    fixture.detectChanges();
    const identity = TestBed.inject(IdentitySession);
    await identity.signIn('a@example.invalid', 'Synthetic#123');
    const page = fixture.componentInstance as unknown as {
      search(reset?: boolean): Promise<void>;
      updateFilter(key: 'q', value: string): void;
      setAttribute(code: string, value: string): void;
      nextCursor(): string | undefined;
      items(): unknown[];
    };
    await page.search();
    const pendingSearch = page.search(false);
    page.updateFilter('q', 'new criteria');
    page.setAttribute('capacity', '5');
    expect(page.nextCursor()).toBeUndefined();
    finishSearch({ safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL', data: { items: [{ space_id: 'stale' }], next_cursor: 'stale-cursor' } });
    await pendingSearch;
    expect(page.items()).toEqual([]);
    expect(page.nextCursor()).toBeUndefined();
    fixture.destroy();
  });

  it('uses a fresh cursor after criteria change and clears a current Bearer on 401 despite filter invalidation', async () => {
    let failSearch!: (error: unknown) => void;
    const pending401 = new Promise((_resolve, reject) => (failSearch = reject));
    const explore = {
      categories: vi.fn().mockResolvedValue({ items: [] }),
      search: vi
        .fn()
        .mockResolvedValueOnce({ safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL', data: { items: [], next_cursor: 'cursor-old' } })
        .mockImplementationOnce(() => pending401),
      profile: vi.fn(),
    };
    TestBed.configureTestingModule({
      imports: [ExplorePage],
      providers: [
        provideHttpClient(), provideRouter([]), PrivateSessionState,
        { provide: ActivatedRoute, useValue: { snapshot: { data: {} } } },
        { provide: IdentityApi, useValue: {
          login: vi.fn().mockResolvedValue(login('account-a', 'bearer-a')),
          session: vi.fn().mockResolvedValue(accountA), logout: vi.fn(),
        } },
        { provide: ExploreApi, useValue: explore },
      ],
    });
    const fixture = TestBed.createComponent(ExplorePage);
    fixture.detectChanges();
    const identity = TestBed.inject(IdentitySession);
    await identity.signIn('a@example.invalid', 'Synthetic#123');
    const page = fixture.componentInstance as unknown as {
      search(reset?: boolean): Promise<void>;
      updateFilter(key: 'q', value: string): void;
    };
    await page.search();
    page.updateFilter('q', 'changed');
    const request = page.search(false);
    expect(explore.search.mock.calls[1][1].cursor).toBeUndefined();
    page.updateFilter('q', 'changed again');
    failSearch({ status: 401 });
    await request;
    expect(identity.accessToken()).toBeNull();
    expect(identity.session()).toBeNull();
    fixture.destroy();
  });

  it('does not let a late A 401 expire B', async () => {
    let failSearch!: (error: unknown) => void;
    const pending = new Promise((_resolve, reject) => (failSearch = reject));
    const explore = {
      categories: vi.fn().mockResolvedValue({ items: [] }),
      search: vi.fn(() => pending),
      profile: vi.fn(),
    };
    TestBed.configureTestingModule({
      imports: [ExplorePage],
      providers: [
        provideHttpClient(), provideRouter([]), PrivateSessionState,
        { provide: ActivatedRoute, useValue: { snapshot: { data: {} } } },
        { provide: IdentityApi, useValue: {
          login: vi.fn().mockResolvedValueOnce(login('account-a', 'bearer-a')).mockResolvedValueOnce(login('account-b', 'bearer-b')),
          session: vi.fn().mockResolvedValueOnce(accountA).mockResolvedValueOnce(accountB),
          logout: vi.fn().mockResolvedValue(undefined),
        } },
        { provide: ExploreApi, useValue: explore },
      ],
    });
    const fixture = TestBed.createComponent(ExplorePage);
    fixture.detectChanges();
    const identity = TestBed.inject(IdentitySession);
    await identity.signIn('a@example.invalid', 'Synthetic#123');
    const page = fixture.componentInstance as unknown as { search(): Promise<void> };
    const request = page.search();
    await identity.signOut();
    await identity.signIn('b@example.invalid', 'Synthetic#456');
    failSearch({ status: 401 });
    await request;
    expect(identity.accessToken()).toBe('bearer-b');
    expect(identity.session()?.account_id).toBe('account-b');
    fixture.destroy();
  });

  it('does not query the catalog for a session whose confirmed role is only arrendador', async () => {
    const explore = {
      categories: vi.fn(),
      search: vi.fn(),
      profile: vi.fn(),
    };
    TestBed.configureTestingModule({
      imports: [ExplorePage],
      providers: [
        provideHttpClient(),
        provideRouter([]),
        PrivateSessionState,
        { provide: ActivatedRoute, useValue: { snapshot: { data: {} } } },
        {
          provide: IdentityApi,
          useValue: {
            login: vi.fn().mockResolvedValue({
              ...login('account-owner', 'owner-token'),
              roles: ['arrendador'],
            }),
            session: vi.fn().mockResolvedValue({
              account_id: 'account-owner',
              roles: ['arrendador'],
              restricted_mode: false,
            }),
            logout: vi.fn(),
          },
        },
        { provide: ExploreApi, useValue: explore },
      ],
    });
    const identity = TestBed.inject(IdentitySession);
    await identity.signIn('owner@example.invalid', 'Synthetic#123');
    const fixture = TestBed.createComponent(ExplorePage);
    fixture.detectChanges();

    expect(explore.categories).not.toHaveBeenCalled();
    expect(explore.search).not.toHaveBeenCalled();
    fixture.destroy();
  });

  it('shows Go permission errors without leaking a stale result', async () => {
    const explore = {
      categories: vi.fn().mockResolvedValue({ items: [] }),
      search: vi.fn().mockRejectedValue({ status: 403 }),
      profile: vi.fn(),
    };
    TestBed.configureTestingModule({
      imports: [ExplorePage],
      providers: [
        provideHttpClient(),
        provideRouter([]),
        PrivateSessionState,
        { provide: ActivatedRoute, useValue: { snapshot: { data: {} } } },
        {
          provide: IdentityApi,
          useValue: {
            login: vi.fn().mockResolvedValue(login('account-a', 'bearer-a')),
            session: vi.fn().mockResolvedValue(accountA),
            logout: vi.fn(),
          },
        },
        { provide: ExploreApi, useValue: explore },
      ],
    });
    const identity = TestBed.inject(IdentitySession);
    await identity.signIn('a@example.invalid', 'Synthetic#123');
    const fixture = TestBed.createComponent(ExplorePage);
    fixture.detectChanges();
    const page = fixture.componentInstance as unknown as {
      search(): Promise<void>;
      error(): string;
      items(): unknown[];
    };
    await page.search();

    expect(page.error()).toContain('Go confirmó que esta cuenta no tiene permiso');
    expect(page.items()).toEqual([]);
    fixture.destroy();
  });
});
