import type { AccessToken } from "@/features/auth/application/auth.port";
import type {
  DocumentVerificationCase,
  DocumentVerificationCreate,
  DocumentVerificationFile,
  OwnerSpaceOption,
} from "../domain/document-verification";

export interface VersionedDocumentVerificationCase {
  readonly value: DocumentVerificationCase;
  readonly etag: string;
}
export interface UploadedDocumentVerificationFile {
  readonly revision: number | string;
  readonly file: DocumentVerificationFile;
  readonly etag: string;
}
export interface DocumentVerificationPort {
  listOwnSpaces(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<OwnerSpaceOption[]>;
  list(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<DocumentVerificationCase[]>;
  create(
    token: AccessToken,
    input: DocumentVerificationCreate,
    signal?: AbortSignal,
  ): Promise<VersionedDocumentVerificationCase>;
  getOwnCase(
    token: AccessToken,
    id: string,
    signal?: AbortSignal,
  ): Promise<VersionedDocumentVerificationCase>;
  submit(
    token: AccessToken,
    id: string,
    etag: string,
    signal?: AbortSignal,
  ): Promise<VersionedDocumentVerificationCase>;
  upload(
    token: AccessToken,
    id: string,
    etag: string,
    file: File,
    signal?: AbortSignal,
  ): Promise<UploadedDocumentVerificationFile>;
  downloadSpaceFile(
    token: AccessToken,
    caseId: string,
    fileId: string,
    signal?: AbortSignal,
  ): Promise<Blob>;
}
