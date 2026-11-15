import { IdentityDeclarationOpenApiAdapter } from "./infrastructure/identity-declaration-openapi-adapter";
import { IdentityDeclarationFacade } from "./application/identity-declaration-facade";

export const identityDeclarationFacade = new IdentityDeclarationFacade(
  new IdentityDeclarationOpenApiAdapter(),
);
