import "server-only";

import { createPublicCatalogService } from "../application/catalog-service";
import { goPublicCatalogAdapter } from "./go-public-catalog-adapter";

export const publicCatalog = createPublicCatalogService(goPublicCatalogAdapter);
