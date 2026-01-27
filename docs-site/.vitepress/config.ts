import { defineConfig } from 'vitepress'

export default defineConfig({
  title: "immich-go",
  description: "Fast, multi-source uploader for Immich",
  
  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/logo.svg' }],
  ],

  themeConfig: {
    logo: '/logo.svg',
    
    nav: [
      { text: 'Guide', link: '/guide/getting-started' },
      { text: 'Commands', link: '/commands/upload' },
      { text: 'Reference', link: '/reference/configuration' },
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Introduction',
          items: [
            { text: 'Getting Started', link: '/guide/getting-started' },
            { text: 'Installation', link: '/guide/installation' },
          ]
        },
        {
          text: 'Migration Guides',
          items: [
            { text: 'Google Photos', link: '/guide/google-photos' },
            { text: 'iCloud', link: '/guide/icloud' },
            { text: 'Server to Server', link: '/guide/server-migration' },
          ]
        },
        {
          text: 'Topics',
          items: [
            { text: 'File Stacking', link: '/guide/stacking' },
            { text: 'Performance Tuning', link: '/guide/performance' },
          ]
        },
      ],
      '/commands/': [
        {
          text: 'Commands',
          items: [
            { text: 'upload', link: '/commands/upload' },
            { text: 'archive', link: '/commands/archive' },
            { text: 'stack', link: '/commands/stack' },
          ]
        },
      ],
      '/reference/': [
        {
          text: 'Reference',
          items: [
            { text: 'Configuration', link: '/reference/configuration' },
            { text: 'Environment Variables', link: '/reference/environment' },
          ]
        },
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/sweepies/immich-go' }
    ],

    search: {
      provider: 'local'
    },

    editLink: {
      pattern: 'https://github.com/sweepies/immich-go/edit/main/docs-site/:path',
      text: 'Edit this page on GitHub'
    },

    footer: {
      message: 'Released under the MIT License.',
      copyright: 'A fork of simulot/immich-go'
    }
  }
})
