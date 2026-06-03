// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import { readFileSync } from 'fs';

// Load custom Gecko grammar for syntax highlighting
const geckoGrammar = JSON.parse(
  readFileSync(new URL('./gecko.tmLanguage.json', import.meta.url), 'utf-8')
);

export default defineConfig({
  integrations: [
    starlight({
      title: 'Gecko',
      description: 'Documentation for the Gecko programming language',
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/neutrino2211/gecko' },
      ],
      customCss: ['./src/styles/custom.css'],
      expressiveCode: {
        shiki: {
          langs: [geckoGrammar],
        },
      },
      sidebar: [
        {
          label: 'Getting Started',
          items: [
            { label: 'Introduction', slug: 'index' },
            { label: 'Installation', slug: 'guide/getting-started' },
          ],
        },
        {
          label: 'Language Reference',
          items: [
            { label: 'Basics', slug: 'language/basics' },
            { label: 'Control Flow', slug: 'language/control-flow' },
            { label: 'Classes', slug: 'language/classes' },
            { label: 'Traits', slug: 'language/traits' },
            { label: 'Generics', slug: 'language/generics' },
            { label: 'Modules & Imports', slug: 'language/modules' },
            { label: 'Hooks', slug: 'language/hooks' },
            { label: 'Error Handling', slug: 'language/error-handling' },
            { label: 'Enums', slug: 'language/enums' },
            { label: 'Visibility', slug: 'language/visibility' },
            { label: 'Intrinsics', slug: 'language/intrinsics' },
            { label: 'FFI & C Interop', slug: 'language/ffi' },
            { label: 'Pointers', slug: 'language/pointers' },
          ],
        },
        {
          label: 'Tooling',
          items: [
            { label: 'CLI Reference', slug: 'tooling/cli' },
            { label: 'Project Configuration', slug: 'tooling/project-config' },
            { label: 'Cross-Compilation', slug: 'tooling/cross-compilation' },
            { label: 'Kernel Development', slug: 'tooling/kernel-development' },
          ],
        },
        {
          label: 'Standard Library',
          items: [
            { label: 'Overview', slug: 'stdlib' },
            {
              label: 'std.core',
              items: [
                { label: 'traits', slug: 'stdlib/std-core-traits' },
                { label: 'ops', slug: 'stdlib/std-core-ops' },
              ],
            },
            {
              label: 'std.collections',
              items: [
                { label: 'vec', slug: 'stdlib/std-collections-vec' },
                { label: 'slice', slug: 'stdlib/std-collections-slice' },
                { label: 'string', slug: 'stdlib/std-collections-string' },
              ],
            },
            {
              label: 'std.memory',
              items: [
                { label: 'box', slug: 'stdlib/std-memory-box' },
                { label: 'buffer', slug: 'stdlib/std-memory-buffer' },
                { label: 'raw', slug: 'stdlib/std-memory-raw' },
                { label: 'rc', slug: 'stdlib/std-memory-rc' },
                { label: 'weak', slug: 'stdlib/std-memory-weak' },
              ],
            },
            { label: 'std.option', slug: 'stdlib/std-option' },
            { label: 'std.result', slug: 'stdlib/std-result' },
          ],
        },
      ],
    }),
  ],
});
