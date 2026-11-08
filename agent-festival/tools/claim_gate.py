#!/usr/bin/env python3
"""Claim gate: counting-closure claims must carry command evidence nearby.

The write-path discipline (runbook §五 rule 1: 数字先跑后写) in mechanical
form. Historical incidents this gate family exists for (all real, all in
this campaign): 56号「17 个新提交」(actual 18), 「12 测试全绿」(actual 11),
61号 command missing `^` (number right, command rebuilt from memory), 64号
「11 处+1 删除」(actual 151 deletions) — every one a count written from
memory with no command run at write time.

Usage:
    python3 tools/claim_gate.py FILE...           # list violations, rc=1 if any
    python3 tools/claim_gate.py --check FILE...   # same, quiet (violations only)

FORM-LIMITED (clause ⑤ family — a text peg, boundaries recorded): it scans
for counting-closure patterns (恰 N 处 / 共 N / N 个提交 / N 条 PASS /
M=11 D=151 A=10 …) and requires an executable-looking command within the
bidirectional ±8-line evidence window. It CANNOT judge whether the command
actually produces the number — that remains the writer's write-time duty
and the reviewer's rerun duty (presence-form only). Non-counting claims
(dates, ids, blob prefixes inside tables that carry commands) are out of
scope. False positives are possible; the gate's verdict is
"flag for human look", never "silently trust".

Evasion boundaries below are SINGULARITY-VERIFIED (2026-11-09 attack run;
regression-checked by --selftest — future gate edits must not silently
reopen a CLOSED form):
  CLOSED in v1.2 — bare-tool backtick (`git` in prose is not evidence;
    the backtick form requires whitespace + argument after the tool name),
    spaced triples ("M = 11"), round-label noise (R74 条款 / iter-3 / 1-3 条
    via lookbehind).
  CLOSED in v1.3 (R76 M-1/M-2) — the unbounded-fence window (fence scope is
    plain ±8 everywhere now; the special-case branch is deleted, so the bug
    class is inexpressible); the unit-family escape (10 unit branches —
    项/次/类/组/波/枚/篇/包/个/PASS — 8 with real-doc carriers under GATE
    semantics: 连续 2 次, 24 个 B/M, 3 PASS 4 FAIL; 波/篇 carry zero gate-
    visible lines — 波's only text form 「N3 波」 is lookbehind-blocked,
    篇 has no raw hit; each pinned); ordinal/slash-list noise
    (第 3 项 / 3/7 项 via lookbehind).
  CLOSED in v1.4/v1.5 (R76 iter-1 B-1 + iter-2 M-2/M-3) — the OUTPUT-ROLE
    structure: a "# → …" pasted-output line takes evidence from the
    generating command ANYWHERE in the SAME fence; a "# →" line is skipped
    ONLY when the fence carries a command — otherwise it falls back to the
    ±8 window and flags when no command is in scope either (the忘贴命令
    disease form; v1.3's unconditional # skip silently deleted that red
    face — pinned). The arrow requirement separates output annotations
    from fence-desynced prose headings and far no-arrow # lines (pinned by
    its own case: deleting the arrow condition flips it). (History note:
    v1.2 never flagged pasted outputs — the unbounded fence let far
    commands satisfy them; the "FP family" framing of the v1.3 skip was
    wrong and is retracted.)
  CLOSED in v1.6 (R76 iter-6 M-3) — the bare compound nouns
    (测试/函数/删除/新增/提交/新提交) entered the unit set and the dead
    个X/处差异/条PASS branches retired: the gate's OWN motivating disease
    form 「12 测试全绿」 (56号: claimed 12, actual 11) previously escaped
    because 个测试 required the 个 prefix; now pinned by its own selftest
    case. Pin census is DURABLE: `--census` deletes each live unit and
    reruns --selftest (21/21 pinned at v1.6).
  CLOSED in v1.7/v1.8 (R77 M-1 + iter-1 B-1 honest rewrite) — the .-adjacent
    FP family (section numbers 「### 3.3 新增」 etc., 20 gate-flag carriers (heading form 4 in impl + 3 in proposal),
    v1.7 closed via `.` in the lookbehind class); the chmod-mode analogy
    family 「0600 类比」 — v1.7's `=` extension was a FALSE closure (killed
    only 1/13 gate-visible carriers; the ×14 count was raw-grep, ㉜ family
    violation — iter-1 B-1); v1.8 closes it structurally: a 类 unit followed
    by 比 is code-level skipped (prefix-independent), and `=` is REMOVED
    from the class — the X=N count form 「31=39 条」 is a TRUE catch
    (minor-ledger:1229) that v1.7 had silently let escape (iter-1 M-1)
  STILL OPEN (advisory, documented — the closed-set boundary record, both
    faces: killed FPs and newly-opened escapes) — the severity-count idiom
    (「0B+2M+8m」「B=1 M=2」) is OUTSIDE the counting domain by adjudication:
    its evidence is the blank-sub round report, not a command-reproducible
    count (rule-1's domain is command-derivable counts; severity counts in
    outbound docs ride rule-2/reviewer rerun — recorded exemption, not
    oversight; note 「M=2」 half-faces via TRIPLE, B/m postfix forms never);
    non-command FALSE-POSITIVE
    forms: pointer claims (「见 §X」 — evidence by reference is not seen),
    label forms (「N6 件」 — item labels), quote forms (transcribing another
    party's historical false claim); closure words without
    digits (「零差异/全部一致/无差异」), Chinese-numeral counts (「三条」),
    English counts ("21 items"), N/M progress forms (no-space "已清偿
    3/7 项" escapes — the numerator dies on unit adjacency (digit followed
    by /), the denominator on the lookbehind; the spaced "3 / 7 项" HAS
    its denominator caught), ordinal
    residuals with double/full-width spaces ("第  3 项"), colon triples
    ("M:11"), command-prefix paste forms ("$ git …" / "- git …" not
    recognized → false positives), and the presence-form limit: an
    unrelated nearby command (git status) satisfies the window; the unit
    table itself is a closed set — units outside it (e.g. 「N 批」「N 步」)
    escape by definition; extending it requires a selftest case per new
    unit.
  OPEN faces CREATED by v1.3–v1.8 (the boundary record's third face — v1.8's
    unscoped 比-skip briefly let ANY unit+比 escape (「3 组比较」「7 条比值」)
    until iter-2 M-1 unit-scoped it in-version (zero real-doc escape
    carriers — closed in-version; the spaced 「类 比」 form is not skipped,
    zero carriers, known FP form; the 「N 类比」 REAL-count form (类比 as a
    bare compound count noun, the 12 测试 disease family) is also swallowed
    by the skip — zero real-doc carriers today, advisory record, known
    escape face; the skip's CHAR-level trigger (类+比) is undecidable against
    Chinese segmentation — a real 「N 类」 count followed by any 比-initial
    word (比较/比值/比重/比对) is also swallowed (「这 3 类比较重要」 → None):
    zero real-doc carriers (all 14 swallowed lines are the 0600 family —
    no-skip mutant vs v1.8 full-repo diff), the 比-initial word family
    adjudicated (类比=skip target; 比较/比值/比重/比对=real-count adjacency,
    recorded advisory — R78 M-1); v1.7's `.` also opened a DECIMAL-adjacent escape face
    (「均值 2.5 处」「平均 3.5 项」 — the fractional digit after . is blocked,
    where v1.6 caught the 「5 处」 part) — 2 real-doc carriers at MATCH
    granularity (52:549 「3.5 个」, proposal/03:66 「2.6 个」); at LINE
    granularity zero misses (both lines stay flagged via sibling matches —
    no silent leak); the 20 figure = flag-killed line count over the
    all-repo md scan domain (recount: rerun the gate with `.` removed from
    the class), structurally inseparable from the FP closure (same
    lookbehind char), recorded advisory);
    recorded, not hidden; the header's version domain must roll with every
    extension, ㉙ rolling rule) — the v1.6 bare-compound tightening:
    测试/函数/删除/新增/提交/新提交 entering the set newly flags legacy
    lines v1.2 passed (real-doc instances: 「2 新增」「5 新增」「10 测试」
    「12 测试」「3 提交」「46 提交」「7 函数」「1 删除」 across impl docs —
    quote/summary forms, some genuine catch candidates); the v1.3 unit-family
    tightening: 包/PASS entering
    the unit set newly flags legacy quote/summary lines that v1.2 passed
    (real-doc instances: 「21 包」×2 /「20 包」/「1 PASS」 in 60/64号 — quote
    and summary-position forms; some are genuine §五 rule-1 catch
    candidates); v1.3 also KILLED the 「第 4 件」 ordinal FP (the
    killed-FP face, recorded — same 第-lookbehind batch as the ordinal-noise
    closure above: same mechanism, same version, iter-4 M-2 fixed the
    mis-attribution); the SELF-DYED output-role escape: a real claim
    written as "# → …" inside a command-bearing fence is exempt
    (indistinguishable from a genuine pasted output; adversarial-only
    form); the bare-line tightening FP: a counting line WITHOUT a # prefix
    far from its fence's command now flags where v1.2 passed (±8 holds
    inside fences too — intended, recorded); the FENCE exemption is
    presence-form at fence granularity (ANY command in the fence
    satisfies it, related or not); nested four-backtick fences flip the
    pairing and can misflag legit outputs (zero usage in real docs);
    ASCII arrows ("# ->") are not exempted (zero usage).
"""
import re
import sys
from pathlib import Path

