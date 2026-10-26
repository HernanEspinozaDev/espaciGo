#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

const ROUTES = [
  { path: "/", name: "home", requiresAuth: false },
  { path: "/explorar", name: "explorar", requiresAuth: false },
  { path: "/acceso", name: "acceso", requiresAuth: false },
  { path: "/registro", name: "registro", requiresAuth: false },
  { path: "/recuperar-acceso", name: "recuperar-acceso", requiresAuth: false },
  { path: "/mi-cuenta", name: "mi-cuenta", requiresAuth: true },
];

const VIEWPORTS = [
  { name: "390x844", width: 390, height: 844 },
  { name: "1365x900", width: 1365, height: 900 },
];

function parseArgs() {
  const args = process.argv.slice(2);
  let stage = null;
  let baseUrl = "http://127.0.0.1:4328";
  for (let i = 0; i < args.length; i++) {
    if (args[i] === "--stage" && i + 1 < args.length) stage = args[++i];
    else if (args[i] === "--base-url" && i + 1 < args.length)
      baseUrl = args[++i];
    else if (args[i].startsWith("--stage=")) stage = args[i].split("=")[1];
    else if (args[i].startsWith("--base-url=")) baseUrl = args[i].split("=")[1];
  }
  if (stage !== "before" && stage !== "after") {
    console.error(
      '[palette-visual] Error: --stage must be "before" or "after"',
    );
    process.exit(1);
  }
  let parsed;
  try {
    parsed = new URL(baseUrl);
  } catch {
    console.error(`[palette-visual] Error: invalid --base-url "${baseUrl}"`);
    process.exit(1);
  }
  if (parsed.username || parsed.password) {
    console.error(
      "[palette-visual] Error: --base-url must not contain credentials",
    );
    process.exit(1);
  }
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    console.error(
      "[palette-visual] Error: --base-url protocol must be http: or https:",
    );
    process.exit(1);
  }
  if (parsed.hostname !== "127.0.0.1") {
    console.error("[palette-visual] Error: use canonical loopback 127.0.0.1");
    process.exit(1);
  }
  return { stage, baseUrl: parsed.origin };
}

function setupOutputDirs() {
  const scriptDir = path.dirname(fileURLToPath(import.meta.url));
  const repoRoot = path.resolve(scriptDir, "../..");
  const outputDir = path.join(repoRoot, ".local/testing/ui-ux-main-palette");
  const screenshotsDir = path.join(outputDir, "screenshots");

  for (const dir of [outputDir, screenshotsDir]) {
    fs.mkdirSync(dir, { recursive: true, mode: 0o700 });
    try {
      fs.chmodSync(dir, 0o700);
    } catch {} // ensure 0700
  }
  return { outputDir, screenshotsDir };
}

function writeSecureJson(filePath, data) {
  fs.writeFileSync(filePath, JSON.stringify(data, null, 2), {
    encoding: "utf8",
    mode: 0o600,
  });
  try {
    fs.chmodSync(filePath, 0o600);
  } catch {} // ensure 0600
}

function valuesDiffer(v1, v2, tolerance = 0.1) {
  if (v1 === v2) return false;
  if (typeof v1 === "string" && typeof v2 === "string") {
    const p1 = v1.trim().split(/\s+/);
    const p2 = v2.trim().split(/\s+/);
    if (p1.length === p2.length) {
      let allMatch = true;
      for (let i = 0; i < p1.length; i++) {
        const n1 = parseFloat(p1[i]);
        const n2 = parseFloat(p2[i]);
        if (
          !isNaN(n1) &&
          !isNaN(n2) &&
          p1[i].endsWith("px") &&
          p2[i].endsWith("px")
        ) {
          if (Math.abs(n1 - n2) > tolerance) {
            allMatch = false;
            break;
          }
        } else if (p1[i] !== p2[i]) {
          allMatch = false;
          break;
        }
      }
      if (allMatch) return false;
    }
  }
  return true;
}

