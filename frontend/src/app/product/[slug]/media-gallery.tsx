"use client";

import { ChevronLeft, ChevronRight, ZoomIn } from "lucide-react";
import { useCallback, useState } from "react";
import { Button } from "@/components/ui/button";
import { cn, isVideoUrl } from "@/lib/utils";
import { TemplateMedia } from "@/lib/template-media";

type MediaGalleryProps = {
  images: string[];
  productName: string;
};

export function MediaGallery({ images, productName }: MediaGalleryProps) {
  const displayImages = images;
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [isZoomed, setIsZoomed] = useState(false);

  const handlePrevious = useCallback(() => {
    setSelectedIndex((prev) =>
      prev === 0 ? displayImages.length - 1 : prev - 1,
    );
  }, [displayImages.length]);

  const handleNext = useCallback(() => {
    setSelectedIndex((prev) =>
      prev === displayImages.length - 1 ? 0 : prev + 1,
    );
  }, [displayImages.length]);

  // Keyboard navigation: ArrowLeft / ArrowRight (scoped to gallery container)
  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLDivElement>) => {
      if (displayImages.length <= 1) return;

      if (e.key === "ArrowLeft") {
        e.preventDefault();
        handlePrevious();
      } else if (e.key === "ArrowRight") {
        e.preventDefault();
        handleNext();
      }
    },
    [displayImages.length, handlePrevious, handleNext],
  );

  if (displayImages.length === 0) {
    return (
      <div className="flex flex-col gap-4 lg:sticky lg:top-24 lg:self-start">
        <div className="aspect-square bg-secondary rounded-2xl flex items-center justify-center">
          <p className="text-muted-foreground">No hay imágenes disponibles</p>
        </div>
      </div>
    );
  }

  return (
    // WAI-APG carousel: a named region; arrow keys bubble up from the focusable controls inside.
    <section
      aria-roledescription="carousel"
      aria-label="Galería de imágenes del espacio"
      onKeyDown={handleKeyDown}
      className="flex flex-col gap-4 outline-none lg:sticky lg:top-24 lg:self-start"
    >
      {/* Main Image */}
      <div className="group relative aspect-square overflow-hidden rounded-2xl bg-secondary">
        {isVideoUrl(displayImages[selectedIndex] ?? "") ? (
          <video
            className="absolute inset-0 w-full h-full object-cover"
            src={displayImages[selectedIndex]}
            muted
            loop
            autoPlay
            playsInline
            controls
          />
        ) : (
          <TemplateMedia
            src={displayImages[selectedIndex] ?? ""}
            alt={`${productName} — vista ${selectedIndex + 1}`}
            fill
            sizes="(max-width: 1024px) 100vw, 50vw"
            className={cn(
              "object-cover transition-transform duration-500",
              isZoomed && "scale-150 cursor-zoom-out",
            )}
            onClick={() => setIsZoomed(!isZoomed)}
            priority
            fetchPriority="high"
          />
        )}

        {/* Navigation Arrows */}
        {displayImages.length > 1 && (
          <div className="absolute inset-x-4 top-1/2 flex -translate-y-1/2 justify-between opacity-0 transition-opacity group-hover:opacity-100 focus-within:opacity-100">
            <Button
              variant="secondary"
              size="icon"
              className="h-10 w-10 rounded-full bg-background/90 shadow-lg backdrop-blur-sm hover:bg-background"
              onClick={(e) => {
                e.stopPropagation();
                handlePrevious();
              }}
              aria-label="Imagen anterior"
            >
              <ChevronLeft className="h-5 w-5" />
            </Button>
            <Button
              variant="secondary"
              size="icon"
              className="h-10 w-10 rounded-full bg-background/90 shadow-lg backdrop-blur-sm hover:bg-background"
              onClick={(e) => {
                e.stopPropagation();
                handleNext();
              }}
              aria-label="Siguiente imagen"
            >
              <ChevronRight className="h-5 w-5" />
            </Button>
          </div>
        )}

        {/* Zoom Indicator (hidden for videos) */}
        {!isVideoUrl(displayImages[selectedIndex] ?? "") && (
          <div className="absolute bottom-4 right-4 opacity-0 transition-opacity group-hover:opacity-100">
            <div className="flex items-center gap-2 rounded-full bg-background/90 px-3 py-1.5 text-xs font-medium backdrop-blur-sm">
              <ZoomIn className="h-3.5 w-3.5" />
              Ampliar imagen
            </div>
          </div>
        )}

        {/* Image Counter */}
        {displayImages.length > 1 && (
          <div className="absolute bottom-4 left-4 rounded-full bg-background/90 px-3 py-1.5 text-xs font-medium backdrop-blur-sm">
            {selectedIndex + 1} / {displayImages.length}
          </div>
        )}
      </div>

      {/* Thumbnails */}
      {displayImages.length > 1 && (
        <div className="flex gap-3 overflow-x-auto p-2 -m-2">
          {displayImages.map((image, index) => (
            <button
              key={`${image}-${index}`}
              type="button"
              onClick={() => setSelectedIndex(index)}
              className={cn(
                "relative aspect-square w-20 shrink-0 overflow-hidden rounded-lg transition-all duration-200",
                selectedIndex === index
                  ? "ring-2 ring-foreground ring-offset-2 ring-offset-background"
                  : "opacity-60 hover:opacity-100",
              )}
            >
              {isVideoUrl(image) ? (
                <video
                  className="absolute inset-0 w-full h-full object-cover"
                  src={image}
                  muted
                  playsInline
                />
              ) : (
                <TemplateMedia
                  src={image}
                  alt={`${productName} — miniatura ${index + 1}`}
                  fill
                  sizes="80px"
                  className="object-cover"
                />
              )}
            </button>
          ))}
        </div>
      )}
    </section>
  );
}
