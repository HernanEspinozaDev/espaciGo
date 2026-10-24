import type { Metadata } from "next";
import { Suspense } from "react";
import { ProductGridSkeleton } from "@/components/product-grid-skeleton";
import { About } from "@/components/sections/about";
import { Hero } from "@/components/sections/hero";
import { Newsletter } from "@/components/sections/newsletter";
import { ProductGrid } from "@/components/sections/product-grid";

function FeaturedSpacesSkeleton() {
  return (
    <section className="mx-auto max-w-7xl px-4 py-16 sm:px-6 sm:py-24 lg:px-8">
      <div className="mb-12">
        <div className="h-8 w-48 animate-pulse rounded bg-secondary motion-reduce:animate-none" />
        <div className="mt-2 h-5 w-64 animate-pulse rounded bg-secondary motion-reduce:animate-none" />
      </div>
      <ProductGridSkeleton className="lg:grid-cols-3" />
    </section>
  );
}

export const metadata: Metadata = {
  title: "Espacios para trabajar, crear y reunirse",
  description: "Explora el catálogo local de espacios de muestra de EspaciGo.",
  alternates: { canonical: "/" },
};

export default function Home() {
  return (
    <>
      <Hero />
      <Suspense fallback={<FeaturedSpacesSkeleton />}>
        <ProductGrid title="Espacios destacados" limit={6} />
      </Suspense>
      <About />
      <Newsletter />
    </>
  );
}
