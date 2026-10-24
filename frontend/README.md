# EspaciGo Next.js frontend

This directory contains the replacement frontend scaffold. It uses Next.js App Router, TypeScript strict mode, and pnpm. The initial page is intentionally a neutral shell; product routes and design systems belong to later approved phases.

## Local checks

```sh
pnpm install --frozen-lockfile
pnpm lint
pnpm typecheck
pnpm test
pnpm format:check
pnpm build
```

Run locally with `pnpm dev`. After `pnpm build`, `pnpm start:local` serves the standalone output on `127.0.0.1:4201`. The initial standalone Docker image is built from this directory and runs as the unprivileged `node` user on port 4000. It has no backend, database, or credential dependencies.

The reference template clones live outside Git in `.local/frontend-template-sources/` at the commits recorded in `../planning/frontend_next/plan_frontend_next_hermes.md`. Use them only as visual/pattern references; do not merge their applications or platform-specific integrations.
