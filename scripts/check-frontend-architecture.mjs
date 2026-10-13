import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const require = createRequire(path.join(root, 'frontend/package.json'));
const ts = require('typescript');
const app = path.join(root, 'frontend/src/app');
// Endpoint namespaces come from the authoritative contract, not a hand-maintained subset.
export function contractRoots(openapi) {
  return new Set([...openapi.matchAll(/^  \/([^/ :]+)(?:[^\n]*):\s*$/gm)].map(match => match[1]));
}
export function isBackendPath(value, roots) {
  const path = value.replace(/^https?:\/\/[^/]+/i, '');
  if (/^\/api\/v\d+(?:\/|$)/.test(path)) return true;
  return roots.has(path.match(/^\/([^/?#]+)/)?.[1]);
}
export function inspectSource(file, text, roots) {
  const errors = [];
  const relative = path.relative(app, file).replaceAll('\\', '/');
  const source = ts.createSourceFile(file, text, ts.ScriptTarget.Latest, true);
  const domain = relative.includes('/domain/') || relative.startsWith('core/contracts/');
  const application = relative.includes('/application/');
  const component = /@Component\s*\(/.test(text);
  const shared = relative.startsWith('shared/');
  for (const node of source.statements) {
    if (!ts.isImportDeclaration(node)) continue;
    const module = node.moduleSpecifier.text;
    const target = module.startsWith('.') ? path.relative(app, path.resolve(path.dirname(file), module)).replaceAll('\\', '/') : module;
    if (domain && (/^(@angular|rxjs)/.test(module) || /generated\/api|infrastructure|application|presentation/.test(target))) errors.push(`${relative}: pure domain/contracts imports ${module}`);
    if (application && (/generated\/api|infrastructure/.test(target) || module === '@angular/common/http')) errors.push(`${relative}: application depends on transport ${module}`);
    if (component && (/generated\/api|infrastructure/.test(target) || module === '@angular/common/http')) errors.push(`${relative}: view depends on transport ${module}`);
    if (shared && target.startsWith('features/')) errors.push(`${relative}: shared UI depends on a feature ${module}`);
    if (/infrastructure/.test(relative) && (/-page$|-panel$|-facade$/.test(target) || target.includes('/application/'))) errors.push(`${relative}: adapter depends on view/application ${module}`);
  }
  function staticPrefix(node) {
    if (ts.isStringLiteralLike(node) || ts.isTemplateHead(node)) return node.text;
    if (ts.isTemplateExpression(node)) return node.head.text;
    if (ts.isBinaryExpression(node) && node.operatorToken.kind === ts.SyntaxKind.PlusToken) {
      const left = staticPrefix(node.left);
      const right = staticPrefix(node.right);
      return left === undefined ? undefined : left + (right ?? '');
    }
    return undefined;
  }
  function visit(node) {
    if (component && ts.isPropertyAssignment(node) && node.name.getText(source) === 'styles') errors.push(`${relative}: inline component CSS bypasses Stylelint; use styleUrl`);
    const prefix = staticPrefix(node);
    if ((application || domain || component) && prefix !== undefined && isBackendPath(prefix, roots)) errors.push(`${relative}: REST path outside infrastructure`);
    if ((application || domain || component) && ts.isCallExpression(node) && /^(fetch|globalThis\.fetch|window\.fetch)$/.test(node.expression.getText(source))) errors.push(`${relative}: HTTP call outside infrastructure`);

    if (application && (ts.isStringLiteralLike(node) || ts.isTemplateHead(node) || ts.isTemplateMiddle(node) || ts.isTemplateTail(node)) && isBackendPath(node.text, roots)) errors.push(`${relative}:${source.getLineAndCharacterOfPosition(node.getStart(source)).line + 1}: REST path outside infrastructure`);
    ts.forEachChild(node, visit);
  }
  visit(source);
  if (component && /\bstandalone:\s*true/.test(text)) errors.push(`${relative}: redundant standalone metadata`);
  return errors;
}
function run() {
  const files = [];
  function walk(dir) {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const file = path.join(dir, entry.name);
      if (entry.isDirectory()) walk(file);
      else if (file.endsWith('.ts') && !file.endsWith('.spec.ts')) files.push(file);
    }
  }
  walk(app);
  const roots = contractRoots(fs.readFileSync(path.join(root, 'planning/openapi.yaml'), 'utf8'));
  if (!roots.size) throw new Error('No endpoint namespaces found in OpenAPI');
  const errors = files.flatMap(file => inspectSource(file, fs.readFileSync(file, 'utf8'), roots));
  if (errors.length) { console.error(errors.join('\n')); process.exitCode = 1; }
  else console.log(`Architecture PASS: ${files.length} production files; endpoint namespaces: ${[...roots].sort().join(', ')}; domain/contracts pure; UI/application independent of SDK/HTTP.`);
}
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) run();
