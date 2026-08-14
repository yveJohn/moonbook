import { fileURLToPath } from 'node:url';
import readerConfig from '../../reader-ui/vitest.config';
import { defineConfig, mergeConfig } from '../../reader-ui/node_modules/vitest/dist/config.js';

export default mergeConfig(readerConfig, defineConfig({
  root: fileURLToPath(new URL('../../reader-ui', import.meta.url)),
  test: {
    include: ['../tests/reader-ui-contract/**/*.test.ts']
  }
}));