COUNTING = re.compile(
    r"(恰|共|总)?\s*(?<![A-Za-z0-9-/.])(?<!第)(?<!第 )\d+\s*(处|件|条|项|次|类|组|波|枚|篇|包|个|轮|行|测试|函数|删除|新增|新提交|提交|PASS)"
)  # lookbehind kills label/range/ordinal/slash-list noise (R74 条款 / 1-3 条 / 第 3 项 / 3/7 项);
   # v1.3 (R76 M-2): unit family extended — 项/次/类/组/波/枚/篇/包/个/PASS were a whole
   # escaping family (10 unit branches, 8 with real-doc carriers: 连续 2 次 / 24 个 B/M / 3 PASS);
   # v1.6 (R76 iter-6 M-3): bare compound nouns (测试/函数/删除/新增/提交/新提交) entered the
   # set and the dead 个X/处差异/条PASS branches retired (换结构: unreachable branches made
   # inexpressible) — the gate's own motivating disease form "12 测试全绿" (56号, actual 11)
   # now matches
TRIPLE = re.compile(r"\b[MDA]\s*=\s*\d+")
CMD = re.compile(
    r"^\s*(git |go |grep |python3 |curl |wc |diff |cd |ls |find )"
    r"|`(git|go|grep|python3|curl|wc|diff|cd)\s+[^`]*`"
)
WINDOW = 8  # evidence-command window, ±WINDOW lines BOTH directions


