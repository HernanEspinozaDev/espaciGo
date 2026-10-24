import { parseVts } from "@/lib/vts";

export type Space = {
  id: string;
  slug: string;
  name: string;
  summary: string;
  description: string;
  category: string;
  categorySlug: string;
  city: string;
  citySlug: string;
  comuna: string;
  comunaSlug: string;
  capacity: string;
  rateClpPerDay: number;
  images: readonly [string, string];
  amenities: readonly string[];
};

export type SpaceFilters = {
  category?: string;
  collection?: string;
  brand?: string;
  priceMin?: string;
  priceMax?: string;
  vts?: string;
  page?: string;
  sort?: string;
  q?: string;
};

export type SpaceFacets = {
  categories: { id: string; slug: string; name: string }[];
  collections: { id: string; slug: string; name: string }[];
  variantTypes: { label: string; values: string[] }[];
  brands: { id: string; slug: string; name: string }[];
  priceBounds: { min: number; max: number };
};

const spaces: Space[] = [
  {
    id: "demo-space-01",
    slug: "sala-luz-providencia",
    name: "Sala Luz · Providencia",
    summary: "Sala de reuniones con luz natural y mesa para seis.",
    description:
      "Espacio de muestra para reuniones pequeñas, entrevistas y jornadas de trabajo concentrado.",
    category: "Reuniones",
    categorySlug: "reuniones",
    city: "Santiago",
    citySlug: "santiago",
    comuna: "Providencia",
    comunaSlug: "providencia",
    capacity: "1–6 personas",
    rateClpPerDay: 48000,
    images: ["/spaces/sala-luz.svg", "/spaces/sala-luz-alt.svg"],
    amenities: ["Luz natural", "Mesa de reuniones", "Wi-Fi (muestra)"],
  },
  {
    id: "demo-space-02",
    slug: "estudio-norte-recoleta",
    name: "Estudio Norte · Recoleta",
    summary: "Estudio amplio para fotografía, creación y talleres.",
    description:
      "Ficha ilustrativa de un estudio flexible, presentado solo como contenido local de demostración.",
    category: "Estudios",
    categorySlug: "estudios",
    city: "Santiago",
    citySlug: "santiago",
    comuna: "Recoleta",
    comunaSlug: "recoleta",
    capacity: "1–10 personas",
    rateClpPerDay: 72000,
    images: ["/spaces/estudio-norte.svg", "/spaces/estudio-norte-alt.svg"],
    amenities: ["Fondo neutro", "Luz amplia", "Acceso flexible (muestra)"],
  },
  {
    id: "demo-space-03",
    slug: "taller-sur-nunoa",
    name: "Taller Sur · Ñuñoa",
    summary: "Taller luminoso para actividades creativas.",
    description:
      "Ejemplo sintético de taller; disponibilidad y condiciones no están conectadas a un servicio.",
    category: "Talleres",
    categorySlug: "talleres",
    city: "Santiago",
    citySlug: "santiago",
    comuna: "Ñuñoa",
    comunaSlug: "nunoa",
    capacity: "1–8 personas",
    rateClpPerDay: 56000,
    images: ["/spaces/taller-sur.svg", "/spaces/taller-sur-alt.svg"],
    amenities: ["Mesón de trabajo", "Lavadero", "Luz de día"],
  },
  {
    id: "demo-space-04",
    slug: "patio-alameda-santiago",
    name: "Patio Alameda · Santiago",
    summary: "Patio interior para encuentros y eventos pequeños.",
    description:
      "Vista de demostración inspirada en la presentación de fichas del catálogo original.",
    category: "Eventos",
    categorySlug: "eventos",
    city: "Santiago",
    citySlug: "santiago",
    comuna: "Santiago Centro",
    comunaSlug: "santiago-centro",
    capacity: "1–18 personas",
    rateClpPerDay: 89000,
    images: ["/spaces/patio-alameda.svg", "/spaces/patio-alameda-alt.svg"],
    amenities: ["Patio interior", "Área flexible", "Mobiliario de muestra"],
  },
  {
    id: "demo-space-05",
    slug: "oficina-bosque-las-condes",
    name: "Oficina Bosque · Las Condes",
    summary: "Oficina privada de muestra para equipos pequeños.",
    description:
      "Contenido sintético; la ficha no reserva ni confirma el uso del espacio.",
    category: "Oficinas",
    categorySlug: "oficinas",
    city: "Santiago",
    citySlug: "santiago",
    comuna: "Las Condes",
    comunaSlug: "las-condes",
    capacity: "1–5 personas",
    rateClpPerDay: 65000,
    images: ["/spaces/oficina-bosque.svg", "/spaces/oficina-bosque-alt.svg"],
    amenities: ["Escritorios", "Sala de descanso", "Luz natural"],
  },
  {
    id: "demo-space-06",
    slug: "cowork-valparaiso-cerro-alegre",
    name: "Casa Taller · Valparaíso",
    summary: "Casa taller para sesiones y trabajo colaborativo.",
    description:
      "Ficha estática local, sin propietario real ni disponibilidad conectada.",
    category: "Coworking",
    categorySlug: "cowork",
    city: "Valparaíso",
    citySlug: "valparaiso",
    comuna: "Cerro Alegre",
    comunaSlug: "cerro-alegre",
    capacity: "1–9 personas",
    rateClpPerDay: 52000,
    images: ["/spaces/casa-taller.svg", "/spaces/casa-taller-alt.svg"],
    amenities: ["Mesas compartidas", "Patio", "Internet (muestra)"],
  },
];

