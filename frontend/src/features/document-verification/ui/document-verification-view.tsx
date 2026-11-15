"use client";

import { useEffect, useId, useRef, useState } from "react";
import {
  Download,
  FileCheck2,
  FileText,
  Loader2,
  RefreshCw,
  UploadCloud,
} from "lucide-react";
import { Button } from "@/components/workspace/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/workspace/ui/card";
import { useAuth } from "@/features/auth/ui/use-auth";
import { ApiError, isAbortError } from "@/lib/api/errors";
import { privateBlobUrls } from "@/lib/api/private-blob-urls";
import { useDocumentVerification } from "./use-document-verification";
import {
  IDENTITY_CAPTURE_CONSENT_VERSION,
  identityEvidenceRoles,
} from "../domain/document-verification";
import type {
  DocumentVerificationCase,
  DocumentVerificationCreate,
  IdentityEvidenceRole,
} from "../domain/document-verification";

const maxFileBytes = 10 * 1024 * 1024;
const allowedMimes = new Set(["application/pdf", "image/jpeg", "image/png"]);
const identityAllowedMimes = new Set(["image/jpeg", "image/png"]);
const identityEvidenceLabels: Record<IdentityEvidenceRole, string> = {
  identity_front: "Anverso de la cédula",
  identity_back: "Reverso de la cédula",
  identity_selfie: "Selfie",
};

const stateLabels: Record<DocumentVerificationCase["state"], string> = {
  borrador: "Borrador",
  enviada: "Enviada",
  en_revision: "En revisión",
  requiere_antecedentes: "Requiere antecedentes",
  aprobada: "Aprobada",
  rechazada: "Rechazada",
  vencida: "Vencida",
  revocada: "Revocada",
};

const fileStateLabels = {
  pendiente: "En inspección",
  limpio: "Validado",
  bloqueado: "No aceptado",
  error: "No disponible",
} as const;

function displayDate(value: string | undefined): string {
  if (!value) return "—";
  const date = new Date(value);
  return Number.isNaN(date.valueOf())
    ? "—"
    : date.toLocaleDateString("es-CL", { dateStyle: "medium" });
}

function toRfc3339(value: string): string | undefined {
  if (!value) return undefined;
  const date = new Date(value);
  return Number.isNaN(date.valueOf()) ? undefined : date.toISOString();
}

function isEditable(value: DocumentVerificationCase): boolean {
  return value.state === "borrador" || value.state === "requiere_antecedentes";
}

function canSubmit(value: DocumentVerificationCase): boolean {
  if (!isEditable(value)) return false;
  if (value.purpose === "identity") {
    return (
      value.files.length === identityEvidenceRoles.length &&
      identityEvidenceRoles.every((role) => {
        const files = value.files.filter((file) => file.evidence_role === role);
        return files.length === 1 && files[0]?.state === "limpio";
      })
    );
  }
  return (
    value.files.length > 0 &&
    value.files.every((file) => file.state === "limpio")
  );
}

export function DocumentVerificationView() {
  const { snapshot: auth } = useAuth();
  const sessionKey = JSON.stringify([
    auth.status,
    auth.accountId,
    [...auth.roles].sort(),
    [...auth.capabilities].sort(),
    auth.restrictedMode,
    auth.expiresAt,
  ]);
  return <DocumentVerificationContent key={sessionKey} auth={auth} />;
}

type AuthSnapshot = ReturnType<typeof useAuth>["snapshot"];

