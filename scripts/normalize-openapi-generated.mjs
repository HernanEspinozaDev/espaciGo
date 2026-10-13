import { readdir, readFile, writeFile } from 'node:fs/promises';
import { join, resolve } from 'node:path';

const directory = resolve(process.argv[2] ?? 'frontend/src/generated/api');
for (const name of await readdir(directory)) {
  if (!name.endsWith('.ts')) continue;
  const path = join(directory, name);
  const source = await readFile(path, 'utf8');
  const normalized = source.replace(/[\t ]+$/gm, '').replace(/\n*$/, '\n');
  if (normalized !== source) await writeFile(path, normalized);
}
