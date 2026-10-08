# Social preview

`../public/og-image.png` is the shared 1200 × 630 social preview. It is committed
so production builds do not need an image renderer or system fonts.

The editable source is `og-image.svg`. To regenerate it, install ImageMagick
and the DejaVu Sans and DejaVu Sans Mono fonts, then run from `www/`:

```sh
bun run og:image
```

Each documentation page sets `ogImage: /og-image.png` in its frontmatter.
Page titles and descriptions remain specific to each route.

Tome 0.9 currently emits documentation HTML without social metadata and with
relative canonical links. `write-meta.mjs` corrects the static HTML after the
Tome build. It uses the site's `baseUrl`, each page's generated title and
description, and the shared PNG. `check-meta.mjs` checks all built routes and
the PNG dimensions. Both scripts run as part of `bun run build`.

After a Tome upgrade, check the generated HTML before removing the correction
step. Keep the metadata check to detect missing tags or broken image URLs.
