import { redirect } from "next/navigation";

interface PageProps {
  params: Promise<{ id: string }>;
}

export default async function EspacioQuickPage({ params }: PageProps) {
  const { id } = await params;
  redirect(`/anfitrion/espacios/${id}/editar`);
}
