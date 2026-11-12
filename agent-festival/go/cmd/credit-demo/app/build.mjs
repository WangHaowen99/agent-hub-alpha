// Two outputs:
//   ../web/graph-flow.js                the plan card (React Flow) for the
//                                       service's own confirm and run pages
//   ../workbench-plugin/frontend/dist   the 编排台 page of the WorkSwarm
//                                       Application Plugin
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

// The workbench: an iframe page of the WorkSwarm Application Plugin.
const wb = await build({
  ...common, entryPoints: ["src/workbench/main.tsx"], jsx: "automatic", loader: { ".css": "text" },
  define: { "process.env.NODE_ENV": '"production"' },
});
mkdirSync("../workbench-plugin/frontend/dist", { recursive: true });
writeFileSync("../workbench-plugin/frontend/dist/app.js", wb.outputFiles[0].contents);
writeFileSync("../workbench-plugin/frontend/dist/index.html", readFileSync("workbench.html", "utf8"));

const kb = (n) => `${(n / 1024).toFixed(1)} KB`;
const page = wb.outputFiles[0].contents;
console.log(`web/graph-flow.js ${kb(lib.length)} (gzip ${kb(gzipSync(lib).length)}), workbench app.js ${kb(page.length)} (gzip ${kb(gzipSync(page).length)})`);
