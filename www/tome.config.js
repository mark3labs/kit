/** @type {import('@tomehq/core').TomeConfig} */
export default {
  name: "Kit",
  logo: "/logo.jpg",
  favicon: "/logo.jpg",
  baseUrl: "https://go-kit.dev",
  theme: {
    preset: "cipher",
    accent: "#e03030",
    mode: "dark",
    fonts: {
      heading: "Space Grotesk",
      body: "Space Grotesk",
      code: "Source Code Pro",
    },
  },
  navigation: [
    {
      group: "Getting Started",
      pages: ["index", "installation", "quick-start"],
    },
    {
      group: "Configuration",
      pages: ["configuration", "providers", "themes"],
    },
    {
      group: "CLI Reference",
      pages: ["cli/flags", "cli/commands"],
    },
    {
      group: "Extensions",
      pages: [
        "extensions/overview",
        "extensions/capabilities",
        "extensions/examples",
        "extensions/loading",
      ],
    },
    {
      group: "Sessions",
      pages: ["sessions"],
    },
    {
      group: "Go SDK",
      pages: ["sdk/overview", "sdk/options", "sdk/callbacks", "sdk/sessions"],
    },
    {
      group: "Advanced",
      pages: ["advanced/subagents", "advanced/json-output", "advanced/testing"],
    },
    {
      group: "Development",
      pages: ["development"],
    },
  ],
  socialLinks: [
    { platform: "github", url: "https://github.com/mark3labs/kit" },
    { platform: "discord", url: "https://discord.gg/RqSS2NQVsY" },
  ],
};
