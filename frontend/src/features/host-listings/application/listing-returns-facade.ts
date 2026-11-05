"use client";

import type { AccessToken } from "@/features/auth/application/auth.port";
import type { ListingReturnsPort } from "./listing-returns.port";
import {
  eligibleReturns,
  initialListingReturnsSnapshot,
  uncertainReturnMessage,
  type ListingReturnsSnapshot,
} from "../domain/listing-returns";

export class ListingReturnsFacade {
  #port: ListingReturnsPort;
  #snapshot: ListingReturnsSnapshot = initialListingReturnsSnapshot;
  #listeners = new Set<() => void>();
  #generation = 0;
  #keys = new Map<string, string>();
  #context: {
    token: AccessToken;
    spaceId: string;
    sessionScope: string;
  } | null = null;
  constructor(port: ListingReturnsPort) {
    this.#port = port;
  }
  subscribe = (listener: () => void) => {
    this.#listeners.add(listener);
    return () => this.#listeners.delete(listener);
  };
  getSnapshot = () => this.#snapshot;
  getServerSnapshot = () => initialListingReturnsSnapshot;
  #set(next: ListingReturnsSnapshot) {
    this.#snapshot = Object.freeze(next);
    this.#listeners.forEach((listener) => listener());
  }
  #current(generation: number, spaceId: string, sessionScope: string) {
    return (
      generation === this.#generation &&
      this.#context?.spaceId === spaceId &&
      this.#context.sessionScope === sessionScope
    );
  }
  clear() {
    this.#generation++;
    this.#context = null;
    this.#keys.clear();
    this.#set(initialListingReturnsSnapshot);
  }
  async load(
    token: AccessToken,
    spaceId: string,
    sessionScope: string,
  ): Promise<void> {
    const sameContext =
      this.#context?.spaceId === spaceId &&
      this.#context.sessionScope === sessionScope;
    if (
      sameContext &&
      this.#snapshot.pendingReservationId !== null &&
      !this.#snapshot.uncertain
    )
      return;
    if (sameContext && this.#snapshot.uncertain) {
      if (this.#snapshot.loading) return;
      const pendingId = this.#snapshot.pendingReservationId;
      if (!pendingId) return;
      const generation = ++this.#generation;
      this.#context = { token, spaceId, sessionScope };
      this.#set({ ...this.#snapshot, loading: true });
      try {
        const result = await this.#port.listReturns(token, spaceId);
        if (!this.#current(generation, spaceId, sessionScope)) return;
        if (result.space_id !== spaceId) throw new Error("Espacio distinto");
        if (
          !result.items.find((entry) => entry.reservation_id === pendingId)
            ?.vacancy_confirmed
        )
          throw new Error(uncertainReturnMessage);
        this.#keys.delete(`${sessionScope}:${spaceId}:${pendingId}`);
        this.#set({
          spaceId,
          sessionScope,
          items: eligibleReturns(result.items),
          loading: false,
          pendingReservationId: null,
          uncertain: false,
          error: null,
        });
      } catch {
        if (this.#current(generation, spaceId, sessionScope))
          this.#set({
            ...this.#snapshot,
            loading: false,
            error: uncertainReturnMessage,
          });
      }
      return;
    }
    if (sameContext && this.#snapshot.loading) return;
    const generation = ++this.#generation;
    this.#context = { token, spaceId, sessionScope };
    this.#set({
      ...initialListingReturnsSnapshot,
      spaceId,
      sessionScope,
      loading: true,
    });
    try {
      const result = await this.#port.listReturns(token, spaceId);
      if (!this.#current(generation, spaceId, sessionScope)) return;
      if (result.space_id !== spaceId) throw new Error("Espacio distinto");
      if (this.#snapshot.uncertain && this.#snapshot.pendingReservationId) {
        const pendingId = this.#snapshot.pendingReservationId;
        if (
          !result.items.find((entry) => entry.reservation_id === pendingId)
            ?.vacancy_confirmed
        )
          throw new Error(uncertainReturnMessage);
        this.#keys.delete(`${sessionScope}:${spaceId}:${pendingId}`);
      }
      this.#set({
        spaceId,
        sessionScope,
        items: eligibleReturns(result.items),
        loading: false,
        pendingReservationId: null,
        uncertain: false,
        error: null,
      });
    } catch (error) {
      if (!this.#current(generation, spaceId, sessionScope)) return;
      if (this.#snapshot.uncertain) {
        this.#set({
          ...this.#snapshot,
          loading: false,
          error: uncertainReturnMessage,
        });
        return;
      }
      this.#set({
        ...this.#snapshot,
        loading: false,
        error:
          error instanceof Error
            ? error.message
            : "No fue posible cargar entregas.",
      });
    }
  }
  async confirm(
    token: AccessToken,
    spaceId: string,
    sessionScope: string,
    reservationId: string,
  ): Promise<boolean> {
    const context = this.#context;
    if (
      !context ||
      context.spaceId !== spaceId ||
      context.sessionScope !== sessionScope ||
      this.#snapshot.uncertain ||
      this.#snapshot.pendingReservationId
    )
      return false;
    const target = this.#snapshot.items.find(
      (item) => item.reservation_id === reservationId,
    );
    if (
      !target ||
      target.state !== "finalizada" ||
      !target.check_out_recorded ||
      target.vacancy_confirmed
    )
      return false;
    const generation = this.#generation;
    const keyId = `${sessionScope}:${spaceId}:${reservationId}`;
    let key = this.#keys.get(keyId);
    if (!key) {
      key = crypto.randomUUID();
      this.#keys.set(keyId, key);
    }
    this.#set({
      ...this.#snapshot,
      pendingReservationId: reservationId,
      error: null,
    });
    try {
      await this.#port.confirmVacancy(token, reservationId, key);
      if (!this.#current(generation, spaceId, sessionScope)) return false;
    } catch {
      if (!this.#current(generation, spaceId, sessionScope)) return false;
      this.#set({
        ...this.#snapshot,
        pendingReservationId: reservationId,
        uncertain: true,
        error: uncertainReturnMessage,
      });
      return false;
    }
    try {
      if (!this.#current(generation, spaceId, sessionScope)) return false;
      this.#set({
        ...this.#snapshot,
        loading: true,
        error: null,
      });
      const result = await this.#port.listReturns(token, spaceId);
      if (!this.#current(generation, spaceId, sessionScope)) return false;
      if (result.space_id !== spaceId) throw new Error("Espacio distinto");
      const item = result.items.find(
        (entry) => entry.reservation_id === reservationId,
      );
      if (!item?.vacancy_confirmed)
        throw new Error("Vacancia aún no confirmada");
      this.#keys.delete(keyId);
      this.#set({
        spaceId,
        sessionScope,
        items: eligibleReturns(result.items),
        loading: false,
        pendingReservationId: null,
        uncertain: false,
        error: null,
      });
      return true;
    } catch {
      if (!this.#current(generation, spaceId, sessionScope)) return false;
      this.#set({
        ...this.#snapshot,
        loading: false,
        pendingReservationId: reservationId,
        uncertain: true,
        error: uncertainReturnMessage,
      });
      return false;
    }
  }
}
