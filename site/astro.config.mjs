import starlight from "@astrojs/starlight";
import mermaid from "astro-mermaid";
import { defineConfig } from "astro/config";
import starlightLinksValidator from "starlight-links-validator";
import starlightOpenAPI, { createOpenAPISidebarGroup } from "starlight-openapi";

const restSidebar = createOpenAPISidebarGroup();

export default defineConfig({
  site: "https://m7medvision.github.io",
  base: "/go-clean-template",
  integrations: [
    mermaid({ theme: "neutral", autoTheme: true, enableLog: false }),
    starlight({
      title: "Go Clean Template",
      logo: { src: "../docs/img/logo.svg", replacesTitle: false },
      social: [
        {
          icon: "github",
          label: "GitHub",
          href: "https://github.com/m7medVision/go-clean-template",
        },
      ],
      editLink: {
        baseUrl: "https://github.com/m7medVision/go-clean-template/edit/master/site/",
      },
      plugins: [
        starlightLinksValidator({ errorOnFallbackPages: false }),
        starlightOpenAPI([
          {
            base: "reference/rest",
            schema: "../docs/openapi/v1.json",
            sidebar: { label: "REST API", group: restSidebar },
          },
        ]),
      ],
      sidebar: [
        { label: "Getting started", items: [{ autogenerate: { directory: "getting-started" } }] },
        { label: "Concepts", items: [{ autogenerate: { directory: "concepts" } }] },
        { label: "Guides", items: [{ autogenerate: { directory: "guides" } }] },
        {
          label: "Reference",
          items: [{ autogenerate: { directory: "reference" } }, restSidebar],
        },
      ],
    }),
  ],
});
