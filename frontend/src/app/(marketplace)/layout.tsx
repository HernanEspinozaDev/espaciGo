import type { Metadata } from "next";
import type { ReactNode } from "react";
import {
  PublicFooter,
  PublicHeader,
} from "@/components/marketplace/public-chrome";

export const metadata: Metadata = {
  title: {
    default: "EspaciGo | Encuentra tu espacio",
    template: "%s | EspaciGo",
  },
  description: "Una maqueta visual local del marketplace EspaciGo.",
};

export default function MarketplaceLayout({
  children,
}: Readonly<{ children: ReactNode }>) {
  return (
    <div className="public-shell" data-density="public">
      <PublicHeader />
      {children}
      <PublicFooter />
    </div>
  );
}
