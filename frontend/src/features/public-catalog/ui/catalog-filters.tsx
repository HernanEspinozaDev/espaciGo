"use client";

import { SlidersHorizontal } from "lucide-react";
import { useRouter } from "next/navigation";
import type { FormEvent } from "react";
import { useState } from "react";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { categoryLabel } from "../domain/presentation";
import type {
  CatalogQuery,
  PublicCategoryFilter,
  PublicLocation,
  PublicSpaceCategory,
} from "../domain/types";

const scalarKeys = [
  "q",
  "category_code",
  "region_code",
  "comuna_code",
  "min_area_m2",
  "rate_unit",
  "min_unit_price_clp",
  "max_unit_price_clp",
  "start_at",
  "end_at",
  "min_total_clp",
  "max_total_clp",
  "sort",
  "page_size",
] as const;

function currentAttribute(query: CatalogQuery, code: string) {
  return query.attribute.filter((token) => token.startsWith(`${code}:`));
}

function numericValue(tokens: string[], operator: "gte" | "lte") {
  const prefix = `:${operator}:`;
  return (
    tokens.find((token) => token.includes(prefix))?.split(prefix, 2)[1] ?? ""
  );
}

function buildSubmission(
  form: HTMLFormElement,
  filters: readonly PublicCategoryFilter[],
) {
  const data = new FormData(form);
  const params = new URLSearchParams();
  for (const key of scalarKeys) {
    const value = data.get(key);
    if (typeof value === "string" && value !== "") params.set(key, value);
  }

  for (const filter of filters) {
    const field = `attribute-${filter.code}`;
    if (filter.type === "integer" || filter.type === "number") {
      const minimum = data.get(`${field}-min`);
      const maximum = data.get(`${field}-max`);
      if (typeof minimum === "string" && minimum !== "")
        params.append("attribute", `${filter.code}:gte:${minimum}`);
      if (typeof maximum === "string" && maximum !== "")
        params.append("attribute", `${filter.code}:lte:${maximum}`);
    } else if (filter.type === "enum_list") {
      const values = data
        .getAll(field)
        .filter(
          (value): value is string => typeof value === "string" && value !== "",
        );
      if (values.length)
        params.append("attribute", `${filter.code}:in:${values.join(",")}`);
    } else {
      const value = data.get(field);
      if (typeof value === "string" && value !== "")
        params.append("attribute", value);
    }
  }
  return params;
}

