"use client";

import { useEffect, useState, useRef } from "react";
import { useAuth } from "@/features/auth/ui/use-auth";
import { useBookings } from "./use-bookings";
import { sanitizeMessageBody } from "../domain/booking-rules";
import { Button } from "@/components/workspace/ui/button";
import {
  MessageSquare,
  Send,
  Loader2,
  AlertCircle,
  CheckCheck,
} from "lucide-react";

interface BookingConversationPanelProps {
  reservationId: string;
}

export function BookingConversationPanel({
  reservationId,
}: BookingConversationPanelProps) {
  const { snapshot: authSnapshot } = useAuth();
  const isAuthenticated = authSnapshot.status === "authenticated";
  const accountId = authSnapshot.accountId;
  const expiresAt = authSnapshot.expiresAt;

  const {
    messages,
    actionLoading,
    error,
    loadMessages,
    sendMessage,
    markMessagesRead,
  } = useBookings();

  const [body, setBody] = useState("");
  const [validationError, setValidationError] = useState<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const accountKey = `${authSnapshot.status}:${accountId}:${expiresAt}`;
  const [prevAccountKey, setPrevAccountKey] = useState(accountKey);

  if (prevAccountKey !== accountKey) {
    setPrevAccountKey(accountKey);
    setBody("");
    setValidationError(null);
  }

  // Load messages on mount
  useEffect(() => {
    if (isAuthenticated && reservationId) {
      loadMessages(reservationId).catch(() => {});
    }
  }, [isAuthenticated, accountId, expiresAt, reservationId, loadMessages]);

  // Advance read cursor if messages exist
  useEffect(() => {
    if (isAuthenticated && reservationId && messages.length > 0) {
      const lastSeq = Math.max(...messages.map((m) => m.sequence));
      if (lastSeq > 0) {
        markMessagesRead(reservationId, lastSeq).catch(() => {});
      }
    }
  }, [
    isAuthenticated,
    accountId,
    expiresAt,
    reservationId,
    messages,
    markMessagesRead,
  ]);

  // Auto scroll to bottom
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages.length]);

  const handleSendMessage = async (e: React.FormEvent) => {
    e.preventDefault();
    setValidationError(null);

    const validation = sanitizeMessageBody(body);
    if (!validation.valid || !validation.sanitized) {
      setValidationError(validation.error || "Mensaje inválido");
      return;
    }

    if (!isAuthenticated) {
      setValidationError("Debes iniciar sesión para enviar mensajes.");
      return;
    }

    try {
      await sendMessage(reservationId, validation.sanitized);
      setBody("");
    } catch (err) {
      setValidationError(
        err instanceof Error ? err.message : "Error al enviar el mensaje.",
      );
    }
  };

  return (
    <section className="rounded-2xl border border-border bg-card p-4 sm:p-6 shadow-sm">
      <div className="border-b border-border pb-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <MessageSquare className="h-5 w-5 text-primary" />
          <h2 className="text-lg font-semibold text-foreground">
            Mensajería Privada de la Reserva
          </h2>
        </div>
        <span className="text-xs text-muted-foreground">
          {messages.length} mensaje{messages.length !== 1 ? "s" : ""}
        </span>
      </div>

      {(validationError || error) && (
        <div
          role="alert"
          className="mt-4 flex items-start gap-2.5 rounded-xl border border-destructive/20 bg-destructive/10 p-3 text-xs text-red-800 dark:text-red-300"
        >
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
          <div>
            <p className="font-medium">Aviso de Mensajería</p>
            <p>{validationError || error}</p>
          </div>
        </div>
      )}

      {/* Messages Scroll Area */}
      <div
        role="log"
        aria-live="polite"
        className="mt-4 max-h-80 min-h-48 overflow-y-auto rounded-xl border border-border bg-muted/20 p-4 space-y-3"
      >
        {messages.length === 0 ? (
          <div className="py-12 text-center text-xs text-muted-foreground">
            No hay mensajes registrados en esta reserva aún.
            <br />
            Escribe un mensaje para comunicarte directamente de forma segura.
          </div>
        ) : (
          messages.map((msg) => (
            <div
              key={msg.id}
              className="rounded-xl border border-border bg-card p-3 shadow-2xs space-y-1"
            >
              <div className="flex items-center justify-between text-[11px] text-muted-foreground">
                <span className="font-mono font-medium text-foreground">
                  Autor: {msg.author_id.slice(0, 8)}...
                </span>
                <span className="flex items-center gap-1">
                  {new Date(msg.created_at).toLocaleTimeString("es-CL", {
                    hour: "2-digit",
                    minute: "2-digit",
                  })}
                  <CheckCheck className="h-3 w-3 text-emerald-500" />
                </span>
              </div>
              {/* Anti-XSS safe text rendering */}
              <p className="whitespace-pre-wrap break-words text-sm text-foreground">
                {msg.body}
              </p>
            </div>
          ))
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* Message input */}
      <form onSubmit={handleSendMessage} className="mt-4 flex gap-2">
        <input
          type="text"
          value={body}
          onChange={(e) => setBody(e.target.value)}
          placeholder="Escribe un mensaje seguro (máx. 2000 caracteres)..."
          maxLength={2000}
          className="flex-1 rounded-lg border border-input bg-background px-3.5 py-2 text-sm shadow-sm placeholder:text-muted-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
          disabled={actionLoading || !isAuthenticated}
        />
        <Button
          type="submit"
          disabled={actionLoading || !isAuthenticated || !body.trim()}
          className="inline-flex items-center gap-1.5"
        >
          {actionLoading ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <>
              <Send className="h-4 w-4" />
              <span>Enviar</span>
            </>
          )}
        </Button>
      </form>
    </section>
  );
}
