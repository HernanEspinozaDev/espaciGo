# Third-party notices — Fase 2R

This notice covers recognizable source code and view structures copied or adapted into the EspaciGo frontend. The frozen source clones are local-only under `.local/frontend-template-sources/`; neither clone's `.git` directory, secrets, deployment files, lockfile, or environment files are included in this repository.

## Your Next Store

- Repository: https://github.com/yournextstore/yournextstore
- Frozen source commit: `1de63c614f4c57c1862b1b14710f2133784749c1`
- License: MIT
- Copyright: © 2024 Your Next Store, Inc.
- Role: source base for the public marketplace routes, navigation, listing/search/detail/gallery compositions, UI primitives, and search interactions.

### Source-to-target inventory

| Upstream path                               | EspaciGo path                                        | Treatment                                                                                          |
| ------------------------------------------- | ---------------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| `app/page.tsx`                              | `src/app/(marketplace)/page.tsx`                     | Adapted; retains hero, grid, about, newsletter composition; uses local fixtures and EspaciGo text. |
| `app/navbar.tsx`                            | `src/app/navbar.tsx`                                 | Adapted; route links and accessible labels localized.                                              |
| `app/footer.tsx`                            | `src/app/footer.tsx`                                 | Adapted; EspaciGo navigation and local-demo limits.                                                |
| `app/products/page.tsx`                     | `src/app/(marketplace)/products/page.tsx`            | Adapted; product listing becomes space catalog; local fixture filtering/sorting.                   |
| `app/products/products-sort-select.tsx`     | `src/app/products/products-sort-select.tsx`          | Adapted; local Next routing and Spanish labels.                                                    |
| `app/search/page.tsx`                       | `src/app/(marketplace)/search/page.tsx`              | Adapted; local search fixtures and EspaciGo semantics.                                             |
| `app/search/search-controls.tsx`            | `src/app/search/search-controls.tsx`                 | Adapted; local category/sort controls and Spanish labels.                                          |
| `app/search/search-pagination.tsx`          | `src/app/search/search-pagination.tsx`               | Adapted; Spanish labels and local route/query parameters.                                          |
| `app/search/sort.ts`                        | `src/app/search/sort.ts`                             | Adapted; local catalog sort keys/labels.                                                           |
| `lib/vts.ts`                                | `src/lib/vts.ts`                                     | Adapted parser/encoder for the source filter-query format; no API call.                            |
| `app/layout.tsx`                            | `src/app/(marketplace)/layout.tsx`                   | Reassembled public chrome from source header/nav/footer; platform providers excluded.              |
| `app/search/loading.tsx`                    | `src/app/(marketplace)/search/loading.tsx`           | Adapted; retains listing skeleton composition.                                                     |
| `app/product/[slug]/page.tsx`               | `src/app/(marketplace)/product/[slug]/page.tsx`      | Adapted; detail route renders a synthetic EspaciGo space.                                          |
| `app/product/[slug]/media-gallery.tsx`      | `src/app/product/[slug]/media-gallery.tsx`           | Adapted; preserves gallery navigation/zoom/thumbnails; localized labels and local assets.          |
| `app/product/[slug]/not-found.tsx`          | `src/app/(marketplace)/product/[slug]/not-found.tsx` | Adapted; local not-found state.                                                                    |
| `app/product/[slug]/error.tsx`              | `src/app/(marketplace)/product/[slug]/error.tsx`     | Adapted; local error state.                                                                        |
| `app/product/[slug]/product-features.tsx`   | `src/app/product/[slug]/product-features.tsx`        | Adapted and used by the detail route; renders only synthetic amenities from local fixtures.        |
| `components/product-card.tsx`               | `src/components/product-card.tsx`                    | Adapted; space fixture data, local paths, synthetic image alt text, illustrative CLP/day.          |
| `components/product-grid-skeleton.tsx`      | `src/components/product-grid-skeleton.tsx`           | Copied/adapted; listing loading state.                                                             |
| `components/listing-pagination.tsx`         | `src/components/listing-pagination.tsx`              | Adapted; local route and filters.                                                                  |
| `components/route-error.tsx`                | `src/components/route-error.tsx`                     | Adapted; local route error state.                                                                  |
| `components/sections/hero.tsx`              | `src/components/sections/hero.tsx`                   | Adapted; retains hierarchy, uses Spanish EspaciGo copy.                                            |
| `components/sections/about.tsx`             | `src/components/sections/about.tsx`                  | Adapted; EspaciGo copy and explicit synthetic-data limitation.                                     |
| `components/sections/product-grid.tsx`      | `src/components/sections/product-grid.tsx`           | Adapted; reads the typed local space fixture.                                                      |
| `components/sections/product-filters.tsx`   | `src/components/sections/product-filters.tsx`        | Adapted; filter view preserved; local fixtures, CLP/day, Spanish labels.                           |
| `components/sections/newsletter.tsx`        | `src/components/sections/newsletter.tsx`             | Adapted; disabled, non-submitting preview; no data collection.                                     |
| `app/globals.css`                           | `src/app/globals.css`                                | Adapted as the active Tailwind/theme base; workspace tokens are scoped separately.                 |
| `lib/utils.ts`                              | `src/lib/utils.ts`                                   | Copied/adapted utility functions used by imported components.                                      |
| `components/ui/accordion.tsx`               | `src/components/ui/accordion.tsx`                    | Copied/adapted primitive.                                                                          |
| `components/ui/badge.tsx`                   | `src/components/ui/badge.tsx`                        | Adapted primitive.                                                                                 |
| `components/ui/breadcrumb.tsx`              | `src/components/ui/breadcrumb.tsx`                   | Copied/adapted primitive.                                                                          |
| `components/ui/button.tsx`                  | `src/components/ui/button.tsx`                       | Adapted primitive and local imports.                                                               |
| `components/ui/checkbox.tsx`                | `src/components/ui/checkbox.tsx`                     | Copied/adapted primitive.                                                                          |
| `components/ui/dialog.tsx`                  | `src/components/ui/dialog.tsx`                       | Copied/adapted primitive.                                                                          |
| `components/ui/dropdown-menu.tsx`           | `src/components/ui/dropdown-menu.tsx`                | Copied/adapted primitive.                                                                          |
| `components/ui/input.tsx`                   | `src/components/ui/input.tsx`                        | Copied/adapted primitive.                                                                          |
| `components/ui/label.tsx`                   | `src/components/ui/label.tsx`                        | Copied/adapted primitive.                                                                          |
| `components/ui/pagination.tsx`              | `src/components/ui/pagination.tsx`                   | Copied/adapted primitive.                                                                          |
| `components/ui/popover.tsx`                 | `src/components/ui/popover.tsx`                      | Copied/adapted primitive.                                                                          |
| `components/ui/scroll-area.tsx`             | `src/components/ui/scroll-area.tsx`                  | Copied/adapted primitive.                                                                          |
| `components/ui/select.tsx`                  | `src/components/ui/select.tsx`                       | Copied/adapted primitive.                                                                          |
| `components/ui/sheet.tsx`                   | `src/components/ui/sheet.tsx`                        | Copied/adapted primitive.                                                                          |
| `components/ui/skeleton.tsx`                | `src/components/ui/skeleton.tsx`                     | Copied/adapted primitive.                                                                          |
| `components/ui/slider.tsx`                  | `src/components/ui/slider.tsx`                       | Copied/adapted primitive.                                                                          |
| `components/ui/sonner.tsx`                  | `src/components/ui/sonner.tsx`                       | Adapted to the installed Sonner package and local theme.                                           |
| `components/ui/tooltip.tsx`                 | `src/components/ui/tooltip.tsx`                      | Copied/adapted primitive.                                                                          |
| `components/search/search-input.tsx`        | `src/components/search/search-input.tsx`             | Adapted as a plain GET form to the Go-backed public catalog; local suggestions were removed.       |
| `components/search/mobile-search-input.tsx` | `src/components/search/mobile-search-input.tsx`      | Adapted as a plain GET form to the Go-backed public catalog; local suggestions were removed.       |

