import { Check } from "lucide-react";
import { Button } from "@/components/workspace/ui/button";

const plans = [
  {
    name: "Exploración",
    description: "Composición ilustrativa del catálogo público.",
    features: [
      "Fichas de espacios de muestra",
      "Búsqueda visual",
      "Sin disponibilidad conectada",
    ],
  },
  {
    name: "Gestión",
    description: "Composición ilustrativa del panel de trabajo.",
    features: [
      "Vistas de cuenta de muestra",
      "Estados sintéticos",
      "Sin sesión ni persistencia",
    ],
  },
] as const;

export default function PricingPage() {
  return (
    <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
      <h1 className="sr-only">Planes ilustrativos</h1>
      <div className="grid md:grid-cols-2 gap-8 max-w-xl mx-auto">
        {plans.map((plan) => (
          <section key={plan.name} className="pt-6">
            <h2 className="text-2xl font-medium text-gray-900 mb-2">
              {plan.name}
            </h2>
            <p className="text-sm text-gray-600 mb-4">{plan.description}</p>
            <p className="text-4xl font-medium text-gray-900 mb-6">
              Por definir{" "}
              <span className="text-xl font-normal text-gray-600">
                · sin tarifa
              </span>
            </p>
            <ul className="space-y-4 mb-8">
              {plan.features.map((feature) => (
                <li key={feature} className="flex items-start">
                  <Check
                    className="h-5 w-5 text-orange-500 mr-2 mt-0.5 flex-shrink-0"
                    aria-hidden="true"
                  />
                  <span className="text-gray-700">{feature}</span>
                </li>
              ))}
            </ul>
            <Button type="button" className="w-full" disabled>
              Solo vista previa
            </Button>
          </section>
        ))}
      </div>
      <p className="mt-8 text-center text-xs text-gray-600">
        Demo no funcional: no hay precios, suscripciones, checkout ni pagos.
      </p>
    </main>
  );
}
