"use client";

import Image, { type ImageProps } from "next/image";
import { useState } from "react";

type PublicMediaImageProps = Omit<ImageProps, "src" | "alt" | "onError"> & {
  src: string;
  fallbackSrc?: string;
  alt: string;
  showPlaceholder?: boolean;
};

export function PublicMediaImage({
  src,
  fallbackSrc,
  alt,
  showPlaceholder = true,
  ...imageProps
}: PublicMediaImageProps) {
  const [usingFallback, setUsingFallback] = useState(false);
  const [failed, setFailed] = useState(false);

  if (failed) {
    if (!showPlaceholder) return null;
    return (
      <span className="absolute inset-0 flex items-center justify-center bg-secondary">
        <Image
          src="/espacigo-mark.svg"
          alt=""
          width={40}
          height={40}
          aria-hidden="true"
          className="opacity-35"
        />
        <span className="sr-only">Imagen no disponible</span>
      </span>
    );
  }

  return (
    <Image
      {...imageProps}
      src={usingFallback && fallbackSrc ? fallbackSrc : src}
      alt={alt}
      onError={() => {
        if (!usingFallback && fallbackSrc && fallbackSrc !== src) {
          setUsingFallback(true);
        } else {
          setFailed(true);
        }
      }}
    />
  );
}
