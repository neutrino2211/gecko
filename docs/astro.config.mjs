// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  integrations: [
    starlight({
      title: 'Gecko',
      description: 'Documentation for the Gecko programming language',
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/neutrino2211/gecko' },
      ],
      customCss: ['./src/styles/custom.css'],
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
            { label: 'Pointers', slug: 'language/pointers' },
            { label: 'C Interop', slug: 'language/c-interop' },
          ],
        },
        {
          label: 'Standard Library',
          autogenerate: { directory: 'stdlib' },
        },
      ],
    }),
  ],
});
