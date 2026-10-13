// One-off migration proof. Requires the baseline commit locally; does not freeze future design work.
import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const require = createRequire(path.join(root, 'frontend/package.json'));
const sass = require('sass');
const ts = require('typescript');
const baseline = process.argv[2] || 'f461c2eab3cdf5f2bd6f94e2981c72b20d0c400e';
const git = (...args) => execFileSync('git', args, { cwd: root, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] });
git('rev-parse', '--verify', `${baseline}^{commit}`);
const files = git('diff', '--name-only', baseline, '--', 'frontend/src').trim().split('\n').filter(file => file.endsWith('.scss'));
let compared = 0;
for (const file of files) {
  if (/\/(?:_metrics|_breakpoints)\.scss$/.test(file)) continue;
  let previous;
  try { previous = git('show', `${baseline}:${file}`); }
  catch {
    const previousTs = git('show', `${baseline}:${file.replace(/\.scss$/, '.ts')}`);
    const ast = ts.createSourceFile(file, previousTs, ts.ScriptTarget.Latest, true);
    function visit(node) {
      if (ts.isPropertyAssignment(node) && node.name.getText(ast) === 'styles') previous = node.initializer.elements[0].text;
      ts.forEachChild(node, visit);
    }
    visit(ast);
    if (previous === undefined) throw new Error(`No baseline styles for ${file}`);
  }
  const options = { loadPaths: [path.dirname(path.join(root, file))], logger: sass.Logger.silent };
  const before = sass.compileString(previous, options).css;
  const after = sass.compileString(fs.readFileSync(path.join(root, file), 'utf8'), options).css;
  if (before !== after) throw new Error(`Compiled CSS changed: ${file}`);
  compared++;
}
if (!compared) throw new Error('No styles compared');
console.log(`PASS: ${compared} styles compile to byte-identical CSS versus ${baseline}`);