def find_claim(line: str):
    """COUNTING with the analogy-compound skip: a 类 unit directly followed
    by 比 (「0600 类比」 chmod-mode analogies) is not a count — the match is
    skipped and the scan continues (v1.8 structural fix; kept code-level so
    the alternation stays parenthesis-free and UNITS derivation holds)."""
    pos = 0
    while True:
        m = COUNTING.search(line, pos)
        if not m:
            return TRIPLE.search(line)
        # unit-scoped: ONLY a 类 unit directly followed by 比 (「0600 类比」);
        # any OTHER unit + 比 (「3 组比较」「7 条比值」) is a REAL count and
        # must NOT be skipped (R77 iter-2 M-1 — the unscoped skip was an
        # over-broad exemption; pinned by the 组比较 selftest case)
        if line[m.end():m.end() + 1] == "比" and line[m.end() - 1] == "类":
            pos = m.end()
            continue
        return m


def violations(path: str) -> list[str]:
    lines = Path(path).read_text(encoding="utf-8").splitlines()
    # fence spans (an unclosed fence extends to the last line — desync-safe)
    fences = []
    start = None
    for i, line in enumerate(lines):
        if line.strip().startswith("```"):
            if start is None:
                start = i
            else:
                fences.append((start, i))
                start = None
    if start is not None:
        fences.append((start, len(lines) - 1))

    def fence_at(idx):
        for s, e in fences:
            if s <= idx <= e:
                return (s, e)
        return None

    hits = []
    for i, line in enumerate(lines):
        if line.strip().startswith("```"):
            continue  # fence markers are never claims
        span = fence_at(i)
        if span and line.lstrip().startswith("#") and "→" in line:
            # OUTPUT role (R76 iter-1 B-1 fix — the unconditional v1.3 skip was
            # a silent guard regression): a "# → …" pasted-output line's evidence
            # is the generating command ANYWHERE in the same fence; a "# →" line
            # in a command-less fence is the忘贴命令 disease form and is NOT
            # skipped. The arrow requirement also stops fence-desynced prose
            # headings ("# 恰 7 处差异") from being mis-skipped as output.
            if any(CMD.search(l) for l in lines[span[0]:span[1] + 1]):
                continue
        claim = find_claim(line)
        if not claim:
            continue
        # evidence window: ±WINDOW lines BOTH directions, fence or not
        # (v1.3 / R76 M-1: the unbounded-fence special case is REMOVED — the
        # declared ±8 semantics now hold in every branch, pinned by selftest)
        scope = lines[max(0, i - WINDOW):i + WINDOW + 1]
        if not any(CMD.search(l) for l in scope):
            hits.append(f"{path}:{i + 1}: count claim {claim.group(0)!r} without a command in the ±{WINDOW}-line window: {line.strip()[:90]}")
    return hits


