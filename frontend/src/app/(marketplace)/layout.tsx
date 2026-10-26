import Image from "next/image";
import Link from "next/link";
import { Footer } from "@/app/footer";
import { Navbar } from "@/app/navbar";
import { SessionControls } from "@/features/auth/ui/session-controls";

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
              <SessionControls variant="public" />
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
