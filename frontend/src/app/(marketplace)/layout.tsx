import Image from "next/image";
import Link from "next/link";
import { Suspense } from "react";
import { SearchInput } from "@/components/search/search-input";
import { Footer } from "@/app/footer";
import { Navbar } from "@/app/navbar";

const links = [
  { href: "/", label: "Inicio" },
  { href: "/explorar", label: "Explorar" },
  { href: "/categorias", label: "Categorías" },
  { href: "/arriendos", label: "Arriendos residenciales" },
  { href: "/como-funciona", label: "Cómo funciona" },
];

export default function MarketplaceLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <div className="flex min-h-screen flex-col">
      <header className="sticky top-0 z-50 border-b border-border bg-background/90 backdrop-blur-md">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="relative flex h-16 items-center justify-between gap-3">
            <div className="flex min-w-0 items-center gap-3">
              <Navbar links={links} />
              <Link
                href="/"
                className="inline-flex shrink-0 items-center gap-2 text-xl font-semibold tracking-tight"
                aria-label="EspaciGo, inicio"
              >
                <Image
                  src="/espacigo-mark.svg"
                  width={30}
                  height={30}
                  alt=""
                  priority
                />
                <span>EspaciGo</span>
              </Link>
            </div>
            <div className="flex items-center gap-1 sm:gap-2">
              <div className="hidden md:block">
                <Suspense
                  fallback={<div className="h-10 w-56" aria-hidden="true" />}
                >
                  <SearchInput />
                </Suspense>
              </div>
              <Link
                href="/login"
                className="inline-flex min-h-11 items-center rounded-full border border-border px-4 text-sm font-medium transition-colors hover:bg-secondary"
              >
                Ingresar a EspaciGo
              </Link>
            </div>
          </div>
        </div>
      </header>
      <main id="main-content" className="flex-1" tabIndex={-1}>
        {children}
      </main>
      <Footer />
    </div>
  );
}
