"use client";

import dynamic from "next/dynamic";
import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { Button } from "@/components/workspace/ui/button";
import { Input } from "@/components/workspace/ui/input";
import { Label } from "@/components/workspace/ui/label";
import { ApiError } from "@/lib/api/errors";
import { onboardingErrorCopy } from "../domain/onboarding-errors";
import { AuthFeedback, AuthShell } from "./auth-shell";
import { useAuth } from "./use-auth";
import { useOnboarding } from "./use-onboarding";
import {
  cancelScheduledClear,
  clearRegistrationResult,
  getRegistrationResult,
  getRegistrationResultServerSnapshot,
  scheduleClearRegistrationResult,
  subscribeRegistrationResult,
} from "../application/registration-result";

const DynamicRegistrationHelp = dynamic(
  () => import("./registration-help").then((mod) => mod.RegistrationHelp),
  {
    loading: () => (
      <p className="mt-4 text-sm text-gray-500" role="status">
        Cargando ayuda…
      </p>
    ),
    ssr: false,
  },
);

export function RegistrationSuccessScreen() {
  const onboarding = useOnboarding();
  const { snapshot: authSnapshot } = useAuth();
  const result = useSyncExternalStore(
    subscribeRegistrationResult,
    getRegistrationResult,
    getRegistrationResultServerSnapshot,
  );
  const [helpOpen, setHelpOpen] = useState(false);
  const [manualEmail, setManualEmail] = useState("");
  const [reissuePending, setReissuePending] = useState(false);
  const [feedback, setFeedback] = useState("");
  const [feedbackKind, setFeedbackKind] = useState<"status" | "error">(
    "status",
  );
  const noticeRef = useRef<HTMLDivElement>(null);
  const initialAccountIdRef = useRef(authSnapshot.accountId);
  const initialStatusRef = useRef(authSnapshot.status);

  useEffect(() => {
    cancelScheduledClear();
    const clearOnHistoryExit = () => {
      if (window.location.pathname !== "/registro-exitoso") {
        clearRegistrationResult();
      }
    };
    window.addEventListener("popstate", clearOnHistoryExit);
    window.addEventListener("pagehide", clearRegistrationResult);
    return () => {
      window.removeEventListener("popstate", clearOnHistoryExit);
      window.removeEventListener("pagehide", clearRegistrationResult);
      scheduleClearRegistrationResult();
    };
  }, []);

  useEffect(() => {
    if (
      authSnapshot.accountId !== initialAccountIdRef.current ||
      (initialStatusRef.current === "authenticated" &&
        authSnapshot.status !== "authenticated") ||
      (initialStatusRef.current !== "authenticated" &&
        authSnapshot.status === "authenticated")
    ) {
      clearRegistrationResult();
    }
  }, [authSnapshot.accountId, authSnapshot.status]);

  useEffect(() => {
    if (typeof BroadcastChannel === "undefined") return;
    try {
      const channel = new BroadcastChannel("espacigo_auth");
      channel.onmessage = (event) => {
        if (
          event.data?.type === "logout" ||
          event.data?.type === "session_changed"
        ) {
          clearRegistrationResult();
        }
      };
      return () => {
        channel.close();
      };
    } catch {
      // ignore
    }
  }, []);

  useEffect(() => {
    if (feedback) noticeRef.current?.focus();
  }, [feedback]);

  async function handleReissue(emailToReissue: string) {
    const target = emailToReissue.trim();
    if (!target || reissuePending) return;
    setReissuePending(true);
    setFeedback("");
    try {
      await onboarding.reissueVerification({ email: target });
      setFeedbackKind("status");
      setFeedback(
        "Si corresponde, recibirás las instrucciones en tu correo para verificar tu cuenta.",
      );
    } catch (err) {
      setFeedbackKind("error");
      setFeedback(
        err instanceof ApiError
          ? onboardingErrorCopy(err.code)
          : "No pudimos procesar la solicitud de reemisión. Inténtalo más tarde.",
      );
    } finally {
      setReissuePending(false);
    }
  }

  let title = "Verifica tu cuenta";
  let description = "Reemisión de instrucciones de verificación";

  if (result?.status === 201) {
    title = "Tu cuenta en EspaciGo fue creada con éxito";
    description = "Siguiente paso: verifica tu correo";
  } else if (result?.status === 202) {
    title = "Tu cuenta en EspaciGo fue creada";
    description = "El envío del correo no pudo confirmarse de inmediato";
  }

  return (
    <AuthShell
      title={title}
      description={description}
      secondaryQuestion="¿Ya confirmaste tu cuenta?"
      secondaryLabel="Ir a acceso"
      secondaryHref="/acceso"
      onNavigateAway={clearRegistrationResult}
    >
      {result?.status === 201 ? (
        <div className="space-y-6">
          <div className="space-y-3 text-sm leading-relaxed text-gray-700">
            <p>
              Te enviamos un enlace de verificación a{" "}
              <strong className="font-semibold text-gray-900">
                {result.email}
              </strong>
              . Ábrelo y confirma tu correo para activar tu cuenta.
            </p>
            <p>
              El mensaje puede tardar unos minutos. Si no aparece en tu bandeja
              de entrada, revisa Spam o Correo no deseado. Si lo encuentras
              allí, márcalo como «No es spam».
            </p>
          </div>

          <AuthFeedback
            message={feedback}
            kind={feedbackKind}
            focusRef={noticeRef}
          />

          <div className="space-y-3">
            <Button
              type="button"
              variant="outline"
              onClick={() => void handleReissue(result.email)}
              disabled={reissuePending}
              className="min-h-11 w-full rounded-full border border-gray-300 text-gray-800 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-orange-700"
            >
              {reissuePending
                ? "Enviando solicitud…"
                : "Reenviar enlace de verificación"}
            </Button>

            <div>
              <button
                type="button"
                onClick={() => setHelpOpen((prev) => !prev)}
                aria-expanded={helpOpen}
                aria-controls="registration-help-content"
                className="inline-flex min-h-11 items-center text-sm font-medium text-orange-800 underline underline-offset-4 hover:text-orange-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700 rounded-xs"
              >
                {helpOpen ? "Ocultar ayuda" : "¿No encuentras el correo?"}
              </button>
              {helpOpen ? <DynamicRegistrationHelp /> : null}
            </div>
          </div>
        </div>
      ) : result?.status === 202 ? (
        <div className="space-y-6">
          <div className="space-y-3 text-sm leading-relaxed text-gray-700">
            <p>
              Tu cuenta fue creada, pero no pudimos confirmar el envío del
              correo de verificación a{" "}
              <strong className="font-semibold text-gray-900">
                {result.email}
              </strong>
              .
            </p>
            <p>
              Solicita una reemisión para recibir las instrucciones de
              activación en tu bandeja de entrada.
            </p>
          </div>

          <AuthFeedback
            message={feedback}
            kind={feedbackKind}
            focusRef={noticeRef}
          />

          <div className="space-y-3">
            <Button
              type="button"
              variant="outline"
              onClick={() => void handleReissue(result.email)}
              disabled={reissuePending}
              className="min-h-11 w-full rounded-full border border-gray-300 text-gray-800 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-orange-700"
            >
              {reissuePending
                ? "Enviando solicitud…"
                : "Solicitar reemisión de verificación"}
            </Button>

            <div>
              <button
                type="button"
                onClick={() => setHelpOpen((prev) => !prev)}
                aria-expanded={helpOpen}
                aria-controls="registration-help-content"
                className="inline-flex min-h-11 items-center text-sm font-medium text-orange-800 underline underline-offset-4 hover:text-orange-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700 rounded-xs"
              >
                {helpOpen ? "Ocultar ayuda" : "¿No encuentras el correo?"}
              </button>
              {helpOpen ? <DynamicRegistrationHelp /> : null}
            </div>
          </div>
        </div>
      ) : (
        <div className="space-y-6">
          <p className="text-sm leading-relaxed text-gray-700">
            No encontramos un registro reciente en esta sesión. Si creaste una
            cuenta y aún no la has verificado, introduce tu correo para recibir
            un enlace de activación.
          </p>

          <form
            className="space-y-4"
            onSubmit={(e) => {
              e.preventDefault();
              void handleReissue(manualEmail);
            }}
          >
            <div>
              <Label
                htmlFor="reissue-email"
                className="block text-sm font-medium text-gray-700"
              >
                Correo electrónico
              </Label>
              <Input
                id="reissue-email"
                name="email"
                type="email"
                autoComplete="email"
                value={manualEmail}
                onChange={(e) => setManualEmail(e.target.value)}
                required
                disabled={reissuePending}
                className="mt-1 min-h-11 rounded-full border border-gray-300 px-3 text-gray-900 focus-visible:ring-2 focus-visible:ring-orange-700 sm:text-sm"
              />
            </div>

            <AuthFeedback
              message={feedback}
              kind={feedbackKind}
              focusRef={noticeRef}
            />

            <Button
              type="submit"
              disabled={reissuePending || !manualEmail}
              className="min-h-11 w-full rounded-full bg-orange-700 text-white hover:bg-orange-800 focus-visible:ring-2 focus-visible:ring-orange-700 focus-visible:ring-offset-2"
            >
              {reissuePending
                ? "Enviando solicitud…"
                : "Reenviar instrucciones"}
            </Button>
          </form>
        </div>
      )}
    </AuthShell>
  );
}