function FilterFields({
  categories,
  locations,
  filters,
  query,
  fixedCategoryCode,
}: {
  categories: readonly PublicSpaceCategory[];
  locations: readonly PublicLocation[];
  filters: readonly PublicCategoryFilter[];
  query: CatalogQuery;
  fixedCategoryCode?: string;
}) {
  const regions = Array.from(
    new Map(
      locations.map((item) => [item.region_code, item.region_name]),
    ).entries(),
  );
  const selectedRegion = query.region_code ?? "";

  return (
    <>
      {query.q ? <input type="hidden" name="q" value={query.q} /> : null}
      {fixedCategoryCode ? (
        <input type="hidden" name="category_code" value={fixedCategoryCode} />
      ) : (
        <label className="block space-y-2 text-sm">
          <span className="font-medium">Categoría</span>
          <select
            name="category_code"
            defaultValue={query.category_code ?? ""}
            className="h-11 w-full rounded-lg border border-border bg-background px-3 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <option value="">Todas las categorías</option>
            {categories.map((category) => (
              <option key={category.code} value={category.code}>
                {categoryLabel(category)}
              </option>
            ))}
          </select>
        </label>
      )}

      <label className="block space-y-2 text-sm">
        <span className="font-medium">Región</span>
        <select
          name="region_code"
          defaultValue={selectedRegion}
          className="h-11 w-full rounded-lg border border-border bg-background px-3 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <option value="">Todas las regiones</option>
          {regions.map(([code, name]) => (
            <option key={code} value={code}>
              {name}
            </option>
          ))}
        </select>
      </label>

      <label className="block space-y-2 text-sm">
        <span className="font-medium">Comuna</span>
        <select
          name="comuna_code"
          defaultValue={query.comuna_code ?? ""}
          className="h-11 w-full rounded-lg border border-border bg-background px-3 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <option value="">Todas las comunas</option>
          {locations
            .filter(
              (location) =>
                !selectedRegion || location.region_code === selectedRegion,
            )
            .map((location) => (
              <option key={location.comuna_code} value={location.comuna_code}>
                {location.comuna_name} · {location.region_name}
              </option>
            ))}
        </select>
      </label>

      <label className="block space-y-2 text-sm">
        <span className="font-medium">Superficie mínima (m²)</span>
        <input
          name="min_area_m2"
          type="number"
          min="0.01"
          step="0.01"
          defaultValue={query.min_area_m2 ?? ""}
          className="h-11 w-full rounded-lg border border-border bg-background px-3 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        />
      </label>

      <label className="block space-y-2 text-sm">
        <span className="font-medium">Modalidad de tarifa</span>
        <select
          name="rate_unit"
          defaultValue={query.rate_unit ?? ""}
          className="h-11 w-full rounded-lg border border-border bg-background px-3 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <option value="">Cualquiera</option>
          <option value="hora">Por hora</option>
          <option value="dia">Por día</option>
          <option value="mes">Por mes</option>
        </select>
      </label>

      <fieldset className="space-y-3">
        <legend className="text-sm font-medium">Tarifa base (CLP)</legend>
        <div className="grid grid-cols-2 gap-2">
          <label className="space-y-1 text-xs text-muted-foreground">
            <span>Mínimo</span>
            <input
              name="min_unit_price_clp"
              inputMode="numeric"
              pattern="[0-9]{1,19}"
              defaultValue={query.min_unit_price_clp ?? ""}
              className="h-11 w-full rounded-lg border border-border bg-background px-3 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            />
          </label>
          <label className="space-y-1 text-xs text-muted-foreground">
            <span>Máximo</span>
            <input
              name="max_unit_price_clp"
              inputMode="numeric"
              pattern="[0-9]{1,19}"
              defaultValue={query.max_unit_price_clp ?? ""}
              className="h-11 w-full rounded-lg border border-border bg-background px-3 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            />
          </label>
        </div>
      </fieldset>

      <fieldset className="space-y-3">
        <legend className="text-sm font-medium">
          Intervalo para estimación
        </legend>
        <p className="text-xs leading-relaxed text-muted-foreground">
          Fecha y hora RFC3339 con offset explícito, por ejemplo
          2026-12-18T15:00:00-03:00. Zona de referencia: America/Santiago.
        </p>
        <label className="block space-y-1 text-xs text-muted-foreground">
          <span>Inicio</span>
          <input
            name="start_at"
            type="text"
            inputMode="text"
            placeholder="2026-12-18T15:00:00-03:00"
            defaultValue={query.start_at ?? ""}
            className="h-11 w-full rounded-lg border border-border bg-background px-3 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          />
        </label>
        <label className="block space-y-1 text-xs text-muted-foreground">
          <span>Término</span>
          <input
            name="end_at"
            type="text"
            inputMode="text"
            placeholder="2026-12-19T15:00:00-03:00"
            defaultValue={query.end_at ?? ""}
            className="h-11 w-full rounded-lg border border-border bg-background px-3 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          />
        </label>
        <div className="grid grid-cols-2 gap-2">
          <label className="space-y-1 text-xs text-muted-foreground">
            <span>Total mínimo (CLP)</span>
            <input
              name="min_total_clp"
              inputMode="numeric"
              pattern="[0-9]{1,19}"
              defaultValue={query.min_total_clp ?? ""}
              className="h-11 w-full rounded-lg border border-border bg-background px-3 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            />
          </label>
          <label className="space-y-1 text-xs text-muted-foreground">
            <span>Total máximo (CLP)</span>
            <input
              name="max_total_clp"
              inputMode="numeric"
              pattern="[0-9]{1,19}"
              defaultValue={query.max_total_clp ?? ""}
              className="h-11 w-full rounded-lg border border-border bg-background px-3 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            />
          </label>
        </div>
      </fieldset>

      {(fixedCategoryCode ?? query.category_code) && filters.length > 0 ? (
        <fieldset className="space-y-4 border-t border-border pt-4">
          <legend className="text-sm font-medium">
            Características de la categoría
          </legend>
          {filters.map((filter) => {
            const tokens = currentAttribute(query, filter.code);
            const field = `attribute-${filter.code}`;
            if (filter.type === "boolean") {
              const selected = tokens[0]?.slice(filter.code.length + 1) ?? "";
              return (
                <label key={filter.code} className="block space-y-2 text-sm">
                  <span>{filter.label}</span>
                  <select
                    name={field}
                    defaultValue={selected ? `${filter.code}:${selected}` : ""}
                    className="h-11 w-full rounded-lg border border-border bg-background px-3 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  >
                    <option value="">Cualquiera</option>
                    <option value={`${filter.code}:true`}>Sí</option>
                    <option value={`${filter.code}:false`}>No</option>
                  </select>
                </label>
              );
            }
            if (filter.type === "enum") {
              const selected = tokens[0]?.split(":eq:", 2)[1] ?? "";
              return (
                <label key={filter.code} className="block space-y-2 text-sm">
                  <span>{filter.label}</span>
                  <select
                    name={field}
                    defaultValue={
                      selected ? `${filter.code}:eq:${selected}` : ""
                    }
                    className="h-11 w-full rounded-lg border border-border bg-background px-3 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  >
                    <option value="">Cualquiera</option>
                    {(filter.options ?? []).map((option) => (
                      <option
                        key={option}
                        value={`${filter.code}:eq:${option}`}
                      >
                        {option}
                      </option>
                    ))}
                  </select>
                </label>
              );
            }
            if (filter.type === "enum_list") {
              const selected = new Set(
                tokens[0]?.split(":in:", 2)[1]?.split(",") ?? [],
              );
              return (
                <fieldset key={filter.code} className="space-y-2">
                  <legend className="text-sm font-medium">
                    {filter.label}
                  </legend>
                  {(filter.options ?? []).map((option) => (
                    <label
                      key={option}
                      className="flex min-h-10 items-center gap-3 text-sm text-muted-foreground"
                    >
                      <input
                        type="checkbox"
                        name={field}
                        value={option}
                        defaultChecked={selected.has(option)}
                        className="size-4 accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                      />
                      {option}
                    </label>
                  ))}
                </fieldset>
              );
            }
            const min = numericValue(tokens, "gte");
            const max = numericValue(tokens, "lte");
            return (
              <fieldset key={filter.code} className="space-y-2">
                <legend className="text-sm font-medium">
                  {filter.label}
                  {filter.unit ? ` (${filter.unit})` : ""}
                </legend>
                <div className="grid grid-cols-2 gap-2">
                  <label className="space-y-1 text-xs text-muted-foreground">
                    <span>Mínimo</span>
                    <input
                      name={`${field}-min`}
                      type="number"
                      min={filter.minimum}
                      max={filter.maximum}
                      step={
                        filter.type === "integer" ? 1 : (filter.step ?? "any")
                      }
                      defaultValue={min}
                      className="h-11 w-full rounded-lg border border-border bg-background px-3 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    />
                  </label>
                  <label className="space-y-1 text-xs text-muted-foreground">
                    <span>Máximo</span>
                    <input
                      name={`${field}-max`}
                      type="number"
                      min={filter.minimum}
                      max={filter.maximum}
                      step={
                        filter.type === "integer" ? 1 : (filter.step ?? "any")
                      }
                      defaultValue={max}
                      className="h-11 w-full rounded-lg border border-border bg-background px-3 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    />
                  </label>
                </div>
              </fieldset>
            );
          })}
        </fieldset>
      ) : null}
    </>
  );
}

