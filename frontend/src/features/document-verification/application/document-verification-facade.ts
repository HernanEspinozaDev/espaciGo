"use client";

import "client-only";

import type { AccessToken } from "@/features/auth/application/auth.port";
import { isAbortError } from "@/lib/api/errors";
import type {
  DocumentVerificationCase,
  DocumentVerificationCreate,
  DocumentVerificationFile,
  OwnerSpaceOption,
} from "../domain/document-verification";
import type {
  DocumentVerificationPort,
  UploadedDocumentVerificationFile,
  VersionedDocumentVerificationCase,
} from "./document-verification.port";

interface AuthSnapshot {
  readonly status: string;
  readonly accountId: string | null;
  readonly roles: readonly string[];
  readonly capabilities: readonly string[];
  readonly restrictedMode: boolean;
  readonly expiresAt: string | null;
}

function sessionFingerprint(snapshot: AuthSnapshot): string {
  return JSON.stringify([
    snapshot.status,
    snapshot.accountId,
    [...snapshot.roles].sort(),
    [...snapshot.capabilities].sort(),
    snapshot.restrictedMode,
    snapshot.expiresAt,
  ]);
}

export interface DocumentVerificationAuth {
  getSnapshot(): AuthSnapshot;
  subscribe(listener: () => void): () => void;
  runAuthenticatedOperation<T>(
    operation: (token: AccessToken, signal: AbortSignal) => Promise<T>,
  ): Promise<T>;
}

export interface DocumentVerificationSnapshot {
  readonly cases: readonly DocumentVerificationCase[];
  readonly spaces: readonly OwnerSpaceOption[];
  readonly selected: VersionedDocumentVerificationCase | null;
  readonly loading: boolean;
  readonly action: string | null;
  readonly error: string | null;
}

const initialSnapshot: DocumentVerificationSnapshot = Object.freeze({
  cases: Object.freeze([]),
  spaces: Object.freeze([]),
  selected: null,
  loading: false,
  action: null,
  error: null,
});

function abortError(): DOMException {
  return new DOMException(
    "The session changed during the request.",
    "AbortError",
  );
}

export class DocumentVerificationFacade {
  #port: DocumentVerificationPort;
  #auth: DocumentVerificationAuth;
  #snapshot: DocumentVerificationSnapshot = initialSnapshot;
  #listeners = new Set<() => void>();
  #sessionFingerprint: string;
  #sessionGeneration = 0;
  #requestGenerations = new Map<string, number>();
  #activeActionToken: symbol | null = null;
  #unsubscribeAuth: () => void;
  #disposed = false;

  constructor(port: DocumentVerificationPort, auth: DocumentVerificationAuth) {
    this.#port = port;
    this.#auth = auth;
    const current = auth.getSnapshot();
    this.#sessionFingerprint = sessionFingerprint(current);
    this.#unsubscribeAuth = auth.subscribe(this.#handleAuthChange);
  }

  getSnapshot = (): DocumentVerificationSnapshot => this.#snapshot;

  getServerSnapshot = (): DocumentVerificationSnapshot => initialSnapshot;

  subscribe = (listener: () => void): (() => void) => {
    this.#listeners.add(listener);
    return () => this.#listeners.delete(listener);
  };

  dispose(): void {
    if (this.#disposed) return;
    this.#disposed = true;
    this.#sessionGeneration += 1;
    this.#activeActionToken = null;
    this.#unsubscribeAuth();
    this.#listeners.clear();
    this.#snapshot = initialSnapshot;
  }

  clearError(): void {
    if (this.#snapshot.error) this.#setSnapshot({ error: null });
  }

  clear(): void {
    this.#sessionGeneration += 1;
    this.#activeActionToken = null;
    this.#setSnapshot({ ...initialSnapshot });
  }

  #handleAuthChange = (): void => {
    const current = this.#auth.getSnapshot();
    const fingerprint = sessionFingerprint(current);
    if (fingerprint === this.#sessionFingerprint) return;
    this.#sessionFingerprint = fingerprint;
    this.#sessionGeneration += 1;
    this.#activeActionToken = null;
    this.#snapshot = initialSnapshot;
    for (const listener of this.#listeners) listener();
  };

  #setSnapshot(partial: Partial<DocumentVerificationSnapshot>): void {
    if (this.#disposed) return;
    this.#snapshot = Object.freeze({ ...this.#snapshot, ...partial });
    for (const listener of this.#listeners) listener();
  }

  async #run<T>(
    operation: (token: AccessToken, signal: AbortSignal) => Promise<T>,
  ): Promise<T> {
    if (this.#disposed) throw abortError();
    const generation = this.#sessionGeneration;
    let result: T;
    try {
      result = await this.#auth.runAuthenticatedOperation(operation);
    } catch (error) {
      if (this.#disposed || generation !== this.#sessionGeneration) {
        throw abortError();
      }
      throw error;
    }
    if (this.#disposed || generation !== this.#sessionGeneration) {
      throw abortError();
    }
    return result;
  }

  async #runLatest<T>(
    key: string,
    operation: (token: AccessToken, signal: AbortSignal) => Promise<T>,
  ): Promise<T> {
    const generation = (this.#requestGenerations.get(key) ?? 0) + 1;
    this.#requestGenerations.set(key, generation);
    try {
      const result = await this.#run(operation);
      if (this.#requestGenerations.get(key) !== generation) {
        throw abortError();
      }
      return result;
    } catch (error) {
      if (this.#requestGenerations.get(key) !== generation) {
        throw abortError();
      }
      throw error;
    }
  }

  async #perform<T>(action: string, operation: () => Promise<T>): Promise<T> {
    const generation = this.#sessionGeneration;
    const actionToken = Symbol(action);
    this.#activeActionToken = actionToken;
    this.#setSnapshot({ action, error: null });
    try {
      return await operation();
    } catch (error) {
      if (this.#disposed || generation !== this.#sessionGeneration) {
        throw abortError();
      }
      if (!isAbortError(error)) {
        this.#setSnapshot({
          error:
            error instanceof Error
              ? error.message
              : "No se pudo completar la operación.",
        });
      }
      throw error;
    } finally {
      if (
        !this.#disposed &&
        generation === this.#sessionGeneration &&
        this.#activeActionToken === actionToken
      ) {
        this.#activeActionToken = null;
        this.#setSnapshot({ action: null });
      }
    }
  }

  #putCase(
    value: DocumentVerificationCase,
  ): readonly DocumentVerificationCase[] {
    const cases = this.#snapshot.cases.filter((item) => item.id !== value.id);
    return Object.freeze([value, ...cases]);
  }

