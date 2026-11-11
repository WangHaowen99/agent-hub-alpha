// Two bundles of the same renderer:
//   dist/graph-app.html  the MCP App (one self-contained ui:// document, embedded by the Go service)
//   ../web/graph.js      the confirm and run pages
import { build } from "esbuild";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";

const common = { bundle: true, format: "iife", target: "es2020", minify: true, write: false };
const app = await build({ ...common, entryPoints: ["src/mcp-app.ts"] });
const js = app.outputFiles[0].text.replaceAll("</script", "<\\/script");
const html = readFileSync("index.html", "utf8").replace("<!--SCRIPT-->", () => `<script>${js}</script>`);
mkdirSync("dist", { recursive: true });
writeFileSync("dist/graph-app.html", html);
const page = await build({ ...common, entryPoints: ["src/page.ts"] });
writeFileSync("../web/graph.js", page.outputFiles[0].text);
console.log(`dist/graph-app.html ${(html.length / 1024).toFixed(1)} KB, web/graph.js ${(page.outputFiles[0].text.length / 1024).toFixed(1)} KB`);
