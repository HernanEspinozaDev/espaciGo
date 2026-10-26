"use client";

import Image from "next/image";
import { ChevronLeft, ChevronRight, ZoomIn, ZoomOut } from "lucide-react";
import { useCallback, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { TemplateMedia } from "@/lib/template-media";

type GalleryImage = { src: string; fallbackSrc?: string };
type MediaGalleryProps = { images: GalleryImage[]; productName: string };

export function MediaGallery({ images, productName }: MediaGalleryProps) {
  const displayImages = useMemo(
    () => images.filter((image) => image.src),
    [images],
  );
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [isZoomed, setIsZoomed] = useState(false);
  const [fallbackIndexes, setFallbackIndexes] = useState<number[]>([]);
  const [failedIndexes, setFailedIndexes] = useState<number[]>([]);

  const select = useCallback((index: number) => {
    setSelectedIndex(index);
    setIsZoomed(false);
  }, []);
  const handlePrevious = useCallback(
    () =>
      select(
        selectedIndex === 0 ? displayImages.length - 1 : selectedIndex - 1,
      ),
    [displayImages.length, select, selectedIndex],
  );
  const handleNext = useCallback(
    () =>
      select(
        selectedIndex === displayImages.length - 1 ? 0 : selectedIndex + 1,
      ),
    [displayImages.length, select, selectedIndex],
  );
  const handleKeyDown = useCallback(
    (event: React.KeyboardEvent<HTMLElement>) => {
      if (displayImages.length <= 1) return;
      if (event.key === "ArrowLeft") {
        event.preventDefault();
        handlePrevious();
      } else if (event.key === "ArrowRight") {
        event.preventDefault();
        handleNext();
      }
    },
    [displayImages.length, handleNext, handlePrevious],
  );
  const onImageError = useCallback(
    (index: number) => {
      const image = displayImages[index];
      if (image?.fallbackSrc && !fallbackIndexes.includes(index))
        setFallbackIndexes((current) => [...current, index]);
      else
        setFailedIndexes((current) =>
          current.includes(index) ? current : [...current, index],
        );
    },
    [displayImages, fallbackIndexes],
  );

  if (displayImages.length === 0) {
    return (
      <div className="lg:sticky lg:top-24 lg:self-start">
        <div className="relative flex aspect-square items-center justify-center rounded-2xl bg-secondary">
          <Image
            src="/espacigo-mark.svg"
            alt=""
            width={64}
            height={64}
            aria-hidden="true"
            className="opacity-35"
          />
          <p className="sr-only">No hay imágenes disponibles</p>
        </div>
      </div>
    );
  }
  const current = displayImages[selectedIndex];
  const src =
    current && fallbackIndexes.includes(selectedIndex)
      ? (current.fallbackSrc ?? current.src)
      : current?.src;
  const failed = failedIndexes.includes(selectedIndex);

  return (
    <section
      role="region"
      aria-roledescription="carrusel"
      aria-label="Galería de imágenes del espacio"
      onKeyDown={handleKeyDown}
      tabIndex={0}
      className="flex flex-col gap-4 outline-none focus-visible:ring-2 focus-visible:ring-ring lg:sticky lg:top-24 lg:self-start"
    >
      <div className="group relative aspect-square overflow-hidden rounded-2xl bg-secondary">
        {failed || !src ? (
          <div className="absolute inset-0 flex items-center justify-center">
            <Image
              src="/espacigo-mark.svg"
              alt=""
              width={56}
              height={56}
              aria-hidden="true"
              className="opacity-35"
            />
            <span className="sr-only">Imagen no disponible</span>
          </div>
        ) : (
          <TemplateMedia
            key={`${selectedIndex}-${src}`}
            src={src}
            alt={`${productName} — vista ${selectedIndex + 1}`}
            fill
            sizes="(max-width: 1024px) 100vw, 50vw"
            className={cn(
              "object-cover transition-transform duration-500",
              isZoomed && "scale-150",
            )}
            onError={() => onImageError(selectedIndex)}
            priority
            fetchPriority="high"
          />
        )}
        {displayImages.length > 1 ? (
          <div className="absolute inset-x-4 top-1/2 flex -translate-y-1/2 justify-between opacity-0 transition-opacity group-hover:opacity-100 focus-within:opacity-100">
            <Button
              type="button"
              variant="secondary"
              size="icon"
              className="h-10 w-10 rounded-full bg-background/90 shadow-lg"
              onClick={handlePrevious}
              aria-label="Imagen anterior"
            >
              <ChevronLeft className="h-5 w-5" />
            </Button>
            <Button
              type="button"
              variant="secondary"
              size="icon"
              className="h-10 w-10 rounded-full bg-background/90 shadow-lg"
              onClick={handleNext}
              aria-label="Siguiente imagen"
            >
              <ChevronRight className="h-5 w-5" />
            </Button>
          </div>
        ) : null}
        {!failed ? (
          <button
            type="button"
            onClick={() => setIsZoomed((value) => !value)}
            className="absolute bottom-4 right-4 inline-flex min-h-10 items-center gap-2 rounded-full bg-background/90 px-3 text-xs font-medium opacity-0 transition-opacity group-hover:opacity-100 focus:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {isZoomed ? (
              <ZoomOut className="h-4 w-4" aria-hidden="true" />
            ) : (
              <ZoomIn className="h-4 w-4" aria-hidden="true" />
            )}
            {isZoomed ? "Reducir" : "Ampliar imagen"}
          </button>
        ) : null}
        {displayImages.length > 1 ? (
          <span
            className="absolute bottom-4 left-4 rounded-full bg-background/90 px-3 py-1.5 text-xs font-medium"
            aria-live="polite"
          >
            {selectedIndex + 1} / {displayImages.length}
          </span>
        ) : null}
      </div>
      {displayImages.length > 1 ? (
        <div
          role="group"
          className="-m-2 flex gap-3 overflow-x-auto p-2"
          aria-label="Elegir imagen"
        >
          {displayImages.map((image, index) => (
            <button
              key={`${image.src}-${index}`}
              type="button"
              onClick={() => select(index)}
              aria-label={`Mostrar imagen ${index + 1}`}
              aria-pressed={selectedIndex === index}
              className={cn(
                "relative aspect-square w-20 shrink-0 overflow-hidden rounded-lg transition-all duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                selectedIndex === index
                  ? "ring-2 ring-foreground ring-offset-2 ring-offset-background"
                  : "opacity-60 hover:opacity-100",
              )}
            >
              {failedIndexes.includes(index) ? (
                <Image
                  src="/espacigo-mark.svg"
                  alt=""
                  width={32}
                  height={32}
                  aria-hidden="true"
                  className="absolute inset-0 m-auto opacity-35"
                />
              ) : (
                <TemplateMedia
                  key={
                    fallbackIndexes.includes(index)
                      ? image.fallbackSrc
                      : image.src
                  }
                  src={
                    fallbackIndexes.includes(index)
                      ? (image.fallbackSrc ?? image.src)
                      : image.src
                  }
                  alt=""
                  fill
                  sizes="80px"
                  className="object-cover"
                  aria-hidden="true"
                  onError={() => onImageError(index)}
                />
              )}
            </button>
          ))}
        </div>
      ) : null}
    </section>
  );
}
