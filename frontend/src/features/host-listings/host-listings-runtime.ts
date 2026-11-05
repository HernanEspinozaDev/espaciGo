import "client-only";

import { authFacade } from "@/features/auth/auth-runtime";
import { HostListingsFacade } from "./application/host-listings-facade";
import { OpenApiHostListingsAdapter } from "./infrastructure/host-listings-openapi-adapter";

export const hostListingsAdapter = new OpenApiHostListingsAdapter();
export const hostListingsFacade = new HostListingsFacade(hostListingsAdapter);

function sessionScope(): string {
  const session = authFacade.getSnapshot();
  return JSON.stringify([
    session.status,
    session.accountId,
    session.roles,
    session.capabilities,
    session.restrictedMode,
  ]);
}

let scope = sessionScope();
authFacade.subscribe(() => {
  const next = sessionScope();
  if (next !== scope) {
    scope = next;
    hostListingsFacade.clear();
  }
});
