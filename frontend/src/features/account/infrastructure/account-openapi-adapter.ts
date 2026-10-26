import "client-only";

import { apiClient } from "@/lib/api/client";
import { ApiProtocolError, normalizeApiError } from "@/lib/api/errors";
import { serializeExactJson } from "@/lib/api/exact-json";
import type { AccountPort } from "../application/account.port";
import type { Profile } from "../domain/profile";
import type { ProfilePhotoMetadata } from "../domain/photo";
import {
  normalizeSyntheticPayoutAccount,
  type SyntheticPayoutAccount,
} from "../domain/payout-account";
import type {
  OwnIdentityExport,
  RightsRequest,
  RightsRequestKind,
} from "../domain/privacy";

export class OpenApiAccountAdapter implements AccountPort {
  async getProfile(signal?: AbortSignal): Promise<Profile | null> {
    const result = await apiClient.GET("/profile", {
      signal,
    });
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return {
      displayName: result.data.display_name,
      phone: result.data.phone ?? null,
      updatedAt: result.data.updated_at,
    };
  }

  async updateProfile(
    payload: { displayName: string; phone?: string },
    signal?: AbortSignal,
  ): Promise<Profile> {
    const result = await apiClient.PUT("/profile", {
      body: {
        display_name: payload.displayName,
        ...(payload.phone !== undefined ? { phone: payload.phone } : {}),
      },
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return {
      displayName: result.data.display_name,
      phone: result.data.phone ?? null,
      updatedAt: result.data.updated_at,
    };
  }

  async getProfilePhoto(
    signal?: AbortSignal,
  ): Promise<ProfilePhotoMetadata | null> {
    const result = await apiClient.GET("/profile/photo", {
      signal,
    });
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    const data = result.data;
    if ("fixture_code" in data && data.fixture_code === "synthetic-png-v1") {
      return {
        id: data.id,
        fixtureCode: "synthetic-png-v1",
        mimeType: "image/png",
        sha256: data.sha256,
        sizeBytes: data.size_bytes,
        createdAt: data.created_at,
        storageKind: "synthetic_fixed",
      };
    }
    const anyData = data as Record<string, unknown>;
    const firstVariant =
      Array.isArray(anyData.variants) && anyData.variants[0]
        ? (anyData.variants[0] as Record<string, unknown>)
        : undefined;
    return {
      id: data.id,
      mimeType: (firstVariant?.mime_type as "image/webp") ?? "image/webp",
      sha256:
        (firstVariant?.sha256_hash as string) ??
        (anyData.sha256 as string) ??
        "",
      sizeBytes:
        (firstVariant?.byte_size as number) ??
        (anyData.size_bytes as number) ??
        0,
      createdAt: data.created_at,
      storageKind: "uploaded_avatar",
    };
  }

  async generateSyntheticPhoto(
    idempotencyKey: string,
    signal?: AbortSignal,
  ): Promise<void> {
    const result = await apiClient.PUT("/profile/photo", {
      params: {
        header: {
          "Idempotency-Key": idempotencyKey,
        },
      },
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
  }

  async uploadProfilePhoto(
    idempotencyKey: string,
    imageFile: Blob | File,
    signal?: AbortSignal,
  ): Promise<void> {
    const formData = new FormData();
    formData.append("image", imageFile);
    const result = await apiClient.PUT("/profile/photo", {
      params: {
        header: {
          "Idempotency-Key": idempotencyKey,
        },
      },
      body: formData as unknown as { image: string },
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
  }

  async removeProfilePhoto(
    idempotencyKey: string,
    signal?: AbortSignal,
  ): Promise<void> {
    const result = await apiClient.DELETE("/profile/photo", {
      params: {
        header: {
          "Idempotency-Key": idempotencyKey,
        },
      },
      signal,
    });
    if (result.response.status === 404) return;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
  }

  async getProfilePhotoContent(signal?: AbortSignal): Promise<Blob | null> {
    const result = await apiClient.GET("/profile/photo/content", {
      parseAs: "blob",
      signal,
    });
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data as Blob) ?? (await result.response.blob());
  }

  async getPayoutAccount(
    signal?: AbortSignal,
  ): Promise<SyntheticPayoutAccount | null> {
    const result = await apiClient.GET("/payout-account", {
      signal,
    });
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return normalizeSyntheticPayoutAccount(result.data);
  }

  async createOrReplacePayoutAccount(
    idempotencyKey: string,
    signal?: AbortSignal,
  ): Promise<SyntheticPayoutAccount> {
    const result = await apiClient.PUT("/payout-account", {
      params: {
        header: {
          "Idempotency-Key": idempotencyKey,
        },
      },
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    const parsed = normalizeSyntheticPayoutAccount(result.data);
    if (!parsed) {
      return {
        referenceId: `payout-${idempotencyKey.slice(0, 8)}`,
        status: "activa_fake",
        notice: "Referencia local ficticia; no contiene datos bancarios.",
      };
    }
    return parsed;
  }

  async revokePayoutAccount(
    idempotencyKey: string,
    signal?: AbortSignal,
  ): Promise<void> {
    const result = await apiClient.DELETE("/payout-account", {
      params: {
        header: {
          "Idempotency-Key": idempotencyKey,
        },
      },
      signal,
    });
    if (result.response.status === 404) return;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
  }

  async listRightsRequests(signal?: AbortSignal): Promise<RightsRequest[]> {
    const result = await apiClient.GET("/rights-requests", {
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data?.items ?? [];
  }

  async requestDataRight(
    kind: RightsRequestKind,
    signal?: AbortSignal,
  ): Promise<RightsRequest> {
    const result = await apiClient.POST("/rights-requests", {
      body: { type: kind },
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return result.data;
  }

  async exportIdentityData(
    signal?: AbortSignal,
  ): Promise<OwnIdentityExport | null> {
    const result = await apiClient.GET("/privacy/export", {
      signal,
    });
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data ?? null;
  }

  async exportIdentityBlob(signal?: AbortSignal): Promise<Blob | null> {
    const result = await apiClient.GET("/privacy/export", {
      parseAs: "blob",
      signal,
    });
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data as Blob) ?? (await result.response.blob());
  }

  async downloadIdentityArchive(signal?: AbortSignal): Promise<Blob | null> {
    const result = await apiClient.GET("/privacy/export/archive", {
      parseAs: "blob",
      signal,
    });
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return (result.data as Blob) ?? (await result.response.blob());
  }
}
