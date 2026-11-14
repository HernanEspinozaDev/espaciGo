"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { MonthCalendar } from "./month-calendar";
import { rentalDemoAdapter } from "../infrastructure/demo-openapi-adapter";
import type {
  DemoActor,
  DemoConfirmation,
  DemoMonth,
  DemoOwnerSpace,
  DemoPublicSpace,
  DemoQuote,
  DemoSession,
} from "../domain/types";
import { localDateTimeToUtcISOString } from "../../host-listings/domain/local-date-time";

type AnySpace = DemoOwnerSpace | DemoPublicSpace;
const initialMonth = () =>
  new Intl.DateTimeFormat("en-CA", {
    timeZone: "America/Santiago",
    year: "numeric",
    month: "2-digit",
  }).format(new Date());
const genericError =
  "No se pudo completar la acción. Actualiza los datos y vuelve a intentarlo.";

export function RentalDemoView() {
  const params = useSearchParams();
  const router = useRouter();
  const mode = params.get("mode") === "host" ? "host" : "public";
  const owner = mode === "host";
  const [session, setSession] = useState<DemoSession | null>(null);
  const [spaces, setSpaces] = useState<AnySpace[]>([]);
  const [spaceId, setSpaceId] = useState(params.get("space") ?? "");
  const [month, setMonth] = useState(params.get("month") ?? initialMonth());
  const [date, setDate] = useState<string | null>(params.get("date"));
  const [duration, setDuration] = useState(1);
  const [calendar, setCalendar] = useState<DemoMonth | null>(null);
  const [slot, setSlot] = useState<{ start_at: string; end_at: string } | null>(
    null,
  );
  const [quote, setQuote] = useState<DemoQuote | null>(null);
  const [approved, setApproved] = useState(false);
  const [result, setResult] = useState<DemoConfirmation | null>(null);
  const [pending, setPending] = useState<{
    quoteId: string;
    intentId: string;
  } | null>(null);
  const [intentId, setIntentId] = useState("");
  const [busy, setBusy] = useState(false);
  const [uncertain, setUncertain] = useState(false);
  const [error, setError] = useState("");
  const generation = useRef(0);
  const controllers = useRef<AbortController[]>([]);
  const fence = useCallback(() => {
    generation.current++;
    controllers.current.forEach((c) => c.abort());
    controllers.current = [];
  }, []);
  const clearJourney = useCallback(() => {
    setQuote(null);
    setApproved(false);
    setResult(null);
    setPending(null);
    setIntentId("");
    setSlot(null);
  }, []);

  useEffect(() => {
    let active = true;
    const c = new AbortController();
    rentalDemoAdapter
      .session(c.signal)
      .then((s) => {
        if (active) setSession(s);
      })
      .catch(() => {
        if (active) setError(genericError);
      });
    return () => {
      active = false;
      c.abort();
    };
  }, []);
  useEffect(() => {
    const op = ++generation.current;
    const c = new AbortController();
    setCalendar(null);
    setSpaces([]);
    clearJourney();
    rentalDemoAdapter
      .spaces(owner, c.signal)
      .then((items) => {
        if (generation.current !== op) return;
        setSpaces([...items]);
        const selected = items.find((item) => item.id === spaceId) ?? items[0];
        if (selected && selected.id !== spaceId) setSpaceId(selected.id);
      })
      .catch(() => {
        if (generation.current === op) setError(genericError);
      });
    return () => c.abort();
  }, [owner, session?.actor, clearJourney]);
  useEffect(() => {
    if (!spaceId) return;
    const op = ++generation.current;
    const c = new AbortController();
    setCalendar(null);
    clearJourney();
    rentalDemoAdapter
      .month(spaceId, month, duration, owner, c.signal)
      .then((value) => {
        if (generation.current === op) setCalendar(value);
      })
      .catch(() => {
        if (generation.current === op) setError(genericError);
      });
    return () => c.abort();
  }, [spaceId, month, duration, owner, session?.actor, clearJourney]);
  const selectedDay = calendar?.days.find((day) => day.date === date);
  const timeZone = useMemo(() => calendar?.time_zone ?? "", [calendar]);
  const ownerSpace = spaces.find(
    (item): item is DemoOwnerSpace => "revision" in item && item.id === spaceId,
  );
  const context = (patch: {
    mode?: "host" | "public";
    space?: string;
    month?: string;
    date?: string | null;
  }) => {
    fence();
    clearJourney();
    setCalendar(null);
    setError("");
    const m = patch.mode ?? mode;
    const s = patch.space ?? spaceId;
    const mo = patch.month ?? month;
    const d = patch.date === undefined ? date : patch.date;
    if (patch.space !== undefined) setSpaceId(patch.space);
    if (patch.month !== undefined) setMonth(patch.month);
    if (patch.date !== undefined) setDate(patch.date);
    const q = new URLSearchParams({ mode: m, month: mo });
    if (s) q.set("space", s);
    if (d) q.set("date", d);
    router.replace(`/demo/arriendo?${q}`, { scroll: false });
  };
  const signIn = async (actor: DemoActor) => {
    fence();
    clearJourney();
    setBusy(true);
    setError("");
    try {
      setSession(await rentalDemoAdapter.login(actor));
    } catch {
      setError(genericError);
    } finally {
      setBusy(false);
    }
  };
  const signOut = async () => {
    fence();
    setBusy(true);
    clearJourney();
    try {
      await rentalDemoAdapter.logout();
      setSession(null);
    } catch {
      setError(genericError);
    } finally {
      setBusy(false);
    }
  };
  const refresh = async () => {
    fence();
    const op = generation.current;
    setError("");
    setBusy(true);
    try {
      const [nextSpaces, nextMonth] = await Promise.all([
        owner ? rentalDemoAdapter.spaces(true) : Promise.resolve(null),
        spaceId
          ? rentalDemoAdapter.month(spaceId, month, duration, owner)
          : Promise.resolve(null),
      ]);
      if (generation.current !== op) return;
      if (nextSpaces) setSpaces([...nextSpaces]);
      if (nextMonth) setCalendar(nextMonth);
      if (!pending) setUncertain(false);
      if (pending) {
        try {
          const saved = await rentalDemoAdapter.attempt(pending.intentId);
          if (generation.current === op) {
            setResult(saved);
            setPending(null);
            setUncertain(false);
          }
        } catch {
          if (generation.current === op)
            setError("Resultado aún no disponible. No se repitió el intento.");
        }
      }
    } catch {
      if (generation.current === op) setError(genericError);
    } finally {
      if (generation.current === op) setBusy(false);
    }
  };
  const loadQuote = async () => {
    if (!slot) return;
    fence();
    clearJourney();
    setBusy(true);
    setError("");
    const op = generation.current;
    try {
      const value = await rentalDemoAdapter.quote(
        spaceId,
        slot.start_at,
        slot.end_at,
      );
      if (generation.current === op) setQuote(value);
    } catch {
      if (generation.current === op) setError(genericError);
    } finally {
      if (generation.current === op) setBusy(false);
    }
  };
  const pay = async () => {
    if (!quote || !approved) return;
    const stableIntent = intentId || crypto.randomUUID();
    setIntentId(stableIntent);
    fence();
    setBusy(true);
    setPending({ quoteId: quote.id, intentId: stableIntent });
    setError("");
    const op = generation.current;
    try {
      const value = await rentalDemoAdapter.confirm(quote.id, {
        approved_contract: true,
        intent_id: stableIntent,
      });
      const updatedMonth = await rentalDemoAdapter.month(
        spaceId,
        month,
        duration,
        false,
      );
      if (generation.current === op) {
        setResult(value);
        setCalendar(updatedMonth);
        setPending(null);
      }
    } catch {
      if (generation.current === op) {
        setUncertain(true);
        setError(
          "Resultado incierto. Consulta el resultado antes de continuar; no repetiremos el pago.",
        );
      }
    } finally {
      if (generation.current === op) setBusy(false);
    }
  };
  const checkAttempt = async () => {
    if (!pending) return;
    const op = generation.current;
    setBusy(true);
    setError("");
    try {
      const value = await rentalDemoAdapter.attempt(pending.intentId);
      if (generation.current === op) {
        setResult(value);
        setPending(null);
        setUncertain(false);
      }
    } catch {
      if (generation.current === op)
        setError("Resultado aún no disponible. No se envió otro intento.");
    } finally {
      if (generation.current === op) setBusy(false);
    }
  };
  return (
    <section className="mx-auto w-full max-w-5xl min-w-0 space-y-6 px-4 py-6 sm:px-6">
      <header className="space-y-2">
        <h1 className="text-2xl font-bold">Recorrido de arriendo (demo)</h1>
        <p className="text-sm text-muted-foreground">
          Entorno sintético local; no es una reserva comercial.
        </p>
      </header>
      <nav
        aria-label="Vistas de demostración"
        className="flex flex-wrap gap-3 text-sm"
      >
        <a
          className="underline"
          href={`/demo/arriendo?mode=host&space=${encodeURIComponent(spaceId)}&month=${month}`}
        >
          Mis espacios (demo)
        </a>
        <a
          className="underline"
          href={`/demo/arriendo?mode=public&space=${encodeURIComponent(spaceId)}&month=${month}`}
        >
          Publicación (demo)
        </a>
      </nav>
      <div className="grid min-w-0 gap-4 sm:grid-cols-2">
        <label className="grid gap-1 text-sm">
          Espacio de demostración
          <select
            className="min-h-11 rounded-md border bg-background px-3"
            value={spaceId}
            onChange={(e) => context({ space: e.target.value, date: null })}
          >
            <option value="">Selecciona un espacio</option>
            {spaces.map((s) => (
              <option key={s.id} value={s.id}>
                {s.title}
              </option>
            ))}
          </select>
        </label>
        <label className="grid gap-1 text-sm">
          Duración
          <select
            className="min-h-11 rounded-md border bg-background px-3"
            value={duration}
            onChange={(e) => {
              setDuration(Number(e.target.value));
              clearJourney();
            }}
          >
            <option value={1}>1 hora</option>
            <option value={2}>2 horas</option>
            <option value={4}>4 horas</option>
          </select>
        </label>
      </div>
      {owner && !session && (
        <div className="rounded border p-4">
          <p className="mb-2">
            Inicia una sesión sintética para consultar el calendario
            propietario.
          </p>
          <button
            disabled={busy}
            onClick={() => signIn("host")}
            className="min-h-11 rounded border px-3"
          >
            Entrar como propietario de demostración
          </button>
        </div>
      )}
      {owner && (
        <p className="rounded-md bg-muted p-3 text-sm">
          Mis espacios (demo): cambios sintéticos en memoria.
        </p>
      )}
      {error && (
        <div
          role="alert"
          className="break-words rounded-md border border-red-300 bg-red-50 p-3 text-sm text-red-800"
        >
          {error}
        </div>
      )}
      {uncertain && (
        <button
          disabled={busy}
          onClick={checkAttempt}
          className="min-h-11 rounded-md border px-4"
        >
          Consultar resultado
        </button>
      )}
      {calendar ? (
        <div className="grid min-w-0 gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(16rem,0.8fr)]">
          <MonthCalendar
            month={month}
            days={calendar.days}
            selectedDate={date}
            today={calendar.today}
            onSelect={(d) => context({ date: d })}
            onMonthChange={(m) => context({ month: m, date: null })}
            disabled={busy || uncertain}
          />
          <div className="min-w-0 space-y-4 rounded-lg border p-4">
            <h2 className="text-lg font-semibold">
              {owner ? "Configuración y bloqueos" : "Intervalos disponibles"}
            </h2>
            <p className="text-sm">
              Zona horaria: <span className="break-all">{timeZone}</span>
            </p>
            {owner && !session && (
              <div className="grid gap-2">
                <p>Usa una sesión sintética para administrar espacios.</p>
                <button
                  disabled={busy}
                  onClick={() => signIn("host")}
                  className="min-h-11 rounded border px-3"
                >
                  Entrar como propietario de demostración
                </button>
              </div>
            )}
            {owner && session?.actor !== "host" && session && (
              <div className="grid gap-2">
                <p>Esta sesión no administra espacios.</p>
                <button
                  disabled={busy}
                  onClick={() => signIn("host")}
                  className="min-h-11 rounded border px-3"
                >
                  Entrar como propietario de demostración
                </button>
                <button
                  disabled={busy}
                  onClick={signOut}
                  className="min-h-11 rounded border px-3"
                >
                  Cerrar sesión de demostración
                </button>
              </div>
            )}
            {owner && session?.actor === "host" && (
              <>
                <label className="grid gap-1 text-sm">
                  Estado demo
                  <select
                    id="demo-space-state"
                    defaultValue={ownerSpace?.state ?? "activa"}
                    className="min-h-11 rounded-md border px-3"
                  >
                    <option value="activa">Activa</option>
                    <option value="borrador">Borrador</option>
                    <option value="oculta">Oculta</option>
                  </select>
                </label>
                <label className="grid gap-1 text-sm">
                  Precio unitario CLP
                  <input
                    id="demo-space-price"
                    type="number"
                    min="1"
                    step="1"
                    defaultValue={ownerSpace?.unit_price_clp ?? ""}
                    className="min-h-11 rounded-md border px-3"
                  />
                </label>
                <div className="grid grid-cols-2 gap-3">
                  <label className="grid gap-1 text-sm">
                    Abre (0–24)
                    <input
                      id="demo-open-hour"
                      type="number"
                      min="0"
                      max="23"
                      step="1"
                      defaultValue={ownerSpace?.open_hour ?? 9}
                      className="min-h-11 rounded-md border px-3"
                    />
                  </label>
                  <label className="grid gap-1 text-sm">
                    Cierra (1–24)
                    <input
                      id="demo-close-hour"
                      type="number"
                      min="1"
                      max="24"
                      step="1"
                      defaultValue={ownerSpace?.close_hour ?? 18}
                      className="min-h-11 rounded-md border px-3"
                    />
                  </label>
                </div>
                <button
                  type="button"
                  disabled={!ownerSpace || busy || uncertain}
                  className="min-h-11 rounded-md border px-3"
                  onClick={async () => {
                    if (!ownerSpace) return;
                    try {
                      const state = (
                        document.getElementById(
                          "demo-space-state",
                        ) as HTMLSelectElement
                      ).value as DemoOwnerSpace["state"];
                      const unit_price_clp = (
                        document.getElementById(
                          "demo-space-price",
                        ) as HTMLInputElement
                      ).value;
                      const open_hour = Number(
                        (
                          document.getElementById(
                            "demo-open-hour",
                          ) as HTMLInputElement
                        ).value,
                      );
                      const close_hour = Number(
                        (
                          document.getElementById(
                            "demo-close-hour",
                          ) as HTMLInputElement
                        ).value,
                      );
                      if (
                        !/^[1-9][0-9]*$/.test(unit_price_clp) ||
                        !Number.isInteger(open_hour) ||
                        !Number.isInteger(close_hour) ||
                        open_hour < 0 ||
                        close_hour > 24 ||
                        close_hour <= open_hour
                      )
                        throw new Error();
                      fence();
                      clearJourney();
                      setBusy(true);
                      await rentalDemoAdapter.updateSpace(spaceId, {
                        expected_revision: ownerSpace.revision,
                        state,
                        unit_price_clp,
                        open_hour,
                        close_hour,
                      });
                      const [nextSpace, nextMonth] = await Promise.all([
                        rentalDemoAdapter.spaces(true),
                        rentalDemoAdapter.month(spaceId, month, duration, true),
                      ]);
                      setSpaces([...nextSpace]);
                      setCalendar(nextMonth);
                    } catch {
                      setUncertain(true);
                      setError(
                        "No se pudo confirmar la configuración. Actualiza antes de volver a guardar.",
                      );
                    } finally {
                      setBusy(false);
                    }
                  }}
                >
                  Guardar configuración
                </button>
                <label className="grid gap-1 text-sm">
                  Bloquear desde (hora local)
                  <input
                    id="demo-block-start"
                    type="datetime-local"
                    className="min-h-11 rounded-md border px-2"
                  />
                </label>
                <label className="grid gap-1 text-sm">
                  Hasta (hora local)
                  <input
                    id="demo-block-end"
                    type="datetime-local"
                    className="min-h-11 rounded-md border px-2"
                  />
                </label>
                <button
                  type="button"
                  disabled={!date || busy || uncertain}
                  className="min-h-11 rounded-md border px-3"
                  onClick={async () => {
                    try {
                      if (!("revision" in calendar)) throw new Error();
                      const a = (
                        document.getElementById(
                          "demo-block-start",
                        ) as HTMLInputElement
                      ).value;
                      const b = (
                        document.getElementById(
                          "demo-block-end",
                        ) as HTMLInputElement
                      ).value;
                      if (!a || !b) throw new Error();
                      const start = localDateTimeToUtcISOString(
                        a,
                        calendar.time_zone,
                      );
                      const end = localDateTimeToUtcISOString(
                        b,
                        calendar.time_zone,
                      );
                      if (Date.parse(start) >= Date.parse(end))
                        throw new Error();
                      clearJourney();
                      fence();
                      setBusy(true);
                      await rentalDemoAdapter.createBlock(spaceId, {
                        expected_revision: calendar.revision,
                        start_at: start,
                        end_at: end,
                      });
                      setCalendar(
                        await rentalDemoAdapter.month(
                          spaceId,
                          month,
                          duration,
                          true,
                        ),
                      );
                    } catch {
                      setUncertain(true);
                      setError(
                        "No se guardó con certeza. Actualiza antes de volver a escribir; revisa fecha y hora.",
                      );
                    } finally {
                      setBusy(false);
                    }
                  }}
                >
                  Bloquear rango exacto
                </button>
                {"blocks" in calendar &&
                  calendar.blocks.map((block) => (
                    <div
                      key={block.id}
                      className="flex min-w-0 flex-wrap items-center justify-between gap-2 border-t py-2 text-xs"
                    >
                      <span className="break-all">
                        {block.start_at} — {block.end_at}
                      </span>
                      <button
                        type="button"
                        disabled={busy || uncertain}
                        className="min-h-11 rounded border px-2"
                        onClick={async () => {
                          try {
                            setBusy(true);
                            await rentalDemoAdapter.deleteBlock(
                              spaceId,
                              block.id,
                              calendar.revision,
                            );
                            setCalendar(
                              await rentalDemoAdapter.month(
                                spaceId,
                                month,
                                duration,
                                true,
                              ),
                            );
                          } catch {
                            setUncertain(true);
                            setError(genericError);
                          } finally {
                            setBusy(false);
                          }
                        }}
                      >
                        Retirar bloqueo
                      </button>
                    </div>
                  ))}
              </>
            )}
            {selectedDay && (
              <>
                <h3 className="font-medium">{selectedDay.date}</h3>
                {selectedDay.intervals.length ? (
                  <div className="grid gap-2">
                    {selectedDay.intervals.map((i) => (
                      <button
                        key={`${i.start_at}/${i.end_at}`}
                        type="button"
                        disabled={busy || uncertain}
                        aria-pressed={slot?.start_at === i.start_at}
                        onClick={() => {
                          clearJourney();
                          setSlot(i);
                        }}
                        className="min-h-11 break-all rounded border p-2 text-left text-xs"
                      >
                        {i.start_at} – {i.end_at}
                      </button>
                    ))}
                  </div>
                ) : (
                  <p className="text-sm">
                    No hay intervalos disponibles para esta fecha.
                  </p>
                )}
              </>
            )}
            {!owner && (
              <>
                <p className="text-sm">KYC omitido en esta demo.</p>
                {!session ? (
                  <div className="grid gap-2">
                    <p>Para continuar, entra con un actor sintético.</p>
                    <button
                      disabled={busy}
                      onClick={() => signIn("renter")}
                      className="min-h-11 rounded border px-3"
                    >
                      Entrar como arrendatario de demostración
                    </button>
                    <button
                      disabled={busy}
                      onClick={() => signIn("renter2")}
                      className="min-h-11 rounded border px-3"
                    >
                      Entrar como segundo arrendatario de demostración
                    </button>
                  </div>
                ) : (
                  <div className="space-y-3">
                    <p>Sesión sintética: {session.actor}</p>
                    <div className="flex flex-wrap gap-2">
                      <button
                        disabled={busy || session.actor === "renter"}
                        onClick={() => signIn("renter")}
                        className="min-h-11 rounded border px-3"
                      >
                        Entrar como arrendatario de demostración
                      </button>
                      <button
                        disabled={busy || session.actor === "renter2"}
                        onClick={() => signIn("renter2")}
                        className="min-h-11 rounded border px-3"
                      >
                        Entrar como segundo arrendatario de demostración
                      </button>
                      <button
                        disabled={busy}
                        onClick={signOut}
                        className="min-h-11 rounded border px-3"
                      >
                        Cerrar sesión de demostración
                      </button>
                    </div>
                    <button
                      disabled={!slot || busy || uncertain}
                      onClick={loadQuote}
                      className="min-h-11 rounded bg-primary px-4 text-primary-foreground"
                    >
                      Consultar cotización del servidor
                    </button>
                    {quote && (
                      <div className="space-y-3 rounded border p-3">
                        <h3 className="font-semibold">
                          Cotización de demostración
                        </h3>
                        <p>
                          {quote.subtotal_clp} {quote.currency} · {quote.units}{" "}
                          unidad(es)
                        </p>
                        <p className="break-all text-xs">
                          Snapshot: {quote.id} · vence {quote.expires_at}
                        </p>
                        <p>
                          Confirmación simulada: no se genera ni firma un
                          contrato.
                        </p>
                        <label className="flex min-h-11 items-center gap-2">
                          <input
                            type="checkbox"
                            checked={approved}
                            onChange={(e) => setApproved(e.target.checked)}
                          />
                          Aprobar contrato
                        </label>
                        <button
                          disabled={!approved || busy || uncertain}
                          onClick={pay}
                          className="min-h-11 rounded bg-primary px-4 text-primary-foreground"
                        >
                          Pagar (simulado)
                        </button>
                      </div>
                    )}
                  </div>
                )}
              </>
            )}
            {result && (
              <div
                role="status"
                className="break-words rounded border border-emerald-400 bg-emerald-50 p-3 text-emerald-800"
              >
                <h3 className="font-semibold">Pagado (simulado)</h3>
                <p>Intento: {result.intent_id}</p>
                <p>
                  Snapshot {result.quote_id} · {result.subtotal_clp} CLP
                </p>
              </div>
            )}
          </div>
        </div>
      ) : (
        <div role="status" className="rounded border p-4">
          {busy ? "Cargando…" : "Calendario no cargado."}
          <button
            type="button"
            onClick={refresh}
            className="ml-3 min-h-11 underline"
          >
            Actualizar
          </button>
        </div>
      )}
      <p className="text-xs text-muted-foreground">
        Confirmación simulada: no se genera ni firma un contrato. Sin cobros
        reales ni reserva comercial.
      </p>
    </section>
  );
}
