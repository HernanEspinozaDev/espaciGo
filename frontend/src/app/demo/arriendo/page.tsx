import { Suspense } from "react";
import { headers as getRequestHeaders } from "next/headers";
import { notFound } from "next/navigation";
import { RentalDemoView } from "@/features/rental-demo/ui/rental-demo-view";

function numericLoopbackHost(host: string | null): boolean {
  if (
    !host ||
    /[^\x21-\x7e]/.test(host) ||
    host.includes(",") ||
    host.includes("@")
  )
    return false;
  const match = /^(127(?:\.\d{1,3}){3}|\[::1\])(?::(\d{1,5}))?$/.exec(host);
  if (!match) return false;
  const validPort =
    !match[2] || (Number(match[2]) > 0 && Number(match[2]) <= 65535);
  if (match[1] === "[::1]") return validPort;
  return (
    match[1]
      .split(".")
      .map(Number)
      .every((octet) => octet <= 255) && validPort
  );
}

export default async function RentalDemoPage() {
  const requestHeaders = await getRequestHeaders();
  if (
    process.env.RENTAL_DEMO_ENABLED !== "1" ||
    !numericLoopbackHost(requestHeaders.get("host"))
  )
    notFound();
  return (
    <Suspense
      fallback={
        <p className="p-6" role="status">
          Cargando demostración…
        </p>
      }
    >
      <RentalDemoView />
    </Suspense>
  );
}
