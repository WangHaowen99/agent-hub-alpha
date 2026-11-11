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
.cg-stage { position: absolute; left: 0; top: 0; transform-origin: 0 0; transition: transform .5s cubic-bezier(.2,.8,.2,1); }
.cg-wait { display: none; position: absolute; inset: 0; align-items: center; justify-content: center; gap: 6px; }
.cg-canvas.empty .cg-wait { display: flex; }
.cg-wait i { width: 6px; height: 6px; border-radius: 50%; background: var(--blue); opacity: .3; animation: cg-dots 1.2s infinite; }
.cg-wait i:nth-child(2) { animation-delay: .15s; } .cg-wait i:nth-child(3) { animation-delay: .3s; }
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
.cg-edges g.gone { opacity: 0; transition: opacity .3s; }

/* nodes */
.cg-slot { position: absolute; left: 0; top: 0; will-change: transform; }
.cg-node { position: relative; width: 184px; padding: 10px 11px 10px 10px; display: grid; grid-template-columns: 30px minmax(0, 1fr); column-gap: 9px;
  background: #fff; border: 1px solid #e3e6eb; border-radius: 12px; cursor: pointer;
  box-shadow: 0 1px 2px rgba(16,24,40,.04); transition: border-color .35s, background .35s, opacity .4s, filter .4s, box-shadow .2s, width .45s cubic-bezier(.2,.8,.2,1); }
.cg-node.leave { animation: cg-out .32s cubic-bezier(.4,0,.2,1) both; pointer-events: none; }
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

