"use client";

import { useState, type FormEvent } from "react";
import { createDiditIntent } from "./verification-api";
import { formatRut, isValidRut } from "./rut-utils";

const CONSENT_VERSION = "kyc-didit-cl-2026-11-06";

function isAdult(date: string): boolean {
  const birth = new Date(`${date}T00:00:00`);
  if (!date || Number.isNaN(birth.getTime())) return false;
  const today = new Date();
  const age =
    today.getFullYear() -
    birth.getFullYear() -
    (today.getMonth() < birth.getMonth() ||
    (today.getMonth() === birth.getMonth() && today.getDate() < birth.getDate())
      ? 1
      : 0);
  return age >= 18 && birth <= today;
}

export function VerificationFlow() {
  const [rut, setRut] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    const data = new FormData(event.currentTarget);
    const date = String(data.get("date_of_birth") ?? "");
    if (!isValidRut(rut) || !isAdult(date)) {
      setError(
        "Revisa el RUN/RUT y confirma que la fecha de nacimiento corresponda a una persona adulta.",
      );
      return;
    }
    if (data.get("consent") !== "on") {
      setError("Debes aceptar el consentimiento para continuar.");
      return;
    }
    setBusy(true);
    try {
      const intent = await createDiditIntent({
        first_name: String(data.get("first_name") ?? "").trim(),
        last_name: String(data.get("last_name") ?? "").trim(),
        date_of_birth: date,
        rut: rut.replace(/\./g, "").toUpperCase(),
        consent: true,
        consent_version: CONSENT_VERSION,
      });
      window.location.assign(intent.session_url);
    } catch {
      setError(
        "No pudimos iniciar la verificación. Inténtalo nuevamente más tarde.",
      );
      setBusy(false);
    }
  }

  return (
    <section className="mx-auto max-w-2xl space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Verifica tu identidad</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Ingresa los datos tal como aparecen en tu cédula chilena. No uses el
          número de serie del documento.
        </p>
      </div>
      <form className="space-y-5" onSubmit={submit}>
        <label className="block space-y-1 text-sm font-medium">
          Nombres legales completos
          <input
            name="first_name"
            autoComplete="given-name"
            required
            maxLength={120}
            className="w-full rounded-md border border-border bg-background px-3 py-2"
          />
        </label>
        <label className="block space-y-1 text-sm font-medium">
          Apellidos legales completos
          <input
            name="last_name"
            autoComplete="family-name"
            required
            maxLength={120}
            className="w-full rounded-md border border-border bg-background px-3 py-2"
          />
        </label>
        <label className="block space-y-1 text-sm font-medium">
          Fecha de nacimiento
          <input
            name="date_of_birth"
            type="date"
            required
            max={new Date().toISOString().slice(0, 10)}
            className="w-full rounded-md border border-border bg-background px-3 py-2"
          />
        </label>
        <label className="block space-y-1 text-sm font-medium">
          RUN / RUT chileno
          <input
            name="rut"
            inputMode="text"
            autoComplete="off"
            value={rut}
            onChange={(event) => setRut(formatRut(event.target.value))}
            required
            aria-describedby="rut-help"
            aria-invalid={rut.length > 0 && !isValidRut(rut)}
            className="w-full rounded-md border border-border bg-background px-3 py-2"
          />
          <span
            id="rut-help"
            className="text-xs font-normal text-muted-foreground"
          >
            Formato: 12.345.678-K
          </span>
        </label>
        <fieldset className="rounded-md border border-border p-4">
          <legend className="px-1 text-sm font-medium">
            Documento admitido
          </legend>
          <p className="text-sm">
            País emisor: Chile (CHL). Cédula de Identidad (ID), ambos lados, y
            validación biométrica en Didit.
          </p>
        </fieldset>
        <label className="flex items-start gap-3 text-sm">
          <input
            name="consent"
            type="checkbox"
            required
            className="mt-1 size-4"
          />
          <span>
            Consiento expresamente el tratamiento de mis datos de identidad y
            biométricos por EspaciGo y Didit para verificar mi identidad.
            Entiendo que se capturarán imágenes de mi cédula y una prueba
            biométrica. Este consentimiento es independiente de comunicaciones
            comerciales.
          </span>
        </label>
        {error ? (
          <p role="alert" className="text-sm font-medium text-red-800">
            {error}
          </p>
        ) : null}
        <button
          disabled={busy}
          className="rounded-md bg-primary px-4 py-2 font-semibold text-primary-foreground disabled:opacity-60"
        >
          {busy ? "Iniciando…" : "Verificar con Didit"}
        </button>
      </form>
    </section>
  );
}