### Local integration adapters and fixtures

These supporting files are EspaciGo integration code rather than copied template files:

- `src/app/layout.tsx`: single-app Spanish root, metadata, and skip link.
- `src/features/public-catalog/`: server-only Go adapter, validated URL query state, public DTO presentation, and catalog UI. Fase 4 no longer ships local listing fixtures.
- `src/lib/workspace-fixtures.ts`: synthetic account, summary, and activity data.
- `src/lib/template-media.tsx`: local `next/image` adapter retaining the imported media component API.
- `src/app/(marketplace)/layout.tsx`: route-group chrome assembled from the listed Your Next Store navbar/footer components and EspaciGo routing.

The source `app/layout.tsx` is not copied wholesale. Its cart/store/theme/cookie/commerce/chat providers, request-time reads, remotely configured metadata, and environment-dependent services are excluded.

The YNS Commerce Kit, remote YNS API, store chat, cart/checkout, Bun runtime, publishing/telemetry and platform-specific data integrations are not part of this frontend. The source `trust-badges.tsx` component was removed because its shipping, warranty and return claims do not apply to EspaciGo. Demo image files under `public/spaces/` are local synthetic assets, not copied product photos.

## Next.js SaaS Starter

- Repository: https://github.com/nextjs/saas-starter
- Frozen source commit: `6e33e58b1e553a41fe22e6b941a7229a002de361`
- License: MIT
- Copyright: © 2025 Vercel
- Role: source for the workspace presentation, responsive dashboard shell, account/security/activity views, pricing preview, and corresponding UI primitives.

