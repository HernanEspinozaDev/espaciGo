import Image from "next/image";
import Link from "next/link";
import type { Space } from "@/lib/marketplace-fixtures";
import { formatDemoClp } from "@/lib/marketplace-fixtures";

export function ProductCard({
  space,
  priority = false,
}: {
  space: Space;
  priority?: boolean;
}) {
  const [primaryImage, secondaryImage] = space.images;

  return (
    <Link href={`/product/${space.slug}`} className="group">
      <div className="relative mb-4 aspect-square overflow-hidden rounded-2xl bg-secondary">
        <Image
          src={primaryImage}
          alt={`Vista principal de ${space.name}`}
          fill
          sizes="(max-width: 640px) 100vw, (max-width: 1024px) 50vw, 33vw"
          className="object-cover transition-opacity duration-500 group-hover:opacity-0"
          priority={priority}
        />
        <Image
          src={secondaryImage}
          alt={`Vista alternativa de ${space.name}`}
          fill
          sizes="(max-width: 640px) 100vw, (max-width: 1024px) 50vw, 33vw"
          className="object-cover opacity-0 transition-opacity duration-500 group-hover:opacity-100"
        />
        <span className="absolute left-3 top-3 rounded-full bg-background/90 px-3 py-1 text-xs font-medium text-foreground backdrop-blur">
          {space.category}
        </span>
      </div>
      <div className="space-y-1">
        <h2 className="text-base font-medium text-foreground">{space.name}</h2>
        <p className="text-sm text-muted-foreground">
          {space.comuna}, {space.city} · {space.capacity}
        </p>
        <p className="pt-1 text-base font-semibold text-foreground">
          {formatDemoClp(space.rateClpPerDay)}{" "}
          <span className="text-sm font-normal text-muted-foreground">
            / día · demo
          </span>
        </p>
      </div>
    </Link>
  );
}
