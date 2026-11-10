// Bundle the console into one self-contained HTML document (MCP Apps load a single ui:// resource).
import { build } from "esbuild";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";

const out = await build({ entryPoints: ["app.ts"], bundle: true, format: "iife", target: "es2020", minify: true, write: false });
const js = out.outputFiles[0].text.replaceAll("</script", "<\\/script");
const html = readFileSync("index.html", "utf8").replace("<!--SCRIPT-->", () => `<script>${js}</script>`);
mkdirSync("dist", { recursive: true });
writeFileSync("dist/index.html", html);
console.log(`dist/index.html ${(html.length / 1024).toFixed(1)} KB`);