  #requireSelected(id: string): VersionedDocumentVerificationCase {
    const selected = this.#snapshot.selected;
    if (!selected || selected.value.id !== id) {
      throw new TypeError("Load the owner case before changing it.");
    }
    return selected;
  }

  async loadCases(): Promise<readonly DocumentVerificationCase[]> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const cases = await this.#runLatest("cases", (token, signal) =>
        this.#port.list(token, signal),
      );
      const selected = this.#snapshot.selected;
      const nextSelected =
        selected && cases.some((item) => item.id === selected.value.id)
          ? selected
          : null;
      this.#setSnapshot({
        cases: Object.freeze([...cases]),
        selected: nextSelected,
        loading: false,
      });
      return this.#snapshot.cases;
    } catch (error) {
      if (!isAbortError(error)) this.#setSnapshot({ loading: false });
      throw error;
    }
  }

  async loadOwnSpaces(): Promise<readonly OwnerSpaceOption[]> {
    return this.#perform("load-spaces", async () => {
      const spaces = await this.#runLatest("spaces", (token, signal) =>
        this.#port.listOwnSpaces(token, signal),
      );
      this.#setSnapshot({ spaces: Object.freeze([...spaces]) });
      return this.#snapshot.spaces;
    });
  }

  async loadOwnCase(
    id: string,
  ): Promise<VersionedDocumentVerificationCase["value"] | null> {
    return this.#perform("load-case", async () => {
      const selected = await this.#runLatest("cases", (token, signal) =>
        this.#port.getOwnCase(token, id, signal),
      );
      this.#setSnapshot({
        selected,
        cases: this.#putCase(selected.value),
      });
      return selected.value;
    });
  }

  async create(
    input: DocumentVerificationCreate,
  ): Promise<VersionedDocumentVerificationCase> {
    return this.#perform("create", async () => {
      const created = await this.#runLatest("cases", (token, signal) =>
        this.#port.create(token, input, signal),
      );
      this.#setSnapshot({
        selected: created,
        cases: this.#putCase(created.value),
      });
      return created;
    });
  }

  async submit(id: string): Promise<VersionedDocumentVerificationCase> {
    return this.#perform("submit", async () => {
      const selected = this.#requireSelected(id);
      const updated = await this.#runLatest("cases", (token, signal) =>
        this.#port.submit(token, id, selected.etag, signal),
      );
      this.#setSnapshot({
        selected: updated,
        cases: this.#putCase(updated.value),
      });
      return updated;
    });
  }

  async upload(
    id: string,
    file: File,
  ): Promise<UploadedDocumentVerificationFile> {
    return this.#perform("upload", async () => {
      const selected = this.#requireSelected(id);
      const uploaded = await this.#runLatest("cases", (token, signal) =>
        this.#port.upload(token, id, selected.etag, file, signal),
      );
      const value: DocumentVerificationCase = {
        ...selected.value,
        revision: uploaded.revision,
        files: [
          ...selected.value.files.filter(
            (item) => item.id !== uploaded.file.id,
          ),
          uploaded.file,
        ],
      };
      const updated = Object.freeze({ value, etag: uploaded.etag });
      this.#setSnapshot({ selected: updated, cases: this.#putCase(value) });
      return uploaded;
    });
  }

  async downloadSpaceFile(caseId: string, fileId: string): Promise<Blob> {
    return this.#perform("download", async () => {
      const selected = this.#requireSelected(caseId);
      const file: DocumentVerificationFile | undefined =
        selected.value.files.find((item) => item.id === fileId);
      if (
        selected.value.purpose !== "space" ||
        !file ||
        file.state !== "limpio"
      ) {
        throw new TypeError("This document is not available for download.");
      }
      return this.#run((token, signal) =>
        this.#port.downloadSpaceFile(token, caseId, fileId, signal),
      );
    });
  }
}
