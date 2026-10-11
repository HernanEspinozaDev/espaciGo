import { defineConfig } from '@hey-api/openapi-ts';

export default defineConfig({
  input: '../planning/openapi.yaml',
  output: {
    path: 'src/generated/api',
    clean: true,
  },
  plugins: [
    '@hey-api/client-angular',
    '@hey-api/typescript',
    {
      name: '@hey-api/sdk',
      operations: { strategy: 'byTags' },
    },
  ],
});
