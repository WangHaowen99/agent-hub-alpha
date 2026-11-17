import { build } from 'esbuild';
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
const built = await build({stdin:{contents:`import React from 'react'; import { renderToStaticMarkup } from 'react-dom/server';
import { HumanReviewNotice } from './src/workbench/HumanReviewNotice';
export const render = review => renderToStaticMarkup(React.createElement(HumanReviewNotice,{review}));`,resolveDir:process.cwd()},
  bundle:true,write:false,format:'cjs',platform:'node',jsx:'automatic'});
const mod={exports:{}};
new Function('require','module','exports',built.outputFiles[0].text)(createRequire(import.meta.url),mod,mod.exports);
const { render } = mod.exports;
test('HUMAN links to the supplied trusted page without inline approval or credentials',()=>{
  const html=render({url:'https://control.example/ui/tasks/T-fixture',checks:[{node:'check',check_artifact:'checker',object_digest:'sha256:report'}]});
  assert.match(html,/HUMAN/); assert.match(html,/href="https:\/\/control.example\/ui\/tasks\/T-fixture"/);
  assert.match(html,/noopener noreferrer/); assert.doesNotMatch(html,/<button|<input/);
});
test('normal or resolved checks add no manual-review notice',()=>{
  assert.equal(render(undefined),''); assert.equal(render({url:'https://control.example',checks:[]}),'');
});
