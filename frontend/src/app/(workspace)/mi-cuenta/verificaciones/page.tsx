import type { Metadata } from "next";
import { VerificationFlow } from "@/features/verifications/verification-flow";
import { VerificationsView } from "@/features/verifications/ui/verifications-view";

export const metadata: Metadata = {
  title: "Verificación de Cuenta y Espacios | Mi Cuenta | EspaciGo",
  description:
    "Verifica tu identidad de usuario y acredita tus espacios de manera fácil y segura en EspaciGo.",
  robots: {
    index: false,
    follow: false,
  },
};

export default function VerificacionesPage() {
  return (
    <div className="space-y-8">
      <div className="border-b border-border/60 pb-5">
        <h1 className="text-2xl font-bold tracking-tight text-foreground">
          Verificación de cuenta y espacios
        </h1>
        <p className="text-sm text-muted-foreground mt-1.5 leading-relaxed">
          Comprueba tu identidad oficial para realizar reservas seguras y
          acredita la titularidad de tus espacios para publicar en EspaciGo.
        </p>
      </div>

      <VerificationFlow />
      <VerificationsView />
    </div>
  );
}
