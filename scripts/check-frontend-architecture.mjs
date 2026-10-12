import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const require = createRequire(path.join(root, 'frontend/package.json'));
const ts = require('typescript');
const app = path.join(root, 'frontend/src/app');
const files = [];
function walk(dir) { for (const entry of fs.readdirSync(dir, { withFileTypes: true })) { const file = path.join(dir, entry.name); if (entry.isDirectory()) walk(file); else if (file.endsWith('.ts') && !file.endsWith('.spec.ts')) files.push(file); } }
walk(app);
const errors = [];
let ports = 0, facades = 0, components = 0;
for (const file of files) {
  const relative = path.relative(app, file).replaceAll('\\', '/');
  const text = fs.readFileSync(file, 'utf8');
  const source = ts.createSourceFile(file, text, ts.ScriptTarget.Latest, true);
  const domain = relative.includes('/domain/') || relative.startsWith('core/contracts/');
  const application = relative.includes('/application/');
  const component = /@Component\s*\(/.test(text);
  const shared = relative.startsWith('shared/');
  if (relative.includes('/domain/ports/')) ports++;
  if (application && relative.endsWith('-facade.ts')) facades++;
  if (component) components++;
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
  function visit(node) {
    if (application && (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node) || ts.isTemplateHead(node)) && /^\/(admin\/|local\/|privacy\/)/.test(node.text)) errors.push(`${relative}:${source.getLineAndCharacterOfPosition(node.getStart(source)).line + 1}: REST path outside infrastructure`);
    ts.forEachChild(node, visit);
  }
  visit(source);
  if (component && /\bstandalone:\s*true/.test(text)) errors.push(`${relative}: redundant standalone metadata`);
}
if (errors.length) { console.error(errors.join('\n')); process.exitCode = 1; }
else console.log(`Architecture PASS: ${ports} ports, ${facades} facades, ${components} components; domain/contracts pure; UI/application independent of SDK/HTTP; shared UI feature-independent.`);