SELFTEST = [
    # (text, should_flag) — the executable boundary spec; singularity 2026-11-09.
    ("本批与基线子树零差异，全部一致。", False),          # OPEN: closure words without digits
    ("经 `git` 复核，恰 7 处差异。", True),               # CLOSED v1.2: bare backtick ≠ evidence
    ("git status\n本批共 9 项新增。", False),            # OPEN: presence-form (unrelated cmd satisfies)
    ("汇总 M = 11、D = 151。", True),                    # CLOSED v1.2: spaced triple
    ("the batch adds 21 items.", False),                 # OPEN: English counts
    ("本轮合计 5 处修改。", True),                       # control: the basic catch
    ("R74 条款与 iter-3 条款如下：", False),             # CLOSED v1.2: round-label lookbehind
    ("（三条，全采纳）", False),                          # OPEN: Chinese numerals
    ("M=11 D=151 A=10", True),                           # triple without nearby command
    ("```\ngit rev-parse HEAD\nx\n恰 7 处差异\n```", False),   # fence, near command (≤8) → pass
    ("```\ngit rev-parse HEAD\n" + "x\n" * 12 + "恰 7 处差异\n```", True),  # fence, far command (>8) → flag (R76 M-1 pins ±8 inside fences)
    ("连续 2 次 0B/0M。", True),                          # v1.3: 次 unit (was escaping)
    ("本批新增 3 项。", True),                             # v1.3: 项 unit (was escaping)
    ("第 3 项与第 7 项如下。", False),                      # ordinal form (lookbehind 第/第␣)
    ("第3 项与第7 项如下。", False),                        # NO-SPACE ordinal (R77 M-2: the (?<!第) component's own pin — deleting it flips this)
    ("controlled_handoff=0600 类比。", False),              # chmod-mode analogy family — blocked by the code-level find_claim skip (类 unit + 比, v1.8), prefix irrelevant
    ("chmod 0600 类比。", False),                            # the SPACE-prefix form (v1.7's = extension only killed 1/13 carriers — iter-1 B-1; this pins the structural fix)
    ("本批 3 组比较。", True),                             # OVER-SCOPE pin (iter-2 M-1): a non-类 unit + 比 is a REAL count — the skip must never eat it
    ("chmod 0600 类比，本批 3 处新增。", True),            # skip-THEN-CONTINUE pin (iter-5 M-2): the exemption must not swallow the line's remaining real counts (the continue→return-None mutant kills this)
    ("0600 类比 M=11 D=151。", True),                      # skip then TRIPLE fallback on the same line — same mutant's second victim
    ("外层 8+内环 31=39 条。", True),                        # X=N count form (R77 iter-1 M-1: v1.7's = created an escape face — the class keeps = OUT; a TRUE catch restored)
    ("### 3.3 新增。", False),                              # .-adjacent FP family — 20 gate-flag carriers (heading form 4 impl + 3 proposal; the ×6 was an uncalibrated raw count, iter-4 M-1), blocked by . in the class
    ("检查 3/7 项。", False),                              # slash-list form (lookbehind /)
    ("```\npython3 -c 'join'\n# → M=11 D=151 A=10\n```", False),  # output role: command in SAME fence
    ("```\ngit rev-parse HEAD\n" + "x\n" * 20 + "# → 恰 7 处差异\n```", False),  # far output, command-bearing fence → exempt (PIN: deleting the skip flips this)
    ("```\n# → M=11 D=151 A=10\n```", True),               # output form in a COMMAND-LESS fence = 忘贴命令 disease (v1.3 regression face, now caught)
    ("```\nx\n# 恰 7 处差异\n", True),                     # fence-desynced heading, no arrow → scanned → flag
    ("```\ngit rev-parse HEAD\n" + "x\n" * 20 + "# 恰 7 处差异\n```", True),  # PINS THE ARROW: command-bearing fence + far no-arrow # line → claim, not output
    ("本批 3 类。", True),                                   # v1.3 unit, pinned (R76 iter-1 M-1)
    ("本批 3 组。", True),
    ("本批 3 波。", True),
    ("本批 3 枚。", True),
    ("本批 3 篇。", True),
    ("共 12 个包。", True),
    ("本批共 21 包。", True),                              # bare-包 pin (R76 iter-4 M-1): the 12个包 case's leftmost match eats 个, deleting 包 stayed green — this mirrors the real-doc carrier form
    ("本批 3 件。", True),                                 # legacy-unit pins (R76 iter-5 M-3): 件/条/轮/行 were live but unpinned
    ("新增 5 条。", True),
    ("审查 2 轮。", True),
    ("改动 4 行。", True),
    ("12 测试全绿。", True),                              # the GATE'S OWN motivating disease form (56号: claimed 12, actual 11) — was escaping (R76 iter-6 M-3)
    ("3 函数。", True),
    ("共 9 删除。", True),
    ("共 3 新增。", True),
    ("3 新提交。", True),
    ("本批 5 提交。", True),
    ("本批 3 个。", True),
    ("结果：3 PASS 4 FAIL。", True),
]


