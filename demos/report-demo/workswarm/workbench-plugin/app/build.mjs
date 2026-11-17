// W02 workbench build, scoped to the reused Application Plugin only.
import {build} from 'esbuild';
import {mkdirSync,readFileSync,writeFileSync} from 'node:fs';
const result=await build({entryPoints:['src/workbench/main.tsx'],bundle:true,format:'iife',target:'es2020',minify:true,write:false,legalComments:'inline',jsx:'automatic',loader:{'.css':'text'},define:{'process.env.NODE_ENV':'"production"'}});
mkdirSync('../frontend/dist',{recursive:true});
for(const file of result.outputFiles)writeFileSync('../frontend/dist/app.js',file.contents);
writeFileSync('../frontend/dist/index.html',readFileSync('workbench.html'));
