import "client-only";

import { AdminFacade } from "./application/admin-facade";
import { OpenApiAdminAdapter } from "./infrastructure/admin-openapi-adapter";

export const adminAdapter = new OpenApiAdminAdapter();
export const adminFacade = new AdminFacade(adminAdapter);
