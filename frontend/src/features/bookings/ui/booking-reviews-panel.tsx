"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@/features/auth/ui/use-auth";
import { useBookings } from "./use-bookings";
import {
  validateReviewRating,
  validateReviewComment,
} from "../domain/booking-rules";
import { Button } from "@/components/workspace/ui/button";
import { Star, Flag, CheckCircle, AlertCircle, Loader2 } from "lucide-react";

interface BookingReviewsPanelProps {
  reservationId: string;
}

export function BookingReviewsPanel({
  reservationId,
}: BookingReviewsPanelProps) {
  const { snapshot: authSnapshot } = useAuth();
  const isAuthenticated = authSnapshot.status === "authenticated";
  const accountId = authSnapshot.accountId;
  const expiresAt = authSnapshot.expiresAt;

  const {
    reviews,
    actionLoading,
    error,
    loadReviews,
    submitReview,
    reportReview,
  } = useBookings();

  const [rating, setRating] = useState<number>(5);
  const [hoverRating, setHoverRating] = useState<number | null>(null);
  const [comment, setComment] = useState<string>("");
  const [targetType, setTargetType] = useState<"espacio" | "arrendatario">(
    "espacio",
  );
  const [reportingReviewId, setReportingReviewId] = useState<string | null>(
    null,
  );
  const [reportReason, setReportReason] = useState<
    "insultos_acoso" | "datos_personales" | "spam" | "ajeno_experiencia"
  >("ajeno_experiencia");
  const [reportComment, setReportComment] = useState<string>("");
  const [statusMessage, setStatusMessage] = useState<string | null>(null);
  const [validationError, setValidationError] = useState<string | null>(null);

  const accountKey = `${authSnapshot.status}:${accountId}:${expiresAt}`;
  const [prevAccountKey, setPrevAccountKey] = useState(accountKey);

  if (prevAccountKey !== accountKey) {
    setPrevAccountKey(accountKey);
    setRating(5);
    setHoverRating(null);
    setComment("");
    setTargetType("espacio");
    setReportingReviewId(null);
    setReportReason("ajeno_experiencia");
    setReportComment("");
    setStatusMessage(null);
    setValidationError(null);
  }

  useEffect(() => {
    if (isAuthenticated && reservationId) {
      loadReviews(reservationId).catch(() => {});
    }
  }, [isAuthenticated, accountId, expiresAt, reservationId, loadReviews]);

  const handleSubmitReview = async (e: React.FormEvent) => {
    e.preventDefault();
    setValidationError(null);
    setStatusMessage(null);

    if (!validateReviewRating(rating)) {
      setValidationError(
        "La calificación debe ser un entero entre 1 y 5 estrellas.",
      );
      return;
    }

    const commentValidation = validateReviewComment(comment);
    if (!commentValidation.valid) {
      setValidationError(commentValidation.error || "Comentario no válido");
      return;
    }

    if (!isAuthenticated) {
      setValidationError("Debes iniciar sesión para publicar una reseña.");
      return;
    }

    try {
      await submitReview(reservationId, {
        rating,
        comment: commentValidation.sanitized,
        target_type: targetType,
      });
      setStatusMessage("¡Reseña publicada con éxito!");
      setComment("");
    } catch (err) {
      setValidationError(
        err instanceof Error ? err.message : "Error al publicar reseña.",
      );
    }
  };

  const handleReport = async (reviewId: string) => {
    if (!isAuthenticated) return;
    try {
      await reportReview(reservationId, reviewId, {
        reason_code: reportReason,
        comment: reportComment,
      });
      setStatusMessage("Reseña reportada para moderación administrativa.");
      setReportingReviewId(null);
      setReportComment("");
    } catch (err) {
      setValidationError(
        err instanceof Error ? err.message : "Error al reportar reseña.",
      );
    }
  };

  return (
    <section className="rounded-2xl border border-border bg-card p-4 sm:p-6 shadow-sm">
      <div className="border-b border-border pb-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Star className="h-5 w-5 text-amber-500 fill-amber-500" />
          <h2 className="text-lg font-semibold text-foreground">
            Reseñas y Evaluación del Arriendo
          </h2>
        </div>
        <span className="text-xs text-muted-foreground">
          {reviews.length} evaluación{reviews.length !== 1 ? "es" : ""}
        </span>
      </div>

      {(validationError || error) && (
        <div
          role="alert"
          className="mt-4 flex items-start gap-2.5 rounded-xl border border-destructive/20 bg-destructive/10 p-3 text-xs text-red-800 dark:text-red-300"
        >
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
          <div>
            <p className="font-medium">Aviso de Reseña</p>
            <p>{validationError || error}</p>
          </div>
        </div>
      )}

      {statusMessage && (
        <div
          role="status"
          className="mt-4 flex items-start gap-2.5 rounded-xl border border-emerald-500/20 bg-emerald-500/10 p-3 text-xs text-emerald-700 dark:text-emerald-400"
        >
          <CheckCircle className="mt-0.5 h-4 w-4 shrink-0" />
          <p>{statusMessage}</p>
        </div>
      )}

      {/* Formulario de nueva reseña */}
      <form
        onSubmit={handleSubmitReview}
        className="mt-5 space-y-4 rounded-xl border border-border bg-muted/20 p-4"
      >
        <h3 className="text-sm font-semibold text-foreground">
          Dejar una reseña sobre esta experiencia
        </h3>

        <div className="flex flex-wrap items-center gap-4">
          <div>
            <label className="block text-xs font-medium text-muted-foreground mb-1">
              Calificación (1 a 5 estrellas)
            </label>
            <div
              className="flex items-center gap-1"
              role="radiogroup"
              aria-label="Puntaje de estrellas"
            >
              {[1, 2, 3, 4, 5].map((star) => {
                const isFilled = (hoverRating ?? rating) >= star;
                return (
                  <button
                    key={star}
                    type="button"
                    role="radio"
                    aria-checked={rating === star}
                    aria-label={`${star} estrella${star > 1 ? "s" : ""}`}
                    onClick={() => setRating(star)}
                    onMouseEnter={() => setHoverRating(star)}
                    onMouseLeave={() => setHoverRating(null)}
                    className="p-1 transition-transform hover:scale-110 focus:outline-none focus:ring-1 focus:ring-primary rounded"
                  >
                    <Star
                      className={`h-6 w-6 ${
                        isFilled
                          ? "fill-amber-400 text-amber-500"
                          : "text-muted-foreground/40"
                      }`}
                    />
                  </button>
                );
              })}
            </div>
          </div>

          <div>
            <label
              htmlFor="review-target"
              className="block text-xs font-medium text-muted-foreground mb-1"
            >
              Evaluar a
            </label>
            <select
              id="review-target"
              value={targetType}
              onChange={(e) =>
                setTargetType(e.target.value as "espacio" | "arrendatario")
              }
              className="rounded-lg border border-input bg-background px-3 py-1.5 text-xs shadow-sm"
            >
              <option value="espacio">El Espacio</option>
              <option value="arrendatario">El Arrendatario</option>
            </select>
          </div>
        </div>

        <div>
          <label
            htmlFor="review-comment"
            className="block text-xs font-medium text-foreground"
          >
            Comentario de la reseña (opcional, máx 1000 caracteres)
          </label>
          <textarea
            id="review-comment"
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            maxLength={1000}
            rows={3}
            placeholder="Comparte detalles sobre el estado del espacio, puntualidad o condiciones del arriendo..."
            className="mt-1 w-full rounded-lg border border-input bg-background p-3 text-xs shadow-sm placeholder:text-muted-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
          />
        </div>

        <Button
          type="submit"
          disabled={actionLoading || !isAuthenticated}
          size="sm"
          className="inline-flex items-center gap-1.5"
        >
          {actionLoading ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <>
              <Star className="h-4 w-4" />
              <span>Publicar Reseña</span>
            </>
          )}
        </Button>
      </form>

      {/* Lista de reseñas existentes */}
      <div className="mt-6 space-y-3">
        <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          Historial de Reseñas de la Reserva ({reviews.length})
        </h3>
        {reviews.length === 0 ? (
          <p className="text-xs text-muted-foreground italic">
            Aún no se han registrado reseñas para este arriendo.
          </p>
        ) : (
          reviews.map((rev) => (
            <div
              key={rev.id}
              className="rounded-xl border border-border bg-card p-4 text-xs space-y-2"
            >
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <div className="flex">
                    {[1, 2, 3, 4, 5].map((s) => (
                      <Star
                        key={s}
                        className={`h-4 w-4 ${
                          s <= rev.rating
                            ? "fill-amber-400 text-amber-500"
                            : "text-muted-foreground/30"
                        }`}
                      />
                    ))}
                  </div>
                  <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] capitalize text-muted-foreground">
                    {rev.target_type ?? "general"}
                  </span>
                </div>

                <div className="flex items-center gap-2">
                  <span className="text-[10px] text-muted-foreground">
                    {new Date(rev.created_at).toLocaleDateString("es-CL")}
                  </span>
                  <button
                    type="button"
                    onClick={() => setReportingReviewId(rev.id)}
                    className="text-muted-foreground hover:text-destructive p-1 rounded transition-colors"
                    title="Reportar reseña inapropiada"
                  >
                    <Flag className="h-3.5 w-3.5" />
                  </button>
                </div>
              </div>

              {rev.comment && (
                <p className="text-sm text-foreground whitespace-pre-wrap">
                  {rev.comment}
                </p>
              )}

              {/* Modal/Formulario de reporte */}
              {reportingReviewId === rev.id && (
                <div className="mt-3 rounded-lg border border-destructive/20 bg-destructive/5 p-3 space-y-2">
                  <p className="font-medium text-destructive text-xs">
                    Reportar esta reseña a moderación
                  </p>
                  <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
                    <div>
                      <label className="block text-[11px] text-muted-foreground">
                        Motivo del reporte:
                      </label>
                      <select
                        value={reportReason}
                        onChange={(e) =>
                          setReportReason(
                            e.target.value as
                              | "insultos_acoso"
                              | "datos_personales"
                              | "spam"
                              | "ajeno_experiencia",
                          )
                        }
                        className="mt-1 w-full rounded border border-input bg-background p-1 text-xs"
                      >
                        <option value="insultos_acoso">Insultos o acoso</option>
                        <option value="datos_personales">
                          Datos personales expuestos
                        </option>
                        <option value="spam">Publicidad o spam</option>
                        <option value="ajeno_experiencia">
                          Ajeno a la experiencia real
                        </option>
                      </select>
                    </div>
                    <div>
                      <label className="block text-[11px] text-muted-foreground">
                        Detalle adicional (opcional):
                      </label>
                      <input
                        type="text"
                        value={reportComment}
                        onChange={(e) => setReportComment(e.target.value)}
                        placeholder="Explica brevemente..."
                        className="mt-1 w-full rounded border border-input bg-background p-1 text-xs"
                      />
                    </div>
                  </div>
                  <div className="flex gap-2 pt-1">
                    <Button
                      size="sm"
                      variant="destructive"
                      onClick={() => handleReport(rev.id)}
                      disabled={actionLoading}
                    >
                      Confirmar Reporte
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => setReportingReviewId(null)}
                    >
                      Cancelar
                    </Button>
                  </div>
                </div>
              )}
            </div>
          ))
        )}
      </div>
    </section>
  );
}
