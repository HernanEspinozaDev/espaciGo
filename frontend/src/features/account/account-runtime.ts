import "client-only";

import { authFacade } from "@/features/auth/auth-runtime";
import { privateBlobUrls } from "@/lib/api/private-blob-urls";
import { AccountFacade } from "./application/account-facade";
import { OpenApiAccountAdapter } from "./infrastructure/account-openapi-adapter";

export const accountAdapter = new OpenApiAccountAdapter();
export const accountFacade = new AccountFacade(accountAdapter, authFacade, () =>
  privateBlobUrls.clear(),
);
