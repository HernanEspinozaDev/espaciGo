export interface QueueTask {
  id: string;
  file: File;
  key: string;
}

export interface QueueItem extends QueueTask {
  state: "queued" | "uploading" | "pending" | "ready" | "failed" | "cancelled";
  retryAllowed: boolean;
  photoId?: string;
  discardAllowed: boolean;
}

interface GalleryUploadQueueCallbacks {
  upload(
    task: QueueTask,
    signal: AbortSignal,
  ): Promise<{ id: string; state?: string }>;
  lookup(task: QueueTask): Promise<{ state: string; photo_id?: string }>;
  confirm(task: QueueTask, photoId: string): Promise<boolean>;
  changed(items: readonly QueueItem[]): void;
}

export class GalleryUploadQueue {
  #callbacks: GalleryUploadQueueCallbacks | null;
  #items = new Map<string, QueueItem>();
  #controllers = new Map<string, AbortController>();
  #cancelledInFlight = new Set<string>();
  #checking = new Set<string>();
  #inFlight = 0;
  #scheduled = false;
  #disposed = false;

  constructor(callbacks: GalleryUploadQueueCallbacks) {
    this.#callbacks = callbacks;
  }

  getSnapshot(): readonly QueueItem[] {
    return [...this.#items.values()].map((item) => ({ ...item }));
  }

  enqueue(tasks: readonly QueueTask[]): void {
    if (this.#disposed) return;
    for (const task of tasks) {
      if (this.#items.has(task.id)) continue;
      this.#items.set(task.id, {
        ...task,
        state: "queued",
        retryAllowed: false,
        discardAllowed: false,
      });
    }
    this.#emit();
    this.#schedule();
  }

  cancel(id: string): void {
    if (this.#disposed) return;
    const item = this.#items.get(id);
    if (!item || ["ready", "failed", "cancelled"].includes(item.state)) return;
    if (item.state === "queued") {
      this.#set(id, { state: "cancelled", retryAllowed: false });
      return;
    }
    this.#set(id, { state: "pending", retryAllowed: false });
    this.#cancelledInFlight.add(id);
    this.#controllers.get(id)?.abort();
  }

  discard(id: string): void {
    const item = this.#items.get(id);
    if (
      this.#disposed ||
      !item ||
      item.state !== "failed" ||
      !item.discardAllowed ||
      this.#controllers.has(id) ||
      this.#checking.has(id)
    )
      return;
    // Local removal only after an authoritative terminal failure. Neither
    // pending nor not_found proves cancellation of an already-sent request.
    this.#items.delete(id);
    this.#emit();
  }

  retry(id: string): void {
    if (this.#disposed) return;
    const item = this.#items.get(id);
    if (!item || item.state !== "failed" || !item.retryAllowed) return;
    this.#set(id, { state: "queued", retryAllowed: false });
    this.#schedule();
  }

  async check(id: string): Promise<void> {
    const item = this.#items.get(id);
    const callbacks = this.#callbacks;
    if (
      this.#disposed ||
      !item ||
      !callbacks ||
      !["pending", "failed"].includes(item.state) ||
      this.#controllers.has(id) ||
      this.#checking.has(id)
    )
      return;
    this.#checking.add(id);
    this.#set(id, { state: "pending", retryAllowed: false });
    try {
      await this.#reconcile(item, callbacks);
    } finally {
      this.#checking.delete(id);
    }
  }

  dispose(): void {
    if (this.#disposed) return;
    this.#disposed = true;
    for (const controller of this.#controllers.values()) controller.abort();
    this.#controllers.clear();
    this.#cancelledInFlight.clear();
    this.#checking.clear();
    this.#items.clear();
    this.#callbacks = null;
  }

  #set(id: string, update: Partial<QueueItem>): void {
    const item = this.#items.get(id);
    if (!item) return;
    this.#items.set(id, {
      ...item,
      ...update,
      discardAllowed: update.discardAllowed ?? false,
    });
    this.#emit();
  }

  #emit(): void {
    if (!this.#disposed) this.#callbacks?.changed(this.getSnapshot());
  }

  #schedule(): void {
    if (this.#scheduled || this.#disposed) return;
    this.#scheduled = true;
    queueMicrotask(() => {
      this.#scheduled = false;
      this.#drain();
    });
  }

  #drain(): void {
    if (this.#disposed || !this.#callbacks) return;
    for (const item of this.#items.values()) {
      if (this.#inFlight >= 2) break;
      if (item.state !== "queued") continue;
      void this.#run(item);
    }
  }

  async #run(item: QueueItem): Promise<void> {
    const callbacks = this.#callbacks;
    if (!callbacks || this.#disposed || item.state !== "queued") return;
    const controller = new AbortController();
    this.#controllers.set(item.id, controller);
    this.#inFlight++;
    this.#set(item.id, { state: "uploading", retryAllowed: false });
    try {
      const uploaded = await callbacks.upload(item, controller.signal);
      if (this.#disposed) return;
      if (this.#cancelledInFlight.has(item.id)) {
        await this.#reconcile(item, callbacks);
        return;
      }
      const photoId = uploaded.id;
      if (uploaded.state && uploaded.state !== "ready") {
        this.#set(item.id, { state: "pending", photoId, retryAllowed: false });
      } else if (await callbacks.confirm(item, photoId)) {
        if (!this.#disposed)
          this.#set(item.id, { state: "ready", photoId, retryAllowed: false });
      } else if (!this.#disposed) {
        this.#set(item.id, { state: "pending", photoId, retryAllowed: false });
      }
    } catch {
      if (!this.#disposed) {
        await this.#reconcile(item, callbacks);
      }
    } finally {
      this.#controllers.delete(item.id);
      this.#cancelledInFlight.delete(item.id);
      this.#inFlight--;
      if (!this.#disposed) this.#schedule();
    }
  }

  async #reconcile(
    item: QueueItem,
    callbacks: GalleryUploadQueueCallbacks,
  ): Promise<void> {
    const current = this.#items.get(item.id);
    const isCurrent = () =>
      !this.#disposed && this.#items.get(item.id) === current;
    try {
      const result = await callbacks.lookup(item);
      if (!isCurrent()) return;
      if (result.state === "failed" || result.state === "not_found") {
        this.#set(item.id, {
          state: "failed",
          retryAllowed: true,
          discardAllowed: result.state === "failed",
        });
      } else if (
        result.state === "ready" &&
        result.photo_id &&
        (await callbacks.confirm(item, result.photo_id))
      ) {
        if (isCurrent())
          this.#set(item.id, {
            state: "ready",
            photoId: result.photo_id,
            retryAllowed: false,
          });
      } else {
        if (!isCurrent()) return;
        this.#set(item.id, {
          state: "pending",
          retryAllowed: false,
          ...(result.photo_id ? { photoId: result.photo_id } : {}),
        });
      }
    } catch {
      if (isCurrent())
        this.#set(item.id, { state: "pending", retryAllowed: false });
    }
  }
}
