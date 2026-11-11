// Two outputs:
//   ../web/graph-flow.js  the plan card (React Flow), served under /static to
//                         the confirm and run pages and to the MCP App
//   dist/graph-app.html   the MCP App: a small ui:// document (bridge + loader)
//                         embedded by the Go service
import { build } from "esbuild";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { gzipSync } from "node:zlib";

const common = { bundle: true, format: "iife", target: "es2020", minify: true, write: false, legalComments: "none" };
const flow = await build({
  ...common, entryPoints: ["src/flow/main.tsx"], jsx: "automatic", loader: { ".css": "text" },
  define: { "process.env.NODE_ENV": '"production"' },
});
const lib = flow.outputFiles[0].contents;
writeFileSync("../web/graph-flow.js", lib);

const app = await build({ ...common, entryPoints: ["src/mcp-app.ts"] });
const js = app.outputFiles[0].text.replaceAll("</script", "<\\/script");
const html = readFileSync("index.html", "utf8").replace("<!--SCRIPT-->", () => `<script>${js}</script>`);
mkdirSync("dist", { recursive: true });
writeFileSync("dist/graph-app.html", html);
const kb = (n) => `${(n / 1024).toFixed(1)} KB`;
console.log(`web/graph-flow.js ${kb(lib.length)} (gzip ${kb(gzipSync(lib).length)}), dist/graph-app.html ${kb(html.length)}`);
