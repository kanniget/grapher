import { svelte } from '@sveltejs/vite-plugin-svelte';

export default {
  plugins: [svelte()],
  build: {
    outDir: '../backend/public',
    emptyOutDir: true
  }
};
