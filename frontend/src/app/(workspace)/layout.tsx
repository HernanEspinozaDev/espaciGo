import Image from "next/image";
import Link from "next/link";
import { Button } from "@/components/workspace/ui/button";

function Header() {
  return (
    <header className="border-b border-gray-200 bg-white">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4 flex justify-between items-center">
        <Link
          href="/"
          className="flex items-center"
          aria-label="EspaciGo, volver al inicio"
        >
          <Image
            src="/espacigo-mark.svg"
            width={28}
            height={28}
            alt=""
            priority
          />
          <span className="ml-2 text-xl font-semibold text-gray-900">
            EspaciGo
          </span>
        </Link>
        <nav
          aria-label="Navegación del panel"
          className="flex items-center space-x-4 text-sm font-medium text-gray-700"
        >
          <Button asChild className="rounded-full">
            <Link href="/acceso">Acceso</Link>
          </Button>
        </nav>
      </div>
    </header>
  );
}

export default function WorkspaceLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col min-h-screen bg-white">
      <Header />
      <div id="main-content" className="flex-1" tabIndex={-1}>
        {children}
      </div>
    </div>
  );
}
