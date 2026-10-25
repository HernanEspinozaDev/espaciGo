import "client-only";

import { AccountFacade } from "./application/account-facade";
import { OpenApiAccountAdapter } from "./infrastructure/account-openapi-adapter";

export const accountAdapter = new OpenApiAccountAdapter();
export const accountFacade = new AccountFacade(accountAdapter);
