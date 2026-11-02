import { OpenApiDocumentVerificationAdapter } from "./infrastructure/document-verification-openapi-adapter";
import { authFacade } from "@/features/auth/auth-runtime";
import { DocumentVerificationFacade } from "./application/document-verification-facade";

const documentVerificationPort = new OpenApiDocumentVerificationAdapter();

export const documentVerificationFacade = new DocumentVerificationFacade(
  documentVerificationPort,
  authFacade,
);
