import { reactRouter } from '@react-router/dev/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [reactRouter()],
  ssr: {
    noExternal: ['animal-island-ui']
  },
  build: {
    chunkSizeWarningLimit: 4500,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) {
            return undefined;
          }
          if (id.includes('animal-island-ui')) {
            return 'animal-island-ui';
          }
          if (id.includes('react-router')) {
            return 'router-vendor';
          }
          if (id.includes('react') || id.includes('scheduler')) {
            return 'react-vendor';
          }
          if (id.includes('axios')) {
            return 'http-vendor';
          }
          return undefined;
        }
      }
    }
  },
  server: {
    host: '0.0.0.0',
    port: 5174,
    proxy: {
      '/dev-api': {
        target: 'http://localhost:55328',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/dev-api/, '')
      }
    }
  }
});
