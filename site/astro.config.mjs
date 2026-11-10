import starlight from "@astrojs/starlight";
import { defineConfig } from "astro/config";

export default defineConfig({
  site: "https://m7medvision.github.io",
  base: "/go-clean-template",
  integrations: [
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
      editLink: { baseUrl: "https://github.com/m7medVision/go-clean-template/edit/master/site/" },
      sidebar: [
        { label: "Getting started", items: [{ autogenerate: { directory: "getting-started" } }] },
        { label: "Concepts", items: [{ autogenerate: { directory: "concepts" } }] },
        { label: "Guides", items: [{ autogenerate: { directory: "guides" } }] },
        { label: "Reference", items: [{ autogenerate: { directory: "reference" } }] },
      ],
    }),
  ],
});
