import type { HTMLAttributes } from "react";

type CardProps = HTMLAttributes<HTMLDivElement> & {
  density?: "public" | "admin";
};

export function Card({ className = "", density, ...props }: CardProps) {
  return (
    <div
      className={`surface-card${density ? ` surface-card--${density}` : ""} ${className}`.trim()}
      {...props}
    />
  );
}
