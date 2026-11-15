import type { AccessToken } from "@/features/auth/application/auth.port";
import {
  buildIdentityDeclarationRequest,
  type IdentityDeclaration,
  type IdentityDeclarationInput,
} from "../domain/identity-declaration";
import type { IdentityDeclarationPort } from "./identity-declaration.port";

export class IdentityDeclarationFacade {
  constructor(private readonly port: IdentityDeclarationPort) {}
  load(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<IdentityDeclaration | null> {
    return this.port.get(token, signal);
  }
  async saveAndReadback(
    token: AccessToken,
    input: IdentityDeclarationInput,
    consentGiven: boolean,
    signal?: AbortSignal,
  ): Promise<IdentityDeclaration> {
    if (consentGiven !== true)
      throw new TypeError("Explicit consent is required");
    const request = buildIdentityDeclarationRequest(input);
    await this.port.save(token, request, signal);
    const readback = await this.port.get(token, signal);
    if (!readback)
      throw new Error("Identity declaration could not be confirmed");
    return readback;
  }
}
