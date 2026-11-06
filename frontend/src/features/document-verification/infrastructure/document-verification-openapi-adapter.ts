import "client-only";

import type { AccessToken } from "@/features/auth/application/auth.port";
import { apiClient } from "@/lib/api/client";
import { ApiProtocolError, normalizeApiError } from "@/lib/api/errors";
import { serializeExactJson } from "@/lib/api/exact-json";
import type {
  DocumentVerificationPort,
  UploadedDocumentVerificationFile,
  VersionedDocumentVerificationCase,
} from "../application/document-verification.port";
import {
  isStrongETag,
  normalizeDocumentVerificationCreate,
  type DocumentVerificationCase,
  type DocumentVerificationCreate,
} from "../domain/document-verification";

function auth(token: AccessToken) {
  return { Authorization: ["Bearer", token].join(" ") };
}
function versioned(
  data: DocumentVerificationCase | undefined,
  response: Response,
): VersionedDocumentVerificationCase {
  const etag = response.headers.get("etag");
  if (!data || !isStrongETag(etag)) throw new ApiProtocolError();
  return { value: data, etag };
}
function requireStrongETag(etag: string): void {
  if (!isStrongETag(etag))
    throw new TypeError("A current strong ETag is required");
}

export class OpenApiDocumentVerificationAdapter implements DocumentVerificationPort {
  async listOwnSpaces(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<{ id: string; title: string }[]> {
    const result = await apiClient.GET("/spaces", {
      headers: auth(token),
      signal,
    });
    if (!result.response.ok)
      throw normalizeApiError(result.response, result.error);
    if (!Array.isArray(result.data?.items)) throw new ApiProtocolError();
    return result.data.items.map(({ id, title }) => ({ id, title }));
  }

  async list(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<DocumentVerificationCase[]> {
    const result = await apiClient.GET("/document-verification/cases", {
      headers: auth(token),
      signal,
    });
    if (!result.response.ok)
      throw normalizeApiError(result.response, result.error);
    if (!Array.isArray(result.data)) throw new ApiProtocolError();
    return result.data;
  }
  async create(
    token: AccessToken,
    input: DocumentVerificationCreate,
    signal?: AbortSignal,
  ): Promise<VersionedDocumentVerificationCase> {
    const body = normalizeDocumentVerificationCreate(input);
    const result = await apiClient.POST("/document-verification/cases", {
      headers: auth(token),
      body,
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok)
      throw normalizeApiError(result.response, result.error);
    return versioned(result.data, result.response);
  }
  async getOwnCase(
    token: AccessToken,
    id: string,
    signal?: AbortSignal,
  ): Promise<VersionedDocumentVerificationCase> {
    const result = await apiClient.GET("/document-verification/cases/{id}", {
      params: { path: { id } },
      headers: auth(token),
      signal,
    });
    if (!result.response.ok)
      throw normalizeApiError(result.response, result.error);
    return versioned(result.data, result.response);
  }
  async submit(
    token: AccessToken,
    id: string,
    etag: string,
    signal?: AbortSignal,
  ): Promise<VersionedDocumentVerificationCase> {
    requireStrongETag(etag);
    const result = await apiClient.POST(
      "/document-verification/cases/{id}/submit",
      {
        params: { path: { id }, header: { "If-Match": etag } },
        headers: auth(token),
        signal,
      },
    );
    if (!result.response.ok)
      throw normalizeApiError(result.response, result.error);
    return versioned(result.data, result.response);
  }
  async upload(
    token: AccessToken,
    id: string,
    etag: string,
    file: File,
    signal?: AbortSignal,
  ): Promise<UploadedDocumentVerificationFile> {
    requireStrongETag(etag);
    const form = new FormData();
    form.append("file", file);
    const result = await apiClient.POST(
      "/document-verification/cases/{id}/files",
      {
        params: { path: { id }, header: { "If-Match": etag } },
        headers: auth(token),
        body: { file: file as unknown as string },
        bodySerializer: () => form,
        signal,
      },
    );
    if (!result.response.ok)
      throw normalizeApiError(result.response, result.error);
    const responseEtag = result.response.headers.get("etag");
    const data = result.data;
    if (!data?.file || !isStrongETag(responseEtag))
      throw new ApiProtocolError();
    return { revision: data.revision, file: data.file, etag: responseEtag };
  }
  async downloadSpaceFile(
    token: AccessToken,
    caseId: string,
    fileId: string,
    signal?: AbortSignal,
  ): Promise<Blob> {
    const result = await apiClient.GET(
      "/document-verification/space-cases/{caseID}/files/{fileID}/content",
      {
        params: { path: { caseID: caseId, fileID: fileId } },
        headers: auth(token),
        parseAs: "blob",
        signal,
      },
    );
    if (!result.response.ok)
      throw normalizeApiError(result.response, result.error);
    const blob = result.data as Blob | undefined;
    if (!(blob instanceof Blob)) throw new ApiProtocolError();
    return blob;
  }
}