# UNITS is DERIVED from COUNTING's own alternation (R76 iter-7 M-3: a
# hand-maintained mirror is a second truth source — regex ⊃ mirror drifts
# SILENT GREEN; deriving makes the fifth-write-site form inexpressible)
UNITS = re.search(r"\(([^()]*)\)\s*$", COUNTING.pattern).group(1).split("|")


def _census() -> int:
    """Pin census (durable form of the ㉜ discipline): delete each live unit
    from COUNTING, run --selftest, count drift lines. Every unit must have
    >=1 pin flip; exit 1 otherwise. Rerunnable: the census claim in the
    runbook points HERE, not at a pasted blob in /tmp."""
    import os
    import subprocess
    import tempfile
    base = Path(__file__).read_text(encoding="utf-8")
    unpinned = []
    for u in UNITS:
        if u == "处":
            token, repl = "(处|", "("
        elif u == "PASS":
            token, repl = "|PASS)", ")"
        else:
            token, repl = f"|{u}|", "|"
        mut = base.replace(token, repl, 1)
        if mut == base:
            print(f"census: unit {u!r} anchor not found")
            unpinned.append(u)
            continue
        with tempfile.NamedTemporaryFile("w", suffix=".py", delete=False) as f:
            f.write(mut)
            path = f.name
        r = subprocess.run([sys.executable, path, "--selftest"], capture_output=True, text=True)
        os.unlink(path)
        flips = sum(1 for l in r.stdout.splitlines() if l.startswith("selftest FAIL"))
        print(f"census: delete {u!r} -> {flips} pin-case flip(s)")
        if flips == 0:
            unpinned.append(u)
    print(f"census: {'ALL ' + str(len(UNITS)) + ' live units pinned' if not unpinned else 'UNPINNED: ' + repr(unpinned)}")
    return 1 if unpinned else 0


def _selftest() -> int:
    import tempfile
    bad = []
    for text, want in SELFTEST:
        with tempfile.NamedTemporaryFile("w", suffix=".md", delete=False) as f:
            f.write(text + "\n")
            path = f.name
        got = bool(violations(path))
        mark = "ok " if got == want else "FAIL"
        if got != want:
            bad.append(text[:30])
        print(f"selftest {mark} want_flag={want} got_flag={got} :: {text[:40]!r}")
        import os
        os.unlink(path)
    if bad:
        print(f"selftest: {len(bad)} case(s) drifted from the boundary spec")
        return 1
    print("selftest: all boundary cases hold")
    return 0


def main() -> int:
    if "--selftest" in sys.argv[1:]:
        return _selftest()
    if "--census" in sys.argv[1:]:
        return _census()
    args = [a for a in sys.argv[1:] if a != "--check"]
    quiet = "--check" in sys.argv[1:]
    hits = []
    for a in args:
        hits += violations(a)
    if hits:
        if not quiet:
            print(f"claim gate: {len(hits)} counting claim(s) lack nearby command evidence:")
        for h in hits:
            print(h)
        return 1
    if not quiet:
        print(f"claim gate: clean ({len(args)} file(s))")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
