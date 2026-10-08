import assert from "node:assert/strict";
import { readFile, readdir } from "node:fs/promises";
import { join } from "node:path";

// Check the built HTML, not only the input frontmatter. Social crawlers do not
// run JavaScript, so each route must contain its own complete metadata.
async function checkDirectory(directory) {
  let count = 0;
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) {
      count += await checkDirectory(path);
    } else if (entry.name === "index.html") {
      const html = await readFile(path, "utf8");
      for (const key of [
        "description", "og:title", "og:description", "og:url",
        "og:site_name", "og:image:alt", "twitter:title",
        "twitter:description", "twitter:image:alt",
      ]) {
        assert.match(html, new RegExp(`<meta (?:name|property)="${key}" content="[^"]+"`), `${path}: missing ${key}`);
      }
      for (const key of ["og:image", "twitter:image"]) {
        assert.match(html, new RegExp(`<meta (?:name|property)="${key}" content="https://go-kit\\.dev/og-image\\.png"`), `${path}: incorrect ${key}`);
      }
      assert.match(html, /<meta name="twitter:card" content="summary_large_image"/, `${path}: incorrect card`);
      assert.match(html, /<link rel="canonical" href="https:\/\/go-kit\.dev(?:\/[^" ]*)?"/, `${path}: missing canonical URL`);
      assert.match(html, /<meta property="og:image:width" content="1200"/, `${path}: incorrect width`);
      assert.match(html, /<meta property="og:image:height" content="630"/, `${path}: incorrect height`);
      count++;
    }
  }
  return count;
}

const image = await readFile("out/og-image.png");
assert.equal(image.subarray(0, 8).toString("hex"), "89504e470d0a1a0a", "Image must be a PNG");
assert.equal(image.readUInt32BE(16), 1200);
assert.equal(image.readUInt32BE(20), 630);
const count = await checkDirectory("out");
assert.ok(count > 0, "No built pages found");
console.log(`Social metadata and image checked for ${count} pages.`);
