// @ts-nocheck


export const routes = {
  "meta": {},
  "id": "_default",
  "name": "",
  "file": {
    "path": "src/routes/_module.svelte",
    "dir": "src/routes",
    "base": "_module.svelte",
    "ext": ".svelte",
    "name": "_module"
  },
  "asyncModule": () => import('../src/routes/_module.svelte'),
  "rootName": "default",
  "routifyDir": import.meta.url,
  "children": [
    {
      "meta": {
        "dynamic": true,
        "order": false
      },
      "id": "_default__streamingService_",
      "name": "[streamingService]",
      "module": false,
      "file": {
        "path": "src/routes/[streamingService]",
        "dir": "src/routes",
        "base": "[streamingService]",
        "ext": "",
        "name": "[streamingService]"
      },
      "children": [
        {
          "meta": {
            "dynamic": true,
            "order": false
          },
          "id": "_default__streamingService___show_",
          "name": "[show]",
          "module": false,
          "file": {
            "path": "src/routes/[streamingService]/[show]",
            "dir": "src/routes/[streamingService]",
            "base": "[show]",
            "ext": "",
            "name": "[show]"
          },
          "children": [
            {
              "meta": {
                "dynamic": true,
                "order": false
              },
              "id": "_default__streamingService___show___season_",
              "name": "[season]",
              "module": false,
              "file": {
                "path": "src/routes/[streamingService]/[show]/[season]",
                "dir": "src/routes/[streamingService]/[show]",
                "base": "[season]",
                "ext": "",
                "name": "[season]"
              },
              "children": [
                {
                  "meta": {
                    "dynamic": true,
                    "order": false
                  },
                  "id": "_default__streamingService___show___season___episode_",
                  "name": "[episode]",
                  "module": false,
                  "file": {
                    "path": "src/routes/[streamingService]/[show]/[season]/[episode]",
                    "dir": "src/routes/[streamingService]/[show]/[season]",
                    "base": "[episode]",
                    "ext": "",
                    "name": "[episode]"
                  },
                  "children": [
                    {
                      "meta": {
                        "isDefault": true,
                        "reset": true
                      },
                      "id": "_default__streamingService___show___season___episode__index_svelte",
                      "name": "index",
                      "file": {
                        "path": "src/routes/[streamingService]/[show]/[season]/[episode]/index.svelte",
                        "dir": "src/routes/[streamingService]/[show]/[season]/[episode]",
                        "base": "index.svelte",
                        "ext": ".svelte",
                        "name": "index"
                      },
                      "asyncModule": () => import('../src/routes/[streamingService]/[show]/[season]/[episode]/index.svelte'),
                      "children": []
                    }
                  ]
                },
                {
                  "meta": {
                    "isDefault": true
                  },
                  "id": "_default__streamingService___show___season__index_svelte",
                  "name": "index",
                  "file": {
                    "path": "src/routes/[streamingService]/[show]/[season]/index.svelte",
                    "dir": "src/routes/[streamingService]/[show]/[season]",
                    "base": "index.svelte",
                    "ext": ".svelte",
                    "name": "index"
                  },
                  "asyncModule": () => import('../src/routes/[streamingService]/[show]/[season]/index.svelte'),
                  "children": []
                }
              ]
            },
            {
              "meta": {
                "isDefault": true
              },
              "id": "_default__streamingService___show__index_svelte",
              "name": "index",
              "file": {
                "path": "src/routes/[streamingService]/[show]/index.svelte",
                "dir": "src/routes/[streamingService]/[show]",
                "base": "index.svelte",
                "ext": ".svelte",
                "name": "index"
              },
              "asyncModule": () => import('../src/routes/[streamingService]/[show]/index.svelte'),
              "children": []
            }
          ]
        },
        {
          "meta": {
            "isDefault": true
          },
          "id": "_default__streamingService__index_svelte",
          "name": "index",
          "file": {
            "path": "src/routes/[streamingService]/index.svelte",
            "dir": "src/routes/[streamingService]",
            "base": "index.svelte",
            "ext": ".svelte",
            "name": "index"
          },
          "asyncModule": () => import('../src/routes/[streamingService]/index.svelte'),
          "children": []
        }
      ]
    },
    {
      "meta": {
        "isDefault": true
      },
      "id": "_default_index_svelte",
      "name": "index",
      "file": {
        "path": "src/routes/index.svelte",
        "dir": "src/routes",
        "base": "index.svelte",
        "ext": ".svelte",
        "name": "index"
      },
      "asyncModule": () => import('../src/routes/index.svelte'),
      "children": []
    },
    {
      "meta": {},
      "id": "_default_search",
      "name": "search",
      "module": false,
      "file": {
        "path": "src/routes/search",
        "dir": "src/routes",
        "base": "search",
        "ext": "",
        "name": "search"
      },
      "children": [
        {
          "meta": {
            "dynamic": true,
            "order": false
          },
          "id": "_default_search__query__svelte",
          "name": "[query]",
          "file": {
            "path": "src/routes/search/[query].svelte",
            "dir": "src/routes/search",
            "base": "[query].svelte",
            "ext": ".svelte",
            "name": "[query]"
          },
          "asyncModule": () => import('../src/routes/search/[query].svelte'),
          "children": []
        }
      ]
    },
    {
      "meta": {
        "dynamic": true,
        "dynamicSpread": true,
        "order": false,
        "inline": false
      },
      "name": "[...404]",
      "file": {
        "path": ".routify/components/[...404].svelte",
        "dir": ".routify/components",
        "base": "[...404].svelte",
        "ext": ".svelte",
        "name": "[...404]"
      },
      "asyncModule": () => import('./components/[...404].svelte'),
      "children": []
    }
  ]
}
export default routes