async function setupAuthRoutes(page, baseUrl) {
  await page.route("**/api/backend/**", async (route) => {
    const req = route.request();
    const pathname = new URL(req.url()).pathname;
    const method = req.method();
    if (new URL(req.url()).origin !== baseUrl)
      return route.abort("blockedbyclient");
    if (pathname === "/api/backend/auth/terms" && method === "GET")
      return route.continue();
    if (pathname === "/api/backend/rights-requests" && method === "GET")
      return route.fulfill({ status: 200, json: { items: [] } });

    if (pathname === "/api/backend/auth/login" && method === "POST") {
      return route.fulfill({
        status: 200,
        headers: { "content-type": "application/json; charset=utf-8" },
        body: JSON.stringify({
          account_id: "770e8400-e29b-41d4-a716-446655440000",
          roles: ["arrendatario", "anfitrion"],
          capabilities: [],
          access_token: "synthetic-palette-token-941",
          token_type: "Bearer",
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
          restricted_mode: false,
        }),
      });
    }
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return route.fulfill({
        status: 200,
        headers: { "content-type": "application/json; charset=utf-8" },
        body: JSON.stringify({
          account_id: "770e8400-e29b-41d4-a716-446655440000",
          roles: ["arrendatario", "anfitrion"],
          capabilities: [],
          restricted_mode: false,
        }),
      });
    }
    if (pathname === "/api/backend/profile" && method === "GET") {
      return route.fulfill({
        status: 200,
        headers: { "content-type": "application/json; charset=utf-8" },
        body: JSON.stringify({
          display_name: "Usuario Sintético",
          phone: "912345678",
          updated_at: "2026-10-25T12:00:00Z",
        }),
      });
    }
    if (pathname === "/api/backend/profile/photo" && method === "GET") {
      return route.fulfill({
        status: 404,
        headers: { "content-type": "application/json; charset=utf-8" },
        body: JSON.stringify({
          error: { code: "photo_not_found", message: "No photo" },
        }),
      });
    }
    if (pathname === "/api/backend/payout-account" && method === "GET") {
      return route.fulfill({
        status: 404,
        headers: { "content-type": "application/json; charset=utf-8" },
        body: JSON.stringify({
          error: { code: "payout_not_found", message: "No payout" },
        }),
      });
    }
    return route.fulfill({
      status: 501,
      json: {
        error: {
          code: "unmapped_palette_fixture",
          message: `Unmapped fixture: ${method} ${pathname}`,
        },
      },
    });
  });
}

