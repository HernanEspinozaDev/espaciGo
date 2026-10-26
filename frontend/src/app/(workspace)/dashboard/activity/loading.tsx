import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/workspace/ui/card";

export default function ActivityPageSkeleton() {
  return (
    <main className="flex-1 p-4 lg:p-8">
      <h1 className="mb-2 text-2xl font-semibold text-gray-900">Actividad</h1>
      <p className="mb-6 text-sm text-gray-600">
        Cargando el historial de muestra…
      </p>
      <Card>
        <CardHeader>
          <CardTitle>Actividad reciente</CardTitle>
        </CardHeader>
        <CardContent className="min-h-24 space-y-4">
          {[0, 1, 2].map((item) => (
            <div
              key={item}
              className="h-10 animate-pulse rounded bg-gray-100 motion-reduce:animate-none"
            />
          ))}
        </CardContent>
      </Card>
    </main>
  );
}