### Source-to-target inventory

| Upstream path                                    | EspaciGo path                                        | Treatment                                                                                             |
| ------------------------------------------------ | ---------------------------------------------------- | ----------------------------------------------------------------------------------------------------- |
| `app/(dashboard)/layout.tsx`                     | `src/app/(workspace)/layout.tsx`                     | Adapted; retains workspace presentation header and navigation.                                        |
| `app/(dashboard)/page.tsx`                       | `src/app/(workspace)/saas/page.tsx`                  | Adapted; retains landing/terminal/features hierarchy, local demo copy.                                |
| `app/(dashboard)/terminal.tsx`                   | `src/app/(workspace)/saas/terminal.tsx`              | Adapted; presentation-only terminal content, no command execution.                                    |
| `app/(dashboard)/dashboard/layout.tsx`           | `src/app/(workspace)/dashboard/layout.tsx`           | Adapted; responsive sidebar navigation, Spanish labels, semantic main landmark.                       |
| `app/(dashboard)/dashboard/page.tsx`             | `src/app/(workspace)/dashboard/page.tsx`             | Adapted; starter dashboard composition with typed local fixtures and inert actions.                   |
| `app/(dashboard)/dashboard/general/page.tsx`     | `src/app/(workspace)/dashboard/general/page.tsx`     | Adapted; account form presentation with synthetic disabled values.                                    |
| `app/(dashboard)/dashboard/security/page.tsx`    | `src/app/(workspace)/dashboard/security/page.tsx`    | Adapted; security form presentation, fields/actions disabled; no credentials handled.                 |
| `app/(dashboard)/dashboard/activity/page.tsx`    | `src/app/(workspace)/dashboard/activity/page.tsx`    | Adapted; retains activity-card/list presentation, replaces Drizzle with synthetic workspace fixtures. |
| `app/(dashboard)/dashboard/activity/loading.tsx` | `src/app/(workspace)/dashboard/activity/loading.tsx` | Adapted loading state with workspace UI primitives.                                                   |
| `app/(dashboard)/pricing/page.tsx`               | `src/app/(workspace)/pricing/page.tsx`               | Adapted; visual-only neutral plan preview, no prices/subscriptions/checkout/payments.                 |
| `app/(login)/login.tsx`                          | `src/app/(workspace)/login.tsx`                      | Adapted; disabled local login/register view, no session or credential submission.                     |
| `app/(login)/sign-in/page.tsx`                   | `src/app/(workspace)/login/page.tsx`                 | Adapted route wrapper for the local login view.                                                       |
| `app/globals.css`                                | `src/app/workspace-template.css`                     | Adapted/scoped under `.saas-application` to avoid changing marketplace tokens.                        |
| `components/ui/button.tsx`                       | `src/components/workspace/ui/button.tsx`             | Copied/adapted workspace primitive.                                                                   |
| `components/ui/card.tsx`                         | `src/components/workspace/ui/card.tsx`               | Copied/adapted workspace primitive.                                                                   |
| `components/ui/input.tsx`                        | `src/components/workspace/ui/input.tsx`              | Copied/adapted workspace primitive.                                                                   |
| `components/ui/label.tsx`                        | `src/components/workspace/ui/label.tsx`              | Copied/adapted workspace primitive.                                                                   |
| `components/ui/avatar.tsx`                       | `src/components/workspace/ui/avatar.tsx`             | Copied/adapted workspace primitive.                                                                   |
| `components/ui/dropdown-menu.tsx`                | `src/components/workspace/ui/dropdown-menu.tsx`      | Copied/adapted workspace primitive.                                                                   |
| `components/ui/radio-group.tsx`                  | `src/components/workspace/ui/radio-group.tsx`        | Copied/adapted workspace primitive.                                                                   |

The starter's Drizzle schema/queries, database, JWT/cookie authentication, bcrypt, team management, Stripe, billing, webhooks and server actions are excluded. Dashboard activity and account data use typed local fixtures; actions that imply persistence are disabled.

## MIT License — Your Next Store

MIT License

Copyright (c) 2024 Your Next Store, Inc.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

## MIT License — Next.js SaaS Starter

MIT License

Copyright (c) 2025 Vercel

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
