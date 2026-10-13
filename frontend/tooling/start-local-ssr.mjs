import { spawn } from 'node:child_process';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const frontendRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const port = process.env['LOCAL_FRONTEND_PORT'] || '4200';
const host = process.env['LOCAL_FRONTEND_HOST'] || '127.0.0.1';

// These are trusted, local-only defaults. Deployments and Compose provide
// explicit API/origin values; canonical metadata never comes from Host.
const env = {
  ...process.env,
  API_INTERNAL_BASE_URL: process.env['API_INTERNAL_BASE_URL'] || 'http://127.0.0.1:8080/api/v1',
  API_PUBLIC_BASE_URL: process.env['API_PUBLIC_BASE_URL'] || 'http://localhost:8080/api/v1',
  FRONTEND_PUBLIC_ORIGIN: process.env['FRONTEND_PUBLIC_ORIGIN'] || `http://localhost:${port}`,
};
const cliPath = resolve(frontendRoot, 'node_modules', '@angular', 'cli', 'bin', 'ng.js');
const child = spawn(process.execPath, [cliPath, 'serve', '--host', host, '--port', port], {
  cwd: frontendRoot,
  env,
  stdio: 'inherit',
});

for (const signal of ['SIGINT', 'SIGTERM']) {
  process.on(signal, () => child.kill(signal));
}
child.on('error', (error) => {
  console.error(`Could not start Angular dev server: ${error.message}`);
  process.exitCode = 1;
});
child.on('exit', (code, signal) => {
  process.exitCode = signal ? 1 : (code ?? 1);
});
