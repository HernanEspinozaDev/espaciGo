import "client-only";

import { authFacade } from "./auth-runtime";
import {
  accountAdapter,
  accountFacade,
} from "@/features/account/account-runtime";
import { SessionIdentityStore } from "./application/session-identity";

export const sessionIdentityStore = new SessionIdentityStore(
  authFacade,
  accountAdapter,
  accountFacade,
);
