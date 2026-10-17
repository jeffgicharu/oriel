// Builds the browser side of Oriel into internal/assets/dist/assets, where the
// Go server embeds it and the release ships it as plain files.
//
// Every file name carries a fingerprint of its contents, and manifest.json maps
// the name a template asks for ("js/site.js", "img/hero.avif") to the
// fingerprinted path the browser loads.

import * as esbuild from "esbuild";
import { createHash } from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.dirname(fileURLToPath(import.meta.url));
const out = path.resolve(root, "../internal/assets/dist/assets");

const entryPoints = {
  "js/site": "src/ts/site.ts",
  "js/desk": "src/ts/desk/index.ts",
  "css/site": "src/css/site.css",
  "css/desk": "src/css/desk.css",
};

await fs.rm(out, { recursive: true, force: true });
await fs.mkdir(out, { recursive: true });

const result = await esbuild.build({
  absWorkingDir: root,
  entryPoints,
  bundle: true,
  splitting: true,
  format: "esm",
  minify: true,
  sourcemap: false,
  legalComments: "none",
  target: ["es2022", "chrome111", "edge111", "firefox114", "safari16.4"],
  outdir: out,
  entryNames: "[dir]/[name]-[hash]",
  chunkNames: "js/chunks/[name]-[hash]",
  assetNames: "media/[name]-[hash]",
  publicPath: "/assets",
  loader: { ".woff2": "file", ".svg": "file", ".png": "file", ".jpg": "file", ".webp": "file", ".avif": "file" },
  metafile: true,
  logLevel: "warning",
});

const manifest = {};
const publicPath = (file) => "/assets/" + path.relative(out, path.resolve(root, file)).split(path.sep).join("/");

for (const [file, info] of Object.entries(result.metafile.outputs)) {
  if (info.entryPoint) {
    const name = Object.entries(entryPoints).find(([, src]) => src === info.entryPoint)?.[0];
    if (name) manifest[name + path.extname(file)] = publicPath(file);
    continue;
  }
  // Files pulled in by CSS (fonts) are listed by their source name, so a
  // template can preload them.
  const inputs = Object.keys(info.inputs);
  if (inputs.length === 1 && inputs[0].startsWith("static/")) {
    manifest[inputs[0].slice("static/".length)] = publicPath(file);
  }
}

// Pictures and icons are copied as they are, under fingerprinted names.
async function copyStatic(dir) {
  const from = path.join(root, "static", dir);
  let entries;
  try {
    entries = await fs.readdir(from, { recursive: true, withFileTypes: true });
  } catch {
    return;
  }
  for (const entry of entries) {
    if (!entry.isFile() || entry.name.startsWith(".")) continue;
    const source = path.join(entry.parentPath, entry.name);
    const rel = path.relative(path.join(root, "static"), source).split(path.sep).join("/");
    const body = await fs.readFile(source);
    const hash = createHash("sha256").update(body).digest("base64url").slice(0, 10);
    const ext = path.extname(rel);
    const target = rel.slice(0, -ext.length) + "-" + hash + ext;
    await fs.mkdir(path.dirname(path.join(out, target)), { recursive: true });
    await fs.writeFile(path.join(out, target), body);
    manifest[rel] = "/assets/" + target;
  }
}
await copyStatic("img");
await copyStatic("icons");

const sorted = Object.fromEntries(Object.entries(manifest).sort(([a], [b]) => a.localeCompare(b)));
await fs.writeFile(path.join(out, "manifest.json"), JSON.stringify(sorted, null, 2) + "\n");

let total = 0;
for (const info of Object.values(result.metafile.outputs)) total += info.bytes;
console.log(`built ${Object.keys(sorted).length} named assets, ${(total / 1024).toFixed(0)} KB of scripts, styles and fonts`);
