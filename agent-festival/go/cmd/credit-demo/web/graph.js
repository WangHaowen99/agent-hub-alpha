"use strict";(()=>{var A=String.raw`
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
`;var H=184,C=64,D=18,M=22,T=5,P=520,G='<svg viewBox="0 0 12 12"><path d="M2.5 6.3l2.3 2.3 4.7-5" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>',V='<svg viewBox="0 0 12 12"><path d="M3 3l6 6M9 3l-6 6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>',N="http://www.w3.org/2000/svg",O=["idle","active","done","skip","bad"],R={idle:"#cfd4dc",active:"#93b4f5",done:"#8fd1a6",skip:"#dcdfe4",bad:"#f2a7a7"},f=g=>g.replace(/[&<>"]/g,e=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;"})[e]),E=g=>g==="done"||g==="reused",F=g=>1-Math.pow(1-g,3),B=0;function I(g){if(g.getElementById("cg-style"))return;let e=g.createElement("style");e.id="cg-style",e.textContent=A,g.head.appendChild(e)}function _(g){if(!g)return;let e=/^grant-(\d+)$/.exec(g);return e?["ok",`\u540C\u610F ${Number(e[1]).toLocaleString("en-US")}`]:g==="deny"?["bad","\u62D2\u7EDD"]:g==="on_missing_evidence"?["mute","\u8BC1\u636E\u4E0D\u8DB3"]:g==="on_undefined_rule"?["mute","\u89C4\u5219\u672A\u8986\u76D6"]:["mute",g]}function Y(g){switch(g){case"approved":return["ok","\u94F6\u884C\u5DF2\u6279\u51C6"];case"declined":return["bad","\u94F6\u884C\u62D2\u7EDD"];case"cancelled":return["mute","\u5DF2\u64A4\u9500"];case"unknown":return["mute","\u7ED3\u679C\u672A\u77E5 \xB7 \u67E5\u8BE2\u4E2D"]}}var W=g=>g.replace(/^[A-Za-z_]+\s*[:：]\s*/,"");function J(g){return!g.negate&&/GRANT/.test(g.contains)?"\u540C\u610F\u65F6":(g.negate?"\u4E0D\u542B ":"\u542B ")+g.contains}var L=class{constructor(e,n={}){this.id=`cg${++B}`;this.nodes=new Map;this.edges=new Map;this.first=!0;this.headSig="";this.footSig="";this.frame=0;this.pending=0;this.sizes=new ResizeObserver(()=>{cancelAnimationFrame(this.pending),this.pending=requestAnimationFrame(()=>{[...this.nodes.values()].some(e=>e.el.offsetHeight!==e.h)&&this.layout(!0)})});this.opts=n,I(e.ownerDocument),this.el=document.createElement("div"),this.el.className="cg",this.el.innerHTML=`<div class="cg-head"></div><div class="cg-canvas"><div class="cg-stage"><svg class="cg-edges"><defs>${O.map(t=>`<marker id="${this.id}-ar-${t}" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M1 1.5L8.5 5L1 8.5" fill="none" stroke="${R[t]}" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></marker>`).join("")}</defs></svg></div></div><div class="cg-foot"></div>`,e.replaceChildren(this.el),this.head=this.el.querySelector(".cg-head"),this.canvas=this.el.querySelector(".cg-canvas"),this.stage=this.el.querySelector(".cg-stage"),this.svg=this.el.querySelector("svg"),this.foot=this.el.querySelector(".cg-foot");let o=0;new ResizeObserver(()=>{this.g&&this.canvas.clientWidth!==o&&(o=this.canvas.clientWidth,this.layout(!1))}).observe(this.canvas),this.el.addEventListener("click",t=>{let s=t.target.closest(".cg-slot");s&&!t.target.closest(".cg-pop")?this.togglePop(s.dataset.id):t.target.closest(".cg-pop")||this.closePop()}),e.ownerDocument.addEventListener("keydown",t=>{t.key==="Escape"&&this.closePop()})}render(e){this.g=e,this.el.classList.toggle("m-run",!!e.run),this.el.classList.toggle("m-plan",!e.run),this.el.classList.toggle("m-stalled",!!e.run?.stalled),this.renderHead(e);let n=new Map;e.layers.forEach((t,s)=>t.forEach(c=>n.set(c,s)));let o=new Set;e.nodes.forEach((t,s)=>{o.add(t.id);let c=this.nodes.get(t.id);if(!c){let i=document.createElement("div");i.className="cg-slot",i.dataset.id=t.id;let a=document.createElement("div");i.appendChild(a),this.stage.appendChild(i),this.sizes.observe(a),c={slot:i,el:a,sig:"",branches:0,x:0,y:0,fx:0,fy:0,tx:0,ty:0,h:0,placed:!1},this.nodes.set(t.id,c);let r=this.first?(n.get(t.id)??0)*110+s*25:0;a.style.setProperty("--d",`${r}ms`),a.style.animationDelay=`${r}ms`,a.dataset.enter="1"}this.renderNode(c,t,e)});for(let[t,s]of this.nodes)o.has(t)||(this.sizes.unobserve(s.el),s.slot.remove(),this.nodes.delete(t));this.renderEdges(e,n),this.renderFoot(e),this.layout(!this.first),this.pop&&this.togglePop(this.pop.dataset.id,!0),this.first=!1}renderHead(e){let n=e.nodes.length,o=e.nodes.filter(l=>E(l.status)||l.status==="skipped").length,t=e.nodes.filter(l=>l.status==="reused").length,s;if(e.verify){let l=e.verify.checks.filter(p=>p.ok).length;s=e.verify.ok?["ok",`\u9A8C\u8BC1\u901A\u8FC7 ${l}/${e.verify.checks.length}`]:["bad",`\u9A8C\u8BC1\u672A\u901A\u8FC7 ${l}/${e.verify.checks.length}`]}else e.run?(s={queued:["wait","\u6392\u961F\u4E2D"],running:["run",`\u6267\u884C\u4E2D ${o}/${n}`],done:["ok","\u5DF2\u5B8C\u6210"],failed:["bad","\u6267\u884C\u5931\u8D25"]}[e.run.status]??["mute",e.run.status],e.run.stalled&&(s=["bad","\u6267\u884C\u5668\u65E0\u54CD\u5E94 \xB7 \u7B49\u5F85\u63A5\u7BA1"])):e.ok?e.superseded?s=["mute",`\u5DF2\u66F4\u65B0\u4E3A v${e.superseded}`]:s=["wait","\u5F85\u6279\u51C6"]:s=["bad","\u9884\u68C0\u672A\u901A\u8FC7"];let c=e.run&&e.run.leases>1?`<span class="cg-chip soft">\u21BB \u6267\u884C\u5668\u91CD\u542F \xB7 \u590D\u7528 ${t} \u6B65</span>`:"",i=e.confirm&&!this.opts.page?`<button class="cg-btn" data-open="${f(e.confirm)}">\u53BB\u6279\u51C6</button>`:"",a=[e.goal,e.case,e.plan,e.version,s.join(),c,i].join("|");a!==this.headSig&&(this.headSig=a,this.head.innerHTML=`<div class="cg-title"><span class="cg-goal">${f(e.goal||`${e.case} \u6388\u4FE1\u5C3D\u8C03`)}</span><span class="cg-ver">${f(e.case)} \xB7 ${f(e.plan)} v${e.version}</span></div>
        <div class="cg-meta">${c}<span class="cg-chip ${s[0]}"><i></i>${f(s[1])}</span>${i}</div><div class="cg-bar"><b></b></div>`,this.head.querySelector("[data-open]")?.addEventListener("click",l=>{l.stopPropagation();let p=l.currentTarget.dataset.open;this.opts.openLink?this.opts.openLink(p):window.open(p,"_blank","noopener")}));let r=this.head.querySelector(".cg-bar");r.classList.toggle("ok",e.run?.status==="done"),requestAnimationFrame(()=>{r.firstElementChild.style.width=e.run?`${o/Math.max(n,1)*100}%`:"0"})}renderNode(e,n,o){let t=n.status??"plan",s=!o.ok&&!o.run&&!!n.denied,c=n.kind==="judge",i=n.kind==="effect",a=c&&E(t)?_(o.run?.decision):i&&E(t)?Y(o.run?.receipt):void 0,r=n.branches??[],l=JSON.stringify([n.name,n.task,t,s,a,r.map(b=>b.status+b.goal)]);if(e.el.className=`cg-node k-${n.kind} s-${t}${s?" denied":""}${e.el.dataset.enter?" enter":""}`,l===e.sig)return;let p=e.sig!==""&&JSON.parse(e.sig)[2]!==t,v=e.sig===""?!1:JSON.parse(e.sig)[3];if(e.sig=l,e.el.dataset.enter&&!e.el.dataset.listening){e.el.dataset.listening="1";let b=x=>{x.target===e.el&&(e.el.removeEventListener("animationend",b),delete e.el.dataset.enter,e.el.classList.remove("enter"),e.el.style.animationDelay="")};e.el.addEventListener("animationend",b)}let h=p||!this.first?"0ms":void 0;h&&e.el.style.setProperty("--d",h);let u=[];n.kind==="blackbox"&&u.push('<span class="cg-tag bb">\u9ED1\u76D2</span>'),n.effect==="irreversible"&&u.push('<span class="cg-tag irrev">\u4E0D\u53EF\u9006</span>'),s&&u.push('<span class="cg-tag deny pop">\u65E0\u6743\u9650</span>'),t==="reused"&&u.push('<span class="cg-tag reuse pop">\u590D\u7528</span>'),t==="skipped"&&u.push('<span class="cg-tag skip">\u672A\u6267\u884C</span>');let k=t==="done"?`<span class="cg-ok">${G}</span>`:t==="reused"?`<span class="cg-ok reused">${G}</span>`:t==="failed"?`<span class="cg-ok fail">${V}</span>`:"",y="";n.kind==="blackbox"&&(r.length?y=`<div class="cg-bb">${r.slice(0,T).map((x,m)=>`<div class="cg-br${m>=e.branches?" enter":""}" style="animation-delay:${Math.max(0,m-e.branches)*90}ms" title="${f(x.goal)}"><em class="${f(x.status)}"></em><span>${f(W(x.goal))}</span></div>`).join("")}${r.length>T?`<div class="cg-br more">\u53E6 ${r.length-T} \u4E2A\u5206\u652F</div>`:""}</div>`:y=`<div class="cg-bb closed"><i></i><i></i><i></i><span>${t==="running"?"\u6B63\u5728\u62C6\u5206":"\u8FD0\u884C\u65F6\u5C55\u5F00"}</span></div>`,e.branches=r.length);let d=a?`<div class="cg-res ${a[0]}">${f(a[1])}</div>`:"";e.el.innerHTML=`<div class="cg-av">${f([...n.name][0]??"?")}</div><div class="cg-body"><div class="cg-name">${f(n.name)}</div><div class="cg-task">${f(n.task)}</div>${d}</div>${y}${u.length?`<div class="cg-tags">${u.join("")}</div>`:""}${k}`,e.el.querySelectorAll(".cg-ok, .cg-res, .cg-tag.pop").forEach(b=>{b.style.animationDelay="calc(var(--d, 0ms) + 280ms)"}),s&&!v&&(e.el.classList.remove("shake"),setTimeout(()=>e.el.classList.add("shake"),this.first?900:50))}renderEdges(e,n){let o=new Map(e.nodes.map(i=>[i.id,i])),t=new Map,s=i=>{let a=t.get(i);if(a)return a;a=new Set,t.set(i,a);for(let r of o.get(i)?.deps??[])a.add(r),s(r).forEach(l=>a.add(l));return a},c=new Set;for(let i of e.nodes){let a=(i.deps??[]).filter(r=>o.has(r));for(let r of a){if(i.when?.step!==r&&a.some(u=>u!==r&&s(u).has(r)))continue;let l=`${r}>${i.id}`;c.add(l);let p=this.edges.get(l);if(!p){let u=document.createElementNS(N,"g"),k=document.createElementNS(N,"path");k.setAttribute("class","base"),k.setAttribute("pathLength","1");let y=document.createElementNS(N,"path");y.setAttribute("class","flow"),u.append(k,y),this.svg.appendChild(u),p={from:r,to:i.id,g:u,base:k,flow:y,drawn:!1},this.edges.set(l,p);let d=this.first?260+(n.get(r)??0)*120:0;k.style.transitionDelay=`${d}ms, 0ms`}i.when&&i.when.step===r&&!p.cond&&(p.cond=document.createElement("div"),p.cond.className="cg-cond",p.cond.textContent=J(i.when),this.stage.appendChild(p.cond));let v=o.get(r),h="idle";if(e.run?i.status==="skipped"?h="skip":E(v.status)&&i.status==="running"?h="active":E(v.status)&&E(i.status)&&(h="done"):!e.ok&&i.denied&&(h="bad"),p.g.setAttribute("class",`e-${h}`),p.base.setAttribute("marker-end",`url(#${this.id}-ar-${h})`),p.cond){let u=e.run?.decision;p.cond.classList.toggle("on",!!u&&/^grant/.test(u)&&!i.when?.negate),p.cond.classList.toggle("off",i.status==="skipped")}}}for(let[i,a]of this.edges)c.has(i)||(a.g.remove(),a.cond?.remove(),this.edges.delete(i))}drawEdges(){let e=new Map,n=new Map;for(let t of this.edges.values())(e.get(t.from)??e.set(t.from,[]).get(t.from)).push(t),(n.get(t.to)??n.set(t.to,[]).get(t.to)).push(t);let o=(t,s,c,i)=>{let a=[...t].sort((v,h)=>(i(v)?.y??0)-(i(h)?.y??0)),r=a.length,l=a.indexOf(s),p=r>1?Math.min(9,c.h*.6/(r-1)):0;return c.y+c.h/2+(l-(r-1)/2)*p};for(let t of this.edges.values()){let s=this.nodes.get(t.from),c=this.nodes.get(t.to);if(!s||!c)continue;let i=s.x+H,a=c.x-3,r=o(e.get(t.from),t,s,h=>this.nodes.get(h.to)),l=o(n.get(t.to),t,c,h=>this.nodes.get(h.from)),p=Math.max(26,(a-i)*.5),v=`M${i},${r} C${i+p},${r} ${a-p},${l} ${a},${l}`;t.base.setAttribute("d",v),t.flow.setAttribute("d",v),t.cond&&(t.cond.style.left=`${(i+a)/2}px`,t.cond.style.top=`${(r+l)/2}px`),t.drawn||(t.drawn=!0,requestAnimationFrame(()=>requestAnimationFrame(()=>t.base.classList.add("drawn"))))}}layout(e){let n=this.g,o=new Map(n.nodes.map(d=>[d.id,d])),t=n.layers.map(d=>d.filter(b=>o.has(b))).filter(d=>d.length),s=new Set(t.flat()),c=n.nodes.map(d=>d.id).filter(d=>!s.has(d));c.length&&t.push(c);for(let d of this.nodes.values())d.h=d.el.offsetHeight;let i=t.map(d=>d.reduce((b,x)=>b+this.nodes.get(x).h,0)+(d.length-1)*D),a=Math.max(...i,60),r=new Map;t.forEach((d,b)=>{let x=d.map(($,w)=>{let S=(o.get($).deps??[]).filter(z=>r.has(z));return{id:$,i:w,bc:S.length?S.reduce((z,q)=>z+r.get(q),0)/S.length:1/0}});b>0&&x.sort(($,w)=>$.bc===w.bc?$.i-w.i:$.bc-w.bc);let m=M+(a-i[b])/2;for(let{id:$}of x){let w=this.nodes.get($);w.tx=M+b*(H+C),w.ty=m,r.set($,m+w.h/2),m+=w.h+D}});let l=M*2+t.length*H+(t.length-1)*C,p=a+M*2,v=this.canvas.clientWidth||l,h=Math.min(1,v/l),u=Math.max(0,(v-l*h)/2);this.stage.style.width=`${l}px`,this.stage.style.height=`${p}px`,this.svg.setAttribute("width",String(l)),this.svg.setAttribute("height",String(p)),this.stage.style.transform=`translate(${u}px,0) scale(${h})`,this.canvas.style.height=`${Math.ceil(p*h)}px`,cancelAnimationFrame(this.frame);for(let d of this.nodes.values())(!d.placed||!e)&&(d.x=d.tx,d.y=d.ty,d.placed=!0),d.fx=d.x,d.fy=d.y;let k=performance.now(),y=d=>{let b=e?Math.min(1,(d-k)/P):1,x=F(b);for(let m of this.nodes.values())m.x=m.fx+(m.tx-m.fx)*x,m.y=m.fy+(m.ty-m.fy)*x,m.slot.style.transform=`translate(${m.x}px,${m.y}px)`;this.drawEdges(),b<1&&(this.frame=requestAnimationFrame(y))};y(k)}renderFoot(e){let n="";if(e.verify){let o=this.first?700:0;n=`<div class="cg-checks">${e.verify.checks.map((t,s)=>`<div class="cg-check${t.ok?"":" no"}" style="animation-delay:${o+s*90}ms" title="${f(t.detail)}"><b>${t.ok?G:V}</b><span>${f(t.name)}</span></div>`).join("")}</div>${e.verify.checks.filter(t=>!t.ok).map(t=>{let s=t.items??[],c=s.filter(i=>i.includes("\u2717"));return(c.length?c:s.length?s:[t.detail]).map(i=>`<div class="cg-prob">${f(`${t.name}\uFF1A${i}`)}</div>`).join("")}).join("")}`}else!e.ok&&e.problems?.length&&(n=e.problems.map((o,t)=>`<div class="cg-prob" style="animation-delay:${(this.first?600:0)+t*80}ms">${f(o)}</div>`).join(""));n!==this.footSig&&(this.footSig=n,this.foot.innerHTML=n)}togglePop(e,n=!1){if(this.pop&&this.pop.dataset.id===e&&!n){this.closePop();return}let o=this.g?.nodes.find(v=>v.id===e),t=this.nodes.get(e);if(!o||!t){this.closePop();return}let s=this.pop??document.createElement("div"),c=!this.pop;s.className="cg-pop",s.dataset.id=e;let i=o.role&&o.role!==o.name?`<div class="role">${f(o.role)}</div>`:"";s.innerHTML=`<h4>${f(o.name)}</h4>${i}<p>${f(o.task)}</p>${o.denied&&!this.g.ok?`<p style="color:var(--red)">${f(o.denied)}</p>`:""}${o.reads?.length?`<div class="k">\u8BFB\u53D6</div>${o.reads.map(v=>`<code>${f(v)}</code>`).join("")}`:""}${o.contract?`<div class="k">\u5951\u7EA6</div><code>${f(o.contract)}</code>`:""}${o.output?`<div class="k">\u4EA7\u7269</div><div class="out">${f(o.output)}</div>`:""}`,c?(this.el.appendChild(s),this.pop=s):n||(s.style.animation="none",s.offsetWidth,s.style.animation="");let a=t.el.getBoundingClientRect(),r=this.el.getBoundingClientRect(),l=Math.min(Math.max(8,a.left-r.left),r.width-s.offsetWidth-8),p=a.bottom-r.top+8;p+s.offsetHeight>r.height-4&&(p=Math.max(4,a.top-r.top-s.offsetHeight-8)),s.style.left=`${l}px`,s.style.top=`${p}px`}closePop(){this.pop?.remove(),this.pop=void 0}};function j(g,e,n=1200){let o=!1,t=0,s=0,c=e.live,i=async()=>{if(!(o||!c)){try{let a=await fetch(c,{cache:"no-store"});if(!a.ok)throw new Error(String(a.status));let r=await a.json();if(o)return;s=0,g.render(r),c=r.live}catch{if(++s>30)return}c&&!o&&(t=window.setTimeout(i,n))}};return c&&(t=window.setTimeout(i,n)),()=>{o=!0,clearTimeout(t)}}for(let g of document.querySelectorAll("[data-graph]")){let e=new L(g,{page:!0});fetch(g.dataset.graph,{cache:"no-store"}).then(n=>n.json()).then(n=>{e.render(n),j(e,n)})}})();
