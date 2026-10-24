import Image from "next/image";
import Link from "next/link";

export function Brand({
  href = "/",
  compact = false,
}: {
  href?: string;
  compact?: boolean;
}) {
  return (
    <Link className="brand-lockup" href={href} aria-label="EspaciGo, inicio">
      <Image
        src="/espacigo-mark.svg"
        alt=""
        width={40}
        height={40}
        priority
        className="brand-lockup__mark"
      />
      {!compact && <span className="brand-lockup__name">EspaciGo</span>}
    </Link>
  );
}
