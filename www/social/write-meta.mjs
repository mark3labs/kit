import { readFile, readdir, writeFile } from "node:fs/promises";
import { join, relative, sep } from "node:path";
import config from "../tome.config.js";

const baseUrl = config.baseUrl.replace(/\/$/, "");
const imageUrl = `${baseUrl}/og-image.png`;
const imageAlt = "Kit — The extensible AI coding agent. Any model. Powerful tools. Built in Go.";
const escape = (text) => text.replaceAll("&", "&amp;").replaceAll('"', "&quot;").replaceAll("<", "&lt;").replaceAll(">", "&gt;");

// Tome 0.9 emits minimal HTML for documentation routes. Add metadata to the
// static output so crawlers do not need to execute the client-side application.
async function processDirectory(directory) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) {
      await processDirectory(path);
    } else if (entry.name === "index.html") {
      let html = await readFile(path, "utf8");
      const title = html.match(/<title>([^<]+)<\/title>/)?.[1];
      const description = html.match(/<meta name="description" content="([^"]+)"\s*\/?>/)?.[1];
      if (!title || !description) throw new Error(`${path}: missing title or description`);
      const route = relative("out", directory).split(sep).join("/");
      const url = `${baseUrl}/${route}`;
      const tag = (attribute, name, content) => `<meta ${attribute}="${name}" content="${content}" />`;
      // Values read from HTML are already escaped. Escape only new values.
      const tags = [
        `<link rel="canonical" href="${escape(url)}" />`,
        tag("name", "theme-color", "#e03030"),
        tag("property", "og:type", route ? "article" : "website"),
        tag("property", "og:site_name", escape(config.name)),
        tag("property", "og:locale", "en_US"),
        tag("property", "og:title", title),
        tag("property", "og:description", description),
        tag("property", "og:url", escape(url)),
        tag("property", "og:image", escape(imageUrl)),
        tag("property", "og:image:type", "image/png"),
        tag("property", "og:image:width", "1200"),
        tag("property", "og:image:height", "630"),
        tag("property", "og:image:alt", escape(imageAlt)),
        tag("name", "twitter:card", "summary_large_image"),
        tag("name", "twitter:title", title),
        tag("name", "twitter:description", description),
        tag("name", "twitter:image", escape(imageUrl)),
        tag("name", "twitter:image:alt", escape(imageAlt)),
      ];
      html = html.replace(/<head>([\s\S]*?)<\/head>/, (_, head) => {
        const clean = head
          .replace(/<meta\b[^>]*(?:name|property)="(?:og:[^"]+|twitter:[^"]+|theme-color)"[^>]*>\s*/g, "")
          .replace(/<link\b[^>]*rel="canonical"[^>]*>\s*/g, "");
        return `<head>${clean}${tags.join("\n")}\n</head>`;
      });
      await writeFile(path, html);
    }
  }
}

await processDirectory("out");