export const spaceCategories = [
  { slug: "cowork", name: "Coworking" },
  { slug: "oficinas", name: "Oficinas" },
  { slug: "reuniones", name: "Reuniones" },
  { slug: "eventos", name: "Eventos" },
  { slug: "estudios", name: "Estudios" },
  { slug: "talleres", name: "Talleres" },
] as const;

export function getSpaceBySlug(slug: string) {
  return spaces.find((space) => space.slug === slug) ?? null;
}

export function searchSpaces(query: string) {
  const normalized = query.trim().toLocaleLowerCase("es-CL");
  if (!normalized) return [];
  return spaces.filter((space) =>
    `${space.name} ${space.summary} ${space.category} ${space.city} ${space.comuna}`
      .toLocaleLowerCase("es-CL")
      .includes(normalized),
  );
}

export function getSpaceFacets(): SpaceFacets {
  const unique = (items: { id: string; slug: string; name: string }[]) =>
    Array.from(new Map(items.map((item) => [item.slug, item])).values());
  const rates = spaces.map((space) => space.rateClpPerDay);
  return {
    categories: unique(
      spaces.map((space) => ({
        id: space.categorySlug,
        slug: space.categorySlug,
        name: space.category,
      })),
    ),
    collections: unique(
      spaces.map((space) => ({
        id: space.citySlug,
        slug: space.citySlug,
        name: space.city,
      })),
    ),
    variantTypes: [
      {
        label: "Capacidad",
        values: Array.from(new Set(spaces.map((space) => space.capacity))),
      },
    ],
    brands: unique(
      spaces.map((space) => ({
        id: space.comunaSlug,
        slug: space.comunaSlug,
        name: space.comuna,
      })),
    ),
    priceBounds: { min: Math.min(...rates), max: Math.max(...rates) },
  };
}

export function getSpaces(filters: SpaceFilters = {}) {
  const capacityValues = Object.values(parseVts(filters.vts ?? null)).flat();
  const filtered = spaces.filter((space) => {
    if (filters.category && space.categorySlug !== filters.category)
      return false;
    if (filters.collection && space.citySlug !== filters.collection)
      return false;
    if (filters.brand && space.comunaSlug !== filters.brand) return false;
    if (filters.priceMin && space.rateClpPerDay < Number(filters.priceMin))
      return false;
    if (filters.priceMax && space.rateClpPerDay > Number(filters.priceMax))
      return false;
    if (capacityValues.length > 0 && !capacityValues.includes(space.capacity))
      return false;
    if (
      filters.q &&
      !searchSpaces(filters.q).some((result) => result.id === space.id)
    )
      return false;
    return true;
  });
  const sorted = [...filtered];
  if (filters.sort === "price-asc")
    sorted.sort((a, b) => a.rateClpPerDay - b.rateClpPerDay);
  if (filters.sort === "price-desc")
    sorted.sort((a, b) => b.rateClpPerDay - a.rateClpPerDay);
  if (filters.sort === "name")
    sorted.sort((a, b) => a.name.localeCompare(b.name, "es-CL"));
  return { data: sorted, total: sorted.length };
}

export function formatDemoClp(amount: number) {
  return new Intl.NumberFormat("es-CL", {
    style: "currency",
    currency: "CLP",
    maximumFractionDigits: 0,
  }).format(amount);
}
