import "client-only";
import type { AccessToken } from "@/features/auth/application/auth.port";
import { apiClient } from "@/lib/api/client";
import { ApiProtocolError, normalizeApiError } from "@/lib/api/errors";
import { serializeExactJson } from "@/lib/api/exact-json";
import type { ReviewCase, ReviewInput, RevokeInput } from "../domain/review";
import type { ReviewPort } from "../application/review.port";

const auth = (token: AccessToken) => ({
  Authorization: ["Bearer", token].join(" "),
});
const clean = <T extends object>(value: T): T =>
  Object.fromEntries(
    Object.entries(value).filter(([, v]) => v !== undefined),
  ) as T;

export class ReviewOpenApiAdapter implements ReviewPort {
  async list(token: AccessToken, signal: AbortSignal): Promise<ReviewCase[]> {
    const r = await apiClient.GET("/document-verification/reviewer/cases", {
      headers: auth(token),
      signal,
    });
    if (!r.response.ok) throw normalizeApiError(r.response, r.error);
    if (!Array.isArray(r.data?.items)) throw new ApiProtocolError();
    return r.data.items;
  }
  async get(
    token: AccessToken,
    caseId: string,
    signal: AbortSignal,
  ): Promise<ReviewCase> {
    const r = await apiClient.GET(
      "/document-verification/reviewer/cases/{caseID}",
      { params: { path: { caseID: caseId } }, headers: auth(token), signal },
    );
    if (!r.response.ok) throw normalizeApiError(r.response, r.error);
    if (!r.data) throw new ApiProtocolError();
    return r.data;
  }
  async download(
    token: AccessToken,
    caseId: string,
    fileId: string,
    signal: AbortSignal,
  ): Promise<Blob> {
    const r = await apiClient.GET(
      "/document-verification/reviewer/cases/{caseID}/files/{fileID}/content",
      {
        params: { path: { caseID: caseId, fileID: fileId } },
        headers: auth(token),
        parseAs: "blob",
        signal,
      },
    );
    if (!r.response.ok) throw normalizeApiError(r.response, r.error);
    if (!(r.data instanceof Blob)) throw new ApiProtocolError();
    return r.data;
  }
  async review(
    token: AccessToken,
    caseId: string,
    input: ReviewInput,
    signal: AbortSignal,
  ): Promise<ReviewCase> {
    const r = await apiClient.POST(
      "/document-verification/reviewer/cases/{caseID}/review",
      {
        params: { path: { caseID: caseId } },
        headers: auth(token),
        body: clean(input),
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!r.response.ok) throw normalizeApiError(r.response, r.error);
    if (!r.data) throw new ApiProtocolError();
    return r.data;
  }
  async revoke(
    token: AccessToken,
    caseId: string,
    input: RevokeInput,
    signal: AbortSignal,
  ): Promise<ReviewCase> {
    const r = await apiClient.POST(
      "/document-verification/reviewer/cases/{caseID}/revoke",
      {
        params: { path: { caseID: caseId } },
        headers: auth(token),
        body: clean(input),
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!r.response.ok) throw normalizeApiError(r.response, r.error);
    if (!r.data) throw new ApiProtocolError();
    return r.data;
  }
}
