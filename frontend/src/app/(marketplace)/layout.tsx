import Image from "next/image";
import Link from "next/link";
import { Footer } from "@/components/footer";
import { Button } from "@/components/workspace/ui/button";

export default function MarketplaceLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="min-h-screen flex flex-col bg-background text-foreground">
      <header className="border-b border-border sticky top-0 z-40 bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
        <div className="container mx-auto px-4 h-16 flex items-center justify-between gap-4">
          <Link
            href="/"
            className="flex items-center gap-2 font-bold text-xl tracking-tight text-primary"
          >
            <Image
              src="/espacigo-mark.svg"
              alt=""
              width={32}
              height={32}
              className="h-8 w-8 rounded-lg"
              priority
            />
            <span>EspaciGo</span>
          </Link>

          <nav
            aria-label="Navegación principal"
            className="flex items-center gap-4 text-sm font-medium"
          >
            <Link
              href="/explorar"
              className="text-muted-foreground hover:text-foreground transition-colors hidden sm:inline-block"
            >
              Explorar
            </Link>
            <Link
              href="/anfitrion"
              className="text-muted-foreground hover:text-foreground transition-colors hidden sm:inline-block"
            >
              Publicar espacio
            </Link>
            <Button asChild size="sm" variant="default">
              <Link href="/acceso">Acceso</Link>
            </Button>
          </nav>
        </div>
      </header>
      <main id="main-content" tabIndex={-1} className="flex-1">
        {children}
      </main>
      <Footer />
    </div>
  );
}