async function captureScenario(
  browser,
  routeDef,
  vp,
  baseUrl,
  stage,
  screenshotsDir,
) {
  const context = await browser.newContext({
    viewport: { width: vp.width, height: vp.height },
    userAgent: "Mozilla/5.0 (Playwright Isolated Palette Visual Runner)",
  });
  const page = await context.newPage();
  const blockedExternalRequests = [];
  const consoleErrors = [];

  // Guard: strictly block non-loopback network traffic
  await page.route("**/*", async (route) => {
    const reqUrl = new URL(route.request().url());
    const isLoopback = reqUrl.origin === baseUrl;
    if (!isLoopback) {
      blockedExternalRequests.push(reqUrl.href);
      await route.abort("blockedbyclient");
      return;
    }
    await route.continue();
  });

  page.on("console", (msg) => {
    if (msg.type() === "error") {
      const text = msg.text();
      const locUrl = msg.location()?.url || "";
      const isExpected =
        (text.includes("404") || text.includes("Failed to load resource")) &&
        (locUrl.includes("/api/backend/profile/photo") ||
          locUrl.includes("/api/backend/payout-account"));
      consoleErrors.push({ text, url: locUrl, isExpected });
    }
  });
  page.on("pageerror", (err) => {
    consoleErrors.push({ text: err.message, url: "", isExpected: false });
  });

  await setupAuthRoutes(page, baseUrl);
  if (routeDef.requiresAuth) {
    await page.goto(
      `${baseUrl}/acceso?return_to=${encodeURIComponent(routeDef.path)}`,
      {
        waitUntil: "domcontentloaded",
        timeout: 30000,
      },
    );
    await page
      .getByLabel("Correo electrónico")
      .fill("phase6-tester@example.invalid");
    await page
      .getByLabel("Contraseña", { exact: true })
      .fill("SyntheticPass123!");
    await page.getByRole("button", { name: "Iniciar sesión" }).click();
    await page.waitForURL((url) => url.pathname === routeDef.path, {
      timeout: 15000,
    });
    await page.waitForLoadState("networkidle").catch(() => {});
  } else {
    await page.goto(`${baseUrl}${routeDef.path}`, {
      waitUntil: "networkidle",
      timeout: 30000,
    });
  }

  const domMetrics = await page.evaluate(() => {
    const docEl = document.documentElement;
    const body = document.body;
    const hasOverflow = docEl.scrollWidth > window.innerWidth;
    const rootStyle = window.getComputedStyle(docEl);
    const bodyStyle = window.getComputedStyle(body);

    const scope = document.querySelector(".saas-application") || body;
    const probe = document.createElement("span");
    probe.style.cssText =
      "color:var(--primary);background-color:var(--background)";
    scope.appendChild(probe);
    const primaryRgb = getComputedStyle(probe).color;
    const canvasRgb = getComputedStyle(probe).backgroundColor;
    probe.remove();
    const colors = {
      scopePrimaryRgb: primaryRgb,
      scopeCanvasRgb: canvasRgb,
      rootPrimary: rootStyle.getPropertyValue("--primary").trim(),
      rootBackground: rootStyle.getPropertyValue("--background").trim(),
      rootColorPrimary: rootStyle.getPropertyValue("--color-primary").trim(),
      rootColorBackground: rootStyle
        .getPropertyValue("--color-background")
        .trim(),
      bodyColor: bodyStyle.color,
      bodyBg: bodyStyle.backgroundColor,
      matchesTargetPalette: {
        primaryTargetHex: "#A44731",
        primaryTargetRgb: "rgb(164, 71, 49)",
        canvasTargetHex: "#FBF7F2",
        canvasTargetRgb: "rgb(251, 247, 242)",
        isPrimaryTarget: primaryRgb === "rgb(164, 71, 49)",
        isCanvasTarget: canvasRgb === "rgb(251, 247, 242)",
      },
    };

    const allElements = Array.from(document.querySelectorAll("*"));
    const visible = [];
    for (let i = 0; i < allElements.length; i++) {
      const el = allElements[i];
      const tag = el.tagName.toLowerCase();
      if (
        [
          "script",
          "style",
          "noscript",
          "link",
          "meta",
          "title",
          "head",
        ].includes(tag)
      )
        continue;
      if (el.ownerSVGElement) continue;

      const rect = el.getBoundingClientRect();
      if (rect.width <= 0 || rect.height <= 0) continue;
      const style = window.getComputedStyle(el);
      if (
        style.display === "none" ||
        style.visibility === "hidden" ||
        Number(style.opacity) === 0
      )
        continue;

      let text = "";
      for (const node of el.childNodes) {
        if (node.nodeType === Node.TEXT_NODE) text += node.textContent || "";
      }
      text = text.trim().replace(/\s+/g, " ");

      let path = tag;
      if (el.id) path = `#${el.id}`;
      else if (el.parentElement) {
        const idx = Array.prototype.indexOf.call(el.parentElement.children, el);
        path = `${el.parentElement.tagName.toLowerCase()} > ${tag}:nth-child(${idx + 1})`;
      }

      visible.push({
        index: i,
        tag,
        path,
        text,
        box: {
          x: Math.round(rect.x * 100) / 100,
          y: Math.round(rect.y * 100) / 100,
          width: Math.round(rect.width * 100) / 100,
          height: Math.round(rect.height * 100) / 100,
        },
        fontFamily: style.fontFamily,
        fontSize: style.fontSize,
        lineHeight: style.lineHeight,
        fontWeight: style.fontWeight,
        borderWidths: `${style.borderTopWidth} ${style.borderRightWidth} ${style.borderBottomWidth} ${style.borderLeftWidth}`,
        borderRadii: `${style.borderTopLeftRadius} ${style.borderTopRightRadius} ${style.borderBottomRightRadius} ${style.borderBottomLeftRadius}`,
        padding: `${style.paddingTop} ${style.paddingRight} ${style.paddingBottom} ${style.paddingLeft}`,
        margin: `${style.marginTop} ${style.marginRight} ${style.marginBottom} ${style.marginLeft}`,
        gap: style.gap,
      });
    }

    return {
      overflow: {
        hasOverflow,
        scrollWidth: docEl.scrollWidth,
        innerWidth: window.innerWidth,
      },
      colors,
      elements: visible,
    };
  });

  const axeReport = await new AxeBuilder({ page }).analyze();

  const violations = axeReport.violations.map((v) => ({
    id: v.id,
    impact: v.impact,
    description: v.description,
    nodesCount: v.nodes.length,
    targets: v.nodes.map((n) => n.target.join(" ")),
  }));

  const screenshotName = `${stage}-${routeDef.name}-${vp.name}.png`;
  const screenshotPath = path.join(screenshotsDir, screenshotName);
  await page.screenshot({ path: screenshotPath, fullPage: true });
  try {
    fs.chmodSync(screenshotPath, 0o600);
  } catch {} // ensure 0600

  await context.close();

  return {
    route: routeDef.path,
    routeName: routeDef.name,
    viewport: vp.name,
    screenshot: screenshotPath,
    metrics: domMetrics,
    axeViolations: violations,
    consoleErrors,
    blockedExternalRequests,
  };
}

