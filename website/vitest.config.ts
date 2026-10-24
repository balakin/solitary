import { defineConfig } from 'vitest/config';

// Separate from vite.config.ts rather than merged into it: the react-router
// plugin there takes over the dev server to serve the app, and a test run
// wants neither its routes nor the MDX and Tailwind passes in front of them.
export default defineConfig({
  resolve: {
    tsconfigPaths: true,
  },
  test: {
    projects: [
      {
        extends: true,
        test: {
          name: 'components',
          include: ['test/components/**/*.test.{ts,tsx}'],
          environment: 'jsdom',
          setupFiles: ['test/components/setup.ts'],
        },
      },
      {
        // What GitHub Pages serves, read from build/client: whether a page was
        // prerendered at all, and what a build put in it, are facts about the
        // files, not about the components. Run `pnpm build:pages` first.
        extends: true,
        test: {
          name: 'build',
          include: ['test/build/**/*.test.ts'],
          environment: 'node',
        },
      },
    ],
  },
});