/* the black box at run time: a Loop Forest group, branches fork out of the host */
.cg-node.group { width: 404px; display: flex; align-items: center; column-gap: 0; background: #fcfaff; }
.cg-host { position: relative; flex: 0 0 163px; display: grid; grid-template-columns: 30px minmax(0, 1fr); column-gap: 9px; }
.cg-fan { position: static; flex: 1 1 auto; min-width: 0; margin-left: 56px; padding: 7px 8px 7px 2px; display: flex; flex-direction: column;
  gap: 7px; overflow: hidden; }
.cg-fan-edges { position: absolute; left: 0; top: 0; width: 100%; height: 100%; overflow: visible; pointer-events: none; }
.cg-fan-edges path { fill: none; stroke-linecap: round; }
.cg-fan-edges .fb { stroke: #d6c6fb; stroke-width: 1.6; stroke-dasharray: 1; stroke-dashoffset: 1;
  transition: stroke-dashoffset .45s cubic-bezier(.4,0,.2,1), stroke .4s; }
.cg-fan-edges .fb.drawn { stroke-dashoffset: 0; }
.cg-fan-edges .b-finished .fb { stroke: #a78bfa; }
.cg-fan-edges .b-interrupted .fb { stroke: #d4d7dd; stroke-dasharray: .03 .03; }
.cg-fan-edges .ff { stroke: var(--violet); stroke-width: 2; stroke-dasharray: 3 9; opacity: 0; animation: cg-flow .7s linear infinite; }
.cg-fan-edges .b-started .ff { opacity: .85; }
.cg-fan-edges .fr { stroke: var(--violet); stroke-width: 4.5; stroke-dasharray: .2 1.3; stroke-dashoffset: .21; opacity: 0;
  filter: drop-shadow(0 0 3px rgba(124,58,237,.7)); }
.cg-fan-edges .fr.ret { animation: cg-ret .8s cubic-bezier(.45,0,.25,1) .1s both; }
.cg-fan-edges .fb.flash { animation: cg-edge-flash 1s ease-out both; }
.cg-ripple { position: absolute; left: 0; top: 0; width: 30px; height: 30px; border-radius: 9px; border: 2px solid var(--violet);
  pointer-events: none; opacity: 0; animation: cg-ripple .6s ease-out .85s forwards; }
.cg-lfb { position: relative; flex: 0 0 auto; width: 152px; display: grid; grid-template-columns: 22px minmax(0, 1fr); column-gap: 7px;
  align-items: center; padding: 6px 8px 6px 6px; background: #fff; border: 1px solid #e4dafb; border-radius: 10px; transition: border-color .35s, opacity .4s; }
.cg-lfb.enter { animation: cg-fork .5s cubic-bezier(.2,.8,.2,1) both; }
.cg-lfb.s-started { border-color: transparent; }
.cg-lfb.s-started::before { content: ""; position: absolute; inset: -1.5px; border-radius: 11px; padding: 1.5px; pointer-events: none;
  background: conic-gradient(from var(--cg-a), rgba(124,58,237,0) 0 55%, #c4b5fd 72%, #7c3aed 88%, rgba(124,58,237,0) 100%);
  -webkit-mask: linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0); -webkit-mask-composite: xor; mask-composite: exclude;
  animation: cg-spin 1.4s linear var(--ph, 0ms) infinite; }
.cg-lfb.s-finished { border-color: #cdb8fa; }
.cg-lfb.s-interrupted { border-style: dashed; border-color: #d4d7dd; opacity: .6; }
.cg-lfb-av { width: 22px; height: 22px; border-radius: 7px; display: grid; place-items: center; font-size: 12px; font-weight: 600;
  color: var(--violet); background: #f1eaff; }
.cg-lfb.s-interrupted .cg-lfb-av { filter: grayscale(1); }
.cg-lfb-body { min-width: 0; }
.cg-lfb-name { font-size: 12px; font-weight: 600; line-height: 16px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.cg-lfb-name em { font-style: normal; color: var(--blue); margin-right: 3px; font-weight: 500; }
.cg-lfb-sub { display: flex; align-items: center; gap: 5px; margin-top: 1px; font-size: 11px; line-height: 16px; color: var(--mute);
  white-space: nowrap; overflow: hidden; }
.cg-lfb-sub span { overflow: hidden; text-overflow: ellipsis; }
.cg-lfb-sub.act { color: var(--violet); }
.cg-lfb-sub.act i { flex: 0 0 5px; height: 5px; border-radius: 50%; background: var(--violet); animation: cg-blink 1s ease-in-out infinite; }
.cg-lfb-sub.act span { animation: cg-slide .35s cubic-bezier(.2,.8,.2,1) both; }
.m-stalled .cg-lfb.s-started::before, .m-stalled .cg-lfb-sub.act i, .m-stalled .cg-fan-edges .ff { animation-play-state: paused; }
.cg-lfb .find { height: 16px; padding: 0 6px; border-radius: 8px; font-weight: 600; font-size: 11px; line-height: 16px; }
.cg-lfb .find.ok { background: #e9f7ee; color: var(--green); }
.cg-lfb .find.warn { background: #fff4e0; color: var(--amber); }
.cg-lfb .find.pop { animation: cg-pop .45s cubic-bezier(.2,1.6,.4,1) .15s both; }
.cg-lfb-ok { position: absolute; top: -6px; right: -6px; width: 16px; height: 16px; border-radius: 50%; background: var(--violet); color: #fff;
  display: grid; place-items: center; box-shadow: 0 0 0 2px #fff; }
.cg-lfb-ok svg { width: 9px; height: 9px; }
.cg-lfb-ok.pop { animation: cg-pop .45s cubic-bezier(.2,1.6,.4,1) both; }
.cg-lfb-more { font-size: 11px; color: var(--faint); padding-left: 8px; }

/* condition label on an edge */
.cg-cond { position: absolute; transform: translate(-50%, -50%); height: 20px; padding: 0 8px; border-radius: 10px;
  background: #fff; border: 1px solid #e3e6eb; font-size: 11px; line-height: 18px; color: var(--mute); white-space: nowrap;
  transition: color .3s, border-color .3s, background .3s, opacity .4s; }
.cg-cond.on { color: var(--green); border-color: #a7dcb9; background: #f2fbf5; }
.cg-cond.off { opacity: .55; text-decoration: line-through; }
.cg-cond.gone { opacity: 0; }

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
@keyframes cg-out { to { opacity: 0; transform: scale(.94); } }
@keyframes cg-fork { from { opacity: 0; transform: translateX(-26px) scale(.86); } to { opacity: 1; transform: none; } }
@keyframes cg-ret { 0% { stroke-dashoffset: -.8; opacity: 1; } 88% { opacity: 1; } 100% { stroke-dashoffset: .21; opacity: 0; } }
@keyframes cg-edge-flash { 0%, 70% { stroke: var(--violet); stroke-width: 2.4; } 100% { stroke: #a78bfa; stroke-width: 1.6; } }
@keyframes cg-ripple { from { opacity: .7; transform: scale(1); } to { opacity: 0; transform: scale(1.9); } }
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
`;var N=184,P=404,C=64,q=18,S=22,R=5,O=520,V=320,M='<svg viewBox="0 0 12 12"><path d="M2.5 6.3l2.3 2.3 4.7-5" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>',D='<svg viewBox="0 0 12 12"><path d="M3 3l6 6M9 3l-6 6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>',G="http://www.w3.org/2000/svg",B=["idle","active","done","skip","bad"],W={idle:"#cfd4dc",active:"#93b4f5",done:"#8fd1a6",skip:"#dcdfe4",bad:"#f2a7a7"},u=h=>h.replace(/[&<>"]/g,e=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;"})[e]),L=h=>h==="done"||h==="reused",I=h=>h==="running"||h==="failed"||L(h),_=()=>`-${Math.round(performance.now()%7e3)}ms`,Y=h=>1-Math.pow(1-h,3),X=0;function J(h){if(h.getElementById("cg-style"))return;let e=h.createElement("style");e.id="cg-style",e.textContent=A,h.head.appendChild(e)}function U(h){if(!h)return;let e=/^grant-(\d+)$/.exec(h);return e?["ok",`\u540C\u610F ${Number(e[1]).toLocaleString("en-US")}`]:h==="deny"?["bad","\u62D2\u7EDD"]:h==="on_missing_evidence"?["mute","\u8BC1\u636E\u4E0D\u8DB3"]:h==="on_undefined_rule"?["mute","\u89C4\u5219\u672A\u8986\u76D6"]:["mute",h]}function K(h){switch(h){case"approved":return["ok","\u94F6\u884C\u5DF2\u6279\u51C6"];case"declined":return["bad","\u94F6\u884C\u62D2\u7EDD"];case"cancelled":return["mute","\u5DF2\u64A4\u9500"];case"unknown":return["mute","\u7ED3\u679C\u672A\u77E5 \xB7 \u67E5\u8BE2\u4E2D"]}}function Q(h){return!h.negate&&/GRANT/.test(h.contains)?"\u540C\u610F\u65F6":(h.negate?"\u4E0D\u542B ":"\u542B ")+h.contains}var z=class{constructor(e,s={}){this.id=`cg${++X}`;this.shown=[];this.nodes=new Map;this.edges=new Map;this.first=!0;this.headSig="";this.footSig="";this.frame=0;this.pending=0;this.sizes=new ResizeObserver(()=>{cancelAnimationFrame(this.pending),this.pending=requestAnimationFrame(()=>{[...this.nodes.values()].some(e=>e.el.offsetHeight!==e.h)&&this.layout(!0)})});this.opts=s,J(e.ownerDocument),this.el=document.createElement("div"),this.el.className="cg",this.el.innerHTML=`<div class="cg-head"></div><div class="cg-canvas"><div class="cg-stage"><svg class="cg-edges"><defs>${B.map(t=>`<marker id="${this.id}-ar-${t}" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M1 1.5L8.5 5L1 8.5" fill="none" stroke="${W[t]}" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></marker>`).join("")}</defs></svg></div><div class="cg-wait"><i></i><i></i><i></i></div></div><div class="cg-foot"></div>`,e.replaceChildren(this.el),this.head=this.el.querySelector(".cg-head"),this.canvas=this.el.querySelector(".cg-canvas"),this.stage=this.el.querySelector(".cg-stage"),this.svg=this.el.querySelector("svg"),this.foot=this.el.querySelector(".cg-foot");let o=0;new ResizeObserver(()=>{this.g&&this.canvas.clientWidth!==o&&(o=this.canvas.clientWidth,this.layout(!1))}).observe(this.canvas),this.el.addEventListener("click",t=>{let i=t.target.closest(".cg-slot");i&&!t.target.closest(".cg-pop")?this.togglePop(i.dataset.id):t.target.closest(".cg-pop")||this.closePop()}),e.ownerDocument.addEventListener("keydown",t=>{t.key==="Escape"&&this.closePop()})}current(){return this.g}render(e){this.g=e,this.el.classList.toggle("m-run",!!e.run),this.el.classList.toggle("m-plan",!e.run),this.el.classList.toggle("m-stalled",!!e.run?.stalled),this.renderHead(e);let s=new Map;e.layers.forEach((t,i)=>t.forEach(d=>s.set(d,i))),this.shown=e.run?e.nodes.filter(t=>I(t.status)):e.nodes;let o=new Set;this.shown.forEach((t,i)=>{o.add(t.id);let d=this.nodes.get(t.id);if(!d){let p=document.createElement("div");p.className="cg-slot",p.dataset.id=t.id;let a=document.createElement("div");p.appendChild(a),this.stage.appendChild(p),this.sizes.observe(a),a.addEventListener("transitionend",c=>{c.target===a&&c.propertyName==="width"&&this.drawEdges()}),d={slot:p,el:a,sig:"",branches:[],wide:!1,x:0,y:0,fx:0,fy:0,tx:0,ty:0,w:N,h:0,placed:!1},this.nodes.set(t.id,d);let n=this.first?(s.get(t.id)??0)*110+i*25:(t.deps??[]).some(c=>this.nodes.has(c))?240:0;a.style.setProperty("--d",`${n}ms`),a.style.animationDelay=`${n}ms`,a.dataset.enter="1"}this.renderNode(d,t,e)});for(let[t,i]of this.nodes)o.has(t)||(this.sizes.unobserve(i.el),this.nodes.delete(t),i.el.classList.remove("enter"),i.el.classList.add("leave"),setTimeout(()=>i.slot.remove(),V));this.renderEdges(e,s),this.renderFoot(e),this.layout(!this.first);for(let t of this.nodes.values())t.wide&&this.drawFan(t);this.pop&&this.togglePop(this.pop.dataset.id,!0),this.first=!1}renderHead(e){let s=e.nodes.length,o=e.nodes.filter(f=>L(f.status)||f.status==="skipped").length,t=e.nodes.filter(f=>f.status==="reused").length,i;if(e.verify){let f=e.verify.checks.filter(m=>m.ok).length;i=e.verify.ok?["ok",`\u9A8C\u8BC1\u901A\u8FC7 ${f}/${e.verify.checks.length}`]:["bad",`\u9A8C\u8BC1\u672A\u901A\u8FC7 ${f}/${e.verify.checks.length}`]}else e.run?(i={queued:["wait","\u6392\u961F\u4E2D"],running:["run",`\u6267\u884C\u4E2D ${o}/${s}`],done:["ok","\u5DF2\u5B8C\u6210"],failed:["bad","\u6267\u884C\u5931\u8D25"]}[e.run.status]??["mute",e.run.status],e.run.stalled&&(i=["bad","\u6267\u884C\u5668\u65E0\u54CD\u5E94 \xB7 \u7B49\u5F85\u63A5\u7BA1"])):e.ok?e.superseded?i=["mute",`\u5DF2\u66F4\u65B0\u4E3A v${e.superseded}`]:i=["wait","\u5F85\u6279\u51C6"]:i=["bad","\u9884\u68C0\u672A\u901A\u8FC7"];let d=e.run&&e.run.leases>1?`<span class="cg-chip soft">\u21BB \u6267\u884C\u5668\u91CD\u542F \xB7 \u590D\u7528 ${t} \u6B65</span>`:"",p=!!e.approve&&!!this.opts.approve&&!this.opts.page,a=p?'<button class="cg-btn" data-approve>\u6279\u51C6</button>':"",n=e.run?.status==="running"&&!e.run.stalled?e.nodes.filter(f=>f.status==="running").map(f=>f.name):[],c=n.length?`<span class="cg-now"><i></i>\u6B63\u5728\uFF1A${u(n.join("\u3001"))}</span>`:`<span class="cg-ver">${u(e.case)} \xB7 ${u(e.plan)} v${e.version}</span>`,r=[e.goal,c,i.join(),d,a].join("|");r!==this.headSig&&(this.headSig=r,this.head.innerHTML=`<div class="cg-title"><span class="cg-goal">${u(e.goal||`${e.case} \u6388\u4FE1\u5C3D\u8C03`)}</span>${c}</div>
        <div class="cg-meta">${d}<span class="cg-chip ${i[0]}"><i></i>${u(i[1])}</span>${a}</div><div class="cg-bar"><b></b></div>`,this.head.querySelector("[data-approve]")?.addEventListener("click",f=>{f.stopPropagation(),this.toggleGate()})),p||this.closeGate();let l=this.head.querySelector(".cg-bar");l.classList.toggle("ok",e.run?.status==="done"),requestAnimationFrame(()=>{l.firstElementChild.style.width=e.run?`${o/Math.max(s,1)*100}%`:"0"})}renderNode(e,s,o){let t=s.status??"plan",i=!o.ok&&!o.run&&!!s.denied,d=s.kind==="judge",p=s.kind==="effect",a=d&&L(t)?U(o.run?.decision):p&&L(t)?K(o.run?.receipt):void 0,n=(s.branches??[]).slice(0,R),c=(s.branches?.length??0)-n.length,r=!!o.run&&s.kind==="blackbox"&&n.length>0,l=t==="running"?s.activity??"":"",f=JSON.stringify([s.name,s.task,t,i,a,l,c,n.map(b=>[b.status,b.title,b.activity,b.finding,b.retried])]);if(e.wide=r,e.el.className=`cg-node k-${s.kind} s-${t}${i?" denied":""}${r?" group":""}${e.el.dataset.enter?" enter":""}`,f===e.sig)return;let m=e.sig!==""&&JSON.parse(e.sig)[2]!==t,v=e.sig===""?!1:JSON.parse(e.sig)[3];if(e.sig=f,e.el.dataset.enter&&!e.el.dataset.listening){e.el.dataset.listening="1";let b=$=>{$.target===e.el&&(e.el.removeEventListener("animationend",b),delete e.el.dataset.enter,e.el.classList.remove("enter"),e.el.style.animationDelay="")};e.el.addEventListener("animationend",b)}let w=m||!this.first?"0ms":void 0;w&&e.el.style.setProperty("--d",w);let k=[];s.kind==="blackbox"&&k.push(`<span class="cg-tag bb">${r?"\u9ED1\u76D2 \xB7 Loop Forest":"\u9ED1\u76D2"}</span>`),s.effect==="irreversible"&&k.push('<span class="cg-tag irrev">\u4E0D\u53EF\u9006</span>'),i&&k.push('<span class="cg-tag deny pop">\u65E0\u6743\u9650</span>'),t==="reused"&&k.push('<span class="cg-tag reuse pop">\u590D\u7528</span>'),t==="skipped"&&k.push('<span class="cg-tag skip">\u672A\u6267\u884C</span>');let g=t==="done"?`<span class="cg-ok">${M}</span>`:t==="reused"?`<span class="cg-ok reused">${M}</span>`:t==="failed"?`<span class="cg-ok fail">${D}</span>`:"",y=(a?`<div class="cg-res ${a[0]}">${u(a[1])}</div>`:"")+(l?`<div class="cg-act" title="${u(l)}"><i></i><span>${u(l)}</span></div>`:""),E=`<div class="cg-av">${u([...s.name][0]??"?")}</div><div class="cg-body"><div class="cg-name">${u(s.name)}</div><div class="cg-task">${u(s.task)}</div>${y}</div>`,x;if(r)x=this.renderGroup(e,E,n,c);else{let b=s.kind==="blackbox"&&!o.run?'<div class="cg-bb closed"><i></i><i></i><i></i><span>\u8FD0\u884C\u65F6\u5C55\u5F00</span></div>':"";x=E+b,e.branches=[]}e.el.innerHTML=`${x}${k.length?`<div class="cg-tags">${k.join("")}</div>`:""}${g}`,e.el.querySelectorAll(".cg-ok, .cg-res, .cg-tag.pop").forEach(b=>{b.closest(".cg-lfb")||(b.style.animationDelay="calc(var(--d, 0ms) + 280ms)")}),r&&(this.drawFan(e),requestAnimationFrame(()=>requestAnimationFrame(()=>e.el.querySelectorAll(".cg-fan-edges .fb:not(.drawn)").forEach(b=>b.classList.add("drawn"))))),i&&!v&&(e.el.classList.remove("shake"),setTimeout(()=>e.el.classList.add("shake"),this.first?900:50))}renderGroup(e,s,o,t){let i=e.branches,d=0,p=!1,a=_(),n=[],c=[];return o.forEach((r,l)=>{let f=l>=i.length,m=!f&&i[l]==="started"&&r.status==="finished";p||(p=m);let v=f?220+d++*160:0;n.push(`<g class="b-${u(r.status)}"><path class="fb${f?"":" drawn"}${m?" flash":""}" pathLength="1" style="transition-delay:${f?v-220:0}ms, 0ms"/><path class="ff" style="animation-delay:${a}"/><path class="fr${m?" ret":""}" pathLength="1"/></g>`);let w=r.status==="started"&&r.activity?`<div class="cg-lfb-sub act"><i style="animation-delay:${a}"></i><span>${u(r.activity)}</span></div>`:r.status==="finished"&&r.finding?`<div class="cg-lfb-sub"><b class="find ${u(r.tone??"")}${m||f?" pop":""}">${u(r.finding)}</b></div>`:r.status==="interrupted"?'<div class="cg-lfb-sub">\u6267\u884C\u5668\u91CD\u542F\uFF0C\u5DF2\u4E2D\u65AD</div>':`<div class="cg-lfb-sub"><span>${u(r.relation??"")}</span></div>`,k=r.status==="finished"?`<span class="cg-lfb-ok${m?" pop":""}">${M}</span>`:"";c.push(`<div class="cg-lfb s-${u(r.status)}${f?" enter":""}" style="animation-delay:${v}ms;--ph:${a}" title="${u(r.goal)}"><div class="cg-lfb-av">${u([...r.title][0]??"?")}</div><div class="cg-lfb-body"><div class="cg-lfb-name">${r.retried?'<em title="\u4E0A\u6B21\u6267\u884C\u4E2D\u65AD\u540E\u91CD\u65B0\u5206\u53C9">\u21BB</em>':""}${u(r.title)}</div>${w}</div>${k}</div>`)}),e.branches=o.map(r=>r.status),`<svg class="cg-fan-edges">${n.join("")}</svg><div class="cg-host">${s}${p?'<i class="cg-ripple"></i>':""}</div><div class="cg-fan">${c.join("")}${t>0?`<div class="cg-lfb-more">\u53E6 ${t} \u4E2A\u5206\u652F</div>`:""}</div>`}drawFan(e){let s=e.el.querySelector(".cg-host");if(!s)return;let o=s.offsetLeft+s.offsetWidth+2,t=s.offsetTop+s.offsetHeight/2,i=e.el.querySelectorAll(".cg-fan-edges g");e.el.querySelectorAll(".cg-lfb").forEach((d,p)=>{let a=d.offsetLeft-2,n=d.offsetTop+d.offsetHeight/2,c=(a-o)*.6,r=`M${o},${t} C${o+c},${t} ${a-c},${n} ${a},${n}`;i[p]?.querySelectorAll("path").forEach(l=>l.setAttribute("d",r))})}renderEdges(e,s){let o=new Map(e.nodes.map(a=>[a.id,a])),t=new Map(this.shown.map(a=>[a.id,a])),i=new Map,d=a=>{let n=i.get(a);if(n)return n;n=new Set,i.set(a,n);for(let c of o.get(a)?.deps??[])n.add(c),d(c).forEach(r=>n.add(r));return n},p=new Set;for(let a of this.shown){let n=(a.deps??[]).filter(c=>t.has(c));for(let c of n){if(a.when?.step!==c&&n.some(v=>v!==c&&d(v).has(c)))continue;let r=`${c}>${a.id}`;p.add(r);let l=this.edges.get(r);if(!l){let v=document.createElementNS(G,"g"),w=document.createElementNS(G,"path");w.setAttribute("class","base"),w.setAttribute("pathLength","1");let k=document.createElementNS(G,"path");k.setAttribute("class","flow"),v.append(w,k),this.svg.appendChild(v),l={from:c,to:a.id,g:v,base:w,flow:k,drawn:!1},this.edges.set(r,l);let g=this.first?260+(s.get(c)??0)*120:0;w.style.transitionDelay=`${g}ms, 0ms`}a.when&&a.when.step===c&&!l.cond&&(l.cond=document.createElement("div"),l.cond.className="cg-cond",l.cond.textContent=Q(a.when),this.stage.appendChild(l.cond));let f=t.get(c),m="idle";if(e.run?a.status==="skipped"?m="skip":L(f.status)&&a.status==="running"?m="active":L(f.status)&&L(a.status)&&(m="done"):!e.ok&&a.denied&&(m="bad"),l.g.setAttribute("class",`e-${m}`),l.base.setAttribute("marker-end",`url(#${this.id}-ar-${m})`),l.cond){let v=e.run?.decision;l.cond.classList.toggle("on",!!v&&/^grant/.test(v)&&!a.when?.negate),l.cond.classList.toggle("off",a.status==="skipped")}}}for(let[a,n]of this.edges)p.has(a)||(this.edges.delete(a),n.g.classList.add("gone"),n.cond?.classList.add("gone"),setTimeout(()=>{n.g.remove(),n.cond?.remove()},V))}drawEdges(){let e=new Map,s=new Map;for(let t of this.edges.values())(e.get(t.from)??e.set(t.from,[]).get(t.from)).push(t),(s.get(t.to)??s.set(t.to,[]).get(t.to)).push(t);let o=(t,i,d,p)=>{let a=[...t].sort((l,f)=>(p(l)?.y??0)-(p(f)?.y??0)),n=a.length,c=a.indexOf(i),r=n>1?Math.min(9,d.h*.6/(n-1)):0;return d.y+d.h/2+(c-(n-1)/2)*r};for(let t of this.edges.values()){let i=this.nodes.get(t.from),d=this.nodes.get(t.to);if(!i||!d)continue;let p=i.x+(i.el.offsetWidth||i.w),a=d.x-3,n=o(e.get(t.from),t,i,f=>this.nodes.get(f.to)),c=o(s.get(t.to),t,d,f=>this.nodes.get(f.from)),r=Math.max(26,(a-p)*.5),l=`M${p},${n} C${p+r},${n} ${a-r},${c} ${a},${c}`;t.base.setAttribute("d",l),t.flow.setAttribute("d",l),t.cond&&(t.cond.style.left=`${(p+a)/2}px`,t.cond.style.top=`${(n+c)/2}px`),t.drawn||(t.drawn=!0,requestAnimationFrame(()=>requestAnimationFrame(()=>t.base.classList.add("drawn"))))}}layout(e){let s=this.g,o=new Map(this.shown.map(g=>[g.id,g])),t=s.layers.map(g=>g.filter(y=>o.has(y))).filter(g=>g.length),i=new Set(t.flat()),d=this.shown.map(g=>g.id).filter(g=>!i.has(g));if(d.length&&t.push(d),this.canvas.classList.toggle("empty",!t.length),this.stage.style.transition=e?"":"none",!t.length){this.canvas.style.height="84px";return}for(let g of this.nodes.values())g.h=g.el.offsetHeight,g.w=g.wide?P:N;let p=t.map(g=>g.reduce((y,E)=>y+this.nodes.get(E).h,0)+(g.length-1)*q),a=[],n=S;for(let g of t)a.push(n),n+=Math.max(...g.map(y=>this.nodes.get(y).w))+C;n+=S-C;let c=Math.max(...p,60),r=new Map;t.forEach((g,y)=>{let E=g.map((b,$)=>{let H=(o.get(b).deps??[]).filter(T=>r.has(T));return{id:b,i:$,bc:H.length?H.reduce((T,j)=>T+r.get(j),0)/H.length:1/0}});y>0&&E.sort((b,$)=>b.bc===$.bc?b.i-$.i:b.bc-$.bc);let x=S+(c-p[y])/2;for(let{id:b}of E){let $=this.nodes.get(b);$.tx=a[y],$.ty=x,r.set(b,x+$.h/2),x+=$.h+q}});let l=c+S*2,f=this.canvas.clientWidth||n,m=Math.min(1,f/n),v=Math.max(0,(f-n*m)/2);this.stage.style.width=`${n}px`,this.stage.style.height=`${l}px`,this.svg.setAttribute("width",String(n)),this.svg.setAttribute("height",String(l)),this.stage.style.transform=`translate(${v}px,0) scale(${m})`,this.canvas.style.height=`${Math.ceil(l*m)}px`,cancelAnimationFrame(this.frame);for(let g of this.nodes.values())(!g.placed||!e)&&(g.x=g.tx,g.y=g.ty,g.placed=!0),g.fx=g.x,g.fy=g.y;let w=performance.now(),k=g=>{let y=e?Math.min(1,(g-w)/O):1,E=Y(y);for(let x of this.nodes.values())x.x=x.fx+(x.tx-x.fx)*E,x.y=x.fy+(x.ty-x.fy)*E,x.slot.style.transform=`translate(${x.x}px,${x.y}px)`;this.drawEdges(),y<1&&(this.frame=requestAnimationFrame(k))};k(w)}renderFoot(e){let s="";if(e.verify){let o=this.first?700:0;s=`<div class="cg-checks">${e.verify.checks.map((t,i)=>`<div class="cg-check${t.ok?"":" no"}" style="animation-delay:${o+i*90}ms" title="${u(t.detail)}"><b>${t.ok?M:D}</b><span>${u(t.name)}</span></div>`).join("")}</div>${e.verify.checks.filter(t=>!t.ok).map(t=>{let i=t.items??[],d=i.filter(p=>p.includes("\u2717"));return(d.length?d:i.length?i:[t.detail]).map(p=>`<div class="cg-prob">${u(`${t.name}\uFF1A${p}`)}</div>`).join("")}).join("")}`}else!e.ok&&e.problems?.length&&(s=e.problems.map((o,t)=>`<div class="cg-prob" style="animation-delay:${(this.first?600:0)+t*80}ms">${u(o)}</div>`).join(""));s!==this.footSig&&(this.footSig=s,this.foot.innerHTML=s)}toggleGate(){if(this.gate){this.closeGate();return}let e=this.g,s=document.createElement("form");s.className="cg-gate",s.innerHTML=`<span class="who">\u4EE5 <b>alice</b>\uFF08\u5BA2\u6237\u7ECF\u7406\uFF09\u6279\u51C6</span>
      <input type="password" placeholder="\u5BC6\u7801" autocomplete="current-password" aria-label="\u5BC6\u7801">
      <button class="cg-btn" type="submit">\u6279\u51C6\u6267\u884C</button>
      <span class="msg"></span>
      <span class="hash" title="${u(e.hash)}">\u7ED1\u5B9A ${u(e.hash.replace("sha256:","").slice(0,8))}</span>`;let o=s.querySelector("input"),t=s.querySelector("button"),i=s.querySelector(".msg");s.addEventListener("click",d=>d.stopPropagation()),s.addEventListener("submit",async d=>{if(d.preventDefault(),!o.value){o.focus();return}t.classList.add("busy"),t.textContent="\u6279\u51C6\u4E2D\u2026",i.textContent="";let p=await this.opts.approve(this.g,"alice",o.value).catch(a=>String(a));if(p){t.classList.remove("busy"),t.textContent="\u6279\u51C6\u6267\u884C",i.textContent=p,s.classList.remove("shake"),s.offsetWidth,s.classList.add("shake"),o.select();return}s.classList.add("done"),s.innerHTML=`<span class="ok">${M}\u5DF2\u6279\u51C6\uFF0C\u5F00\u59CB\u6267\u884C</span>`,this.opts.onApproved?.(),setTimeout(()=>this.closeGate(),1600)}),this.head.after(s),this.gate=s,o.focus()}closeGate(){if(!this.gate)return;let e=this.gate;this.gate=void 0,e.classList.add("leave"),setTimeout(()=>e.remove(),260)}togglePop(e,s=!1){if(this.pop&&this.pop.dataset.id===e&&!s){this.closePop();return}let o=this.g?.nodes.find(l=>l.id===e),t=this.nodes.get(e);if(!o||!t){this.closePop();return}let i=this.pop??document.createElement("div"),d=!this.pop;i.className="cg-pop",i.dataset.id=e;let p=o.role&&o.role!==o.name?`<div class="role">${u(o.role)}</div>`:"";i.innerHTML=`<h4>${u(o.name)}</h4>${p}<p>${u(o.task)}</p>${o.denied&&!this.g.ok?`<p style="color:var(--red)">${u(o.denied)}</p>`:""}${o.reads?.length?`<div class="k">\u8BFB\u53D6</div>${o.reads.map(l=>`<code>${u(l)}</code>`).join("")}`:""}${o.contract?`<div class="k">\u5951\u7EA6</div><code>${u(o.contract)}</code>`:""}${o.output?`<div class="k">\u4EA7\u7269</div><div class="out">${u(o.output)}</div>`:""}`,d?(this.el.appendChild(i),this.pop=i):s||(i.style.animation="none",i.offsetWidth,i.style.animation="");let a=t.el.getBoundingClientRect(),n=this.el.getBoundingClientRect(),c=Math.min(Math.max(8,a.left-n.left),n.width-i.offsetWidth-8),r=a.bottom-n.top+8;r+i.offsetHeight>n.height-4&&(r=Math.max(4,a.top-n.top-i.offsetHeight-8)),i.style.left=`${c}px`,i.style.top=`${r}px`}closePop(){this.pop?.remove(),this.pop=void 0}};function F(h,e,s=i=>i,o=1200,t=!1){let i=!1,d=0,p=0,a=e.live,n=async()=>{if(!(i||!a)){try{let c=await fetch(a,{cache:"no-store"});if(!c.ok)throw new Error(String(c.status));let r=await c.json();if(i)return;p=0;for(let l of["live","confirm","approve"])r[l]&&(r[l]=s(r[l]));r.run&&(r.run.url=s(r.run.url)),h.render(r),a=r.live}catch{if(++p>30)return}a&&!i&&(d=window.setTimeout(n,o))}};return a&&(d=window.setTimeout(n,t?0:o)),()=>{i=!0,clearTimeout(d)}}for(let h of document.querySelectorAll("[data-graph]")){let e=new z(h,{page:!0});fetch(h.dataset.graph,{cache:"no-store"}).then(s=>s.json()).then(s=>{e.render(s),F(e,s)})}})();
