import { reactRouter } from '@react-router/dev/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';
import { fumadocsMdx } from 'fumadocs-mdx/vite';
import { basename } from './app/lib/shared.ts';

export default defineConfig({
  // Bundled assets are requested from the same prefix GitHub Pages serves this
  // site under. This rather than Vite's `base`, which prefixes the URLs
  // react-router prerenders as well as the ones it emits, so every resource
  // route is then asked for at a path that does not exist and the prerender
  // fails. `renderBuiltUrl` moves only the emitted URLs.
  experimental: {
    renderBuiltUrl(filename: string) {
      return `${basename}/${filename}`;
    },
  },
  plugins: [fumadocsMdx(), tailwindcss(), reactRouter()],
  resolve: {
    tsconfigPaths: true,
  },
});
