// Styles for the plan graph, injected once per document.
export const css = String.raw`
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
`;
