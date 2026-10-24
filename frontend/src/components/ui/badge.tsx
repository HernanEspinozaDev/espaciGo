import type { HTMLAttributes } from "react";

type BadgeTone = "neutral" | "success" | "warning" | "error" | "info";

type BadgeProps = HTMLAttributes<HTMLSpanElement> & {
  tone?: BadgeTone;
};

export function Badge({
  className = "",
  tone = "neutral",
  ...props
}: BadgeProps) {
  return (
    <span className={`badge badge--${tone} ${className}`.trim()} {...props} />
  );
}
