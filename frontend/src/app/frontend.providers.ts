import { SessionBoundary } from './core/identity/domain/ports/session-boundary';
import { IdentitySession } from './core/identity/application/identity-session';
import type { Provider } from '@angular/core';
import { IdentityPort } from './core/identity/domain/ports/identity-port';
import { IdentityApi } from './core/identity/infrastructure/identity-api';
import { AccountPrivacyPort } from './features/account/domain/ports/account-privacy-port';
import { AccountPrivacyApi } from './features/account/infrastructure/account-privacy-api';
import { AdminPort } from './features/admin/domain/ports/admin-port';
import { AdminGateway } from './features/admin/infrastructure/admin-gateway';
import { BookingCommunicationPort } from './features/bookings/domain/ports/booking-communication-port';
import { BookingLifecyclePort } from './features/bookings/domain/ports/booking-lifecycle-port';
import { BookingPort } from './features/bookings/domain/ports/booking-port';
import { BookingApi } from './features/bookings/infrastructure/booking-api';
import { BookingCommunicationApi } from './features/bookings/infrastructure/booking-communication-api';
import { BookingLifecycleApi } from './features/bookings/infrastructure/booking-lifecycle-api';
import { ExplorePort } from './features/explore/domain/ports/explore-port';
import { ExploreApi } from './features/explore/infrastructure/explore-api';
import { FinancePort } from './features/finance/domain/ports/finance-port';
import { FinanceGateway } from './features/finance/infrastructure/finance-gateway';
import { HostingPort } from './features/hosting/domain/ports/hosting-port';
import { HostingApi } from './features/hosting/infrastructure/hosting-api';
import { VerificationPort } from './features/verification/domain/ports/verification-port';
import { VerificationApi } from './features/verification/infrastructure/verification-api';
import { GlobalAdminPort } from './features/global-admin/domain/ports/global-admin-port';
import { GlobalAdminGateway } from './features/global-admin/infrastructure/global-admin-gateway';

/** Composition root: application ports are bound to HTTP adapters only here. */
export function provideFrontendPorts(): Provider[] {
  return [
    { provide: SessionBoundary, useExisting: IdentitySession },
    { provide: IdentityPort, useExisting: IdentityApi },
    { provide: AccountPrivacyPort, useExisting: AccountPrivacyApi },
    { provide: AdminPort, useExisting: AdminGateway },
    { provide: BookingPort, useExisting: BookingApi },
    { provide: BookingCommunicationPort, useExisting: BookingCommunicationApi },
    { provide: BookingLifecyclePort, useExisting: BookingLifecycleApi },
    { provide: ExplorePort, useExisting: ExploreApi },
    { provide: FinancePort, useExisting: FinanceGateway },
    { provide: HostingPort, useExisting: HostingApi },
    { provide: VerificationPort, useExisting: VerificationApi },
    { provide: GlobalAdminPort, useExisting: GlobalAdminGateway },
  ];
}
