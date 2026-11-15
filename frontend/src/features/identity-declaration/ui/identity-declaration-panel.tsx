"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/workspace/ui/card";
import { Button } from "@/components/workspace/ui/button";
import { Input } from "@/components/workspace/ui/input";
import { Label } from "@/components/workspace/ui/label";
import type { AccessToken } from "@/features/auth/application/auth.port";
import { isAbortError } from "@/lib/api/errors";
import { identityDeclarationFacade } from "../identity-declaration-runtime";
import type {
  IdentityDeclaration,
  IdentityDeclarationInput,
} from "../domain/identity-declaration";

type LoadState =
  "loading" | "empty" | "saved" | "error" | "uncertain" | "saving";

const emptyDraft: IdentityDeclarationInput = {
  firstName: "",
  lastName: "",
  dateOfBirth: "",
  rut: "",
};

function draftFrom(declaration: IdentityDeclaration): IdentityDeclarationInput {
  return {
    firstName: declaration.firstName,
    lastName: declaration.lastName,
    dateOfBirth: declaration.dateOfBirth,
    rut: declaration.rut,
  };
}

export function IdentityDeclarationPanel({
  accessToken,
}: {
  accessToken: AccessToken | null;
}) {
  const [loadState, setLoadState] = useState<LoadState>("loading");
  const [declaration, setDeclaration] = useState<IdentityDeclaration | null>(
    null,
  );
  const [draft, setDraft] = useState<IdentityDeclarationInput>(emptyDraft);
  const [consentGiven, setConsentGiven] = useState(false);
  const [editing, setEditing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [refreshSequence, setRefreshSequence] = useState(0);
  const operationController = useRef<AbortController | null>(null);

  useEffect(() => {
    if (!accessToken) return;

    const controller = new AbortController();
    let active = true;
    operationController.current = controller;
    void identityDeclarationFacade
      .load(accessToken, controller.signal)
      .then((result) => {
        if (!active) return;
        setDeclaration(result);
        setDraft(result ? draftFrom(result) : emptyDraft);
        setConsentGiven(false);
        setEditing(false);
        setError(null);
        setLoadState(result ? "saved" : "empty");
      })
      .catch((cause: unknown) => {
        if (!active || isAbortError(cause)) return;
        setDeclaration(null);
        setDraft(emptyDraft);
        setConsentGiven(false);
        setEditing(false);
        setError(
          "No pudimos consultar la declaración. Intenta actualizar el estado.",
        );
        setLoadState("error");
      })
      .finally(() => {
        if (operationController.current === controller) {
          operationController.current = null;
        }
      });

    return () => {
      active = false;
      controller.abort();
      if (operationController.current === controller) {
        operationController.current = null;
      }
    };
  }, [accessToken, refreshSequence]);

  const handleRefresh = () => {
    if (!accessToken || loadState === "saving") return;
    setError(null);
    setLoadState("loading");
    setRefreshSequence((sequence) => sequence + 1);
  };

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!accessToken || !consentGiven || loadState === "saving") return;

    operationController.current?.abort();
    const controller = new AbortController();
    operationController.current = controller;
    setError(null);
    setLoadState("saving");
    try {
      const saved = await identityDeclarationFacade.saveAndReadback(
        accessToken,
        draft,
        consentGiven,
        controller.signal,
      );
      if (
        controller.signal.aborted ||
        operationController.current !== controller
      ) {
        return;
      }
      setDeclaration(saved);
      setDraft(draftFrom(saved));
      setConsentGiven(false);
      setEditing(false);
      setLoadState("saved");
    } catch (cause: unknown) {
      if (
        controller.signal.aborted ||
        operationController.current !== controller ||
        isAbortError(cause)
      ) {
        return;
      }
      setDeclaration(null);
      setDraft(emptyDraft);
      setConsentGiven(false);
      setEditing(false);
      setError(
        "No pudimos confirmar el guardado. Consulta el estado antes de volver a intentarlo.",
      );
      setLoadState("uncertain");
    } finally {
      if (operationController.current === controller) {
        operationController.current = null;
      }
    }
  };

  const canSubmit =
    Boolean(accessToken) &&
    loadState !== "saving" &&
    draft.firstName.trim().length > 0 &&
    draft.lastName.trim().length > 0 &&
    draft.dateOfBirth.length > 0 &&
    draft.rut.trim().length > 0 &&
    consentGiven;
  const showForm =
    Boolean(accessToken) &&
    (loadState === "empty" || (loadState === "saved" && editing));

  return (
    <section aria-labelledby="identity-declaration-title">
      <Card>
        <CardHeader>
          <CardTitle id="identity-declaration-title">
            Declaración de identidad
          </CardTitle>
          <CardDescription>
            Guardar la declaración no inicia una verificación ni carga
            documentos.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <p
            id="identity-declaration-local-notice"
            role="note"
            className="rounded-md border border-amber-300 bg-amber-50 p-3 text-sm text-amber-950 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-100"
          >
            Solo pruebas locales con datos sintéticos. No ingreses datos reales.
            El aviso para usar datos reales requiere revisión y autorización.
          </p>

          {!accessToken && (
            <p role="status" className="text-sm text-muted-foreground">
              Inicia sesión para consultar o guardar tu declaración.
            </p>
          )}

          {accessToken && loadState === "loading" && (
            <p role="status" className="text-sm text-muted-foreground">
              Consultando declaración...
            </p>
          )}

          {accessToken &&
            (loadState === "error" || loadState === "uncertain") && (
              <div className="space-y-3" role="alert">
                <p className="text-sm text-destructive">{error}</p>
                <Button type="button" variant="outline" onClick={handleRefresh}>
                  Consultar estado
                </Button>
              </div>
            )}

          {accessToken && loadState === "saving" && (
            <p role="status" className="text-sm text-muted-foreground">
              Guardando y confirmando la lectura...
            </p>
          )}

          {accessToken && loadState === "saved" && declaration && !editing && (
            <div className="space-y-4">
              <p role="status" className="text-sm font-medium">
                Declaración guardada.
              </p>
              <p className="text-sm text-muted-foreground">
                La declaración está guardada; esto no significa que la identidad
                esté verificada ni que exista un expediente enviado.
              </p>
              <dl className="grid gap-3 sm:grid-cols-2">
                <div>
                  <dt className="text-sm font-medium">Nombres legales</dt>
                  <dd className="text-sm text-muted-foreground">
                    {declaration.firstName}
                  </dd>
                </div>
                <div>
                  <dt className="text-sm font-medium">Apellidos legales</dt>
                  <dd className="text-sm text-muted-foreground">
                    {declaration.lastName}
                  </dd>
                </div>
                <div>
                  <dt className="text-sm font-medium">Fecha de nacimiento</dt>
                  <dd className="text-sm text-muted-foreground">
                    {declaration.dateOfBirth}
                  </dd>
                </div>
                <div>
                  <dt className="text-sm font-medium">RUN/RUT</dt>
                  <dd className="text-sm text-muted-foreground">
                    {declaration.rut}
                  </dd>
                </div>
              </dl>
              <div className="flex flex-wrap gap-2">
                <Button type="button" variant="outline" onClick={handleRefresh}>
                  Consultar estado
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => {
                    setDraft(draftFrom(declaration));
                    setConsentGiven(false);
                    setEditing(true);
                  }}
                >
                  Editar declaración
                </Button>
                <Button asChild>
                  <Link href="/mi-cuenta/documentos">
                    Iniciar verificación de identidad
                  </Link>
                </Button>
              </div>
            </div>
          )}

          {showForm && (
            <form className="space-y-4" onSubmit={handleSubmit}>
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="identity-declaration-first-name">
                    Nombres legales
                  </Label>
                  <Input
                    id="identity-declaration-first-name"
                    name="first_name"
                    autoComplete="off"
                    required
                    value={draft.firstName}
                    onChange={(event) => {
                      const firstName = event.currentTarget.value;
                      setDraft((previous) => ({
                        ...previous,
                        firstName,
                      }));
                    }}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="identity-declaration-last-name">
                    Apellidos legales
                  </Label>
                  <Input
                    id="identity-declaration-last-name"
                    name="last_name"
                    autoComplete="off"
                    required
                    value={draft.lastName}
                    onChange={(event) => {
                      const lastName = event.currentTarget.value;
                      setDraft((previous) => ({
                        ...previous,
                        lastName,
                      }));
                    }}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="identity-declaration-date-of-birth">
                    Fecha de nacimiento
                  </Label>
                  <Input
                    id="identity-declaration-date-of-birth"
                    name="date_of_birth"
                    type="date"
                    autoComplete="off"
                    required
                    value={draft.dateOfBirth}
                    onChange={(event) => {
                      const dateOfBirth = event.currentTarget.value;
                      setDraft((previous) => ({
                        ...previous,
                        dateOfBirth,
                      }));
                    }}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="identity-declaration-rut">RUN/RUT</Label>
                  <Input
                    id="identity-declaration-rut"
                    name="rut"
                    type="text"
                    autoComplete="off"
                    required
                    value={draft.rut}
                    onChange={(event) => {
                      const rut = event.currentTarget.value;
                      setDraft((previous) => ({
                        ...previous,
                        rut,
                      }));
                    }}
                  />
                </div>
              </div>

              <label
                htmlFor="identity-declaration-consent"
                className="flex items-start gap-3 text-sm"
              >
                <input
                  id="identity-declaration-consent"
                  type="checkbox"
                  required
                  checked={consentGiven}
                  onChange={(event) =>
                    setConsentGiven(event.currentTarget.checked)
                  }
                  aria-describedby="identity-declaration-local-notice"
                  className="mt-0.5 size-4 shrink-0 accent-primary"
                />
                <span>
                  Confirmo que usaré solo datos sintéticos y acepto guardar esta
                  declaración para la prueba.
                </span>
              </label>

              <div className="flex flex-wrap gap-2">
                <Button type="submit" disabled={!canSubmit}>
                  Guardar declaración
                </Button>
                {editing && (
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => {
                      if (declaration) setDraft(draftFrom(declaration));
                      setConsentGiven(false);
                      setEditing(false);
                    }}
                  >
                    Cancelar edición
                  </Button>
                )}
              </div>
            </form>
          )}

          {accessToken && loadState === "empty" && !showForm && (
            <Button type="button" variant="outline" onClick={handleRefresh}>
              Consultar estado
            </Button>
          )}
        </CardContent>
      </Card>
    </section>
  );
}