function DocumentVerificationContent({ auth }: { auth: AuthSnapshot }) {
  const {
    snapshot,
    loadCases,
    loadOwnSpaces,
    loadOwnCase,
    create,
    submit,
    upload,
    downloadSpaceFile,
  } = useDocumentVerification();
  const purposeId = useId();
  const spaceId = useId();
  const expirationId = useId();
  const fileId = useId();
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [selectedCaseId, setSelectedCaseId] = useState<string | null>(null);
  const [purpose, setPurpose] = useState<"identity" | "space">("identity");
  const [documentFormatDeclared, setDocumentFormatDeclared] = useState<
    "" | "legacy" | "new" | "unsure"
  >("");
  const [captureNoticeConfirmed, setCaptureNoticeConfirmed] = useState(false);
  const [spaceID, setSpaceID] = useState("");
  const [expiration, setExpiration] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [identityEvidenceRole, setIdentityEvidenceRole] =
    useState<IdentityEvidenceRole>("identity_front");
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [needsRefresh, setNeedsRefresh] = useState(false);

  useEffect(() => {
    if (auth.status !== "authenticated") return;
    void loadCases().catch((cause) => {
      if (!isAbortError(cause)) {
        setError("No se pudieron cargar tus expedientes. Intenta actualizar.");
      }
    });
    void loadOwnSpaces().catch((cause) => {
      if (!isAbortError(cause)) {
        setError(
          "No se pudieron cargar tus espacios para crear el expediente.",
        );
      }
    });
  }, [auth.accountId, auth.status, loadCases, loadOwnSpaces]);

  useEffect(() => {
    return () => privateBlobUrls.revoke("document-verification-download");
  }, []);

  const selected =
    snapshot.selected?.value.id === selectedCaseId ? snapshot.selected : null;
  const busy = snapshot.loading || snapshot.action !== null;

  async function handleCreate(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setMessage(null);
    const documentExpiresAt = toRfc3339(expiration);
    if (
      purpose === "identity" &&
      (!documentFormatDeclared || !captureNoticeConfirmed)
    ) {
      setError(
        !documentFormatDeclared
          ? "Selecciona el formato declarado de la cédula."
          : "Confirma el aviso de prueba local antes de crear el expediente.",
      );
      return;
    }
    if (
      purpose === "space" &&
      !snapshot.spaces.some((item) => item.id === spaceID)
    ) {
      setError("Selecciona un espacio de tu cuenta.");
      return;
    }
    try {
      let input: DocumentVerificationCreate;
      if (purpose === "identity") {
        input = {
          purpose,
          document_format_declared: documentFormatDeclared as Exclude<
            typeof documentFormatDeclared,
            ""
          >,
          capture_consent_given: true,
          capture_consent_version: IDENTITY_CAPTURE_CONSENT_VERSION,
        };
      } else {
        input = documentExpiresAt
          ? {
              purpose,
              space_id: spaceID,
              document_expires_at: documentExpiresAt,
            }
          : { purpose, space_id: spaceID };
      }
      const created = await create(input);
      setSelectedCaseId(created.value.id);
      setNeedsRefresh(false);
      setMessage(
        purpose === "identity"
          ? "Expediente sintético creado. Puedes cargar evidencias sintéticas locales."
          : "Expediente creado. Puedes cargar los documentos requeridos.",
      );
      setExpiration("");
      setSpaceID("");
      setDocumentFormatDeclared("");
      setCaptureNoticeConfirmed(false);
    } catch (cause) {
      if (isAbortError(cause)) return;
      setError(
        "No se pudo crear el expediente. Revisa los datos e inténtalo nuevamente.",
      );
    }
  }

  async function handleSelectCase(id: string) {
    setError(null);
    setMessage(null);
    setFile(null);
    if (fileInputRef.current) fileInputRef.current.value = "";
    setIdentityEvidenceRole("identity_front");
    setSelectedCaseId(id);
    setNeedsRefresh(false);
    privateBlobUrls.revoke("document-verification-download");
    try {
      await loadOwnCase(id);
    } catch (cause) {
      if (isAbortError(cause)) return;
      setError("No se pudo cargar el expediente seleccionado.");
    }
  }

  async function handleUpload(evidenceRole?: IdentityEvidenceRole) {
    if (!selected || !file) return;
    setError(null);
    setMessage(null);
    const isIdentity = selected.value.purpose === "identity";
    if ((isIdentity && !evidenceRole) || (!isIdentity && evidenceRole)) {
      setError("Selecciona un rol de evidencia válido para este expediente.");
      return;
    }
    const acceptedMimes = isIdentity ? identityAllowedMimes : allowedMimes;
    if (
      !acceptedMimes.has(file.type) ||
      file.size <= 0 ||
      file.size > maxFileBytes
    ) {
      setError(
        isIdentity
          ? "Usa una imagen JPEG o PNG de hasta 10 MiB."
          : "Usa un PDF, JPEG o PNG de hasta 10 MiB.",
      );
      return;
    }
    try {
      await upload(
        selected.value.id,
        file,
        isIdentity ? evidenceRole : undefined,
      );
      setFile(null);
      if (fileInputRef.current) fileInputRef.current.value = "";
      setMessage(
        isIdentity
          ? "Evidencia recibida y pendiente de inspección."
          : "Documento recibido y pendiente de inspección.",
      );
    } catch (cause) {
      if (isAbortError(cause)) return;
      setNeedsRefresh(true);
      if (cause instanceof ApiError && cause.status === 409) {
        setError(
          "El expediente cambió desde la última consulta. Actualízalo antes de reintentar.",
        );
        return;
      }
      setError(
        "No se pudo cargar el documento. Actualiza el expediente antes de reintentar.",
      );
    }
  }

  async function handleSubmit() {
    if (!selected) return;
    setError(null);
    setMessage(null);
    try {
      await submit(selected.value.id);
      setMessage("Expediente enviado a revisión.");
    } catch (cause) {
      if (isAbortError(cause)) return;
      setNeedsRefresh(true);
      if (cause instanceof ApiError && cause.status === 409) {
        setError(
          "El expediente cambió desde la última consulta. Actualízalo antes de volver a enviar.",
        );
        return;
      }
      setError(
        "No se pudo enviar. Actualiza el expediente y verifica el estado de sus documentos.",
      );
    }
  }

  async function handleRefresh() {
    setError(null);
    try {
      const cases = await loadCases();
      if (selectedCaseId) {
        if (!cases.some((item) => item.id === selectedCaseId)) {
          setError(
            "El expediente seleccionado ya no aparece en tu lista. Actualízala y vuelve a seleccionarlo.",
          );
          return;
        }
        const refreshedCase = await loadOwnCase(selectedCaseId);
        if (!refreshedCase || refreshedCase.id !== selectedCaseId) {
          throw new Error("Selected owner case was not refreshed.");
        }
        if (!isEditable(refreshedCase)) {
          setFile(null);
          if (fileInputRef.current) fileInputRef.current.value = "";
        }
      }
      setNeedsRefresh(false);
    } catch (cause) {
      if (isAbortError(cause)) return;
      setError(
        "No se pudieron actualizar los expedientes. Intenta nuevamente.",
      );
    }
  }

  async function handleDownload(caseId: string, fileIdValue: string) {
    setError(null);
    setMessage(null);
    privateBlobUrls.revoke("document-verification-download");
    try {
      const blob = await downloadSpaceFile(caseId, fileIdValue);
      const registered = privateBlobUrls.create(
        "document-verification-download",
        blob,
      );
      const link = document.createElement("a");
      link.href = registered.url;
      link.download =
        blob.type === "application/pdf"
          ? "documento-de-espacio.pdf"
          : blob.type === "image/jpeg"
            ? "documento-de-espacio.jpg"
            : "documento-de-espacio.png";
      link.rel = "noreferrer";
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.setTimeout(() => registered.revoke(), 2000);
    } catch (cause) {
      if (isAbortError(cause)) return;
      setError("El documento no está disponible para descarga.");
    }
  }

  if (auth.status === "checking") {
    return (
      <p role="status" className="text-sm text-muted-foreground">
        Comprobando sesión…
      </p>
    );
  }
  if (auth.status !== "authenticated") {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Inicia sesión para ver tus documentos</CardTitle>
          <CardDescription>
            Los expedientes y archivos son privados y solo están disponibles
            para su titular.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button asChild>
            <a href="/acceso?returnTo=%2Fmi-cuenta%2Fdocumentos">
              Ir a iniciar sesión
            </a>
          </Button>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-6">
      <section
        aria-labelledby="document-verification-heading"
        className="space-y-2"
      >
        <h2
          id="document-verification-heading"
          className="text-2xl font-semibold tracking-tight"
        >
          Documentos de verificación
        </h2>
        <p className="text-sm text-muted-foreground">
          Crea y consulta tus expedientes. Los documentos se inspeccionan en
          almacenamiento privado; solo archivos limpios de espacios propios
          pueden descargarse.
        </p>
      </section>

      {error && (
        <p
          role="alert"
          className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-red-800"
        >
          {error}
        </p>
      )}
      {message && (
        <p
          role="status"
          className="rounded-md border border-emerald-700/30 bg-emerald-50 p-3 text-sm text-emerald-800"
        >
          {message}
        </p>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Crear expediente</CardTitle>
          <CardDescription>
            El servidor valida la titularidad del espacio y determina al titular
            desde la sesión. Los expedientes de identidad de este corte son
            únicamente sintéticos.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleCreate} className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <label htmlFor={purposeId} className="text-sm font-medium">
                Tipo de expediente
              </label>
              <select
                id={purposeId}
                value={purpose}
                onChange={(event) =>
                  setPurpose(event.target.value as "identity" | "space")
                }
                className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm"
              >
                <option value="identity">Identidad</option>
                <option value="space">Espacio</option>
              </select>
            </div>
            {purpose === "space" && (
              <div className="space-y-2">
                <label htmlFor={spaceId} className="text-sm font-medium">
                  Espacio propio
                </label>
                <select
                  id={spaceId}
                  value={spaceID}
                  onChange={(event) => setSpaceID(event.target.value)}
                  required
                  className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm"
                  aria-describedby={`${spaceId}-hint`}
                >
                  <option value="">Selecciona un espacio</option>
                  {snapshot.spaces.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.title}
                    </option>
                  ))}
                </select>
                <p
                  id={`${spaceId}-hint`}
                  className="text-xs text-muted-foreground"
                >
                  Solo se muestran espacios asociados a tu cuenta.
                </p>
              </div>
            )}
            {purpose === "identity" && (
              <div className="space-y-4 sm:col-span-2">
                <div className="space-y-2">
                  <label
                    htmlFor="identity-format"
                    className="text-sm font-medium"
                  >
                    Formato declarado de la cédula
                  </label>
                  <select
                    id="identity-format"
                    value={documentFormatDeclared}
                    onChange={(event) =>
                      setDocumentFormatDeclared(
                        event.target.value as "" | "legacy" | "new" | "unsure",
                      )
                    }
                    className="h-10 w-full max-w-sm rounded-md border border-input bg-background px-3 text-sm"
                  >
                    <option value="">Selecciona una opción</option>
                    <option value="legacy">Formato anterior</option>
                    <option value="new">Formato nuevo</option>
                    <option value="unsure">No estoy seguro</option>
                  </select>
                </div>
                <div className="rounded-md border border-amber-300 bg-amber-50 p-3 text-sm text-amber-950">
                  En este corte solo se usan expedientes sintéticos locales; no
                  cargues documentos ni datos reales. El aviso definitivo para
                  captura real aún no está aprobado.
                </div>
                <label className="flex items-start gap-3 text-sm">
                  <input
                    type="checkbox"
                    checked={captureNoticeConfirmed}
                    onChange={(event) =>
                      setCaptureNoticeConfirmed(event.target.checked)
                    }
                    className="mt-1 size-4 rounded border-border text-primary focus:ring-ring"
                  />
                  <span>
                    Confirmo el aviso de prueba local para un expediente
                    sintético
                  </span>
                </label>
              </div>
            )}
            {purpose === "space" && (
              <div className="space-y-2 sm:col-span-2">
                <label htmlFor={expirationId} className="text-sm font-medium">
                  Vencimiento del documento (opcional)
                </label>
                <input
                  id={expirationId}
                  type="datetime-local"
                  value={expiration}
                  onChange={(event) => setExpiration(event.target.value)}
                  className="h-10 w-full max-w-sm rounded-md border border-input bg-background px-3 text-sm"
                />
                <p className="text-xs text-muted-foreground">
                  La hora ingresada se convierte a RFC 3339 desde tu zona
                  horaria local.
                </p>
              </div>
            )}
            <div className="sm:col-span-2">
              <Button type="submit" disabled={busy}>
                {snapshot.action === "create" ? (
                  <Loader2
                    className="mr-2 size-4 animate-spin"
                    aria-hidden="true"
                  />
                ) : (
                  <FileCheck2 className="mr-2 size-4" aria-hidden="true" />
                )}
                Crear expediente
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between gap-3">
            <div>
              <CardTitle>Mis expedientes</CardTitle>
              <CardDescription>
                Solo se muestran expedientes asociados a tu sesión.
              </CardDescription>
            </div>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => void handleRefresh()}
              disabled={busy}
              aria-label="Actualizar expedientes"
            >
              {snapshot.loading ? (
                <Loader2 className="size-4 animate-spin" aria-hidden="true" />
              ) : (
                <RefreshCw className="size-4" aria-hidden="true" />
              )}
            </Button>
          </div>
        </CardHeader>
        <CardContent className="space-y-3">
          {snapshot.cases.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              Aún no tienes expedientes documentales.
            </p>
          ) : (
            <ul className="divide-y rounded-md border">
              {snapshot.cases.map((item) => (
                <li key={item.id}>
                  <button
                    type="button"
                    onClick={() => void handleSelectCase(item.id)}
                    aria-current={
                      selectedCaseId === item.id ? "true" : undefined
                    }
                    className="flex w-full items-center justify-between gap-3 p-4 text-left hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  >
                    <span className="flex min-w-0 items-center gap-3">
                      <FileText
                        className="size-4 shrink-0 text-muted-foreground"
                        aria-hidden="true"
                      />
                      <span className="min-w-0">
                        <span className="block font-medium">
                          {item.purpose === "identity"
                            ? "Identidad"
                            : "Documentos del espacio"}
                        </span>
                        <span className="block text-xs text-muted-foreground">
                          Actualizado · revisión {item.revision}
                        </span>
                      </span>
                    </span>
                    <span className="shrink-0 rounded-full border px-2.5 py-1 text-xs">
                      {stateLabels[item.state]}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      {selected && (
        <Card>
          <CardHeader>
            <CardTitle>Detalle del expediente</CardTitle>
            <CardDescription>
              {selected.value.purpose === "identity"
                ? "Expediente de identidad"
                : "Expediente documental de espacio"}{" "}
              · {stateLabels[selected.value.state]}
              {selected.value.purpose === "space" &&
                ` · Vence ${displayDate(selected.value.document_expires_at)}`}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-5">
            {selected.value.purpose === "identity" &&
              selected.value.state === "requiere_antecedentes" &&
              selected.value.owner_decision_message && (
                <p
                  role="status"
                  className="rounded-md border border-sky-200 bg-sky-50 p-3 text-sm text-sky-900"
                >
                  {selected.value.owner_decision_message}
                </p>
              )}
            {selected.value.purpose === "identity" ? (
              <div className="space-y-3">
                <p className="text-sm text-muted-foreground">
                  La revisión humana ocurre en un flujo separado para revisores
                  autorizados. Usa solo archivos sintéticos locales; no cargues
                  documentos reales.
                </p>
                <ul
                  aria-label="Estado de evidencias de identidad"
                  className="space-y-2"
                >
                  {identityEvidenceRoles.map((role) => {
                    const matchingFiles = selected.value.files.filter(
                      (item) => item.evidence_role === role,
                    );
                    const [matchingFile] = matchingFiles;
                    const statusLabel =
                      matchingFiles.length === 0
                        ? "Pendiente de carga"
                        : matchingFiles.length === 1 && matchingFile
                          ? fileStateLabels[matchingFile.state]
                          : "Revisar archivos";
                    return (
                      <li key={role} className="rounded-md border p-3 text-sm">
                        {identityEvidenceLabels[role]} · {statusLabel}
                      </li>
                    );
                  })}
                </ul>
                {selected.value.files.some(
                  (item) =>
                    item.evidence_role === undefined ||
                    !identityEvidenceRoles.some(
                      (role) => role === item.evidence_role,
                    ),
                ) && (
                  <p className="text-sm text-amber-900">
                    Hay archivos sin un rol reconocido; no se podrá enviar este
                    expediente.
                  </p>
                )}
              </div>
            ) : selected.value.files.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                Todavía no hay documentos cargados.
              </p>
            ) : (
              <ul className="space-y-2">
                {selected.value.files.map((item) => (
                  <li
                    key={item.id}
                    className="flex flex-wrap items-center justify-between gap-3 rounded-md border p-3"
                  >
                    <span className="min-w-0">
                      <span className="block text-sm font-medium">
                        {item.mime === "application/pdf"
                          ? "PDF"
                          : item.mime === "image/jpeg"
                            ? "Imagen JPEG"
                            : "Imagen PNG"}
                      </span>
                      <span className="block text-xs text-muted-foreground">
                        {fileStateLabels[item.state]} · {item.pages}{" "}
                        {item.pages === 1 ? "página" : "páginas"}
                      </span>
                    </span>
                    {selected.value.purpose === "space" &&
                      item.state === "limpio" && (
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          onClick={() =>
                            void handleDownload(selected.value.id, item.id)
                          }
                          disabled={snapshot.action === "download"}
                        >
                          {snapshot.action === "download" ? (
                            <Loader2
                              className="mr-2 size-4 animate-spin"
                              aria-hidden="true"
                            />
                          ) : (
                            <Download
                              className="mr-2 size-4"
                              aria-hidden="true"
                            />
                          )}
                          Descargar
                        </Button>
                      )}
                  </li>
                ))}
              </ul>
            )}

            {isEditable(selected.value) && (
              <div className="space-y-3 border-t pt-4">
                <h3 className="text-sm font-semibold">
                  {selected.value.purpose === "identity"
                    ? "Agregar evidencia sintética"
                    : "Agregar documento"}
                </h3>
                {selected.value.purpose === "identity" && (
                  <div className="space-y-2">
                    <label
                      htmlFor={`${fileId}-evidence-role`}
                      className="text-sm font-medium"
                    >
                      Evidencia a cargar
                    </label>
                    <select
                      id={`${fileId}-evidence-role`}
                      value={identityEvidenceRole}
                      onChange={(event) =>
                        setIdentityEvidenceRole(
                          event.target.value as IdentityEvidenceRole,
                        )
                      }
                      className="h-10 w-full max-w-sm rounded-md border border-input bg-background px-3 text-sm"
                    >
                      {identityEvidenceRoles.map((role) => (
                        <option key={role} value={role}>
                          {identityEvidenceLabels[role]}
                        </option>
                      ))}
                    </select>
                  </div>
                )}
                <div className="space-y-2">
                  <label htmlFor={fileId} className="text-sm font-medium">
                    {selected.value.purpose === "identity"
                      ? "JPEG o PNG · máximo 10 MiB"
                      : "PDF, JPEG o PNG · máximo 10 MiB"}
                  </label>
                  <input
                    ref={fileInputRef}
                    id={fileId}
                    type="file"
                    accept={
                      selected.value.purpose === "identity"
                        ? "image/jpeg,image/png"
                        : "application/pdf,image/jpeg,image/png"
                    }
                    onChange={(event) =>
                      setFile(event.target.files?.[0] ?? null)
                    }
                    className="block w-full max-w-xl text-sm file:mr-3 file:rounded-md file:border-0 file:bg-muted file:px-3 file:py-2"
                  />
                </div>
                <div className="flex flex-wrap gap-2">
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() =>
                      void handleUpload(
                        selected.value.purpose === "identity"
                          ? identityEvidenceRole
                          : undefined,
                      )
                    }
                    disabled={!file || busy || needsRefresh}
                  >
                    {snapshot.action === "upload" ? (
                      <Loader2
                        className="mr-2 size-4 animate-spin"
                        aria-hidden="true"
                      />
                    ) : (
                      <UploadCloud className="mr-2 size-4" aria-hidden="true" />
                    )}
                    {selected.value.purpose === "identity"
                      ? "Cargar evidencia"
                      : "Cargar documento"}
                  </Button>
                  {canSubmit(selected.value) && (
                    <Button
                      type="button"
                      onClick={() => void handleSubmit()}
                      disabled={busy || needsRefresh}
                    >
                      {snapshot.action === "submit" ? (
                        <Loader2
                          className="mr-2 size-4 animate-spin"
                          aria-hidden="true"
                        />
                      ) : null}
                      Enviar a revisión
                    </Button>
                  )}
                </div>
                <p className="text-xs text-muted-foreground">
                  {selected.value.purpose === "identity"
                    ? "Solo se envía con exactamente una evidencia limpia por rol. Actualiza el expediente para consultar el resultado de la inspección."
                    : "El expediente puede enviarse cuando todos sus documentos estén validados. Actualiza el expediente para consultar el resultado de la inspección."}
                </p>
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
