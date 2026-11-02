"use client";

import { useCallback, useSyncExternalStore } from "react";
import { documentVerificationFacade } from "../document-verification-runtime";
import type { DocumentVerificationSnapshot } from "../application/document-verification-facade";
import type {
  UploadedDocumentVerificationFile,
  VersionedDocumentVerificationCase,
} from "../application/document-verification.port";
import type {
  DocumentVerificationCreate,
  DocumentVerificationCase,
  OwnerSpaceOption,
} from "../domain/document-verification";

export interface DocumentVerificationView {
  readonly snapshot: DocumentVerificationSnapshot;
  loadCases(): Promise<readonly DocumentVerificationCase[]>;
  loadOwnSpaces(): Promise<readonly OwnerSpaceOption[]>;
  loadOwnCase(id: string): Promise<DocumentVerificationCase | null>;
  create(
    input: DocumentVerificationCreate,
  ): Promise<VersionedDocumentVerificationCase>;
  submit(id: string): Promise<VersionedDocumentVerificationCase>;
  upload(id: string, file: File): Promise<UploadedDocumentVerificationFile>;
  downloadSpaceFile(caseId: string, fileId: string): Promise<Blob>;
  clearError(): void;
}

export function useDocumentVerification(): DocumentVerificationView {
  const snapshot = useSyncExternalStore(
    documentVerificationFacade.subscribe,
    documentVerificationFacade.getSnapshot,
    documentVerificationFacade.getServerSnapshot,
  );
  const loadCases = useCallback(
    () => documentVerificationFacade.loadCases(),
    [],
  );
  const loadOwnCase = useCallback(
    (id: string) => documentVerificationFacade.loadOwnCase(id),
    [],
  );
  const loadOwnSpaces = useCallback(
    () => documentVerificationFacade.loadOwnSpaces(),
    [],
  );
  const create = useCallback(
    (input: DocumentVerificationCreate) =>
      documentVerificationFacade.create(input),
    [],
  );
  const submit = useCallback(
    (id: string) => documentVerificationFacade.submit(id),
    [],
  );
  const upload = useCallback(
    (id: string, file: File) => documentVerificationFacade.upload(id, file),
    [],
  );
  const downloadSpaceFile = useCallback(
    (caseId: string, fileId: string) =>
      documentVerificationFacade.downloadSpaceFile(caseId, fileId),
    [],
  );
  const clearError = useCallback(
    () => documentVerificationFacade.clearError(),
    [],
  );

  return {
    snapshot,
    loadCases,
    loadOwnSpaces,
    loadOwnCase,
    create,
    submit,
    upload,
    downloadSpaceFile,
    clearError,
  };
}
