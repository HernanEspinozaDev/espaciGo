"use client";
import "client-only";
import { authFacade } from "@/features/auth/auth-runtime";
import { ApiError } from "@/lib/api/errors";
import { privateBlobUrls } from "@/lib/api/private-blob-urls";
import type { ReviewCase, ReviewInput, RevokeInput } from "../domain/review";
import { revisionNumber } from "../domain/review";
import type { ReviewPort } from "./review.port";

export interface ReviewSnapshot {
  readonly accountId: string | null;
  readonly cases: readonly ReviewCase[];
  readonly selected: ReviewCase | null;
  readonly loading: boolean;
  readonly pending: boolean;
  readonly error: string | null;
}
const SERVER_SNAPSHOT: ReviewSnapshot = Object.freeze({
  accountId: null,
  cases: Object.freeze([]),
  selected: null,
  loading: false,
  pending: false,
  error: null,
});
export class ReviewFacade {
  #snapshot: ReviewSnapshot = SERVER_SNAPSHOT;
  #listeners = new Set<() => void>();
  #generation = 0;
  #selection = 0;
  #account: string | null = null;
  #permissionsKey = "";
  #mutation: symbol | null = null;
  #refreshRequired = false;
  constructor(private readonly port: ReviewPort) {}
  subscribe = (f: () => void) => {
    this.#listeners.add(f);
    return () => this.#listeners.delete(f);
  };
  getSnapshot = () => this.#snapshot;
  getServerSnapshot = () => SERVER_SNAPSHOT;
  setSession(account: string | null, permissionsKey = "") {
    if (account === this.#account && permissionsKey === this.#permissionsKey)
      return;
    this.#account = account;
    this.#permissionsKey = permissionsKey;
    this.#refreshRequired = false;
    this.#generation++;
    this.#selection++;
    this.#mutation = null;
    privateBlobUrls.revoke("document-review");
    this.#publish({ ...SERVER_SNAPSHOT, accountId: account });
  }
  clearPrivateData(error: string | null = null) {
    this.#refreshRequired = false;
    this.#generation++;
    this.#selection++;
    this.#mutation = null;
    privateBlobUrls.revoke("document-review");
    this.#publish({ ...SERVER_SNAPSHOT, accountId: this.#account, error });
  }
  #publish(next: ReviewSnapshot) {
    this.#snapshot = Object.freeze({
      ...next,
      cases: Object.freeze([...next.cases]),
    });
    for (const f of this.#listeners) f();
  }
  #current(g: number, account: string) {
    return (
      g === this.#generation &&
      account === this.#account &&
      authFacade.getSnapshot().accountId === account
    );
  }
  #error(error: unknown) {
    if (error instanceof ApiError && [401, 403, 404].includes(error.status)) {
      return "No tienes acceso vigente al expediente. Se limpiaron los datos privados.";
    }
    return "No fue posible completar la operación.";
  }
  #denied(error: unknown) {
    return error instanceof ApiError && [401, 403, 404].includes(error.status);
  }
  #abort(error: unknown) {
    return error instanceof Error && error.name === "AbortError";
  }
  async load() {
    const account = this.#account;
    if (!account || this.#mutation) return;
    const g = this.#generation,
      fence = ++this.#selection;
    this.#publish({
      ...this.#snapshot,
      loading: true,
      error: this.#refreshRequired ? this.#snapshot.error : null,
    });
    try {
      const cases = await authFacade.runAuthenticatedOperation((t, s) =>
        this.port.list(t, s),
      );
      if (this.#current(g, account) && fence === this.#selection) {
        const previous = this.#snapshot.selected;
        this.#refreshRequired = false;
        const selected = previous
          ? (cases.find((x) => x.id === previous.id) ?? null)
          : null;
        if (previous && (!selected || selected.revision !== previous.revision))
          privateBlobUrls.revoke("document-review");
        this.#publish({
          ...this.#snapshot,
          cases,
          selected,
          loading: false,
          error: null,
        });
      }
    } catch (e) {
      if (
        this.#current(g, account) &&
        fence === this.#selection &&
        !this.#abort(e)
      ) {
        const error = this.#error(e);
        const message = this.#refreshRequired
          ? this.#snapshot.error
          : `${error} Actualiza expedientes antes de continuar.`;
        this.clearPrivateData(message);
        this.#refreshRequired = true;
      }
    }
  }
  async select(id: string) {
    const account = this.#account;
    if (!account || this.#mutation || this.#refreshRequired) return;
    const g = this.#generation,
      fence = ++this.#selection;
    privateBlobUrls.revoke("document-review");
    this.#publish({
      ...this.#snapshot,
      loading: true,
      error: null,
      selected: null,
    });
    try {
      const c = await authFacade.runAuthenticatedOperation((t, s) =>
        this.port.get(t, id, s),
      );
      if (this.#current(g, account) && fence === this.#selection)
        this.#publish({ ...this.#snapshot, selected: c, loading: false });
    } catch (e) {
      if (
        this.#current(g, account) &&
        fence === this.#selection &&
        !this.#abort(e)
      ) {
        const error = this.#error(e);
        if (this.#denied(e)) {
          this.clearPrivateData(error);
          return;
        }
        if (g === this.#generation)
          this.#publish({
            ...this.#snapshot,
            selected: null,
            loading: false,
            error,
          });
      }
    }
  }
  async download(fileId: string) {
    const account = this.#account,
      c = this.#snapshot.selected;
    if (!account || !c || this.#mutation) return;
    const g = this.#generation,
      fence = this.#selection;
    try {
      const verified = await authFacade.runAuthenticatedOperation((t, s) =>
        this.port.get(t, c.id, s),
      );
      if (
        !this.#current(g, account) ||
        fence !== this.#selection ||
        this.#snapshot.selected?.id !== c.id ||
        verified.state !== c.state ||
        verified.revision !== c.revision
      )
        return;
      const f = verified.files.find(
        (x) => x.id === fileId && x.state === "limpio",
      );
      if (!f) throw new Error();
      const blob = await authFacade.runAuthenticatedOperation((t, s) =>
        this.port.download(t, c.id, fileId, s),
      );
      if (
        !this.#current(g, account) ||
        fence !== this.#selection ||
        this.#snapshot.selected?.id !== c.id
      )
        return;
      const u = privateBlobUrls.create("document-review", blob),
        a = document.createElement("a");
      a.href = u.url;
      a.download = "documento-verificacion";
      a.rel = "noopener";
      document.body.append(a);
      a.click();
      a.remove();
    } catch (e) {
      if (
        this.#current(g, account) &&
        fence === this.#selection &&
        !this.#abort(e)
      ) {
        const error = this.#error(e);
        if (this.#denied(e)) {
          this.clearPrivateData(error);
          return;
        }
        if (g === this.#generation) this.#publish({ ...this.#snapshot, error });
      }
    }
  }
  async #write(
    kind: "review" | "revoke",
    input:
      | Omit<ReviewInput, "expected_revision">
      | Omit<RevokeInput, "expected_revision">,
  ) {
    const c = this.#snapshot.selected,
      account = this.#account;
    if (!c || !account || this.#mutation) return;
    const lock = Symbol();
    this.#mutation = lock;
    const g = this.#generation,
      fence = ++this.#selection;
    this.#publish({ ...this.#snapshot, pending: true, error: null });
    try {
      await authFacade.runAuthenticatedOperation((t, s) =>
        kind === "review"
          ? this.port.review(
              t,
              c.id,
              {
                ...(input as Omit<ReviewInput, "expected_revision">),
                expected_revision: revisionNumber(c.revision),
              },
              s,
            )
          : this.port.revoke(
              t,
              c.id,
              {
                ...(input as Omit<RevokeInput, "expected_revision">),
                expected_revision: revisionNumber(c.revision),
              },
              s,
            ),
      );
      if (!this.#current(g, account) || fence !== this.#selection) return;
      const readback = await authFacade.runAuthenticatedOperation((t, s) =>
        this.port.get(t, c.id, s),
      );
      if (!this.#current(g, account) || fence !== this.#selection) return;
      if (
        readback.id !== c.id ||
        revisionNumber(readback.revision) <= revisionNumber(c.revision)
      )
        throw new Error();
      this.#publish({
        ...this.#snapshot,
        selected: readback,
        cases: this.#snapshot.cases.map((x) =>
          x.id === readback.id ? readback : x,
        ),
      });
    } catch (e) {
      if (this.#current(g, account)) {
        const error = this.#error(e);
        if (g === this.#generation)
          this.clearPrivateData(
            `${error} Actualiza expedientes antes de continuar; no se confirmó el resultado.`,
          );
        this.#refreshRequired = true;
      }
    } finally {
      if (this.#mutation === lock) {
        this.#mutation = null;
        if (this.#current(g, account))
          this.#publish({ ...this.#snapshot, pending: false });
      }
    }
  }
  decide(input: Omit<ReviewInput, "expected_revision">) {
    return this.#write("review", input);
  }
  revoke(input: Omit<RevokeInput, "expected_revision">) {
    return this.#write("revoke", input);
  }
}
