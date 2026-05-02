import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [sveltekit()],
  server: {
    port: 5174,
    strictPort: false,
    proxy: {
      '/v0/management': {
        target: 'http://localhost:8317',
        changeOrigin: true,
        ws: false
      }
    }
  }
});
