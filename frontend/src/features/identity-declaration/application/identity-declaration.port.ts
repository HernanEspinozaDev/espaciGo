import type { AccessToken } from "@/features/auth/application/auth.port";
import type {
  IdentityDeclaration,
  IdentityDeclarationRequest,
} from "../domain/identity-declaration";

export interface IdentityDeclarationPort {
  get(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<IdentityDeclaration | null>;
  save(
    token: AccessToken,
    request: IdentityDeclarationRequest,
    signal?: AbortSignal,
  ): Promise<IdentityDeclaration>;
}
