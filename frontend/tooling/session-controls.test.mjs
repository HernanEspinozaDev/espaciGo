import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");
const sessionControlsPath = path.join(
  repoRoot,
  "src/features/auth/ui/session-controls.tsx",
);
const workspaceLayoutPath = path.join(
  repoRoot,
  "src/app/(workspace)/layout.tsx",
);
const marketplaceLayoutPath = path.join(
  repoRoot,
  "src/app/(marketplace)/layout.tsx",
);
const accessScreenPath = path.join(
  repoRoot,
  "src/features/auth/ui/access-screen.tsx",
);

test("SessionControls component exports, variants and contract rules", async () => {
  const source = await readFile(sessionControlsPath, "utf8");
  assert.ok(
    source.includes("export function SessionControls"),
    "SessionControls should be exported",
  );
  assert.ok(
    source.includes("useSessionIdentity"),
    "Must use useSessionIdentity hook",
  );
  assert.ok(source.includes("useAuth"), "Must use useAuth for logout");
  assert.ok(
    source.includes("Cerrar sesión"),
    "Must include Cerrar sesión button",
  );
  assert.ok(
    source.includes("Mi cuenta"),
    "Public variant must link to Mi cuenta",
  );
  assert.ok(
    source.includes('href="/mi-cuenta"'),
    "Mi cuenta link must point directly to /mi-cuenta",
  );
  assert.ok(
    source.includes("Ingresar a EspaciGo"),
    "Public anonymous variant must show Ingresar a EspaciGo",
  );
  assert.ok(
    source.includes("Acceso"),
    "Workspace anonymous variant must show Acceso",
  );
  assert.ok(
    source.includes("truncate"),
    "Must contain truncation classes for 390px responsive overflow prevention",
  );
  assert.ok(
    source.includes("aria-label=") && source.includes("title="),
    "Must keep accessible label/title for full identity",
  );
  assert.ok(
    source.includes("useRouter"),
    "SessionControls must import useRouter for post-logout navigation",
  );
  assert.ok(
    source.includes('router.replace("/acceso")'),
    "SessionControls must navigate to /acceso upon logout",
  );
});

test("Workspace layout replaces Acceso button with SessionControls variant workspace", async () => {
  const source = await readFile(workspaceLayoutPath, "utf8");
  assert.ok(
    source.includes('<SessionControls variant="workspace" />'),
    "Workspace layout must use SessionControls variant='workspace'",
  );
  assert.ok(
    !source.includes('<Link href="/acceso">Acceso</Link>'),
    "Workspace layout must not contain static Acceso link",
  );
});

test("Marketplace layout replaces Ingresar link with SessionControls variant public", async () => {
  const source = await readFile(marketplaceLayoutPath, "utf8");
  assert.ok(
    source.includes('<SessionControls variant="public" />'),
    "Marketplace layout must use SessionControls variant='public'",
  );
  assert.ok(
    !source.includes(">Ingresar a EspaciGo</Link>"),
    "Marketplace layout must not contain static Ingresar a EspaciGo link",
  );
});

test("AccessScreen redirects authenticated session without displaying second login form", async () => {
  const source = await readFile(accessScreenPath, "utf8");
  assert.ok(
    source.includes('snapshot.status === "authenticated"'),
    "AccessScreen checks authenticated status",
  );
  assert.ok(
    source.includes("router.replace(target)"),
    "AccessScreen performs router.replace on confirmed session",
  );
  assert.ok(
    source.includes('snapshot.status === "checking"'),
    "AccessScreen handles checking state with a clean placeholder",
  );
  assert.ok(
    source.includes('data-testid="access-checking"'),
    "AccessScreen renders checking placeholder element",
  );
  assert.ok(
    source.includes('data-testid="access-redirecting"'),
    "AccessScreen renders redirecting placeholder element",
  );
});