function compareScenarios(baselineList, afterList) {
  const diffReport = [];
  let hasRegressions = false;

  for (const afterItem of afterList) {
    const baseItem = baselineList.find(
      (b) =>
        b.routeName === afterItem.routeName &&
        b.viewport === afterItem.viewport,
    );
    if (!baseItem) {
      diffReport.push({
        error: `Missing baseline for ${afterItem.routeName} (${afterItem.viewport})`,
      });
      hasRegressions = true;
      continue;
    }

    const scenarioDiff = {
      route: afterItem.route,
      viewport: afterItem.viewport,
      layoutDiffs: [],
      axeDiffs: [],
      overflowDiff: null,
      consoleErrorDiff: [],
      colorProbe: {
        baselineColors: baseItem.metrics.colors,
        afterColors: afterItem.metrics.colors,
      },
    };

    const baseEls = baseItem.metrics.elements;
    const afterEls = afterItem.metrics.elements;
    if (baseEls.length !== afterEls.length) {
      scenarioDiff.layoutDiffs.push({
        kind: "element_count_mismatch",
        baselineCount: baseEls.length,
        afterCount: afterEls.length,
      });
      hasRegressions = true;
    }

    const count = Math.min(baseEls.length, afterEls.length);
    for (let i = 0; i < count; i++) {
      const b = baseEls[i];
      const a = afterEls[i];
      const issues = [];

      if (b.tag !== a.tag) issues.push(`tag: ${b.tag} -> ${a.tag}`);
      if (b.text !== a.text) issues.push(`text: "${b.text}" -> "${a.text}"`);

      const dx = Math.abs(b.box.x - a.box.x);
      const dy = Math.abs(b.box.y - a.box.y);
      const dw = Math.abs(b.box.width - a.box.width);
      const dh = Math.abs(b.box.height - a.box.height);
      if (dx > 0.1 || dy > 0.1 || dw > 0.1 || dh > 0.1) {
        issues.push(
          `box (diff > 0.1px): [${b.box.x},${b.box.y},${b.box.width},${b.box.height}] vs [${a.box.x},${a.box.y},${a.box.width},${a.box.height}]`,
        );
      }

      if (b.fontFamily !== a.fontFamily)
        issues.push(`fontFamily: ${b.fontFamily} vs ${a.fontFamily}`);
      if (valuesDiffer(b.fontSize, a.fontSize, 0.1))
        issues.push(`fontSize: ${b.fontSize} vs ${a.fontSize}`);
      if (valuesDiffer(b.lineHeight, a.lineHeight, 0.1))
        issues.push(`lineHeight: ${b.lineHeight} vs ${a.lineHeight}`);
      if (b.fontWeight !== a.fontWeight)
        issues.push(`fontWeight: ${b.fontWeight} vs ${a.fontWeight}`);
      if (valuesDiffer(b.borderWidths, a.borderWidths, 0.1))
        issues.push(`borderWidths: ${b.borderWidths} vs ${a.borderWidths}`);
      if (valuesDiffer(b.borderRadii, a.borderRadii, 0.1))
        issues.push(`borderRadii: ${b.borderRadii} vs ${a.borderRadii}`);
      if (valuesDiffer(b.padding, a.padding, 0.1))
        issues.push(`padding: ${b.padding} vs ${a.padding}`);
      if (valuesDiffer(b.margin, a.margin, 0.1))
        issues.push(`margin: ${b.margin} vs ${a.margin}`);
      if (valuesDiffer(b.gap, a.gap, 0.1))
        issues.push(`gap: ${b.gap} vs ${a.gap}`);

      if (issues.length > 0) {
        scenarioDiff.layoutDiffs.push({ element: b.path, issues });
        hasRegressions = true;
      }
    }

    const baseViolationsMap = new Map(
      baseItem.axeViolations.map((v) => [v.id, new Set(v.targets)]),
    );
    for (const v of afterItem.axeViolations) {
      const baseTargets = baseViolationsMap.get(v.id) || new Set();
      const baseCount = baseTargets.size;
      if (v.targets.some((target) => !baseTargets.has(target))) {
        scenarioDiff.axeDiffs.push({
          id: v.id,
          impact: v.impact,
          description: v.description,
          baselineOccurrences: baseCount,
          afterOccurrences: v.nodesCount,
          newNodes: v.nodesCount - baseCount,
        });
        hasRegressions = true;
      }
    }

    if (afterItem.metrics.overflow.hasOverflow) {
      scenarioDiff.overflowDiff = afterItem.metrics.overflow;
      hasRegressions = true;
    }

    if (
      !afterItem.metrics.colors.matchesTargetPalette.isPrimaryTarget ||
      !afterItem.metrics.colors.matchesTargetPalette.isCanvasTarget
    ) {
      scenarioDiff.paletteFailure = afterItem.metrics.colors;
      hasRegressions = true;
    }
    const unexpectedErrors = afterItem.consoleErrors.filter(
      (e) => !e.isExpected,
    );
    if (unexpectedErrors.length > 0) {
      scenarioDiff.consoleErrorDiff = unexpectedErrors;
      hasRegressions = true;
    }

    if (afterItem.blockedExternalRequests.length > 0) {
      scenarioDiff.blockedExternalRequests = afterItem.blockedExternalRequests;
      hasRegressions = true;
    }

    diffReport.push(scenarioDiff);
  }

  return { hasRegressions, diffReport };
}

