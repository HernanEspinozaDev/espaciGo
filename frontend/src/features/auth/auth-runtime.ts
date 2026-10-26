import "client-only";

import { AuthFacade } from "./application/auth-facade";
import { OpenApiAuthAdapter } from "./infrastructure/auth-openapi-adapter";

export const authFacade = new AuthFacade(new OpenApiAuthAdapter());