function FilterForm({
  actionPath,
  categories,
  locations,
  filters,
  query,
  fixedCategoryCode,
  onComplete,
}: {
  actionPath: string;
  categories: readonly PublicSpaceCategory[];
  locations: readonly PublicLocation[];
  filters: readonly PublicCategoryFilter[];
  query: CatalogQuery;
  fixedCategoryCode?: string;
  onComplete?: () => void;
}) {
  const router = useRouter();
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const params = buildSubmission(event.currentTarget, filters);
    const suffix = params.toString();
    router.push(suffix ? `${actionPath}?${suffix}` : actionPath, {
      scroll: false,
    });
    onComplete?.();
  };

  return (
    <form
      action={actionPath}
      method="get"
      onSubmit={onSubmit}
      className="space-y-5"
    >
      {query.sort ? (
        <input type="hidden" name="sort" value={query.sort} />
      ) : null}
      {query.page_size ? (
        <input type="hidden" name="page_size" value={query.page_size} />
      ) : null}
      <FilterFields
        categories={categories}
        locations={locations}
        filters={filters}
        query={query}
        fixedCategoryCode={fixedCategoryCode}
      />
      <div className="flex flex-wrap gap-2 border-t border-border pt-4">
        <button
          type="submit"
          className="inline-flex min-h-11 items-center rounded-full bg-foreground px-5 text-sm font-medium text-background hover:bg-foreground/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          Aplicar filtros
        </button>
        <a
          href={
            fixedCategoryCode
              ? `${actionPath}?category_code=${encodeURIComponent(fixedCategoryCode)}`
              : actionPath
          }
          className="inline-flex min-h-11 items-center rounded-full border border-border px-4 text-sm font-medium hover:bg-secondary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          Limpiar
        </a>
      </div>
    </form>
  );
}

export function CatalogFilters(props: {
  actionPath: string;
  categories: readonly PublicSpaceCategory[];
  locations: readonly PublicLocation[];
  filters: readonly PublicCategoryFilter[];
  query: CatalogQuery;
  fixedCategoryCode?: string;
}) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <aside className="hidden lg:block">
        <div className="sticky top-24 rounded-2xl border border-border p-5">
          <h2 className="mb-5 text-sm font-medium">Filtros</h2>
          <FilterForm {...props} />
        </div>
      </aside>
      <div className="lg:hidden">
        <Sheet open={open} onOpenChange={setOpen}>
          <SheetTrigger className="inline-flex min-h-11 items-center gap-2 rounded-full border border-border px-4 text-sm font-medium hover:bg-secondary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
            <SlidersHorizontal className="size-4" aria-hidden="true" />
            Filtrar
          </SheetTrigger>
          <SheetContent
            side="bottom"
            className="max-h-[85vh] overflow-y-auto rounded-t-2xl p-5"
          >
            <SheetHeader className="mb-5 text-left">
              <SheetTitle className="text-2xl font-medium">
                Refinar publicaciones
              </SheetTitle>
              <SheetDescription>
                Los filtros disponibles dependen del contrato público de cada
                categoría.
              </SheetDescription>
            </SheetHeader>
            <FilterForm {...props} onComplete={() => setOpen(false)} />
          </SheetContent>
        </Sheet>
      </div>
    </>
  );
}