async function main() {
  const { stage, baseUrl } = parseArgs();
  const { outputDir, screenshotsDir } = setupOutputDirs();
  console.log(`[palette-visual] Starting stage: ${stage} against ${baseUrl}`);
  console.log(`[palette-visual] Output directory: ${outputDir}`);

  const browser = await chromium.launch({ headless: true });
  const results = [];
  let fatalError = false;

  try {
    for (const routeDef of ROUTES) {
      for (const vp of VIEWPORTS) {
        console.log(
          `[palette-visual] Capturing ${routeDef.path} @ ${vp.name}...`,
        );
        const res = await captureScenario(
          browser,
          routeDef,
          vp,
          baseUrl,
          stage,
          screenshotsDir,
        );
        results.push(res);

        const unexpectedErrors = res.consoleErrors.filter((e) => !e.isExpected);
        if (unexpectedErrors.length > 0) {
          console.error(
            `[palette-visual] Unexpected console errors on ${routeDef.path} (${vp.name}):`,
            unexpectedErrors,
          );
          if (stage === "before") fatalError = true;
        }
        if (res.blockedExternalRequests.length > 0) {
          console.error(
            `[palette-visual] Blocked external requests on ${routeDef.path} (${vp.name}):`,
            res.blockedExternalRequests,
          );
          fatalError = true;
        }
        if (res.metrics.overflow.hasOverflow) {
          console.error(
            `[palette-visual] Horizontal overflow detected on ${routeDef.path} (${vp.name})`,
          );
          if (stage === "before") fatalError = true;
        }
      }
    }
  } finally {
    await browser.close();
  }

  if (stage === "before") {
    const baselinePath = path.join(outputDir, "baseline.json");
    writeSecureJson(baselinePath, {
      timestamp: new Date().toISOString(),
      baseUrl,
      scenarios: results,
    });
    console.log(
      `[palette-visual] Baseline saved successfully to ${baselinePath}`,
    );
    if (fatalError) {
      console.error(
        "[palette-visual] Baseline run encountered guard failures, unexpected console errors or overflow.",
      );
      process.exit(1);
    }
    console.log("[palette-visual] Stage BEFORE completed cleanly (exit 0).");
    process.exit(0);
  } else {
    const baselinePath = path.join(outputDir, "baseline.json");
    if (!fs.existsSync(baselinePath)) {
      console.error(
        `[palette-visual] Error: baseline file not found at ${baselinePath}. Run --stage before first.`,
      );
      process.exit(1);
    }
    const baselineData = JSON.parse(fs.readFileSync(baselinePath, "utf8"));
    const { hasRegressions, diffReport } = compareScenarios(
      baselineData.scenarios,
      results,
    );

    writeSecureJson(path.join(outputDir, "after.json"), {
      baseUrl,
      scenarios: results,
    });
    const comparisonPath = path.join(outputDir, "comparison.json");
    writeSecureJson(comparisonPath, {
      timestamp: new Date().toISOString(),
      baseUrl,
      hasRegressions,
      diffReport,
    });
    console.log(`[palette-visual] Comparison saved to ${comparisonPath}`);

    if (hasRegressions) {
      console.error(
        "[palette-visual] Regression detected in stage AFTER (exit 1)!",
      );
      for (const d of diffReport) {
        if (d.layoutDiffs?.length > 0)
          console.error(
            `  [${d.route} ${d.viewport}] ${d.layoutDiffs.length} layout differences`,
          );
        if (d.axeDiffs?.length > 0)
          console.error(
            `  [${d.route} ${d.viewport}] ${d.axeDiffs.length} new/increased Axe violations`,
          );
        if (d.overflowDiff)
          console.error(`  [${d.route} ${d.viewport}] Horizontal overflow`);
        if (d.consoleErrorDiff?.length > 0)
          console.error(`  [${d.route} ${d.viewport}] Console errors`);
      }
      process.exit(1);
    } else {
      console.log(
        "[palette-visual] Stage AFTER matched baseline within <=0.1px tolerance with no regressions (exit 0).",
      );
      process.exit(0);
    }
  }
}

main().catch((err) => {
  console.error("[palette-visual] Fatal error:", err);
  process.exit(1);
});
