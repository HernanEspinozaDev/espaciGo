/**
 * Inventario formal de atribuciones, licencias y fuentes congeladas de terceros
 * utilizadas en la construcción del frontend Next.js de EspaciGo (Fase 11 / Fase 2R).
 */

export interface ThirdPartyAttribution {
  name: string;
  repository: string;
  frozenCommit: string;
  license: "MIT";
  copyright: string;
  role: string;
  noticeFile: string;
}

export const THIRD_PARTY_ATTRIBUTIONS: readonly ThirdPartyAttribution[] = [
  {
    name: "Your Next Store",
    repository: "https://github.com/yournextstore/yournextstore",
    frozenCommit: "1de63c614f4c57c1862b1b14710f2133784749c1",
    license: "MIT",
    copyright: "© 2024 Your Next Store, Inc.",
    role: "Base de vistas de marketplace público, navegación, catálogo, ficha y primitivas de UI.store",
    noticeFile: "frontend/THIRD_PARTY_NOTICES.md",
  },
  {
    name: "Next.js SaaS Starter",
    repository: "https://github.com/nextjs/saas-starter",
    frozenCommit: "6e33e58b1e553a41fe22e6b941a7229a002de361",
    license: "MIT",
    copyright: "© 2025 Vercel",
    role: "Estructura de panel administrativo/workspace, shells responsivos, vistas de seguridad y primitivas de interfaz.",
    noticeFile: "frontend/THIRD_PARTY_NOTICES.md",
  },
] as const;

export function getAttributionByName(
  name: string,
): ThirdPartyAttribution | undefined {
  return THIRD_PARTY_ATTRIBUTIONS.find(
    (item) => item.name.toLowerCase() === name.toLowerCase(),
  );
}
