import Image from "next/image";
import Link from "next/link";
import type { PublicSpace } from "@/features/public-catalog/domain/types";
import {
  publicOriginalImageUrl,
  publicCoverUrl,
  publicImageUrl,
} from "@/features/public-catalog/domain/media";
import { PublicMediaImage } from "@/features/public-catalog/ui/public-media-image";
import {
  formatPublicClp,
  rateUnitLabel,
  safePublicText,
} from "@/features/public-catalog/domain/presentation";

export function ProductCard({
  space,
  priority = false,
}: {
  space: PublicSpace;
  priority?: boolean;
}) {
  const cover = publicCoverUrl(space, "thumbnail");
  const images = space.images ?? [];
  const primaryRef =
    images.find((image) => image.image_id === space.cover_image_id) ??
    images.find((image) => image.is_cover) ??
    images[0];
  const secondaryRef = images.find(
    (image) => image.image_id !== primaryRef?.image_id,
  );
  const primaryImage =
    cover ??
    (primaryRef
      ? publicImageUrl(space.space_id, primaryRef, "thumbnail")
      : null);
  const primaryFallback = primaryRef
    ? publicOriginalImageUrl(space.space_id, primaryRef)
    : null;
  const secondaryImage = secondaryRef
    ? publicImageUrl(space.space_id, secondaryRef, "thumbnail")
    : null;
  const secondaryFallback = secondaryRef
    ? publicOriginalImageUrl(space.space_id, secondaryRef)
    : null;
  const title = safePublicText(space.title);
  const price = formatPublicClp(space.base_price_clp);
  const estimate = formatPublicClp(space.estimated_total_clp);

  return (
    <Link
      href={`/espacios/${encodeURIComponent(space.space_id)}`}
      className="group block rounded-2xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-4"
    >
      <div className="relative mb-4 aspect-square overflow-hidden rounded-2xl bg-secondary">
        {primaryImage ? (
          <PublicMediaImage
            key={primaryImage}
            src={primaryImage}
            fallbackSrc={primaryFallback ?? undefined}
            alt={`Vista de ${title}`}
            fill
            unoptimized
            sizes="(max-width: 640px) 100vw, (max-width: 1024px) 50vw, 33vw"
            className={`object-cover transition-opacity duration-500 ${secondaryImage ? "group-hover:opacity-0" : ""}`}
            priority={priority}
          />
        ) : (
          <div className="absolute inset-0 flex items-center justify-center">
            <Image
              src="/espacigo-mark.svg"
              alt=""
              width={48}
              height={48}
              aria-hidden="true"
              className="opacity-35"
            />
            <span className="sr-only">Imagen no disponible</span>
          </div>
        )}
        {secondaryImage ? (
          <PublicMediaImage
            key={secondaryImage}
            src={secondaryImage}
            fallbackSrc={secondaryFallback ?? undefined}
            alt={`Vista alternativa de ${title}`}
            fill
            unoptimized
            sizes="(max-width: 640px) 100vw, (max-width: 1024px) 50vw, 33vw"
            className="pointer-events-none object-cover opacity-0 transition-opacity duration-500 group-hover:opacity-100"
            showPlaceholder={false}
          />
        ) : null}
        <span className="absolute left-3 top-3 rounded-full bg-background/90 px-3 py-1 text-xs font-medium text-foreground backdrop-blur">
          {space.category_name}
        </span>
      </div>
      <div className="space-y-1">
        <h2 className="text-base font-medium text-foreground">{title}</h2>
        <p className="text-sm text-muted-foreground">
          {[space.comuna_name, space.region_name].filter(Boolean).join(", ")}
          {space.area_m2 > 0 ? ` · ${space.area_m2} m²` : ""}
        </p>
        <p className="pt-1 text-base font-semibold text-foreground">
          {price ?? "Tarifa no disponible"}
          {price ? (
            <span className="text-sm font-normal text-muted-foreground">
              {` / ${rateUnitLabel(space.rate_unit)}`}
            </span>
          ) : null}
        </p>
        {estimate ? (
          <p className="text-sm text-muted-foreground">
            Estimación del intervalo: {estimate}. No confirma disponibilidad ni
            constituye cotización o reserva.
          </p>
        ) : null}
      </div>
    </Link>
  );
}
