"use strict";(()=>{var C=String.raw`
@property --cg-a { syntax: '<angle>'; inherits: false; initial-value: 0deg; }
.cg {
  --ink: #1f2328; --mute: #6b7280; --faint: #a3a9b3; --line: #e7e9ee; --bg: #fff;
  --blue: #2563eb; --green: #16a34a; --amber: #d97706; --red: #dc2626; --violet: #7c3aed; --teal: #0d9488; --rose: #e11d48;
  font: 13px/1.45 -apple-system, BlinkMacSystemFont, "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
  color: var(--ink); background: var(--bg); border: 1px solid var(--line); border-radius: 14px; overflow: hidden;
  position: relative; -webkit-font-smoothing: antialiased;
}
.cg * { box-sizing: border-box; }

/* header */
.cg-head { display: flex; align-items: center; gap: 12px; padding: 12px 16px 11px; position: relative; }
.cg-title { flex: 1; min-width: 0; display: flex; align-items: baseline; gap: 8px; }
.cg-goal { font-weight: 600; font-size: 14px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.cg-ver { color: var(--faint); font-size: 12px; white-space: nowrap; font-variant-numeric: tabular-nums; }
.cg-meta { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }
.cg-now { display: inline-flex; align-items: center; gap: 6px; color: var(--blue); font-size: 12px; white-space: nowrap; overflow: hidden;
  text-overflow: ellipsis; min-width: 0; animation: cg-slide .35s cubic-bezier(.2,.8,.2,1) both; }
.cg-now i { flex: 0 0 6px; height: 6px; border-radius: 50%; background: var(--blue); animation: cg-blink 1s ease-in-out infinite; }
.cg-chip { display: inline-flex; align-items: center; gap: 6px; height: 24px; padding: 0 10px; border-radius: 12px;
  font-size: 12px; font-weight: 500; white-space: nowrap; transition: background .3s, color .3s; }
.cg-chip i { width: 6px; height: 6px; border-radius: 50%; background: currentColor; }
.cg-chip.ok { background: #e9f7ee; color: var(--green); }
.cg-chip.bad { background: #fdecec; color: var(--red); }
.cg-chip.wait { background: #fff4e0; color: var(--amber); }
.cg-chip.run { background: #e8f0fe; color: var(--blue); }
.cg-chip.run i { animation: cg-blink 1s ease-in-out infinite; }
.cg-chip.mute { background: #f2f3f5; color: var(--mute); }
.cg-chip.soft { background: #f2f3f5; color: var(--mute); font-weight: 400; }
.cg-btn { height: 26px; padding: 0 12px; border-radius: 13px; border: 0; background: var(--ink); color: #fff; font: inherit;
  font-size: 12px; font-weight: 500; cursor: pointer; transition: transform .15s, background .2s; }
.cg-btn:hover { background: #000; transform: translateY(-1px); }
.cg-bar { position: absolute; left: 0; bottom: 0; height: 2px; width: 100%; background: transparent; }
.cg-bar b { display: block; height: 100%; width: 0; background: linear-gradient(90deg, #60a5fa, var(--blue));
  transition: width .6s cubic-bezier(.2,.8,.2,1), background .4s; }
.cg-bar.ok b { background: linear-gradient(90deg, #4ade80, var(--green)); }

/* approval in the card */
.cg-gate { display: flex; align-items: center; gap: 10px; padding: 10px 16px; border-top: 1px solid var(--line); background: #fffaf0;
  animation: cg-gate-in .28s cubic-bezier(.2,.8,.2,1) both; overflow: hidden; }
.cg-gate.leave { animation: cg-gate-out .26s ease both; }
.cg-gate.shake { animation: cg-shake .45s cubic-bezier(.36,.07,.19,.97) both; }
.cg-gate .who { font-size: 12.5px; color: var(--mute); white-space: nowrap; }
.cg-gate .who b { color: var(--ink); font-weight: 600; }
.cg-gate input { height: 28px; width: 150px; padding: 0 10px; border: 1px solid #e3e6eb; border-radius: 8px; font: inherit; font-size: 12.5px;
  outline: none; transition: border-color .2s, box-shadow .2s; background: #fff; }
.cg-gate input:focus { border-color: #93b4f5; box-shadow: 0 0 0 3px rgba(37,99,235,.12); }
.cg-gate .msg { color: var(--red); font-size: 12px; white-space: nowrap; }
.cg-gate .hash { margin-left: auto; color: var(--faint); font: 11px ui-monospace, SFMono-Regular, Menlo, monospace; white-space: nowrap; }
.cg-gate .ok { display: inline-flex; align-items: center; gap: 6px; color: var(--green); font-weight: 600; font-size: 12.5px;
  animation: cg-pop .4s cubic-bezier(.2,1.6,.4,1) both; }
.cg-gate .ok svg { width: 14px; height: 14px; }
.cg-gate.done { background: #f2fbf5; }
.cg-btn.busy { opacity: .6; pointer-events: none; }

/* canvas */
.cg-canvas { position: relative; overflow: hidden; border-top: 1px solid var(--line); transition: height .45s cubic-bezier(.2,.8,.2,1);
  background-color: #fafbfc; background-image: radial-gradient(#e2e5ea 1px, transparent 1.3px); background-size: 16px 16px; }
.cg-stage { position: absolute; left: 0; top: 0; transform-origin: 0 0; }
.cg-edges { position: absolute; left: 0; top: 0; overflow: visible; pointer-events: none; }
.cg-edges path { fill: none; stroke-width: 1.6; stroke-linecap: round; transition: stroke .4s, opacity .4s; }
.cg-edges .base { stroke: #cfd4dc; stroke-dasharray: 1; stroke-dashoffset: 1; transition: stroke-dashoffset .6s cubic-bezier(.4,0,.2,1), stroke .4s; }
.cg-edges .base.drawn { stroke-dashoffset: 0; }
.cg-edges .flow { stroke: var(--blue); stroke-width: 2; stroke-dasharray: 3 9; opacity: 0; animation: cg-flow .7s linear infinite; }
.cg-edges g.e-active .base { stroke: #93b4f5; }
.cg-edges g.e-active .flow { opacity: 1; }
.cg-edges g.e-done .base { stroke: #8fd1a6; }
.cg-edges g.e-skip .base { stroke: #dcdfe4; stroke-dasharray: .015 .015; }
.cg-edges g.e-bad .base { stroke: #f2a7a7; }

/* nodes */
.cg-slot { position: absolute; left: 0; top: 0; will-change: transform; }
.cg-node { position: relative; width: 184px; padding: 10px 11px 10px 10px; display: grid; grid-template-columns: 30px minmax(0, 1fr); column-gap: 9px;
  background: #fff; border: 1px solid #e3e6eb; border-radius: 12px; cursor: pointer;
  box-shadow: 0 1px 2px rgba(16,24,40,.04); transition: border-color .35s, background .35s, opacity .4s, filter .4s, box-shadow .2s; }
.cg-node:hover { box-shadow: 0 4px 14px rgba(16,24,40,.08); }
.cg-node.enter { animation: cg-in .5s cubic-bezier(.2,.8,.2,1) both; }
.cg-av { width: 30px; height: 30px; border-radius: 9px; display: grid; place-items: center; font-weight: 600; font-size: 14px;
  color: var(--c); background: var(--cbg); transition: filter .4s; }
.k-investigate { --c: var(--blue); --cbg: #e9f0ff; }
.k-blackbox { --c: var(--violet); --cbg: #f1eaff; }
.k-judge { --c: var(--amber); --cbg: #fff2dd; }
.k-write { --c: var(--teal); --cbg: #e3f5f2; }
.k-effect { --c: var(--rose); --cbg: #ffe9ee; }
.cg-body { min-width: 0; flex: 1; }
.cg-name { font-weight: 600; font-size: 13px; line-height: 18px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.cg-task { color: var(--mute); font-size: 11.5px; line-height: 16px; margin-top: 2px; display: -webkit-box;
  -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.cg-tags { position: absolute; top: -9px; left: 48px; display: flex; gap: 4px; }
.cg-tag { height: 18px; padding: 0 7px; border-radius: 9px; font-size: 11px; line-height: 16px; font-weight: 500;
  background: #fff; border: 1px solid currentColor; white-space: nowrap; }
.cg-tag.bb { color: var(--violet); }
.cg-tag.irrev { color: var(--rose); }
.cg-tag.deny { color: #fff; background: var(--red); border-color: var(--red); }
.cg-tag.reuse { color: var(--blue); }
.cg-tag.skip { color: var(--faint); }
.cg-tag.pop { animation: cg-pop .45s cubic-bezier(.2,1.6,.4,1) both; }
.cg-ok { position: absolute; top: -8px; right: -8px; width: 20px; height: 20px; border-radius: 50%; background: var(--green);
  color: #fff; display: grid; place-items: center; box-shadow: 0 0 0 3px #fff; animation: cg-pop .5s cubic-bezier(.2,1.6,.4,1) both; }
.cg-ok svg { width: 11px; height: 11px; }
.cg-ok.reused { background: var(--blue); }
.cg-ok.fail { background: var(--red); }
.cg-act { margin-top: 6px; display: flex; align-items: center; gap: 6px; font-size: 11px; line-height: 16px; color: var(--blue); min-width: 0; }
.cg-act i { flex: 0 0 6px; height: 6px; border-radius: 50%; background: var(--blue); animation: cg-blink 1s ease-in-out infinite; }
.cg-act span { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; animation: cg-slide .35s cubic-bezier(.2,.8,.2,1) both; }
.m-stalled .cg-act { color: var(--faint); }
.m-stalled .cg-act i { animation-play-state: paused; background: var(--faint); }
.cg-res { margin-top: 7px; display: inline-flex; align-items: center; height: 22px; padding: 0 9px; border-radius: 11px;
  font-size: 12px; font-weight: 600; animation: cg-pop .5s cubic-bezier(.2,1.6,.4,1) both; }
.cg-res.ok { background: #e9f7ee; color: var(--green); }
.cg-res.bad { background: #fdecec; color: var(--red); }
.cg-res.mute { background: #f2f3f5; color: var(--mute); }

/* node states */
.m-run .cg-node.s-pending { opacity: .5; }
.m-run .cg-node.s-pending .cg-av { filter: grayscale(1); }
.cg-node.s-running { border-color: transparent; }
.cg-node.s-running::before { content: ""; position: absolute; inset: -1.5px; border-radius: 13px; padding: 1.5px; pointer-events: none;
  background: conic-gradient(from var(--cg-a), rgba(37,99,235,0) 0 55%, #93c5fd 72%, #2563eb 88%, rgba(37,99,235,0) 100%);
  -webkit-mask: linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0); -webkit-mask-composite: xor; mask-composite: exclude;
  animation: cg-spin 1.4s linear infinite; }
.cg-node.s-running .cg-av { animation: cg-breathe 1.4s ease-in-out infinite; }
.m-stalled .cg-node.s-running::before { animation-play-state: paused; filter: grayscale(1); opacity: .5; }
.m-stalled .cg-node.s-running .cg-av, .m-stalled .cg-br em.started { animation-play-state: paused; }
.m-stalled .cg-edges .flow { animation-play-state: paused; opacity: .35; }
.cg-node.s-done { border-color: #bfe5cb; }
.cg-node.s-reused { border-color: #bcd3fb; }
.cg-node.s-skipped { border-style: dashed; opacity: .45; }
.cg-node.s-failed, .cg-node.denied { border-color: #f3a5a5; background: #fffafa; }
.cg-node.shake { animation: cg-shake .5s cubic-bezier(.36,.07,.19,.97) both; }

/* the black box: closed before it runs, opens as branches fork */
.cg-node.k-blackbox { border-style: dashed; border-color: #c9b5f5; }
.cg-node.k-blackbox.s-done, .cg-node.k-blackbox.s-reused { border-style: solid; }
.cg-bb { grid-column: 1 / -1; margin-top: 8px; border-radius: 8px; background: #f8f5ff; padding: 6px 8px; overflow: hidden; }
.cg-bb.closed { display: flex; align-items: center; gap: 5px; color: var(--violet); font-size: 11px; }
.cg-bb.closed i { width: 4px; height: 4px; border-radius: 50%; background: var(--violet); opacity: .3; animation: cg-dots 1.2s infinite; }
.cg-bb.closed i:nth-child(2) { animation-delay: .15s; } .cg-bb.closed i:nth-child(3) { animation-delay: .3s; }
.cg-bb.closed span { margin-left: 3px; }
.cg-br { display: flex; align-items: center; gap: 6px; font-size: 11px; line-height: 18px; color: #4b5563; white-space: nowrap; }
.cg-br span { overflow: hidden; text-overflow: ellipsis; }
.cg-br.enter { animation: cg-slide .45s cubic-bezier(.2,.8,.2,1) both; }
.cg-br em { flex: 0 0 8px; height: 8px; border-radius: 50%; border: 1.5px solid var(--violet); }
.cg-br em.started { border-color: #c4b5fd; border-top-color: var(--violet); animation: cg-rot .8s linear infinite; }
.cg-br em.finished { background: var(--violet); }
.cg-br em.interrupted { border-color: var(--faint); border-style: dashed; }
.cg-br.more { color: var(--faint); padding-left: 14px; }

/* condition label on an edge */
.cg-cond { position: absolute; transform: translate(-50%, -50%); height: 20px; padding: 0 8px; border-radius: 10px;
  background: #fff; border: 1px solid #e3e6eb; font-size: 11px; line-height: 18px; color: var(--mute); white-space: nowrap;
  transition: color .3s, border-color .3s, background .3s, opacity .4s; }
.cg-cond.on { color: var(--green); border-color: #a7dcb9; background: #f2fbf5; }
.cg-cond.off { opacity: .55; text-decoration: line-through; }

/* footer: precheck problems, verification */
.cg-foot:empty { display: none; }
.cg-foot { border-top: 1px solid var(--line); padding: 10px 16px 12px; }
.cg-prob { display: flex; gap: 8px; align-items: baseline; font-size: 12.5px; padding: 3px 0; animation: cg-fade .4s both; }
.cg-prob::before { content: ""; flex: 0 0 6px; height: 6px; border-radius: 50%; background: var(--red); transform: translateY(-1px); }
.cg-checks { display: grid; grid-template-columns: 1fr 1fr; gap: 4px 18px; }
.cg-checks + .cg-prob { margin-top: 8px; }
.cg-check { display: flex; gap: 8px; align-items: center; font-size: 12.5px; min-width: 0; animation: cg-slide .45s cubic-bezier(.2,.8,.2,1) both; }
.cg-check b { flex: 0 0 18px; height: 18px; border-radius: 50%; display: grid; place-items: center; color: #fff; background: var(--green); }
.cg-check b svg { width: 10px; height: 10px; }
.cg-check.no b { background: var(--red); }
.cg-check span { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

/* detail popover */
.cg-pop { position: absolute; z-index: 5; width: 300px; background: #fff; border: 1px solid #e3e6eb; border-radius: 12px; padding: 12px 14px;
  box-shadow: 0 12px 32px rgba(16,24,40,.14); font-size: 12px; animation: cg-popin .22s cubic-bezier(.2,.8,.2,1) both; }
.cg-pop h4 { margin: 0 0 2px; font-size: 13.5px; }
.cg-pop .role { color: var(--mute); margin-bottom: 8px; }
.cg-pop p { margin: 0 0 8px; color: #374151; }
.cg-pop .k { color: var(--faint); font-size: 11px; margin: 8px 0 3px; }
.cg-pop code { font: 11px ui-monospace, SFMono-Regular, Menlo, monospace; background: #f3f4f6; border-radius: 4px; padding: 1px 5px;
  display: inline-block; margin: 0 4px 3px 0; }
.cg-pop .out { white-space: pre-wrap; background: #f8f9fb; border-radius: 8px; padding: 7px 9px; max-height: 120px; overflow: auto; color: #374151; }

@keyframes cg-in { from { opacity: 0; transform: translateY(10px) scale(.96); } to { opacity: 1; transform: none; } }
@keyframes cg-gate-in { from { opacity: 0; max-height: 0; padding-top: 0; padding-bottom: 0; } to { opacity: 1; max-height: 60px; } }
@keyframes cg-gate-out { from { opacity: 1; max-height: 60px; } to { opacity: 0; max-height: 0; padding-top: 0; padding-bottom: 0; } }
@keyframes cg-pop { 0% { transform: scale(0); opacity: 0; } 100% { transform: scale(1); opacity: 1; } }
@keyframes cg-popin { from { opacity: 0; transform: translateY(-4px) scale(.98); } to { opacity: 1; transform: none; } }
@keyframes cg-fade { from { opacity: 0; } to { opacity: 1; } }
@keyframes cg-slide { from { opacity: 0; transform: translateX(-8px); } to { opacity: 1; transform: none; } }
@keyframes cg-spin { to { --cg-a: 360deg; } }
@keyframes cg-rot { to { transform: rotate(360deg); } }
@keyframes cg-flow { to { stroke-dashoffset: -12; } }
@keyframes cg-blink { 50% { opacity: .3; } }
@keyframes cg-breathe { 50% { transform: scale(1.08); } }
@keyframes cg-dots { 0%, 100% { opacity: .25; transform: translateY(0); } 40% { opacity: 1; transform: translateY(-2px); } }
@keyframes cg-shake { 10%, 90% { margin-left: -1px; } 20%, 80% { margin-left: 2px; } 30%, 50%, 70% { margin-left: -4px; } 40%, 60% { margin-left: 4px; } }
@media (prefers-reduced-motion: reduce) { .cg *, .cg *::before { animation: none !important; transition: none !important; } }
`;var T=184,N=64,D=18,M=22,G=5,j=520,L='<svg viewBox="0 0 12 12"><path d="M2.5 6.3l2.3 2.3 4.7-5" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>',V='<svg viewBox="0 0 12 12"><path d="M3 3l6 6M9 3l-6 6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>',A="http://www.w3.org/2000/svg",R=["idle","active","done","skip","bad"],F={idle:"#cfd4dc",active:"#93b4f5",done:"#8fd1a6",skip:"#dcdfe4",bad:"#f2a7a7"},h=p=>p.replace(/[&<>"]/g,e=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;"})[e]),E=p=>p==="done"||p==="reused",O=p=>1-Math.pow(1-p,3),B=0;function I(p){if(p.getElementById("cg-style"))return;let e=p.createElement("style");e.id="cg-style",e.textContent=C,p.head.appendChild(e)}function W(p){if(!p)return;let e=/^grant-(\d+)$/.exec(p);return e?["ok",`\u540C\u610F ${Number(e[1]).toLocaleString("en-US")}`]:p==="deny"?["bad","\u62D2\u7EDD"]:p==="on_missing_evidence"?["mute","\u8BC1\u636E\u4E0D\u8DB3"]:p==="on_undefined_rule"?["mute","\u89C4\u5219\u672A\u8986\u76D6"]:["mute",p]}function Y(p){switch(p){case"approved":return["ok","\u94F6\u884C\u5DF2\u6279\u51C6"];case"declined":return["bad","\u94F6\u884C\u62D2\u7EDD"];case"cancelled":return["mute","\u5DF2\u64A4\u9500"];case"unknown":return["mute","\u7ED3\u679C\u672A\u77E5 \xB7 \u67E5\u8BE2\u4E2D"]}}var _=p=>p.replace(/^[A-Za-z_]+\s*[:：]\s*/,"");function J(p){return!p.negate&&/GRANT/.test(p.contains)?"\u540C\u610F\u65F6":(p.negate?"\u4E0D\u542B ":"\u542B ")+p.contains}var S=class{constructor(e,s={}){this.id=`cg${++B}`;this.nodes=new Map;this.edges=new Map;this.first=!0;this.headSig="";this.footSig="";this.frame=0;this.pending=0;this.sizes=new ResizeObserver(()=>{cancelAnimationFrame(this.pending),this.pending=requestAnimationFrame(()=>{[...this.nodes.values()].some(e=>e.el.offsetHeight!==e.h)&&this.layout(!0)})});this.opts=s,I(e.ownerDocument),this.el=document.createElement("div"),this.el.className="cg",this.el.innerHTML=`<div class="cg-head"></div><div class="cg-canvas"><div class="cg-stage"><svg class="cg-edges"><defs>${R.map(t=>`<marker id="${this.id}-ar-${t}" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M1 1.5L8.5 5L1 8.5" fill="none" stroke="${F[t]}" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></marker>`).join("")}</defs></svg></div></div><div class="cg-foot"></div>`,e.replaceChildren(this.el),this.head=this.el.querySelector(".cg-head"),this.canvas=this.el.querySelector(".cg-canvas"),this.stage=this.el.querySelector(".cg-stage"),this.svg=this.el.querySelector("svg"),this.foot=this.el.querySelector(".cg-foot");let a=0;new ResizeObserver(()=>{this.g&&this.canvas.clientWidth!==a&&(a=this.canvas.clientWidth,this.layout(!1))}).observe(this.canvas),this.el.addEventListener("click",t=>{let i=t.target.closest(".cg-slot");i&&!t.target.closest(".cg-pop")?this.togglePop(i.dataset.id):t.target.closest(".cg-pop")||this.closePop()}),e.ownerDocument.addEventListener("keydown",t=>{t.key==="Escape"&&this.closePop()})}current(){return this.g}render(e){this.g=e,this.el.classList.toggle("m-run",!!e.run),this.el.classList.toggle("m-plan",!e.run),this.el.classList.toggle("m-stalled",!!e.run?.stalled),this.renderHead(e);let s=new Map;e.layers.forEach((t,i)=>t.forEach(d=>s.set(d,i)));let a=new Set;e.nodes.forEach((t,i)=>{a.add(t.id);let d=this.nodes.get(t.id);if(!d){let n=document.createElement("div");n.className="cg-slot",n.dataset.id=t.id;let o=document.createElement("div");n.appendChild(o),this.stage.appendChild(n),this.sizes.observe(o),d={slot:n,el:o,sig:"",branches:0,x:0,y:0,fx:0,fy:0,tx:0,ty:0,h:0,placed:!1},this.nodes.set(t.id,d);let r=this.first?(s.get(t.id)??0)*110+i*25:0;o.style.setProperty("--d",`${r}ms`),o.style.animationDelay=`${r}ms`,o.dataset.enter="1"}this.renderNode(d,t,e)});for(let[t,i]of this.nodes)a.has(t)||(this.sizes.unobserve(i.el),i.slot.remove(),this.nodes.delete(t));this.renderEdges(e,s),this.renderFoot(e),this.layout(!this.first),this.pop&&this.togglePop(this.pop.dataset.id,!0),this.first=!1}renderHead(e){let s=e.nodes.length,a=e.nodes.filter(g=>E(g.status)||g.status==="skipped").length,t=e.nodes.filter(g=>g.status==="reused").length,i;if(e.verify){let g=e.verify.checks.filter(m=>m.ok).length;i=e.verify.ok?["ok",`\u9A8C\u8BC1\u901A\u8FC7 ${g}/${e.verify.checks.length}`]:["bad",`\u9A8C\u8BC1\u672A\u901A\u8FC7 ${g}/${e.verify.checks.length}`]}else e.run?(i={queued:["wait","\u6392\u961F\u4E2D"],running:["run",`\u6267\u884C\u4E2D ${a}/${s}`],done:["ok","\u5DF2\u5B8C\u6210"],failed:["bad","\u6267\u884C\u5931\u8D25"]}[e.run.status]??["mute",e.run.status],e.run.stalled&&(i=["bad","\u6267\u884C\u5668\u65E0\u54CD\u5E94 \xB7 \u7B49\u5F85\u63A5\u7BA1"])):e.ok?e.superseded?i=["mute",`\u5DF2\u66F4\u65B0\u4E3A v${e.superseded}`]:i=["wait","\u5F85\u6279\u51C6"]:i=["bad","\u9884\u68C0\u672A\u901A\u8FC7"];let d=e.run&&e.run.leases>1?`<span class="cg-chip soft">\u21BB \u6267\u884C\u5668\u91CD\u542F \xB7 \u590D\u7528 ${t} \u6B65</span>`:"",n=!!e.approve&&!!this.opts.approve&&!this.opts.page,o=n?'<button class="cg-btn" data-approve>\u6279\u51C6</button>':"",r=e.run?.status==="running"&&!e.run.stalled?e.nodes.filter(g=>g.status==="running").map(g=>g.name):[],f=r.length?`<span class="cg-now"><i></i>\u6B63\u5728\uFF1A${h(r.join("\u3001"))}</span>`:`<span class="cg-ver">${h(e.case)} \xB7 ${h(e.plan)} v${e.version}</span>`,c=[e.goal,f,i.join(),d,o].join("|");c!==this.headSig&&(this.headSig=c,this.head.innerHTML=`<div class="cg-title"><span class="cg-goal">${h(e.goal||`${e.case} \u6388\u4FE1\u5C3D\u8C03`)}</span>${f}</div>
        <div class="cg-meta">${d}<span class="cg-chip ${i[0]}"><i></i>${h(i[1])}</span>${o}</div><div class="cg-bar"><b></b></div>`,this.head.querySelector("[data-approve]")?.addEventListener("click",g=>{g.stopPropagation(),this.toggleGate()})),n||this.closeGate();let u=this.head.querySelector(".cg-bar");u.classList.toggle("ok",e.run?.status==="done"),requestAnimationFrame(()=>{u.firstElementChild.style.width=e.run?`${a/Math.max(s,1)*100}%`:"0"})}renderNode(e,s,a){let t=s.status??"plan",i=!a.ok&&!a.run&&!!s.denied,d=s.kind==="judge",n=s.kind==="effect",o=d&&E(t)?W(a.run?.decision):n&&E(t)?Y(a.run?.receipt):void 0,r=s.branches??[],f=t==="running"&&s.kind!=="blackbox"?s.activity??"":"",c=JSON.stringify([s.name,s.task,t,i,o,r.map(x=>x.status+x.goal),f]);if(e.el.className=`cg-node k-${s.kind} s-${t}${i?" denied":""}${e.el.dataset.enter?" enter":""}`,c===e.sig)return;let u=e.sig!==""&&JSON.parse(e.sig)[2]!==t,g=e.sig===""?!1:JSON.parse(e.sig)[3];if(e.sig=c,e.el.dataset.enter&&!e.el.dataset.listening){e.el.dataset.listening="1";let x=b=>{b.target===e.el&&(e.el.removeEventListener("animationend",x),delete e.el.dataset.enter,e.el.classList.remove("enter"),e.el.style.animationDelay="")};e.el.addEventListener("animationend",x)}let m=u||!this.first?"0ms":void 0;m&&e.el.style.setProperty("--d",m);let v=[];s.kind==="blackbox"&&v.push('<span class="cg-tag bb">\u9ED1\u76D2</span>'),s.effect==="irreversible"&&v.push('<span class="cg-tag irrev">\u4E0D\u53EF\u9006</span>'),i&&v.push('<span class="cg-tag deny pop">\u65E0\u6743\u9650</span>'),t==="reused"&&v.push('<span class="cg-tag reuse pop">\u590D\u7528</span>'),t==="skipped"&&v.push('<span class="cg-tag skip">\u672A\u6267\u884C</span>');let $=t==="done"?`<span class="cg-ok">${L}</span>`:t==="reused"?`<span class="cg-ok reused">${L}</span>`:t==="failed"?`<span class="cg-ok fail">${V}</span>`:"",l="";s.kind==="blackbox"&&(r.length?l=`<div class="cg-bb">${r.slice(0,G).map((b,y)=>`<div class="cg-br${y>=e.branches?" enter":""}" style="animation-delay:${Math.max(0,y-e.branches)*90}ms" title="${h(b.goal)}"><em class="${h(b.status)}"></em><span>${h(_(b.goal))}</span></div>`).join("")}${r.length>G?`<div class="cg-br more">\u53E6 ${r.length-G} \u4E2A\u5206\u652F</div>`:""}</div>`:l=`<div class="cg-bb closed"><i></i><i></i><i></i><span>${t==="running"?"\u6B63\u5728\u62C6\u5206":"\u8FD0\u884C\u65F6\u5C55\u5F00"}</span></div>`,e.branches=r.length);let k=(o?`<div class="cg-res ${o[0]}">${h(o[1])}</div>`:"")+(f?`<div class="cg-act" title="${h(f)}"><i></i><span>${h(f)}</span></div>`:"");e.el.innerHTML=`<div class="cg-av">${h([...s.name][0]??"?")}</div><div class="cg-body"><div class="cg-name">${h(s.name)}</div><div class="cg-task">${h(s.task)}</div>${k}</div>${l}${v.length?`<div class="cg-tags">${v.join("")}</div>`:""}${$}`,e.el.querySelectorAll(".cg-ok, .cg-res, .cg-tag.pop").forEach(x=>{x.style.animationDelay="calc(var(--d, 0ms) + 280ms)"}),i&&!g&&(e.el.classList.remove("shake"),setTimeout(()=>e.el.classList.add("shake"),this.first?900:50))}renderEdges(e,s){let a=new Map(e.nodes.map(n=>[n.id,n])),t=new Map,i=n=>{let o=t.get(n);if(o)return o;o=new Set,t.set(n,o);for(let r of a.get(n)?.deps??[])o.add(r),i(r).forEach(f=>o.add(f));return o},d=new Set;for(let n of e.nodes){let o=(n.deps??[]).filter(r=>a.has(r));for(let r of o){if(n.when?.step!==r&&o.some(m=>m!==r&&i(m).has(r)))continue;let f=`${r}>${n.id}`;d.add(f);let c=this.edges.get(f);if(!c){let m=document.createElementNS(A,"g"),v=document.createElementNS(A,"path");v.setAttribute("class","base"),v.setAttribute("pathLength","1");let $=document.createElementNS(A,"path");$.setAttribute("class","flow"),m.append(v,$),this.svg.appendChild(m),c={from:r,to:n.id,g:m,base:v,flow:$,drawn:!1},this.edges.set(f,c);let l=this.first?260+(s.get(r)??0)*120:0;v.style.transitionDelay=`${l}ms, 0ms`}n.when&&n.when.step===r&&!c.cond&&(c.cond=document.createElement("div"),c.cond.className="cg-cond",c.cond.textContent=J(n.when),this.stage.appendChild(c.cond));let u=a.get(r),g="idle";if(e.run?n.status==="skipped"?g="skip":E(u.status)&&n.status==="running"?g="active":E(u.status)&&E(n.status)&&(g="done"):!e.ok&&n.denied&&(g="bad"),c.g.setAttribute("class",`e-${g}`),c.base.setAttribute("marker-end",`url(#${this.id}-ar-${g})`),c.cond){let m=e.run?.decision;c.cond.classList.toggle("on",!!m&&/^grant/.test(m)&&!n.when?.negate),c.cond.classList.toggle("off",n.status==="skipped")}}}for(let[n,o]of this.edges)d.has(n)||(o.g.remove(),o.cond?.remove(),this.edges.delete(n))}drawEdges(){let e=new Map,s=new Map;for(let t of this.edges.values())(e.get(t.from)??e.set(t.from,[]).get(t.from)).push(t),(s.get(t.to)??s.set(t.to,[]).get(t.to)).push(t);let a=(t,i,d,n)=>{let o=[...t].sort((u,g)=>(n(u)?.y??0)-(n(g)?.y??0)),r=o.length,f=o.indexOf(i),c=r>1?Math.min(9,d.h*.6/(r-1)):0;return d.y+d.h/2+(f-(r-1)/2)*c};for(let t of this.edges.values()){let i=this.nodes.get(t.from),d=this.nodes.get(t.to);if(!i||!d)continue;let n=i.x+T,o=d.x-3,r=a(e.get(t.from),t,i,g=>this.nodes.get(g.to)),f=a(s.get(t.to),t,d,g=>this.nodes.get(g.from)),c=Math.max(26,(o-n)*.5),u=`M${n},${r} C${n+c},${r} ${o-c},${f} ${o},${f}`;t.base.setAttribute("d",u),t.flow.setAttribute("d",u),t.cond&&(t.cond.style.left=`${(n+o)/2}px`,t.cond.style.top=`${(r+f)/2}px`),t.drawn||(t.drawn=!0,requestAnimationFrame(()=>requestAnimationFrame(()=>t.base.classList.add("drawn"))))}}layout(e){let s=this.g,a=new Map(s.nodes.map(l=>[l.id,l])),t=s.layers.map(l=>l.filter(k=>a.has(k))).filter(l=>l.length),i=new Set(t.flat()),d=s.nodes.map(l=>l.id).filter(l=>!i.has(l));d.length&&t.push(d);for(let l of this.nodes.values())l.h=l.el.offsetHeight;let n=t.map(l=>l.reduce((k,x)=>k+this.nodes.get(x).h,0)+(l.length-1)*D),o=Math.max(...n,60),r=new Map;t.forEach((l,k)=>{let x=l.map((y,w)=>{let z=(a.get(y).deps??[]).filter(H=>r.has(H));return{id:y,i:w,bc:z.length?z.reduce((H,P)=>H+r.get(P),0)/z.length:1/0}});k>0&&x.sort((y,w)=>y.bc===w.bc?y.i-w.i:y.bc-w.bc);let b=M+(o-n[k])/2;for(let{id:y}of x){let w=this.nodes.get(y);w.tx=M+k*(T+N),w.ty=b,r.set(y,b+w.h/2),b+=w.h+D}});let f=M*2+t.length*T+(t.length-1)*N,c=o+M*2,u=this.canvas.clientWidth||f,g=Math.min(1,u/f),m=Math.max(0,(u-f*g)/2);this.stage.style.width=`${f}px`,this.stage.style.height=`${c}px`,this.svg.setAttribute("width",String(f)),this.svg.setAttribute("height",String(c)),this.stage.style.transform=`translate(${m}px,0) scale(${g})`,this.canvas.style.height=`${Math.ceil(c*g)}px`,cancelAnimationFrame(this.frame);for(let l of this.nodes.values())(!l.placed||!e)&&(l.x=l.tx,l.y=l.ty,l.placed=!0),l.fx=l.x,l.fy=l.y;let v=performance.now(),$=l=>{let k=e?Math.min(1,(l-v)/j):1,x=O(k);for(let b of this.nodes.values())b.x=b.fx+(b.tx-b.fx)*x,b.y=b.fy+(b.ty-b.fy)*x,b.slot.style.transform=`translate(${b.x}px,${b.y}px)`;this.drawEdges(),k<1&&(this.frame=requestAnimationFrame($))};$(v)}renderFoot(e){let s="";if(e.verify){let a=this.first?700:0;s=`<div class="cg-checks">${e.verify.checks.map((t,i)=>`<div class="cg-check${t.ok?"":" no"}" style="animation-delay:${a+i*90}ms" title="${h(t.detail)}"><b>${t.ok?L:V}</b><span>${h(t.name)}</span></div>`).join("")}</div>${e.verify.checks.filter(t=>!t.ok).map(t=>{let i=t.items??[],d=i.filter(n=>n.includes("\u2717"));return(d.length?d:i.length?i:[t.detail]).map(n=>`<div class="cg-prob">${h(`${t.name}\uFF1A${n}`)}</div>`).join("")}).join("")}`}else!e.ok&&e.problems?.length&&(s=e.problems.map((a,t)=>`<div class="cg-prob" style="animation-delay:${(this.first?600:0)+t*80}ms">${h(a)}</div>`).join(""));s!==this.footSig&&(this.footSig=s,this.foot.innerHTML=s)}toggleGate(){if(this.gate){this.closeGate();return}let e=this.g,s=document.createElement("form");s.className="cg-gate",s.innerHTML=`<span class="who">\u4EE5 <b>alice</b>\uFF08\u5BA2\u6237\u7ECF\u7406\uFF09\u6279\u51C6</span>
      <input type="password" placeholder="\u5BC6\u7801" autocomplete="current-password" aria-label="\u5BC6\u7801">
      <button class="cg-btn" type="submit">\u6279\u51C6\u6267\u884C</button>
      <span class="msg"></span>
      <span class="hash" title="${h(e.hash)}">\u7ED1\u5B9A ${h(e.hash.replace("sha256:","").slice(0,8))}</span>`;let a=s.querySelector("input"),t=s.querySelector("button"),i=s.querySelector(".msg");s.addEventListener("click",d=>d.stopPropagation()),s.addEventListener("submit",async d=>{if(d.preventDefault(),!a.value){a.focus();return}t.classList.add("busy"),t.textContent="\u6279\u51C6\u4E2D\u2026",i.textContent="";let n=await this.opts.approve(this.g,"alice",a.value).catch(o=>String(o));if(n){t.classList.remove("busy"),t.textContent="\u6279\u51C6\u6267\u884C",i.textContent=n,s.classList.remove("shake"),s.offsetWidth,s.classList.add("shake"),a.select();return}s.classList.add("done"),s.innerHTML=`<span class="ok">${L}\u5DF2\u6279\u51C6\uFF0C\u5F00\u59CB\u6267\u884C</span>`,this.opts.onApproved?.(),setTimeout(()=>this.closeGate(),1600)}),this.head.after(s),this.gate=s,a.focus()}closeGate(){if(!this.gate)return;let e=this.gate;this.gate=void 0,e.classList.add("leave"),setTimeout(()=>e.remove(),260)}togglePop(e,s=!1){if(this.pop&&this.pop.dataset.id===e&&!s){this.closePop();return}let a=this.g?.nodes.find(u=>u.id===e),t=this.nodes.get(e);if(!a||!t){this.closePop();return}let i=this.pop??document.createElement("div"),d=!this.pop;i.className="cg-pop",i.dataset.id=e;let n=a.role&&a.role!==a.name?`<div class="role">${h(a.role)}</div>`:"";i.innerHTML=`<h4>${h(a.name)}</h4>${n}<p>${h(a.task)}</p>${a.denied&&!this.g.ok?`<p style="color:var(--red)">${h(a.denied)}</p>`:""}${a.reads?.length?`<div class="k">\u8BFB\u53D6</div>${a.reads.map(u=>`<code>${h(u)}</code>`).join("")}`:""}${a.contract?`<div class="k">\u5951\u7EA6</div><code>${h(a.contract)}</code>`:""}${a.output?`<div class="k">\u4EA7\u7269</div><div class="out">${h(a.output)}</div>`:""}`,d?(this.el.appendChild(i),this.pop=i):s||(i.style.animation="none",i.offsetWidth,i.style.animation="");let o=t.el.getBoundingClientRect(),r=this.el.getBoundingClientRect(),f=Math.min(Math.max(8,o.left-r.left),r.width-i.offsetWidth-8),c=o.bottom-r.top+8;c+i.offsetHeight>r.height-4&&(c=Math.max(4,o.top-r.top-i.offsetHeight-8)),i.style.left=`${f}px`,i.style.top=`${c}px`}closePop(){this.pop?.remove(),this.pop=void 0}};function q(p,e,s=i=>i,a=1200,t=!1){let i=!1,d=0,n=0,o=e.live,r=async()=>{if(!(i||!o)){try{let f=await fetch(o,{cache:"no-store"});if(!f.ok)throw new Error(String(f.status));let c=await f.json();if(i)return;n=0;for(let u of["live","confirm","approve"])c[u]&&(c[u]=s(c[u]));c.run&&(c.run.url=s(c.run.url)),p.render(c),o=c.live}catch{if(++n>30)return}o&&!i&&(d=window.setTimeout(r,a))}};return o&&(d=window.setTimeout(r,t?0:a)),()=>{i=!0,clearTimeout(d)}}for(let p of document.querySelectorAll("[data-graph]")){let e=new S(p,{page:!0});fetch(p.dataset.graph,{cache:"no-store"}).then(s=>s.json()).then(s=>{e.render(s),q(e,s)})}})();
