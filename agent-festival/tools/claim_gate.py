#!/usr/bin/env python3
"""Claim gate v2.7.1: counting-closure claims must carry command evidence nearby.

The write-path discipline (runbook §五 rule 1: 数字先跑后写) in mechanical
form. Historical incidents this gate family exists for (all real, all in
this campaign): 56号「17 个新提交」(actual 18), 「12 测试全绿」(actual 11),
61号 command missing `^` (number right, command rebuilt from memory), 64号
「11 处+1 删除」(actual 151 deletions) — every one a count written from
memory with no command run at write time.

Current execution scope (v2.7.1, unchanged from v2.6): only constrained
wc/grep commands may be replayed. All git commands are reported as
SKIPPED-UNSAFE (git-replay-disabled), never executed. v2.6.1–v2.6.2
(detection-set bugfixes: CMD branch single-source, scoped rerun-selection
fix) land via the engineering chamber; v2.7/v2.7.1 add the detection-set
extensions (+样本/用例 units, PROGRESS completion claims, 零样本 guard —
the adjudication chamber, with inner-loop remediation). The execution
surface is identical to v2.6. Historical version notes below describe
older behavior, not the current execution surface.

Usage:
    python3 tools/claim_gate.py FILE...           # list violations, rc=1 if any
    python3 tools/claim_gate.py --check FILE...   # same, quiet (violations only)
    python3 tools/claim_gate.py --rerun FILE...   # tier 1 (v2.0, default OFF):
        for every claim line that DOES carry an evidence command in the ±8
        window, extract the command text, execute it (argv, no shell, read-only
        whitelist, 10 s per command, cwd = the document's own directory) and
        compare against the pasted output block in the document. Failure /
        value drift / no pasted block = RERUN-MISMATCH. Non-whitelisted /
        write-effect / network / shell-metachar forms = SKIPPED-UNSAFE: they
        are NOT executed and are NOT errors. v2.1 adds the OPERAND FENCE: a
        whitelisted command whose file operands are absolute-path / ..-going
        / symlink-escaping the document's own directory is also SKIPPED-UNSAFE
        (outside-docdir) — not executed, not echoed; and the value-drift line
        no longer echoes executed output content (sha256-prefix instead).
        v2.2 closes the F-1 bypass of that fence (inner-audit): a '-'-leading
        flag token carrying a path (-f/PATH, --file=PATH, -f~, -f..) is no
        longer
        skipped — it refuses as SKIPPED-UNSAFE (flag-path-form). The v2.1
        "not executed" sentence was FALSE for those forms as shipped (see the
        annotated M3 entry below).
        RERUN_TIMEOUT=<sec> overrides the per-command timeout. --rerun prints
        the default verdicts first and leaves them byte-identical; rc=1 when
        violations OR mismatches exist. Zero default hits print the v1.8 clean
        line "claim gate: clean (N file(s))" (v2.1); the --rerun summary ends
        with an explicit "executed=K/SKIPPED=J" count so "0 OK + 0 MISMATCH
        because nothing ever ran" is machine-visible without touching rc.
    python3 tools/claim_gate.py --selftest        # boundary spec, exit 1 on drift
    python3 tools/claim_gate.py --census          # four axes: units (23) × pin,
                                                  # execution whitelist (2 live; git retired) × pin,
                                                  # refusal reasons (13) × pin,
                                                  # grammar table entries (5) ×
                                                  # pin (v2.3 added the reasons
                                                  # axis, v2.5 the grammar axis;
                                                  # member counts are frozen
                                                  # tuples, derived and diffed —
                                                  # this help line, like every
                                                  # count in this file, is
                                                  # re-verified by the run, not
                                                  # by memory); axis-labelled,
                                                  # no shrink-green

FORM-LIMITED (clause ⑤ family — a text peg, boundaries recorded): the DEFAULT
path scans for counting-closure patterns (恰 N 处 / 共 N / N 个提交 / N 条 PASS /
M=11 D=151 A=10 / 十行 / 十二条 …) and requires an executable-looking command
within the bidirectional ±8-line evidence window. It CANNOT judge whether the
command actually produces the claimed number — that remains the writer's
write-time duty and the reviewer's rerun duty (presence-form only). The v2.0
--rerun tier narrows the second half of that sentence ONLY OPTIONALLY and ONLY
for a whitelisted command shape with a pasted block to compare against: it
verifies COMMAND-vs-PASTE fidelity (is the paste still what the command prints
now), never CLAIM-vs-COMMAND (does the paste equal the number in the prose —
that comparison stays human/reviewer work, pinned by RR07 where a sha paste is
"equal" while the count claim next to it is unrelated by construction).
Non-counting claims (dates, ids, blob prefixes inside tables that carry
commands) are out of scope. False positives are possible; the gate's verdict
is "flag for human look", never "silently trust".

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
    reruns --selftest (21/21 pinned at v1.6; two axes since v2.0).
  CLOSED in v1.7/v1.8 (R77 M-1 + iter-1 B-1 honest rewrite) — the .-adjacent
    FP family (section numbers 「### 3.3 新增」 etc., 20 gate-flag carriers (heading form 4 in impl + 3 in proposal),
    v1.7 closed via `.` in the lookbehind class); the chmod-mode analogy
    family 「0600 类比」 — v1.7's `=` extension was a FALSE closure (killed
    only 1/13 gate-visible carriers; the ×14 count was raw-grep, ㉜ family
    violation — iter-1 B-1); v1.8 closes it structurally: a 类 unit followed
    by 比 is code-level skipped (prefix-independent), and `=` is REMOVED
    from the class — the X=N count form 「31=39 条」 is a TRUE catch
    (minor-ledger:1229) that v1.7 had silently let escape (iter-1 M-1)
  CLOSED in v2.0 (build-bit #2, two tiers) —
    (a) the Chinese-numeral blind spot: 一二三四五六七八九十百 (plus 零〇两
        and multi-char runs like 二十一/两百) entered the COUNTING DIGIT SLOT
        only, sharing the unchanged 21-member units group, so census derives
        21/21 exactly as before (P-Ⅰ shared-group rule: a second units group
        would silently un-pin half the census; P-Ⅲ: exemptions were taken on
        the numeral/lookbehind side, NEVER by touching the unit table);
        the 「十二项」 right-shift FP is blocked by the numeral-backchain
        lookbehind (CN-B4 pins it: deleting the backchain flips it), the
        ordinal forms 第三条 / 第 三条 stay blocked by the existing 第 pair,
        初十 stays blocked by the 初 pair; and the date/time forms the
        campaign feared (三十天 / 十一点 / 三个月 / 百分之三十 / 十分困难)
        are safe by CLOSED-SET arithmetic — 天/点/月/日/年/秒/分/格/档 are
        NOT units, so no guard was needed for them at all
        [v2.1 corrected: see M2 — FALSE for the 三个月 half of this list:
        个 IS a unit, so the duration reading rode the unit and a guard WAS
        needed (M2 added it). m-3 (inner-audit) in-place note: kept the
        historical sentence verbatim instead of leaving the correction only
        in the M2 cross-ref, which a v2.0-standalone reader consumed as
        truth. 天/格/档 and the other class-outsides stay genuinely
        unguarded-safe.] (calibration:
        measured over the 85-file corpus snapshot, 1500 gained lines, every
        one CN-bearing, lost=0, key-position diff; zero carriers for the
        word-family suspects 次方/十项全能/一元二次/十分/百分之 — grepped,
        empty; pins CN-A1..CN-C6).
    (b) the command-vs-paste fidelity tier (--rerun): a claim line whose
        ±WINDOW scope carries a CMD-shaped line is now re-checkable against
        reality instead of only against presence. The evidence selection is
        deterministic (top-down FIRST command in the window, RR13 pins it —
        "nearest" or "runnable-first" would flip that case); the execution
        surface is a NEW closed set (5 members) strictly narrower than the
        detection set CMD (go/python3 build and write, curl is the network:
        detectable evidence is NOT executable evidence), each member pinned
        and census-mutable on its own axis; argv-direct exec, never shell,
        per-command timeout, cwd pinned to the document's directory; every
        refuse branch is a SKIP, never an error (fail-safe direction,
        R2 of the collision table). Execution is gated twice BEYOND the
        tool table: count tools must name an operand (bare 「`wc -l`」 reads
        stdin — a mention, not evidence, RR19) and git log must be bounded
        (bare 「git log」 is unbounded output, RR20 — 35号:3 carrier). Both
        gates entered mid-implementation from the whole-dir face and were
        added as matrix-addendum rows, not as silent tightening.
  CLOSED in v2.1 (R78 fix-A, review-loop outer-r1 M1–M4/N4). [v2.3 honest
    amendment, outer-r2 M-1: the original heading read "each with its own
    permanent probe" — FALSE as written: the probes pinned VERDICTS but not
    refusal-reason labels, and the quote state machine's OUTSIDE-quote
    normalization half had zero pins (the whole-function identity mutant of
    _md_unescape stayed all-green; outer-r2 镜二② P4). The sentence that
    holds: every v2.1–v2.3 fix ships with a probe that pins its verdict AND
    its reason label (want_reason + census AXIS3, v2.3), and the ㉝④ law
    (no silent behaviour change, no silent exemption) always held]:
    (M1) the zero-hit clean line: v2.0 deleted v1.8's "claim gate: clean
        (N file(s))" stdout line (rc=0 but nothing to paste — a zero-hit
        claim became unfalsifiable). Restored byte-equal to v1.8 (quiet
        --check still silent, rc untouched); pinned by a CLI-face selftest
        case (deleting the print flips it).
    (M2) the 个+time-noun duration FP: 「过去三个月无逾期」 flagged as count
        claim '三个' because 个 sits INSIDE the unit set while 月 sits outside
        it — the v2.0 header's "三个月 safe by CLOSED-SET arithmetic" claim
        was false (this M2 entry corrected that sentence IN PLACE in the
        v2.0(a) block above; the v2.1 text cited "L97-100", a bare line
        anchor that had ALREADY drifted onto the v1.7/v1.8 entries —
        outer-r2 m-3: this header now names entries, never line numbers,
        and carries no self-line-count, because line numbers are exactly
        the from-memory counts this gate flags). Fixed structurally, not
        by an exemption table: a trailing zero-width guard on the units
        group — (?!(?<=个)[月日年点时分秒]) — a 个-unit directly followed by a
        time noun is not a count. The guard lives OUTSIDE the units capture
        group so the single-trailing-capture invariant (P-Ⅰ) and the shared
        21-member UNITS stay byte-unchanged; UNITS derivation is now the
        last plain capture group (robust to trailing assertions) and census
        anchors locate units position-independently. Discrimination is
        bidirectional (CLAUDE.md rule 2): 「三个月/3个月」 False ∧ 「三个新文件/
        3 个文件/共十二行」 True ∧ 「两个星期」 still True (星期 is NOT in the
        adjudicated 7-char class — recorded, not silently widened).
    (M3) the execution operand fence: whitelisted tools used to run with ANY
        file operand (outer-r1 proved `wc -l /etc/hosts` executing and the
        value-drift line echoing the outside content into stdout). Every
        non-flag argv token must now be relative, ".."-free, and realpath-
        contained in the document's directory; violations = SKIPPED-UNSAFE
        (outside-docdir). [v2.2 truth-in-words, F-1: this entry read "…
        never executed, never echoed" and that was FALSE AS SHIPPED — the
        fence skipped EVERY '-'-leading token, so a path hidden inside a
        flag (-f/PATH, --file=PATH, -f~, -f..) bypassed all three checks and
        DID execute against docdir-external files (inner-audit F-1: a 1-bit
        read oracle, oracle_hit OK vs oracle_miss rc≠0, sha12 confirming).
        The "never executed, never echoed" sentence holds only UNDER the
        v2.2 flag-path rule (below and in CLOSED v2.2): a flag token
        containing / ~ .. refuses as SKIPPED-UNSAFE (flag-path-form) —
        pinned by RR28/RR29, which go red if the rule is deleted.]
        Execution output is
        no longer echoed on drift: sha256-prefix of actual replaces the
        first-line echo (choice among bool/length/hash: bool cannot
        distinguish drifts across reruns, length leaks a size oracle, the
        prefix hash leaks no content yet stays comparable). Residual faces
        after the fence are recorded below (third face).
    (M4) the 68号 unbalanced-quote over-skip (v2.0 face 3): the naive split
        on && || ; | shredded a QUOTED pipe (grep -cE '^\\*\\*(真裁|知悉)'
        → orphan quote fragments → SKIPPED before shlex, 7/7 on 68号, the
        acceptance ④ "0 mismatch" was vacuous). v2.1: quote-state segment
        splitting (quoted |/;/&& stay inside their token) + markdown-escape
        normalization OUTSIDE shell quotes only (backslash+backtick,
        backslash+dquote, backslash+star → bare star/backtick/quote; the
        68号 '\\*\\*' inside single quotes is regex, NOT markdown, and must
        survive verbatim — inside-quote text is untouched). RR18 FLIPS from
        SKIPPED-UNSAFE to OK exactly as face 3's pin demanded ("a
        quote-parsing tokenizer MUST flip this case and re-adjudicate"),
        RR25 re-pins the full 68号 shape end-to-end, RR26 the quoted-`;`
        twin. Detection semantics (CMD) are unchanged; the 「21 行」-style
        quote carriers on the default face stay FLAGGED-for-adjudicate as
        before — v2.3 note (outer-r2 m-4): the phrase "quote-exemption"
        used here was a MISNOMER, there is no code-level exemption path,
        only the recorded adjudication practice (flag, human looks, doc is
        not a fix target).
    (N4) census structure: both axes are now labelled (AXIS1(units) /
        AXIS2(whitelist)); the derived lists are checked against frozen
        expectations (21 units / 5 members) FIRST, so deleting a member can
        no longer shrink the iterated axis into a false "ALL 4 pinned" green
        — a removal is named in the FAIL line, and both axes print FLAT
        string lists (missing=['PASS'] / missing=['wc -l'] — the tuple
        form ('wc','-l') this entry used to promise was never emitted,
        outer-r2 m-2 corrected) instead of being mis-attributed to a
        neighbouring anchor (the v2.0 last-branch special case made the PASS
        deletion blame 提交). v2.3: a THIRD axis, AXIS3(reasons), inherits
        this exact structure (derived from the source by _derive_reasons,
        diffed against the frozen _EXPECTED_REASONS both ways, missing
        printed flat and named AXIS3 — the member list itself is the
        register the singularity section asked for). Unit anchors are
        located by a unit-boundary regex, the
        first/last special cases are retired — and the locator semantics make
        a caution law: prose quoting a live anchor token would poison the
        first-occurrence search (caught by the site check below, not by
        silence).
    (rc) rc semantics are UNCHANGED (0/1 exactly as v1.8/v2.0 — a change
        here would be a behaviour amendment needing higher authorisation);
        the --rerun summary line gains the executed=K/SKIPPED=J count so a
        consumer can see "vacuous pass" (0 OK + 0 MISMATCH + executed=0)
        without inventing an rc.
  CLOSED in v2.2 (inner-audit F-1, the one M-grade finding of the inner
    round — review-loop discipline: every fix ships with its permanent red
    pin, RR28/RR29): the flag-value bypass of the M3 operand fence.
    _operands_in_scope skipped EVERY '-'-leading token, so a path hidden
    INSIDE a flag — concatenated (`-f/PATH`, `-f~`, `-f../outside/x`) or
    equals-valued (`--file=PATH`, `--exclude-from=PATH`, `--files-from=PATH`) —
    walked past all three fence checks (absolute / ..-going / symlink) and
    the executor ran the whitelisted tool against a file OUTSIDE the
    document's directory, using it as a pattern table (measured on the
    inner round's live probes: `grep -c -f../outside_secret/candidate.txt
    x.txt` OK vs the same against y.txt cmd-failed — a 1-bit
    existence oracle for any path, sha12 echo confirming the value;
    `-f/etc/hosts` and `--file=/etc/hosts` the absolute twins, both OK).
    Fix is in the CONSERVATIVE direction — never execute by mistake: a
    '-'-leading token containing /, ~ or .. now refuses as SKIPPED-UNSAFE
    (flag-path-form), not executed, not echoed. Two verified no-change
    halves: (i) the SPACE-separated form (`-f /etc/hosts`) needed no new
    code — its value is a plain token the pre-existing /-leading non-flag
    rule already refused (live-verified: oob_doc/atkD SKIPPED before and
    after); (ii) pure path-free flags (`--version`, `-c`, `-race`-style
    short clusters, RR09/RR24 shapes) are untouched — all 101 pre-v2.2
    pins staying green IS the regression self-proof, plus the acceptance
    ④ key-diff. The over-refusal this opens (a legitimate flag value
    containing / ~ .., e.g. `--include='src/*.py'`) is recorded as the
    newly-opened safe-direction face below, per the boundary-record law.
  CLOSED in v2.3 (R2 fixR-A: outer-r2 M-1/M-2 + 镜二② 13-face register +
    镜三①-④ residue; the round's own lesson, "a pin must pin the reason"):
    (a) want_reason (the M-2 structural half): every RERUN_CASES tuple is
        now SIX fields with a third slot asserting the refusal LABEL of a
        SKIPPED detail ("(<reason>) …" prefix). 18 existing SKIP pins
        gained their reasons; the 13-member label set is derived from the
        source and census-enforced on AXIS3 — rename/delete any emission
        site ⇒ at least one red pin, named AXIS3. This closes the root
        cause 镜二②#5 blamed for path-disguise ("no reason-string hostage,
        no want ever checked a reason"): deleting the branch now emits
        not-whitelisted ≠ path-disguise and RR04d/RR13 go red.
    (b) the 13 no-pin faces absorbed (face# → holder, each mutation-checked
        out of tree before landing): #1 outside-quote markdown-unescape ⇒
        RR48 (M-1: the P4 identity mutant is no longer silent) + RR47 (the
        state-machine backslash-pair half; a backtick-escape outside quotes
        is BEHAVIOURALLY INVISIBLE — the raw backtick already trips the
        shell-metachar check before and after unescape — so it is 并入
        RR04e, recorded, not fake-pinned); #2 unbalanced-quote refusal ⇒
        RR32 (the face whose header attribution to RR14 was corrected
        above); #3 missing-required-flag ⇒ RR34; #4 _LONG_FLAG long twins
        ⇒ RR35 (verdict-stable BSD/GNU: reaches the executor ⇒ MISMATCH
        either way, while deleting the map ⇒ SKIP — flips under mutation
        on both platforms); #5 path-disguise ⇒ want_reason on RR04d/RR13;
        #6 empty-segment ⇒ RR33, exec-unavailable ⇒ RR46 (the bare-env
        option constructs the branch outer-r2 called unconstructible);
        #7 unclosed-fence extension ⇒ RR38 (the (start,i) pairing routes
        stay pinned by SELFTEST fence cases via _fence_at; the EXTEND line
        had no holder — now it does); #11 the rerun-side "#→" twin ⇒ RR37
        (asserts no-verdict AND no-violation: the "SAME rule as
        violations()" claim is now pinned in BOTH copies, m-14); #12 the
        _block_after stop branches (bare-# / CMD / arrow-join), the
        no-fence-no-medium rule and _split_inline's trailing-# cut ⇒
        RR39/RR40/RR41/RR42/RR43; #13 「（…）」-suffix cut and 「…」-stem
        prefix ⇒ RR44/RR45; #10 the fence-marker skip in violations() ⇒
        one new SELFTEST carrier (m-15). Faces #8 and #9 were NOT pinned
        in place — they were FIXED: see (d).
    (c) dead-code register (镜三①②), selftest count and key diff recorded
        before/after each deletion (all in the fix report): rerun_report()
        (zero callers — main() formats the verdict tuples itself) deleted;
        the shlex except-ValueError layer is replaced by an EXPLICIT
        parity refusal inside _split_segments (an odd trailing-backslash
        run ⇒ None ⇒ the SAME emitted unbalanced-quote) — outer-r2 called
        the layer unreachable; a command ending in one lone backslash says
        otherwise: rather than delete the fail-safe, the state machine now
        ABSORBS its only remaining input, so shlex has no rejection input
        left and the emitted verdict/label byte-face is unchanged where
        reachable.
    (d) M3 fence pre-checks pruned (镜三③, Simplicity): the /-prefix and
        ..-segment checks in _operands_in_scope were behaviour-redundant
        with the realpath containment (same verdict, same label — RR21/22
        now refuse at the realpath site, verified) and are deleted; the
        ~-prefix check STAYS — it is NOT redundant (argv exec never
        tilde-expands, so a planted file literally named ~ is a real
        read target) and is pinned by RR36, which is the proof of the
        asymmetry (deleting only the ~ branch flips RR36 alone; deleting
        the / branch flips nothing because containment catches it —
        redundancy is what the deletion was for).
  CLOSED in v2.5 (fix-25 round, outer-v24 F1–F9 minimal repairs — every item
    below was RED-proofed before landing; the "B-lite: retire the git
    execution face" route was REVOKED in-loop: it is a 主线 105 strategic
    call, and both strategic alternatives (i) refuse-on-exec-capable-config-key
    / (ii) disable the face stay REGISTERED PENDING, not silently adopted):
    (F5/B) the repo-config execution face that v2.4's env block CLAIMED closed
        ("no external diff/textconv, no prompts … must not re-spawn our
        sandbox escapees") was FALSE for the victim repo's OWN .git/config:
        log.showSignature=true + gpg.program=<evil> + a crafted gpgsig commit
        made the whitelisted POSITIVE shape `git log --oneline` re-spawn the
        attacker (outer q3d A: verdict OK, marker=True). Fixed minimally: the
        hardened prefix now carries "-c","log.showSignature=false" (the
        out-of-tree proof in run/gate-showsig-fix.py). The load-bearing half
        is the pin, not the prose: M9 had shown NO existing pin held the
        second layer, so RR69 poisons a hermetic repo the same way, asserts
        verdict OK AND the marker sentinel absent — delete the -c pair and
        RR69 goes red on a real observable effect, not on a string.
    (F7/M) the --no-ext-diff injection was a DEAD branch (argv spy: after the
        v2.4 prefix insert argv[2] is ALWAYS '-c', so the
        `argv[2] in ("log","show","diff")` test never fired — M2=0 FAIL, and
        81 §九 listed the flag as shipped content). The prefix and the
        post-sub flag now come from ONE helper (_git_harden), the diff-capable
        sub receives --no-ext-diff IMMEDIATELY AFTER the sub token (a position
        git actually parses), and the position is pinned structurally
        (ENV-harden-argv-log / ENV-harden-argv-revparse: mutating the insert
        index, dropping the flag, or adding it to a non-diff sub flips them).
    (F1/M) _flag_grammar is fail-CLOSED: a whitelisted tool/sub with no
        grammar table now refuses as "no-grammar-table" (the 15th refusal
        reason; AXIS3 member; RR70 holds its want_reason via the new
        no-table-member opt, which temporarily extends RERUN_READONLY exactly
        the way the outer's fail-open experiment did) and the ENV-grammar-
        invariant pin asserts every RERUN_READONLY member HAS a table — the
        v2.4 disease ("add a member, forget the table ⇒ 0 FAIL and zero
        constraint") now cannot pass green.
    (F2/M) the grammar tables became a CLOSED SET: census AXIS4(grammar)
        derives the 22 entries from the LIVE dicts, diffs them against the
        frozen _EXPECTED_GRAMMAR both ways (member count included — the N4-B
        lesson, fourth axis), then blanks each entry on its own table line and
        requires ≥1 pin flip. The coverage battery's 15 silent acceptance
        entries (real accept-face effect, zero pins — typo/delete shipped
        green while real docs would have flipped to refused) now each carry an
        OK positive pin, RR53–RR68.
    (F6/M) PATH is no longer inherited unchecked: the one variable the
        sanitized env passed through verbatim, including empty/relative
        components — an empty component means "search the CWD", and the CWD is
        the DOCUMENT's directory, so a planted ./git executed under the
        whitelist (outer q3d F6: value-drift line from the attacker's output).
        _run_cmd now filters components to absolute ones and executes only the
        resolution of that filtered PATH by _exe_search — a local execvp-style
        search (no hit ⇒ exec-unavailable SKIP). WHY not shutil.which, said
        out because it is a lesson not a footnote: which resolves a RELATIVE
        component against the GATE's cwd while the child resolves it against
        the DOCUMENT's — so the first version of this fix shipped a pin whose
        plant was invisible to the resolver and mutation ME stayed green on
        the exec side. Pinned by RR71 (PATH=".:<real>" + a planted ./wc that
        touches a marker ⇒ the REAL wc answers, marker absent; ME-unfilter-PATH
        now flips RR71 + ENV-path-clean) and ENV-path-clean (unit half).
    (F8/M) marker/leak evidence left /tmp: --selftest now runs the repo-root
        review_guard_probes.py (邢's verbatim probes, the file the mainline
        shipped as the consumer of this gate) as the PRB-* tier — accepted=
        False ∧ synthetic_marker_written=False ∧ read_outside_synthetic_file=
        False asserted in the permanent suite; a moved copy prints a VISIBLE
        ABSENT line (never silence) and still counts as the run's own record.
    (F9/minor) the v2.4 trio RENUMBERED — RR37-files0-from→RR50-files0-from,
        RR38-ext-diff-p→RR51-ext-diff-p, RR39-cluster-positive→
        RR52-cluster-positive: the shorthand "RR37-39" collided with the
        v2.3-era holders RR37-rerun-arrow-twin / RR38-unclosed-fence /
        RR39-block-stop-hash (full cids were unique; reviewers searching by
        shorthand read the wrong pin). Every bare RR37/RR38/RR39 mention in
        the v2.3-era text of this file means the OLD holders. Convention debt
        (numbering rules) stays 移交登记 4, for the next brief.
    (F3/minor) single-source cleanup: the `--pretty` special case inside
        _flag_grammar is DELETED — the table regex "--pretty.*" is now the one
        truth (they were the same rule written twice; M3 proved deleting
        either half flipped nothing — and the entry itself was listed NO EFFECT
        by the coverage battery until RR68 gave it a holder); `import re as _re`
        is gone (module-level re was already imported — the function-local
        import habit belongs to os/subprocess in this file, not to re).
  OPEN faces CREATED by v2.5 (third-face record — killed / newly opened /
    platform-scoped; all measured this round, none assumed):
    (1) KILLED on the DEFAULT face: none (key diff v2.4→v2.5 = 0, re-measured
        by running both gates over the corpus). On the --rerun face the
        executed SET is unchanged (20 executed rows, verdict lines identical —
        recount by rerunning both versions, never by editing this prose): what
        changed for the 2 executed git rows is only the hardened argv (one
        more -c pair, absolute tool path), never the verdict.
    (2) NEWLY OPENED, over-skip side ONLY: a tool that exists ONLY under a
        relative PATH component now SKIPS as exec-unavailable instead of
        executing — zero corpus carriers, RR71 pins the direction; and a
        whitelist member without a grammar table now refuses
        (no-grammar-table) where v2.4 would have executed it unconstrained —
        constructible only by editing the whitelist, which the invariant pin
        makes red anyway, so no real-doc form changed verdict.
    (3) `--` (end-of-options) STILL refuses as non-allowlist-flag for wc/grep/
        log. Evaluated this round against the coordinator's ruling (评估入表,
        当前语料零载体，登记即可): 32 constructed over-refusal forms, zero
        corpus carriers → deliberately NOT widened (adding it opens an
        execution face nobody pastes; the loop law is register, not gamble).
        A future doc that hits it must re-adjudicate HERE, at this line.
    (4) the repo-config exec set remains an OPEN SET of keys (fsmonitor,
        hooks, textconv, credential helpers, …): v2.5 blocks and pins the ONE
        constructible gpg face; faces probed inert this round stay inert by
        measurement, not by claim (core.pager without a tty, builtin-priority
        vs alias, --ext-diff refused by grammar). The (i)/(ii) strategy choice
        is 主线 105's; this file does not claim "no external execution" beyond
        what RR69/PRB/q3d-battery re-runs demonstrate.
    (5) PRB tier coupling: it locates the probes file as
        <gate>/../../review_guard_probes.py — correct for the repo layout
        (agent-festival/tools vs root), prints the visible ABSENT line for any
        other placement (out-of-tree mutant copies included, which is why the
        census mutation counts are NOT inflated by it).
    (6) BSD/GNU platform scope of the new positive pins: `wc --lines` is NOT a
        BSD option — RR53 asserts the platform-STABLE half only (the shape
        REACHES the executor ⇒ MISMATCH either way: BSD cmd-failed, GNU
        value-drift; same discipline as RR35); `grep --extended-regexp` and
        every git-family entry exist on both sides and pin OK. macOS ran every
        pin here; the Linux/GNU face is 105's to re-run.
  OPEN faces CREATED by v2.3 (third-face record — killed / newly opened /
    FP; all measured this round, none assumed):
    (1) KILLED: none on the DEFAULT face — snapshot key diff 0 and
        --rerun 68号 keeps executed=7/SKIPPED=0; what v2.3 killed are
        UNPINNED states, not verdicts (the 13 faces above).
    (2) AXIS3's own blind spots, stated AS blind spots (the lesson applies
        to the axis itself): the axis mutates LABELS, not branch semantics
        — a refactor that relocates a refuse while re-emitting the SAME
        string survives every mutation (the want_reason holder keeps
        passing), and per-SITE coverage inside one label is not
        census-provable (outside-docdir has 3 live sites; one AXIS3
        mutation renames all three at once — the manual per-site
        discriminators RR21/22/23/36 are only as strong as their shapes).
        A reason firing in NO case is UNPINNED (caught), but coverage
        means "every label has a holder", NOT "every path is walked". The
        singularity section's "reason is a closed set: a literal set" is
        true exactly at this granularity — recorded so no round over-reads
        the axis.
    (3) the (d) widening, honestly: an operand whose ..-loops resolve BACK
        inside the doc dir (a `sub/../a.txt`-shaped token) now EXECUTES
        where v2.2 SKIPPED it — confinement to in-tree reads is preserved
        (the M3 face-2a allowance). MEASURED zero delta: --rerun over the
        85-file snapshot domain gave identical verdict lines before/after
        (recount by rerunning both versions, never by editing this prose).
    (4) NEW FPs: NONE — no default-path pattern moved (key diff 0); every
        v2.3 pin asserts CURRENT behaviour, none changed it. RR36's ~-file
        and RR47's pipe-filename shapes are new SUITE members, not new
        refusals.
    (5) line-anchor/count governance (m-3/M-1/m-4/m-2): the header carries
        NO self-line-count and no bare line-number cross-references —
        entries are named, never numbered, because a drifting line anchor
        in a counting gate is the ㉜-family disease itself; every numeric
        self-claim (pin counts, key diffs, flip counts) is re-measured per
        round by running the tool, and numbers quoted in fix reports are
        run output, never copied forward.
  CLOSED in v2.6.1 (external-campaign bugfix MR — engineering chamber; the
    adjudication chamber for detection-set EXTENSIONS is a separate followup
    MR): the CMD two-branch whitelist split (family-wide since v1.8; measured
    real carrier: a counting claim whose evidence is a SAME-LINE
    `ls …` backtick was rejected as "no command in the ±8 window"). Both
    branches are now DERIVED from one _CMD_TOOLS tuple (the P-Ⅰ
    single-source law, UNITS precedent) — future branch divergence is
    inexpressible. Detection-set only: membership grants NO --rerun
    execution. Pins: bare `ls` stays non-evidence; `ls samples/` /
    `find tools -name` backticks ARE evidence; the real-doc carrier
    form 「（`ls gate-attack-samples/`＝12 个 s*.md…）」 passes. Execution
    surface unchanged from v2.6.
  CLOSED in v2.6.2 (inner-loop remediation — redteam-0 of the two-chamber
    inner review): the rerun-tier evidence-command SELECTION was hijacked
    by v2.6.1's own fix — newly admitted ls/find backtick spans became
    top-down-FIRST picks, _guard refused them (not-whitelisted), and
    executed verdicts silently degraded to SKIPPED-UNSAFE (rc untouched) —
    a later whitelisted command that WOULD have been replayed (catching the
    pasted value drift) was never evaluated (three-version repro: v2.6
    RERUN-MISMATCH rc=1 → v2.6.1 SKIPPED rc=0). The fix is SCOPED to
    respect RR13's pinned determinism law (top-down FIRST over the
    v2.6-era match set, "never the runnable one"): exactly the
    newly-admitted backtick members (ls/find — frozen historical set
    _V26_BACKTICK_TOOLS) do not participate in selection priority; they
    remain as fallback, so ls-only windows keep an informative
    SKIPPED-UNSAFE verdict. Pins: RR75 (leading ls + drifted wc paste →
    RERUN-MISMATCH, executed), RR76 (ls-only window → SKIPPED-UNSAFE
    not-whitelisted), RR13 unchanged (determinism preserved). Execution
    surface unchanged (whitelist still grep -c / wc -l only).
  CLOSED in v2.7 (external-campaign detection-set MR — adjudication
    chamber, submitted as FIELDING asks with alternatives and an
    accept-any-override clause; execution surface UNCHANGED):
    (1) unit family +2 (样本/用例 — corpus vocabulary of the contributing
        campaign's own ledgers; its flagship escape was an arrow-tail
        backfill claim 「9→12 样本＋1 合法」 whose count slot never matched).
        Census expectation rolled 21→23; both new members carry their own
        flip pins; the pre-existing 「百分之三十的样本」 calibration case
        keeps guarding the 的-separated FP face. A measured FP guard ships
        WITH the unit: 「零样本」 (the ML zero-shot term; carriers measured
        in the NGA campaign repo's experiments reference doc — not present
        in this public repo) is skipped code-level — 零+样本 collocation
        only, skip-then-continue, pinned both ways — every other 样本 match
        returns.
    (2) the completion-state progress family (「3/5 已改」「已清偿 3/7 项」 —
        recorded escaping since v1.8's header: the numerator dies on unit
        adjacency, the denominator on the lookbehind, so neither fraction
        slot ever reached COUNTING) — closed by the PROGRESS regex, where
        已<verb> adjacent to the fraction is the discriminator against
        slash-list noise. Both ADJACENT orders pinned; ≤3-digit slots plus
        the shared lookbehind keep YEAR-LEADING date forms out (pinned:
        「2026/11 已改」); M/D short forms (「11/26 已改」) DO enter as
        legitimate progress readings (pinned, v2.7.1); 已<verb> without a
        fraction stays clean (pinned). Real carriers: a campaign's false
        progress claim 「3/5 已改」 (actual: fewer) and the v1.8-recorded
        「已清偿 3/7 项」.
        OPEN faces created (recorded, not hidden): noun-first count forms
        (「恰红 4」「探针合计 23」) remain outside the digit+unit grammar by
        design — extending them is a separate adjudication; the severity
        idiom (「0B+2M」) stays OUT per the recorded exemption.
  CLOSED in v2.7.1 (inner-loop remediation of the two-chamber review —
    consistency-0 + simplicity-pareto-0 + doc-to-code-0):
    ① the unit-intervening progress order 「5/8 项已完成」/「3/7 项已清偿」
      (the direct re-ordering of the v1.8 carrier) escaped the v2.7 regex —
      the first alternative's \s* could not cross the unit noun; an
      optional unit slot (项|件|处|条|个) now sits between the fraction and
      已<verb> (two new pins, pre-v2.7.1 unflagged).
    ② the verb list was hand-written in BOTH PROGRESS alternatives (the
      P-Ⅰ mirror disease this same change-set's _CMD_TOOLS comment declares
      inexpressible) and the second alternative was a pin blind zone —
      both now DERIVE from one _PROGRESS_VERBS tuple, and every verb plus
      both orders carry per-member pins.
    ③ the date-guard claim narrowed to what the cap actually delivers
      (year-leading 4-digit forms); the M/D short form (「11/26 已改」) is
      registered-and-pinned as a legitimate progress reading rather than
      an excluded date.
    ④ the 零样本 guard's boundary faces are registered and pinned (true
      zero-count reading swallowed — recorded adjudication; spaced variant
      still flags); the STILL OPEN progress clause and the stale 21-member
      arithmetics rolled to the 23-member current state.
      OPEN faces created (recorded, not hidden): no-已 verb forms
      (「3/7 清偿」) and inserted-clause forms (「3/7 已于本周清偿」) remain
      outside the two adjacent orders; fractions with ≥4-digit members
      escape the 3-digit cap by design.

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
    party's historical false claim — SINCE v2.0 this face has teeth for the
    Chinese side too: 「五处更正」 inside 「」 now FLAGS (CN-C1 pins the
    re-based expectation True; the 199-carrier 「…」 measurement is in the
    CREATED section) — "flag and adjudicate, never silently fix" is the
    68号「21 行」 precedent this form inherits, and 「21 行」-style quote
    carriers are explicitly NOT a fix target); closure words without
    digits (「零差异/全部一致/无差异」), English counts ("21 items"),
    N/M progress forms — CLOSED in v2.7/v2.7.1 (both adjacent orders plus
    the unit-intervening order, every verb per-member pinned; the
    remaining members are recorded in the CLOSED v2.7.1 entry: no-已 verb
    forms, inserted-clause forms, ≥4-digit fractions), ordinal
    residuals with double/full-width spaces ("第  3 项" and the CN twin
    "第  三条" — the (?<!第 ) pair is single-space only), colon triples
    ("M:11"), command-prefix paste forms ("$ git …" / "- git …" not
    recognized → false positives; the rerun tier inherits the same blindness
    because it reuses CMD — a "$ " line is never extracted as an evidence
    command, so such lines stay in the default violation face and never
    reach the executor), and the presence-form limit: an unrelated nearby command (git status) satisfies the window; the unit
    table itself is a closed set — units outside it (e.g. 「N 批」「N 步」,
    and the v2.0 example 「三档」) escape by definition; extending it requires a selftest case per new unit
    (P-Ⅲ: a unit-set change would flip census 21 → 22 and break the pinned
    23-member acceptance, so 档/批/步 stay out and CN-C3 pins 三档 as False);
    the 万/千 numeral residuals (v2.0 digit class = 零〇一二三四五六七八九十百千两):
    「三万条」 and the mixed form 「20万条」 escape by design (1 real carrier
    measured in the domain: 67号:92 「五万」); 廿/卅/□ and full-width digits
    are not in the class; the 〇/零 guards stop WORD forms only — the
    closure-phrase forms 「零差异/全部一致」 stay open unchanged.
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
  OPEN faces CREATED by v2.0 (third face, measured on the 85-file corpus
    snapshot domain, all numbers from this round's own runs — recount by
    rerunning the two-gate key diff, never by editing this prose):
    (1) CN TIGHTENING = +1500 newly flagged lines over the snapshot domain
        (379 → 1879 hits; lost=0 keys, so the change is monotone at line
        granularity). Composition: every gained claim literal contains a
        Chinese numeral (verified by key-diff census, 1500/1500); the volume
        is dominated by the numeral 一 and the classifier 个/次 forms —
        一+个 185 lines, 一+次 113, 两+个 112, 两+轮 81, 三+件 58 — and
        534/1500 gained literals start with 一, the Chinese indefinite-
        article reading of 「一个/一次」 ("a/one time") being the largest
        STRUCTURAL FP family this tier opens. It is NOT guarded: no decidable
        char-level discriminator separates 「对应一个需求」 (article) from
        「修复一次入口」 (count) — the same undecidability the v1.8 类+比
        record names — and an undecidable guess would be a silent-exemption
        (㉝④ class), so the family ships FLAGGED-for-human-look and the
        existing-doc hits stay register-only (§七 immunity). Quote forms
        inside 「」 gained 199 carriers on this face (CN05 re-based, above).
    (2) LITERAL LEFT-SHIFT on already-flagged lines: 53 carriers keep their
        file:line key and their flag but their quoted claim changes
        (minor-ledger:221 「 18 条」 → 「 十二轮」, 18-blank-r9:87 the TRIPLE
        fallback 「M=0」 → COUNTING 「 三条」) because the CN alternative can
        win the leftmost position. Consequence for consumers: the
        `count claim '…'` string is NOT a stable ledger key — key the audit
        on file:line, not on the literal (accepted consequence of closing
        the digit slot inside the SAME pattern; a separate CN regex would
        have broken the single-units-group invariant instead).
    (3) rerun over-skip face: the guard splits the raw command text on
        `&& | ; ||` BEFORE tokenizing, so a QUOTED `|` inside a legitimate
        read-only command (「`grep -cE '^\\*\\*(真裁|知悉)'`」, the 68号:136
        carrier form) classifies SKIPPED-UNSAFE and is never executed. The direction
        is safe (a paste that could have been verified is left unverified —
        the default verdict for its claim line is untouched, and SKIP is not
        an error), the cost is coverage; RR18/RR16 pin it so a future
        tokenizer that parses quotes first MUST flip a case and re-adjudicate
        here rather than silently widen the execution surface. → CLOSED in
        v2.1 by the quote-aware segmenter + escape normalization (M4 above);
        RR18 was flipped to OK (the mandated re-adjudication) and the
        execution surface widened ONLY for forms that were previously
        mis-SKIPPED, under the NEW operand fence (M3) — net narrower.
    (4) rerun cwd/anchor face: the cd segment is dropped and the tail runs
        in the DOCUMENT's directory (task-specified), so a paste taken under
        「cd <other-root> && …」 (64号:32 form) either fails or mismatches
        rather than being verified in place; git forms are root-insensitive
        (rev-parse/log), path-relative forms are not (wc -l testdata/…) —
        RR06a pins the accepted shape (cd . && read-only tail).
    (5) paste-equality tolerance face: comparison is whitespace-token
        prefix matching with 「（…）」-suffix and 「——」-tail cut and a
        「…」-stem prefix rule (the real-doc carrier forms are truncated shas
        and wc-with-filename lines) — a change confined to the TAIL of a
        pasted line beyond the expected tokens can be masked by design;
        RR02/RR08 pin the value-drift direction, the tail-mask itself is
        recorded (not guarded) because tightening it would reject 64号-style
        truncated pastes.
    (6) rerun is opt-in and rc-affecting: without --rerun nothing in the
        default path moved (byte-diff acceptance ③ is the pin); with
        --rerun, mismatches join rc=1 while SKIPPED lines never do.
  OPEN faces CREATED by v2.1 (the third face for this version — killed FPs,
    newly opened escapes, new FPs; all measured, none hidden):
    (1) M2 killed-FP face: every default-path key lost v2.0→v2.1 carries the
        个-unit immediately followed by [月日年点时分秒] (the duration class;
        census in dg-v21.txt, lost-outside-class=0 is acceptance ③'s pin).
        New FP of the fix: a REAL count whose 个 is followed by a class char
        is now skipped the same way (「两个时代的对比」 个+时, 「三个日历」 个+日)
        — char-level undecidable against segmentation, same class as the
        v1.8 类+比 record; zero carriers measured in the snapshot domain
        (recount by rerunning the gate with the guard removed, never by
        editing this prose). Spaced 「三个 月」 is NOT guarded (the guard is
        direct-adjacency, lookbehind fixed-width 1) — zero carriers.
    (2) M3 fence residual faces: (a) READING inside the doc directory stays
        reachable — the fence bounds WHERE operands resolve, not whether the
        whitelisted tools may read anything under the document's directory
        (a planted in-tree file is legitimate evidence by design); (b) the
        sha-prefix echo of drifted output leaks no content, but a rerun can
        CONFIRM a guessed low-entropy output (existence/known-value oracle
        survives as a weak channel; the strong leak — first-line content in
        stdout — is closed); (c) non-flag tokens that are not paths (a grep
        PATTERN) are path-checked too — an absolute-looking or ..-looking
        pattern (e.g. `grep -c /etc file.txt`) over-skips: safe direction,
        zero real-doc carriers; (d) escaped backticks inside a PROSE
        backtick span are cut by the span regex before normalization can
        see them (extraction ordering unchanged) — such lines stay in the
        default face, never reach the executor.
    (3) M4 face: the execution surface widened exactly for quoted-metachar
        forms (previously mis-SKIPPED); they now run under argv-direct exec,
        still whitelist-bounded, now also operand-fenced — no new shell
        reach (splitting on unquoted metachars is unchanged). Unterminated
        quotes still refuse as unbalanced-quote — behaviour TRUE (live probed), but the "(RR14 lineage)"
        attribution was FALSE (outer-r2 M-2: RR14 pins skip-precedence
        (multi-segment), never this refusal face — the unbalanced layer
        shipped untested for two generations; its first real holder is
        v2.3's RR32, asserted through want_reason). A `\"`→`"`
        normalization OUTSIDE quotes changes shell tokenization of that
        exact byte sequence — markdown intent (escaped display quote) is
        what shlex then sees as a real quote; recorded, zero carriers
        disagreeing.
    (4) N4 face: the frozen census expectations (21 units / 5 members / the
        named member lists) are a deliberate SECOND source of truth at the
        pin site — the P-Ⅰ mirror-drift disease is inverted here on
        purpose: a units-set or whitelist edit must touch the census
        expectation too or it goes RED (extension by accident becomes
        impossible to green silently).
  OPEN faces CREATED by v2.2 (the third-face record for this version —
    killed forms, newly opened over-skips, residuals; measured on the same
    85-file corpus snapshot domain this round, none hidden):
    (1) KILLED face: the concatenated / equals-value flag-path forms —
        the inner-audit oracle trio (oracle_hit / oracle_abs /
        oracle_host, each executed=1 under v2.1) flips to
        SKIPPED-UNSAFE (flag-path-form) with executed=0 under v2.2
        (out-of-selftest live re-run, transcripts in the fix report);
        RR28/RR29 keep the same boundary pinned in-tree and go red the
        moment the rule is deleted.
    (2) NEWLY OPENED face, over-skip direction ONLY (safe side): a
        legitimate flag or pattern VALUE containing / ~ .. (an
        --include='src/*.py' style flag, a range flag) now refuses
        WITHOUT executing — a paste that could have been verified stays
        unverified; SKIP is not an error and the default verdict is
        untouched (same family as the M3 face-2c non-flag-pattern
        over-skip, same measure-or-go-zero discipline: MEASURED ZERO
        carriers this round — the v2.1→v2.2 rerun-verdict diff over the
        85-file snapshot domain is 0 changed lines (222 verdict lines
        identical both versions) and the default-face key diff is 0
        (1845==1845); the only real flip is the out-of-tree oob_doc
        attack trio (executed→SKIPPED). Recount by rerunning both
        versions, never by editing this prose).
    (3) RESIDUALS 2a-2d UNCHANGED and standing as recorded under v2.1:
        (a) reads INSIDE the doc directory stay reachable (the fence
        bounds WHERE operands resolve, not what in-tree evidence may
        read); (b) the sha12 existence/low-entropy confirmation channel
        survives; (c) non-flag pattern operands stay path-checked; (d)
        escaped-backtick spans stay cut before the executor. None
        reopened, none widened — v2.2 only closes what the flag rule
        smuggled past the fence.
"""
import re
import shlex
import sys
from pathlib import Path

COUNTING = re.compile(
    r"(恰|共|总)?\s*(?<![A-Za-z0-9-/.])(?<!第)(?<!第 )"
    # v2.0 (build-bit #2 tier 2): the digit slot learned Chinese numerals. The
    # 初 pair stops 「初十」 lunar dates; the numeral-BACKCHAIN stops the
    # right-shift FP (「第十二项」 must not match at 二 — CN-B4 pins it, and
    # deleting the backchain flips that case). The digit alternative is
    # NON-capturing on purpose: the units group below stays the pattern's ONLY
    # trailing capture, so UNITS derivation (P-Ⅰ) and the census anchors (P-Ⅱ)
    # keep working unchanged. 万 and 廿/卅 are deliberately NOT in the class
    # (recorded escaping face, 1 measured carrier).
    r"(?<!初)(?<!初 )(?<![零〇一二两三四五六七八九十百千])"
    r"(?:\d+|[零〇一二两三四五六七八九十百千两]+)"
    r"\s*(处|件|条|项|次|类|组|波|枚|篇|包|个|轮|行|测试|函数|删除|新增|新提交|提交|PASS|样本|用例)"
    # v2.1 M2: the duration guard — a 个-unit DIRECTLY followed by a
    # time noun (三个月/3个月/两个日…) is a duration reading, not a closure
    # count. Implemented as a trailing zero-width assertion so the units
    # alternation stays the pattern's ONLY plain capture group byte-for-byte
    # (P-Ⅰ): (?<=个) fires only when the unit that matched was 个, so every
    # other unit (行/件/条…) is untouched — 「共十二行」 still flags. The guard
    # sits AFTER the capture group; UNITS derivation takes the last plain
    # group, the frozen 21-member census expectation pins the set.
    r"(?!(?<=个)[月日年点时分秒])"
)  # lookbehind kills label/range/ordinal/slash-list noise (R74 条款 / 1-3 条 / 第 3 项 / 3/7 项);
   # v1.3 (R76 M-2): unit family extended — 项/次/类/组/波/枚/篇/包/个/PASS were a whole
   # escaping family (10 unit branches, 8 with real-doc carriers: 连续 2 次 / 24 个 B/M / 3 PASS);
   # v1.6 (R76 iter-6 M-3): bare compound nouns (测试/函数/删除/新增/提交/新提交) entered the
   # set and the dead 个X/处差异/条PASS branches retired (换结构: unreachable branches made
   # inexpressible) — the gate's own motivating disease form "12 测试全绿" (56号, actual 11)
   # now matches; v2.0: digit slot only — the units group text is byte-unchanged.
TRIPLE = re.compile(r"\b[MDA]\s*=\s*\d+")
# v2.7 (adjudication-chamber item 2; v2.7.1 remediation): completion-state
# progress claims — 「3/5 已改」「已清偿 3/7 项」「5/8 项已完成」— were an
# escaping family recorded OPEN since v1.8: the numerator dies on unit
# adjacency (digit followed by /), the denominator on the lookbehind, so
# neither slot of the fraction ever reaches COUNTING. 已<verb> adjacent to
# the fraction is the discriminator against slash-list noise (「检查 3/7 项」
# has no completion verb). v2.7.1 (inner-loop consistency-0 +
# simplicity-pareto-0): ① the verb list is DERIVED from one tuple (P-Ⅰ —
# it was hand-written in BOTH alternatives, the second being a pin blind
# zone); ② an optional unit noun is allowed between the fraction and
# 已<verb> (「3/7 项已清偿」 — the direct re-ordering of the v1.8 carrier,
# unflagged pre-v2.7.1). Digits capped at 3 per slot, which keeps
# YEAR-LEADING date forms (「2026/11 已改」, 4-digit slots) out — pinned;
# M/D short forms (「11/26 已改」) DO enter as legitimate progress readings
# — pinned as current behaviour. 已<verb> without a fraction stays clean
# (pinned).
_PROGRESS_VERBS = ("改", "修", "回填", "清偿", "处置", "完成", "落地")
PROGRESS = re.compile(
    r"(?<![A-Za-z0-9-/.])\d{1,3}\s*/\s*\d{1,3}\s*(?:[项件处条个]\s*)?已(?:" + "|".join(_PROGRESS_VERBS) + r")"
    r"|已(?:" + "|".join(_PROGRESS_VERBS) + r")[^0-9\n]{0,4}\d{1,3}\s*/\s*\d{1,3}(?![\d/])"
)
# v2.6.1 (engineering-chamber bugfix): the two CMD branches below were
# hand-maintained mirrors that silently diverged — the backtick branch lacked
# ls/find while the line-start branch had them (family-wide since v1.8;
# measured carrier: a counting claim whose evidence is a SAME-LINE `ls …`
# backtick was rejected as "no command in the ±8 window"). Both branches are
# now DERIVED from one tool tuple (the P-Ⅰ single-source law, same precedent
# as UNITS from COUNTING) — future branch divergence is inexpressible.
# Detection-set only: membership here does NOT grant --rerun execution
# (RERUN_READONLY governs that surface).
_CMD_TOOLS = ("git", "go", "grep", "python3", "curl", "wc", "diff", "cd", "ls", "find")
CMD = re.compile(
    r"^\s*(" + "|".join(t + " " for t in _CMD_TOOLS) + r")"
    r"|`(" + "|".join(_CMD_TOOLS) + r")\s+[^`]*`"
)
WINDOW = 8  # evidence-command window, ±WINDOW lines BOTH directions


def find_claim(line: str):
    """COUNTING with the two code-level skips, else TRIPLE/PROGRESS:
    ① a 类 unit directly followed by 比 (「0600 类比」 chmod-mode analogies)
    is not a count — skipped, scan continues (v1.8 structural fix; kept
    code-level so the alternation stays parenthesis-free and UNITS
    derivation holds); ② v2.7: 「零样本」 (the ML zero-shot term) is not a
    zero-count — the 零+样本 collocation only is skipped, scan continues;
    no COUNTING match falls through to TRIPLE, then PROGRESS (v2.7)."""
    pos = 0
    while True:
        m = COUNTING.search(line, pos)
        if not m:
            return TRIPLE.search(line) or PROGRESS.search(line)
        # unit-scoped: ONLY a 类 unit directly followed by 比 (「0600 类比」);
        # any OTHER unit + 比 (「3 组比较」「7 条比值」) is a REAL count and
        # must NOT be skipped (R77 iter-2 M-1 — the unscoped skip was an
        # over-broad exemption; pinned by the 组比较 selftest case)
        if line[m.end():m.end() + 1] == "比" and line[m.end() - 1] == "类":
            pos = m.end()
            continue
        # v2.7 (adjudication-chamber MR item 1, FP guard): 「零样本」 is the
        # ML zero-shot TERM (measured carriers: the NGA campaign repo's
        # experiments reference doc — not present in this public repo), not
        # a zero-count claim — skip ONLY the 零+样本 collocation and keep
        # scanning (skip-then-continue, the 类比-skip pattern); every other
        # 样本 match still returns. Boundary faces REGISTERED (v2.7.1,
        # pinned): a true zero-count reading 「本批新增零样本。」 is swallowed
        # with the term (recorded adjudication), and the spaced variant
        # 「零 样本」 is not the collocation and still flags.
        if m.group(0).endswith("零样本"):
            pos = m.end()
            continue
        return m


def _fence_spans(lines):
    """Fence pairing (an unclosed fence extends to the last line — desync-safe).
    Shared by violations() and the rerun pass: ONE truth source for fence scope
    (v2.0 extraction — the pairing rules themselves are v1.5-era and unchanged)."""
    spans = []
    start = None
    for i, line in enumerate(lines):
        if line.strip().startswith("```"):
            if start is None:
                start = i
            else:
                spans.append((start, i))
                start = None
    if start is not None:
        spans.append((start, len(lines) - 1))
    return spans


def _fence_at(spans, idx):
    for s, e in spans:
        if s <= idx <= e:
            return (s, e)
    return None


def violations(path: str) -> list[str]:
    lines = Path(path).read_text(encoding="utf-8").splitlines()
    fences = _fence_spans(lines)

    def fence_at(idx):
        return _fence_at(fences, idx)

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
    ("经 `ls` 复核，恰 7 处差异。", True),                # v2.6.1: bare `ls` (no space+args) still ≠ evidence — the fix must not newly pass the bare form
    ("复核产物见下，`ls samples/` 共 12 个文件。", False),   # v2.6.1: backtick `ls` with args IS evidence — two-branch whitelist split (line-start had ls/find, backtick lacked them since v1.8) killed by the _CMD_TOOLS derivation; deleting the derivation flips this and the find twin
    ("清单经 `find tools -name` 复核，本批共 3 处新增。", False),   # v2.6.1: backtick `find` twin
    ("（`ls gate-attack-samples/`＝12 个 s*.md＋coresha.txt 数据文件）", False),   # v2.6.1: the real-doc carrier form — same-line `ls` backtick was wrongly judged evidence-less (measured on a live campaign ledger)
    ("更正稿 9→12 样本＋1 合法。", True),                      # v2.7: 样本 unit — the arrow-tail 「12 样本」 form; real carrier = a live campaign's backfill claim that escaped (修复声明与实态差 family, POST-time caught by human); deleting 样本 flips this
    ("全量 1826 用例全绿。", True),                            # v2.7: 用例 unit — real carrier form (test-count claims written as N 用例); deleting 用例 flips this
    ("完整零样本与微调数据如下。", False),                     # v2.7 零样本 guard: the ML zero-shot TERM (measured carriers, see the skip comment), not a zero-count claim — code-level skip, both-way pinned with the next case
    ("零样本术语已界定；本批共 3 样本。", True),               # v2.7 零样本 guard skip-THEN-CONTINUE: the skip must not swallow the line's remaining real 样本 counts (the continue→return-None mutant kills this)
    ("进度：3/5 已改。", True),                                # v2.7 PROGRESS: fraction-then-verb order — real carrier (a campaign's false progress claim "3/5 已改", actual fewer); neither fraction slot ever reaches COUNTING
    ("已清偿 3/7 项。", True),                                 # v2.7 PROGRESS: verb-then-fraction order — the form recorded OPEN since v1.8's header (no-space variant escaped; the denominator never matched)
    ("2026/11 已改。", False),                                 # v2.7 PROGRESS date-guard: year-leading (4-digit) slots cannot enter (≤3-digit cap + shared lookbehind) — pinned so a future widening flips red
    ("清单已清偿完毕，零差异。", False),                        # v2.7 PROGRESS overreach-guard: 已<verb> WITHOUT a fraction is not a progress claim (no ASCII digits in scope)
    ("5/8 项已完成。", True),                                  # v2.7.1: unit-intervening order (inner-loop consistency-0) — unflagged pre-v2.7.1; deleting the unit-slot alternative flips this and the next case
    ("3/7 项已清偿。", True),                                  # v2.7.1: the v1.8 carrier's direct re-ordering — same fix's second victim
    ("2/5 已修。", True),                                      # v2.7.1 per-verb pin: 修 (the verb list was 5-member pin-blind pre-v2.7.1)
    ("3/8 已回填。", True),                                    # v2.7.1 per-verb pin: 回填
    ("4/9 已处置。", True),                                    # v2.7.1 per-verb pin: 处置
    ("2/6 已完成。", True),                                    # v2.7.1 per-verb pin: 完成
    ("1/4 已落地。", True),                                    # v2.7.1 per-verb pin: 落地
    ("已改 3/5。", True),                                      # v2.7.1 verb-then-fraction order per-verb pin: 改 — the second alternative was a pin blind zone (deleting 改 from it ALONE stayed green pre-v2.7.1)
    ("11/26 已改。", True),                                    # v2.7.1 date-face registration: M/D short forms DO enter as legitimate progress readings; the guard's claim is narrowed to year-leading forms — pinned as current behaviour so a future narrowing/widening flips
    ("本批新增零样本。", False),                               # v2.7.1 零样本 boundary, REGISTERED: the term guard swallows the true zero-count reading (「新增零项。」 flags — recorded adjudication, not hidden)
    ("零 样本（空格变体）。", True),                            # v2.7.1 零样本 boundary, REGISTERED: the spaced variant is not the collocation and still flags as a claim — current behaviour, pinned
    ("git status\n本批共 9 项新增。", False),            # OPEN: presence-form (unrelated cmd satisfies)
    ("汇总 M = 11、D = 151。", True),                    # CLOSED v1.2: spaced triple
    ("the batch adds 21 items.", False),                 # OPEN: English counts
    ("本轮合计 5 处修改。", True),                       # control: the basic catch
    ("R74 条款与 iter-3 条款如下：", False),             # CLOSED v1.2: round-label lookbehind
    ("（三条，全采纳）", True),                           # CLOSED v2.0: Chinese-numeral counts entered the DIGIT SLOT (spec flipped True with the regex, same commit — ㉝/㉚)
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
    # ---- v2.0 tier 2: Chinese-numeral pins (GATE-MATRIX v2.0, frozen BEFORE
    # implementation; RED-proven against v1.8 first — 6 TP cases could not
    # fire there). Discrimination pairs: A1↔A2/A3 (十 as count vs 十 as
    # date/time word), B1↔B2/B3 (real count vs ordinal), B4 alone (the
    # right-shift backchain), C1↔C2 (quote form vs bare count), C3/C5 (unit-
    # set and 百分之 closed-set arithmetic). ----
    ("现十行，全部核对。", True),                          # CN-A1: 十+行 (the task's 「十行」 form) — pair with A2
    ("十一点开会。", False),                               # CN-A2: 点 ∉ units → date/time cannot false-flag by construction
    ("三十天前发放。", False),                             # CN-A3: 天 ∉ units (matrix CN02) — same 十/三十 carrier as A1, opposite verdict
    ("共十二条新增。", True),                              # CN-B1: prefix 共 + 十二条 (matrix CN03 — rides the SAME ±8 window logic, no new branch)
    ("第三条与第七条如下。", False),                        # CN-B2: the (?<!第) lookbehind works on the CN side too
    ("第 三条与第 七条如下。", False),                       # CN-B3: the (?<!第 ) spaced pair, CN twin (matrix CN04)
    ("第十项与第二十项。", False),                           # CN-B4: BACKCHAIN PIN — delete (?<![零〇一二…]) and the match right-shifts to 二项 (twelve项 FP); this case goes True. The core date-adjacent discrimination for 「十二」 forms.
    ("您说的「五处更正」属实。", True),                       # CN-C1: quote form newly FLAGS (matrix CN05 RE-BASED — see header STILL-OPEN/CREATED: the v1.8 escape was a blind-spot side effect; 「21 行」 precedent = flag, human adjudicates, doc is NOT a fix target)
    ("本批二十一件。", True),                              # CN-C2: multi-char numeral run (greedy 二十一 + 件) — the task's 「二十一件」 form
    ("本批分三档推进。", False),                            # CN-C3: 档 ∉ the closed 21-unit set → escape stays; P-Ⅲ forbids treating exemptions by touching the unit table (matrix divergence note 2)
    ("这是百分之三十的样本。", False),                        # CN-C5: 百分之 — 分 ∉ units (calibration battery: 十分/每次/again forms safe the same way; zero carriers in the domain)
    ("知识类笔记三篇。", True),                             # CN-C6: the backchain does NOT eat word-internal left starts (识+类 has no 比 suffix so the v1.8 skip stays put; 篇 is caught). Deleting the backchain leaves this True → C6 is not a false-green of C4's pin.
    # ---- v2.1 M2: duration-guard pins. Discrimination is BIDIRECTIONAL by
    # mandate (CLAUDE.md rule 2 — a one-sided probe is a fake probe): each
    # False carrier below flips True when the (?!(?<=个)[月日年点时分秒]) guard
    # is deleted, each True twin flips False if the guard is ever widened
    # beyond the adjudicated 个+class adjacency. ----
    ("规则：过去三个月无逾期即授信，维持三个月不变。", False),      # M2-A: the 67/76号 real carrier sentence — 「三个」+月 duration must NOT claim (v2.0 FP, outer-r1 M2)
    ("新增三个新文件。", True),                                # M2-B: 个 followed by a non-class char is a REAL count — deleting an over-wide guard kills this (pair of M2-A)
    ("共十二行，全部核对。", True),                             # M2-C: non-个 unit — the guard's (?<=个) must leave 行 untouched (pair; overlaps CN-A1 from the duration side)
    ("过去 3 个月无逾期记录。", False),                          # M2-D: arabic digit twin of M2-A — the guard is digit-form agnostic (67号 arabic duration forms were the same FP family)
    ("检查 3 个文件。", True),                                 # M2-E: arabic real count (个+文) — pair of M2-D
    ("历时两个星期完成。", True),                                # M2-F: RECORDED RESIDUAL pin — 星期 is NOT in the frozen 7-char class [月日年点时分秒], so 两个星期 still flags; silently widening the class to 周/星 flips this red and forces re-adjudication (㉝④)
    # ---- v2.3 收编 (outer-r2 镜二②): face #10 — the fence-marker skip in
    # violations() (a line that STARTS with ``` is never a claim, even when
    # it also contains one — the inline ```-包裹 form). P39 proved this
    # branch shipped with zero holders: deleting the skip flipped every pin
    # green. One minimal discriminating carrier, pinning CURRENT behaviour:
    # base False (skipped as marker); remove the skip → the line is scanned,
    # 「共 3 处」 matches, no command in scope → True → this case flips. ----
    ("``` 本批共 3 处差异 ```", False),
]


# UNITS is DERIVED from COUNTING's own alternation (R76 iter-7 M-3: a
# hand-maintained mirror is a second truth source — regex ⊃ mirror drifts
# SILENT GREEN; deriving makes the fifth-write-site form inexpressible).
# v2.1 M2: the derivation takes the LAST plain capture group (a body that
# starts with neither ( nor ? — assertions excluded) instead of anchoring at
# the pattern end, because the duration guard may trail the units group as
# a zero-width assertion. P-Ⅰ still holds in the strengthened form: the
# units alternation is the last PLAIN group; if a future edit appends a
# second plain alternation group, the census count pin (23) goes red on
# drift instead of silently iterating the wrong group.
_plain_groups = re.findall(r"\(([^()?][^()]*)\)", COUNTING.pattern)
UNITS = _plain_groups[-1].split("|")


# ---------------------------------------------------------------------------
# TIER 1 (v2.0): --rerun — command-vs-paste fidelity. Default path untouched;
# every function below is reachable ONLY from main()'s --rerun branch (or the
# selftest/census harnesses). Separation of concerns: the judging regexes
# (COUNTING/TRIPLE/CMD/find_claim/WINDOW/UNITS) never import subprocess; the
# executor never re-implements a judgement regex (it reuses the primitives —
# no fifth write-site for a pattern literal).
# ---------------------------------------------------------------------------

RERUN_TIMEOUT_DEFAULT = 10  # seconds per executed command; env RERUN_TIMEOUT overrides

# The EXECUTION whitelist — a NEW closed set (v2.0 boundary record: extending
# a closed set requires a pin per member, done below in RERUN_CASES and
# enforced by census axis 2). Deliberately narrower than the DETECTION set CMD:
# go/python3 build and write, curl is the network, diff/ls/find can be made to
# touch anything — a whitelisted EXECUTION shape must be read-only AND print a
# small, paste-comparable value. Keep the entry format exactly one tuple per
# line, trailing comma included: the census token for each member is the whole
# line, and it must occur exactly ONCE in this file (P-Ⅱ anchor semantics), so
# nothing above this block may quote the pair form verbatim.
RERUN_READONLY = (
    # v2.6: the three git members are RETIRED (moved to _RETIRED_GIT_MEMBERS
    # below) — the git replay face refuses wholesale in _guard before this
    # table is consulted; a dead whitelist member has no behaviour face to
    # pin (census AXIS2 flagged all three unpinned), so retirement removes
    # them here and registers them explicitly.
    ("grep", "-c"),
    ("wc", "-l"),
)
# v2.6.2 (inner-loop redteam-0): the v1.8–v2.6 backtick-branch tool set —
# frozen HISTORICAL fact. v2.6.1's single-source unified the two CMD
# branches and thereby admitted ls/find into the BACKTICK form for the
# first time (the line-start branch had them since v1.8). The rerun-tier
# SELECTION passes over exactly those newly-admitted backtick members: RR13
# pins top-down-FIRST over the v2.6-era match set ("never the runnable
# one"), and the admission must not silently change WHICH command a doc's
# verdict hangs on.
_V26_BACKTICK_TOOLS = ("git", "go", "grep", "python3", "curl", "wc", "diff", "cd")
_RETIRED_GIT_MEMBERS = (("git", "rev-parse"), ("git", "log"), ("git", "ls-tree"))
# census AXIS2 keeps watching the RETIRED trio: re-adding any of them to
# RERUN_READONLY without removing the _guard retirement is a no-op (still
# refused); removing the retirement revives RR74/RR74b red. Both directions
# stay covered.

_LONG_FLAG = {"c": "--count", "l": "--lines"}
_ANN = re.compile(r"(?:#\s*)?→\s*(.*)$")   # the inline paste annotation: "cmd → out" / "cmd # → out"



def _has_flag(argv, short):
    """True when the required count-style flag is present: exact, long twin,
    or inside a combined short cluster (-cE / -El)."""
    if short in argv or _LONG_FLAG.get(short[1:]) in argv:
        return True
    return any(t.startswith("-") and not t.startswith("--") and short[1] in t[1:]
               for t in argv[1:])


# One table entry PER LINE, including its regex tuple: census AXIS4 anchors an
# entry at the line that starts with its key (the same token may sit on another
# line — "-l" is both a wc and an ls-tree entry — so a whole-file replace would
# mutate the wrong site and measure nothing, the P-Ⅱ anchor lesson), and a
# wrapped entry simply has no anchor. v2.5 wrote "log" across two lines and
# AXIS4 correctly refused to guess: token-anchor count 0 → UNPINNED, three
# entries (the layout, not the pinning, was the bug).
_GRAMMAR = {
    "wc":   ({"-l", "--lines"}, ()),
    "grep": ({"--count", "--extended-regexp"}, ("-[cE]+",)),
}
_GIT_GRAMMAR = {}
# v2.6: the git grammar table is RETIRED EMPTY — every git subcommand refuses
# at the _guard entry (git-replay-disabled) before any grammar lookup; the 17
# git entries census flagged unpinned had no behaviour face left. The wc/grep
# tables survive unchanged (AXIS4 pins them).

def _flag_grammar(tool, sub, argv):
    """v2.4 (outer request-changes): the whitelist must constrain the FULL
    option grammar, not just the required flag — indirect-input and
    execution-extending options (`wc --files0-from`, `git log --ext-diff -p`)
    pass the surface checks yet read outside data or spawn external programs.
    Every '-'-leading token must match the per-tool allowlist (exact set plus
    regexes); anything else refuses as non-allowlist-flag WITHOUT executing.
    v2.5 F1: FAIL-CLOSED — a whitelisted member that has NO table used to get
    ZERO constraint (the outer's fail-open experiment: add ("git","show") +
    sync _EXPECTED_CMD ⇒ 0 FAIL, every shape accepted). No table now refuses
    as "no-grammar-table" (AXIS3 member, RR70 want_reason holder), and the
    ENV-grammar-invariant pin asserts every RERUN_READONLY member HAS one.
    v2.5 F3: `--pretty` lives ONLY in the table regex now — the former code
    special case double-wrote the same rule (deleting either half flipped
    nothing, so the entry was unmutable-proof); bare `--pretty` still matches
    via "--pretty.*" (.* over the empty string)."""
    table = _GRAMMAR.get(tool)   # v2.6: git branch retired with the face
    rest = argv[1:]
    if table is None:
        # v2.6: the labelled refuse died with its only holder (RR70 flipped
        # to git-replay-disabled). The defence itself stays, as fail-loud:
        # every live whitelist member carries a table (ENV-grammar-invariant
        # pins that), so reaching here is an invariant break — crash red,
        # never silently accept.
        raise AssertionError(f"grammar table missing for whitelisted {tool!r}")
    exact, regexes = table
    for t in rest:
        if not t.startswith("-"):
            continue
        if t in exact or any(re.fullmatch(rx, t) for rx in regexes):
            continue
        return "non-allowlist-flag"
    return None


_MD_ESC_CHARS = ("`", '"', "*")  # v2.1 M4: markdown-escaped glyphs (task spec: \` \" \* 类)


def _md_unescape(text: str) -> str:
    """Normalize markdown backslash-escapes OUTSIDE shell quotes (v2.1 M4):
    docs that paste commands into prose escape ` " * with a backslash for
    display; the command the author actually ran has the bare glyph, so
    normalization must precede the segment split and shlex. INSIDE quotes
    the text is copied untouched — there a backslash belongs to the regex
    pattern, not to markdown (the 68号 '\\*\\*' lives inside single quotes
    and MUST survive verbatim or grep changes meaning). A backslash-escape
    that shell itself defines inside double quotes (\" \\ $ `) is preserved
    pair-wise."""
    out, i, n, q = [], 0, len(text), None
    while i < n:
        c = text[i]
        if q == "'":
            out.append(c)
            if c == "'":
                q = None
            i += 1
            continue
        if q == '"':
            if c == "\\" and i + 1 < n and text[i + 1] in ('"', "\\", "$", "`"):
                out.append(c)
                out.append(text[i + 1])
                i += 2
                continue
            out.append(c)
            if c == '"':
                q = None
            i += 1
            continue
        if c in "\"'":
            q = c
            out.append(c)
            i += 1
            continue
        if c == "\\" and i + 1 < n and text[i + 1] in _MD_ESC_CHARS:
            out.append(text[i + 1])
            i += 2
            continue
        out.append(c)
        i += 1
    return "".join(out)


def _split_segments(text: str):
    """Split on unquoted && / || / ; / | with a shell-quote state machine
    (v2.1 M4 — replaces the naive re.split that shredded a QUOTED pipe into
    unbalanced fragments, the 68号 SKIPPED-UNSAFE(unbalanced-quote) face).
    Returns None when a quote is left unterminated OR the text ends with an
    ODD run of backslashes (shlex would raise "No escaped character" on
    exactly that input — v2.3: the caller's except-ValueError layer is gone
    because the machine now refuses the only input that could reach it, with
    the identical emitted reason; parity: an EVEN trailing run is a literal
    backslash shlex accepts, so it must pass). Otherwise a list of segment
    strings."""
    segs, cur, i, n, q = [], [], 0, len(text), None
    while i < n:
        c = text[i]
        if q == "'":
            cur.append(c)
            if c == "'":
                q = None
            i += 1
            continue
        if q == '"':
            if c == "\\" and i + 1 < n:
                cur.append(c)
                cur.append(text[i + 1])
                i += 2
                continue
            cur.append(c)
            if c == '"':
                q = None
            i += 1
            continue
        if c in "\"'":
            q = c
            cur.append(c)
            i += 1
            continue
        if c == "\\" and i + 1 < n:
            cur.append(c)
            cur.append(text[i + 1])
            i += 2
            continue
        if text.startswith(("&&", "||"), i):
            segs.append("".join(cur))
            cur = []
            i += 2
            continue
        if c in ";|":
            segs.append("".join(cur))
            cur = []
            i += 1
            continue
        cur.append(c)
        i += 1
    if q:
        return None
    trail = len(text) - len(text.rstrip("\\"))  # odd trailing backslash run
    if trail % 2:                               # == shlex's only remaining ValueError input
        return None
    segs.append("".join(cur))
    return [s.strip() for s in segs]


def _operands_in_scope(argv, docdir: str):
    """v2.1 M3 operand fence + v2.2 F-1 flag-value rule: returns None when
    EVERY argv token is in scope, else the refuse reason ("outside-docdir" /
    "flag-path-form"). Non-flag tokens (grep patterns are path-checked too —
    the over-skip direction of a / -leading pattern is recorded, zero
    carriers) must RESOLVE inside the document's directory: absolute,
    ..-escaping and symlink-escaping operands are refused as outside-docdir
    by the realpath containment. v2.3 (outer-r2 Simplicity ③): the former
    pre-checks (a / prefix, an explicit .. segment) are DELETED — they were
    behaviour-redundant with that containment (same verdict, same reason,
    measured zero rerun-verdict delta over the 85-file snapshot domain);
    the ~-leading check STAYS and is now pinned (it is NOT redundant: argv-
    direct exec never tilde-expands, so "~" can name a real in-tree file —
    deleting the check would EXECUTE against it, flipped by RR36).
    v2.2 F-1 (inner-audit): a '-'-leading token is NO LONGER skipped
    unconditionally — the fence was bypassable because a path hidden INSIDE
    a flag (`-f/etc/hosts`, `--file=/etc/hosts`, `-f../outside/x`) walked
    past all three checks and the tool DID execute against a file outside
    the document directory; any flag token containing /, ~ or .. now refuses
    as "flag-path-form". The over-refusal this adds for legitimate
    path-bearing flags (e.g. `--include='src/*.py'`) is the recorded safe-
    direction residual (OPEN faces v2.2 (2)). Fail-safe: any refusal means
    SKIPPED, not error."""
    import os
    if docdir is None:
        return None
    root = os.path.realpath(docdir)
    for t in argv[1:]:
        if t.startswith("-"):
            if "/" in t or "~" in t or ".." in t:            # v2.2 F-1
                return "flag-path-form"
            continue
        if t.startswith("~"):
            # v2.3: the / and ..-segment pre-checks are deleted — the
            # realpath containment below refused the same tokens with the
            # same reason; ~ stays because shlex/argv never expand it, so
            # an in-tree file literally named ~ would otherwise execute.
            return "outside-docdir"
        rp = os.path.realpath(os.path.join(root, t))
        if rp != root and not rp.startswith(root + os.sep):
            return "outside-docdir"
    return None


def _guard(text: str, docdir=None):
    """(argv, reason) — argv None ⇒ SKIPPED-UNSAFE(reason): refuse WITHOUT
    executing and WITHOUT raising (the fail-safe direction; the collision
    table R2 makes SKIP the safe side and only EXECUTION the red side).
    Rules, in order: markdown-escape normalization outside quotes first
    (v2.1 M4), then raw shell-metacharacters are rejected before any
    parsing; the text is split into segments on unquoted && / || / ; / |
    with a quote state machine (v2.1 — the v2.0 naive split over-skipped
    quoted pipes, face 3, now CLOSED); a leading cd segment is dropped and
    its tail is what runs, in the document's own directory (RR06a shape);
    exactly one runnable segment may survive, so pipelines are refused
    (they would need a shell); shlex tokenises; the head token must be a
    BARE whitelisted tool (a path form like /usr/bin/wc is path-disguise —
    same tool, unapproved entry), git needs a whitelisted read subcommand,
    no --output write flag and (for log) a bounded paste-comparable shape,
    grep/wc need their required count/lines flag AND an operand (a count
    tool with no target reads stdin — a prose mention, never a pasted
    value); finally EVERY surviving shape passes the operand fence (v2.1
    M3, flag-value rule added by v2.2 F-1): non-flag tokens must resolve
    inside docdir or the whole command refuses as outside-docdir, and a
    '-'-leading token carrying / ~ or .. (the -fPATH / --file=PATH /
    -f~ / -f.. forms that bypassed v2.1) refuses as flag-path-form — either
    way not executed, not echoed. v2.3: every reason string this function
    emits is a member of the closed set enforced by census AXIS3 — each has
    at least one RERUN_CASE pin asserting it via the want_reason field, so
    renaming or deleting any generation site flips at least one pin red.
    v2.5 adds the 15th member, no-grammar-table (the fail-closed half of the
    grammar layer — see _flag_grammar), and the PATH/exec-resolution face is
    now constrained downstream in _run_cmd (F6), not here."""
    text = _md_unescape(text)
    for ch in ("`", "$(", "<", ">", "\n"):
        if ch in text:
            return None, "shell-metachar"
    segs = _split_segments(text)
    if segs is None:
        return None, "unbalanced-quote"
    kept = []
    for s in segs:
        if not s:
            return None, "empty-segment"
        toks = shlex.split(s)   # v2.3: no except-ValueError layer — the
                                # state machine above refuses every input
                                # shlex can reject (unbalanced quote, odd
                                # trailing backslash), emitting the same
                                # unbalanced-quote reason at refusal time
        if not toks:
            return None, "empty-segment"
        if toks[0] == "cd":
            continue
        kept.append(toks)
    if len(kept) != 1:
        return None, "multi-segment"
    argv = kept[0]
    tool = argv[0]
    if "/" in tool:
        return None, "path-disguise"
    if tool == "git":
        # v2.6 (2026-11-13, reviewer round-3 P1): the git replay face is
        # RETIRED wholesale — before the whitelist is even consulted (the
        # three git members left RERUN_READONLY with the face; see
        # _RETIRED_GIT_MEMBERS). Four grammar holes in a row (--files0-from
        # indirect input → --ext-diff/-p config spawn → log.showSignature
        # injection → --format=%G? explicit gpg.program invocation) showed
        # the git syntax-execution surface cannot be converged by
        # enumeration. Per the reviewer's own preferred option, every git
        # invocation refuses as git-replay-disabled BEFORE execution. Pure
        # scanning and the wc/grep replay tiers are untouched.
        return None, "git-replay-disabled"
    for t, m in RERUN_READONLY:
        if t != tool:
            continue
        if not _has_flag(argv, m):
            return None, "missing-required-flag"
        if not any(not a.startswith("-") for a in argv[1:]):
            return None, "no-operand"
        bad = _operands_in_scope(argv, docdir)              # v2.2 F-1: reason-carrying
        if bad:
            return None, bad
        g = _flag_grammar(tool, None, argv)                 # v2.4 P1-fix, AFTER path checks
        if g:
            return None, g
        return argv, None
    return None, "not-whitelisted"


# v2.6 retired-with-the-face: _GIT_HARDEN_PREFIX, _GIT_DIFF_SUBS and
# _git_harden/_bounded_log are DELETED (git replay refuses at _guard entry;
# the harden prefix would only have guarded an execution that can no longer
# happen). RR74/RR74b + the 26 flipped pins guard the retirement.



def _path_clean(raw):
    """v2.5 F6 — the PATH components a rerun may search, in order, ABSOLUTE
    entries only. The v2.4 executor inherited os.environ["PATH"] verbatim, and
    an EMPTY component means "search the CWD" — the CWD being the document's
    own directory — so `PATH=:$PATH` plus a planted ./git ran the attacker's
    binary under the whitelist (outer q3d F6: the value-drift line quoted the
    evil output); a RELATIVE component (., ./bin) is the same cwd-search by
    another name and is just as common in hand-edited CI PATHs. Dropping them
    keeps resolution deterministic. A tool that exists ONLY under such a
    component now skips as exec-unavailable — safe direction, OPEN faces v2.5
    (2), zero corpus carriers, pinned by RR71 and ENV-path-clean."""
    import os
    return [c for c in raw.split(os.pathsep) if c and os.path.isabs(c)]


def _exe_search(name, comps, cwd):
    """v2.5 F6 — a deterministic execvp-style PATH search. shutil.which was
    the obvious choice and is WRONG for this job: it resolves a RELATIVE
    component against the GATE's own cwd, while the child would resolve it
    against the DOCUMENT's directory (the spawn cwd) — so a planted ./wc in
    the doc dir stayed invisible to the resolver and RR71 could not see the
    hole it pins. This mirror resolves "" (the POSIX cwd-search spelling) and
    any relative component against the spawn cwd exactly as execvp does; a
    name containing '/' is refused (the guard's path-disguise rule asserted
    at the last mile too). Fed the _path_clean()'d list the answer is
    absolute by construction, and no hit ⇒ the "unavailable" SKIP branch."""
    import os
    if not name or "/" in name:
        return None
    for c in comps:
        d = cwd if not c else (c if os.path.isabs(c) else os.path.join(cwd, c))
        p = os.path.join(d, name)
        try:
            if os.path.isfile(p) and os.access(p, os.X_OK):
                return p
        except OSError:
            continue
    return None


def _run_cmd(argv, cwd, timeout):
    """argv-direct exec, never shell, capture stdout, bounded by timeout.
    stdin is /dev/null: a whitelisted count tool that was pasted WITHOUT its
    target would otherwise read the terminal (real-doc noise found while
    calibrating; the operand gate below refuses that shape first anyway).
    v2.5 F6: the head token is resolved over the _path_clean()'d PATH by
    _exe_search BEFORE the spawn and the resolved path is what execs — the
    guard's path-disguise rule (bare token only) can no longer be bypassed
    through PATH itself, and no empty/relative component can point the search
    at the document's own directory. A missing resolution is the
    "unavailable" status ⇒ SKIPPED-UNSAFE (exec-unavailable), never an error
    (collision R2: SKIP is the safe side).
    Returns (status, stdout, stderr) with status in ok/failed/timeout/unavailable."""
    import subprocess, os
    argv = list(argv)
    if not argv:
        return "unavailable", "", ""
    comps = _path_clean(os.environ.get("PATH", ""))
    env = {"PATH": os.pathsep.join(comps), "LANG": "C.UTF-8", "LC_ALL": "C.UTF-8"}
    exe = _exe_search(argv[0], comps, cwd)
    if not exe:
        return "unavailable", "", ""
    is_git = argv[0] == "git"
    argv[0] = exe
    if is_git:
        # v2.6: UNREACHABLE for whitelisted git replay (retired at _guard
        # entry) — kept as defence-in-depth for any future re-admission:
        # config-layer isolation without the deleted _git_harden prefix.
        env.update({"GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_SYSTEM":
                    "/dev/null", "GIT_CONFIG_NOSYSTEM": "1",
                    "GIT_OPTIONAL_LOCKS": "0", "GIT_TERMINAL_PROMPT": "0"})
    try:
        p = subprocess.run(argv, cwd=cwd, capture_output=True, text=True,
                           encoding="utf-8", errors="replace",
                           env=env, stdin=subprocess.DEVNULL, timeout=timeout)
    except subprocess.TimeoutExpired:
        return "timeout", "", ""
    except OSError:
        return "unavailable", "", ""
    return ("ok" if p.returncode == 0 else "failed"), p.stdout, p.stderr


def _split_inline(text):
    """(cmd, annotation_or_None): cut a same-line paste annotation at the
    arrow form, else drop a trailing '#' comment (which is NOT a value)."""
    m = _ANN.search(text)
    if m:
        return text[:m.start()].strip(), m.group(1).strip()
    m = re.search(r"\s#\S*", text)
    if m:
        return text[:m.start()].strip(), None
    return text, None


def _block_after(lines, ci, fences):
    """The pasted output block under an evidence line: following lines INSIDE
    the same fence, stopping at a blank line, the fence end, the next
    command-shaped line, an arrow line (which IS the paste), or a non-arrow #
    line (prose heading). Outside a fence there is no block medium — a prose
    command with no same-line value has no paste to verify (RR03 face)."""
    span = _fence_at(fences, ci)
    if not span:
        return None
    block = []
    for j in range(ci + 1, span[1] + 1):
        s = lines[j].strip()
        if not s or s.startswith("```") or CMD.search(lines[j]):
            break
        a = _ANN.search(s)
        if a:
            block.append(a.group(1).strip())
            break
        if s.startswith("#"):
            break
        block.append(s)
    return "\n".join(block) if block else None


def _evidence_command(lines, ci, fences):
    """(cmd_text, expected_text_or_None) for the evidence line at index ci.
    Deterministic extraction order: the first CMD-shaped backtick span, else
    the whole line (a leading "$ " paste marker stripped — note the detection
    CMD does NOT match such lines, so this is belt-and-braces for the span
    form); the annotation may live inside the span or after it."""
    raw = lines[ci]
    for m in re.finditer(r"`([^`]*)`", raw):
        s = m.group(1).strip()
        if CMD.search(s):
            cmd, ann_in = _split_inline(s)
            a = _ANN.search(raw[m.end():])
            return cmd, (ann_in or (a.group(1).strip() if a else None)) or _block_after(lines, ci, fences)
    cmd, ann = _split_inline(re.sub(r"^\s*\$\s*", "", raw).strip())
    if ann is None:
        ann = _block_after(lines, ci, fences)
    return cmd, ann


def _outputs_equal(expected, actual):
    """Pasted-block equality under the RECORDED tolerances (boundary record
    face 5): per-line whitespace-token comparison, the actual line may carry
    MORE tokens than the paste (wc prints the file name; 52号 pastes only the
    number), a trailing 「（…）」 annotation and a 「——」 explanation tail are
    cut, and a paste ending in 「…」/「...」 is a TRUNCATED stem (64号 sha
    form) which must prefix the actual token. Value changes inside the
    compared tokens are caught (RR02/RR08); a change only beyond the pasted
    tokens is masked by design — recorded, not hidden."""
    elines = [l.strip() for l in expected.splitlines() if l.strip()]
    alines = [l.rstrip() for l in actual.splitlines()]
    while alines and not alines[-1].strip():
        alines.pop()
    if not elines or len(elines) > len(alines):
        return False
    for e, a in zip(elines, alines):
        e = re.split(r"——", e)[0]
        e = re.sub(r"（[^（）]*）\s*$", "", e).strip()
        et, at = e.split(), a.split()
        if not et or len(et) > len(at):
            return False
        if et[-1].endswith(("…", "...")):
            stem = re.sub(r"(…|\.\.\.)$", "", et[-1])
            if et[:-1] != at[:len(et) - 1] or not at[len(et) - 1].startswith(stem):
                return False
            continue
        if et != at[:len(et)]:
            return False
    return True


def rerun_verdicts(path: str, timeout=None):
    """[(line_no, claim, verdict, detail)] for claim lines whose ±WINDOW scope
    carries an evidence command. Claim lines WITHOUT a window command are the
    default gate's face and get no rerun verdict (they are already reported);
    an output-role "# →" line in a command-bearing fence is skipped here by
    the SAME rule as violations() (it is a paste, not a claim). Verdicts: OK /
    RERUN-MISMATCH (execution failed, value drift, or no pasted block) /
    SKIPPED-UNSAFE (v2.1: including the operand-fence outside-docdir shape;
    v2.2 F-1 adds the flag-path-form refuse for -fPATH / --file=PATH /
    -f~ / -f.. — every one of these labels is now a want_reason holder and
    an AXIS3 census member) /
    SKIPPED-TIMEOUT. v2.1 M3: executed output content is never echoed into
    stdout — a value-drift line carries a sha256 prefix of the actual output
    instead (comparable across reruns, leaks no content; the paste side is
    document text and may be echoed)."""
    import hashlib
    import os
    if timeout is None:
        timeout = float(os.environ.get("RERUN_TIMEOUT", RERUN_TIMEOUT_DEFAULT))
    lines = Path(path).read_text(encoding="utf-8").splitlines()
    fences = _fence_spans(lines)
    cwd = str(Path(path).resolve().parent)
    out = []
    for i, line in enumerate(lines):
        if line.strip().startswith("```"):
            continue
        span = _fence_at(fences, i)
        if span and line.lstrip().startswith("#") and "→" in line:
            if any(CMD.search(l) for l in lines[span[0]:span[1] + 1]):
                continue
        claim = find_claim(line)
        if not claim:
            continue
        lo = max(0, i - WINDOW)
        hi = min(len(lines), i + WINDOW + 1)
        ci = None
        fallback = None
        # v2.6.2 (inner-loop redteam-0): v2.6.1's CMD single-source newly
        # admitted ls/find into the BACKTICK form of the detection set, and
        # the top-down FIRST pick then selected a leading `ls …` carrier
        # line as THE evidence command — _guard refused it (not-whitelisted)
        # and the verdict silently degraded to SKIPPED-UNSAFE (rc untouched):
        # a later whitelisted command that v2.6 WOULD have replayed (catching
        # the pasted value drift) was never evaluated. RR13 pins top-down
        # FIRST over the v2.6-era match set ("never the runnable one"), so
        # the fix is SCOPED: exactly the newly-admitted backtick members do
        # not participate in selection priority (they stay as fallback, so
        # an ls-only window keeps an informative SKIPPED verdict). Pins:
        # RR75 (restoration), RR76 (fallback), RR13 (unchanged).
        for j in range(lo, hi):
            if lines[j].strip().startswith("```"):
                continue
            m = CMD.search(lines[j])
            if not m:
                continue
            tool = (m.group(1) or m.group(2) or "").strip()
            if m.group(2) is not None and tool not in _V26_BACKTICK_TOOLS:
                if fallback is None:
                    fallback = j
                continue
            ci = j
            break
        if ci is None:
            ci = fallback
        if ci is None:
            continue
        cmd, expected = _evidence_command(lines, ci, fences)
        argv, why = _guard(cmd, docdir=cwd)
        if argv is None:                             # SKIP precedence is pinned (RR14)
            out.append((i + 1, claim.group(0), "SKIPPED-UNSAFE", f"({why}) cmd={cmd!r}"))
            continue
        if expected is None or not expected.strip():
            # R2 F1: a dangling arrow (annotation glued, value forgotten) yields
            # ann="" — NOT a paste. Same family as no-pasted-output-block; the
            # old path crashed here on expected.splitlines()[0] (IndexError,
            # aborting the whole multi-file pass). Red: /tmp/rl2/arrow-crash/.
            out.append((i + 1, claim.group(0), "RERUN-MISMATCH", f"(no-pasted-output-block) cmd={cmd!r}"))
            continue
        status, actual, _err = _run_cmd(argv, cwd, timeout)
        detail = f"cmd={cmd!r}"
        if status == "timeout":
            out.append((i + 1, claim.group(0), "SKIPPED-TIMEOUT", f"(> {timeout:g}s) {detail}"))
        elif status == "unavailable":
            out.append((i + 1, claim.group(0), "SKIPPED-UNSAFE", f"(exec-unavailable) {detail}"))
        elif status == "failed":
            out.append((i + 1, claim.group(0), "RERUN-MISMATCH", f"(cmd-failed rc≠0) {detail}"))
        elif _outputs_equal(expected, actual):
            out.append((i + 1, claim.group(0), "OK", detail))
        else:
            # v2.1 M3: no actual-content echo — prefix hash instead (choice
            # among bool/length/hash recorded in the header: hash keeps
            # rerun-to-rerun comparability without leaking content or size)
            ash = hashlib.sha256(actual.encode("utf-8", "replace")).hexdigest()[:12]
            out.append((i + 1, claim.group(0), "RERUN-MISMATCH",
                        f"(value-drift) {detail} expected={expected.splitlines()[0][:60]!r} "
                        f"actual-sha256={ash}"))
    return out


# v2.3 (outer-r2 Simplicity ①): rerun_report() deleted — it had ZERO
# callers: main() prints the verdict tuples directly and _rerun_selftest
# consumes rerun_verdicts; the list-of-formatted-strings middleman was
# v2.0-era convenience that no consumer ever adopted (grep-verified before
# removal; behaviour surface unchanged — keys-diff 0, --rerun 68号 stable).

_D10 = "x\n" * 10
_GREP2 = "x\ny\nx\n"
RERUN_CASES = [
    # v2.3 assertion-surface amendment (outer-r2 singularity prescription,
    # the "钉要钉到 reason" lesson): the tuple is SIX fields —
    # (id, expect_verdict, want_reason, doc_lines, planted_files, opts)
    #   expect_verdict "OK"/"RERUN-MISMATCH"/"SKIPPED-UNSAFE"/"SKIPPED-TIMEOUT",
    #   or None = "this line must produce NO rerun verdict at all".
    #   want_reason (v2.3): the SKIPPED detail's refusal label "(<reason>) …"
    #   is asserted to start with this string — None = verdict-level only.
    #   Every member of the closed refusal-reason set (census AXIS3,
    #   _EXPECTED_REASONS) has at least one non-None case, so renaming or
    #   deleting any generation site flips at least one pin RED (the v2.2
    #   disease this closes: path-disguise deleted wholesale left verdicts
    #   identical — nobody held the reason strings).
    #   files: {name: text} planted in the temp dir; value "FIFO" makes a fifo.
    #   opts: repo (make a one-commit git repo here), git ({KEY: cmd} → run and
    #   substitute {KEY} into the doc), timeout (seconds), sentinel (must NOT
    #   exist after the run), commits (expected commit count after the run),
    #   no-echo-head (v2.1 RR27), viol (v2.3 RR37-rerun-arrow-twin: expected
    #   bool(violations)
    #   for want=None cases — default True like RR15b, False asserts the
    #   violations/rerun #→ exemptions are TWINS), bare-env (v2.3 RR46: empty
    #   PATH for the case window so the executed tool is absent → the
    #   exec-unavailable skip branch).
    #   v2.5 NEW opts (each one the harness side of a permanent pin):
    #   gpg-poison (RR69, F5): after the repo is built, craft a commit object
    #   with a gpgsig header and set log.showSignature=true + gpg.program to an
    #   in-temp script that touches MARKER — the exact outer q3d A construction
    #   — so the sentinel assertion (marker must NOT exist after the run) has
    #   teeth on the executed env layer M9 proved nothing held.
    #   no-table-member (RR70, F1): temporarily extends RERUN_READONLY with a
    #   ("git","show") member that has NO grammar table — the outer's fail-open
    #   experiment as a permanent want_reason holder for no-grammar-table.
    #   path-poison (RR71, F6): runs the case with PATH=".:<real PATH>" and a
    #   planted executable ./wc that touches MARKER — the empty/relative-
    #   component attack; the real wc must answer and the sentinel stay absent.
    ("RR01-equal", "OK", None, ["```", "wc -l data.txt", "10 data.txt", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {}),                                          # matrix RR01
    ("RR02-stale", "RERUN-MISMATCH", None, ["```", "wc -l data.txt", "11 data.txt", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {}),                                          # the pair of RR01: paste drifted (过期贴值)
    ("RR03-no-block", "RERUN-MISMATCH", None, ["```", "wc -l data.txt", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {}),                                          # 窗内有命令、无粘贴输出块 = 忘贴块形
    ("RR04a-write-flag", "SKIPPED-UNSAFE", "git-replay-disabled", ["```", "git log --output=SENTINEL -1", "1 x", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {"repo": True, "git": {}, "sentinel": "SENTINEL"}),
    ("RR04b-network", "SKIPPED-UNSAFE", "not-whitelisted", ["```", "curl https://example.invalid/x", "200", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {}),                                          # detectable ≠ executable
    ("RR04c-pipe-sh", "SKIPPED-UNSAFE", "multi-segment", ["```", "cd . && grep -c x data.txt | sh", "2", "```", "本批共 10 行。"],
     {"data.txt": _GREP2}, {}),                                        # matrix RR04/RR06 dual shape
    ("RR04d-path-disguise", "SKIPPED-UNSAFE", "path-disguise", ["```", "cd . && /usr/bin/wc -l data.txt", "10 data.txt", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {}),
    ("RR75-selection-preference", "RERUN-MISMATCH", None,
     ["（`ls samples/` 产物清单见下）", "```", "wc -l data.txt", "11 data.txt", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {}),   # v2.6.2 redteam-0: the leading `ls` backtick (a v2.6.1-admitted detection member, NOT rerunnable) must not displace the whitelisted wc — the drift paste (11 vs actual 10) is executed and caught. Pre-v2.6.2 this case degraded to SKIPPED-UNSAFE/not-whitelisted (executed=0, drift never evaluated).
    ("RR76-ls-fallback", "SKIPPED-UNSAFE", "not-whitelisted",
     ["（`ls samples/` 产物清单）", "本批共 10 行。"],
     {"data.txt": _D10}, {}),   # v2.6.2 fallback semantics: an ls-only window keeps the FIRST hit → refused not-whitelisted (informative skip; v2.6-era this doc had no CMD hit at all)
    ("RR04e-subst", "SKIPPED-UNSAFE", "shell-metachar", ["```", "wc -l $(echo data.txt)", "10 data.txt", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {}),
    ("RR05a-slow", "SKIPPED-TIMEOUT", None, ["```", "grep -c x fifo", "2", "```", "本批共 10 行。"],
     {"fifo": "FIFO", "data.txt": _GREP2}, {"timeout": 1}),             # a fifo with no writer blocks grep → the timeout branch, not a hang
    ("RR05b-fast-under-switch", "OK", None, ["```", "wc -l data.txt", "10 data.txt", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {"timeout": 1}),                               # the pair: same switch, wall clock must not inflate
    ("RR06a-cd-tail", "OK", None, ["```", "cd . && wc -l data.txt", "10 data.txt", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {}),                                           # cd 后只读段执行
    ("RR07-revparse", "SKIPPED-UNSAFE", "git-replay-disabled", ["```", "git rev-parse --short HEAD", "{V1}", "```", "本批共 10 行。"],
     {}, {"repo": True, "git": {"V1": "git rev-parse --short HEAD"}}),  # whitelist member: rev-parse
    ("RR08-revparse-stale", "SKIPPED-UNSAFE", "git-replay-disabled", ["```", "git rev-parse --short HEAD", "deadbee", "```", "本批共 10 行。"],
     {}, {"repo": True, "git": {}}),                                    # the 68号/bb85222 stale-short-sha disease form
    # RR09-log: RETIRED at v2.6 — superseded by RR74b (same shape, expected
    # flipped OK→SKIPPED-UNSAFE/git-replay-disabled when the git face retired)
    ("RR10-lstree", "SKIPPED-UNSAFE", "git-replay-disabled", ["```", "git ls-tree HEAD", "{V1}", "```", "本批共 10 行。"],
     {}, {"repo": True, "git": {"V1": "git ls-tree HEAD"}}),            # member: ls-tree (tab-bearing output exercises the token comparison)
    ("RR11-grepc", "OK", None, ["```", "grep -c x data.txt", "2", "```", "本批共 10 行。"],
     {"data.txt": _GREP2}, {}),                                         # member: grep -c (RR04c pins the pipe-refusal side of the same tool)
    ("RR12-git-commit-refused", "SKIPPED-UNSAFE", "git-replay-disabled", ["```", "git commit -m second", "1 file changed", "```", "本批共 10 行。"],
     {"f.txt": "a\n"}, {"repo": True, "git": {}, "commits": 1}),        # WRITE subcommand: must not run (HEAD must still have 1 commit)
    ("RR13-first-in-window", "SKIPPED-UNSAFE", "path-disguise", ["```", "cd . && /usr/bin/wc -l data.txt", "wc -l data.txt", "10 data.txt", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {}),                                           # determinism pin: top-down FIRST is used, never "the runnable one"
    ("RR14-skip-precedence", "SKIPPED-UNSAFE", "multi-segment", ["```", "wc -l data.txt | sh", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {}),                                           # guard BEFORE the block check: UNSAFE ≠ no-pasted-block
    ("RR74-git-replay-retired", "SKIPPED-UNSAFE", "git-replay-disabled",
     ["```", "git log -1 --format=%G?", "N", "```", "本批共 1 行。"],
     {}, {}),  # v2.6: the reviewer round-3 P1 shape itself (%G? requests
     # signature verification → repo-local gpg.program executes). With the
     # git face retired the attack class is unreachable: EVERY git command
     # refuses as git-replay-disabled before execution — this pin guards the
     # retirement against silent revival (delete the retirement branch and
     # this case must flip to a red executed verdict).
    ("RR74b-git-retired-legacy-log", "SKIPPED-UNSAFE", "git-replay-disabled",
     ["```", "git log --oneline -1", "abc1234 fix", "```", "本批共 10 行。"],
     {}, {}),
     # the once-legal positive (RR09) now also refuses — the tier's honest
     # cost, pinned so the retirement cannot selectively leak old shapes
    ("RR73-dangling-arrow", "RERUN-MISMATCH", None,
     ["```", "wc -l data.txt →", "本批共 10 行。"],
     {"data.txt": _D10}, {}),  # R2 F1 / inner-loop IN-2 pin: the TRUE crash
     # form is a dangling arrow ON the command line inside the fence
     # (inner-loop mutation proof: the previous pin shape — arrow on its own
     # line after the closed fence — routes through the expected=None path
     # and stays green with the fix DELETED, i.e. the pin did not bite; this
     # shape yields ann="" and crashed pre-fix with IndexError)
    ("RR73b-arrow-cmdline-span", "RERUN-MISMATCH", None,
     [" 复核：`wc -l data.txt` → ", "本批共 10 行。"],
     {"data.txt": _D10}, {}),  # sibling: backtick-span form (pre-fix it
     # escaped via the or-chain fallback — same input family must now be
     # uniformly no-pasted-output-block, no divergent behaviour)
    ("RR15a-window-8", "OK", None, ["```", "wc -l data.txt", "10 data.txt", "```"] + ["x"] * 5 + ["本批共 10 行。"],
     {"data.txt": _D10}, {}),                                           # claim exactly WINDOW lines below the command
    ("RR15b-window-9", None, None, ["```", "wc -l data.txt", "10 data.txt", "```"] + ["x"] * 6 + ["本批共 10 行。"],
     {"data.txt": _D10}, {}),                                           # one line further: NO rerun verdict (the WINDOW is the single truth, no hardcoded 8 here) — and it must still be a DEFAULT violation
    ("RR16-span-pipe", "SKIPPED-UNSAFE", "multi-segment", ["复核：`grep -c x data.txt | sh` → 2 —— 本批共 10 行。"],
     {"data.txt": _GREP2}, {}),                                         # prose backtick-span extraction, 68号:136 shape
    ("RR17-inline-arrow", "OK", None, ["wc -l data.txt → 10 data.txt", "", "本批共 10 行。"],
     {"data.txt": _D10}, {}),                                           # prose same-line arrow paste (52号:494 shape)
    ("RR18-quoted-pipe-executes", "OK", None, ["复核：`grep -cE 'a|b' data.txt` → 2 —— 引号内竖线", "本批共 2 行。"],
     {"data.txt": "a\nb\nx\n"}, {}),                                     # v2.1 FLIP (the face-3 mandated re-adjudication): the quote-aware segmenter keeps 'a|b' in one token — it EXECUTES and compares (data has one a-line + one b-line = 2). The old overskip doc-line form glued prose after the value without ——; that shape now belongs to the MISMATCH face, restructured here so the pin tests the tokenizer, not the tolerances.
    # ---- RR21–RR24: v2.1 M3 operand fence, the four adjudicated shapes
    # (outer-r1 §5 P3′): absolute / ..-traversal / in-tree symlink pointing
    # out → SKIPPED-UNSAFE (outside-docdir), never executed, never echoed;
    # a legitimate in-tree relative operand → still executes (RR24 mirrors
    # the doc3e grep shape INSIDE the fence — deleting the fence keeps it
    # green, deleting the legit-relative allowance flips it red).
    # ---- RR25–RR26: v2.1 M4 extraction pins — RR25 replants the 68号:136
    # command verbatim (single-quoted `\*\*` regex + quoted pipe + relative
    # operand) against a planted target: the escapes must survive unescaping
    # INSIDE quotes while the pipe stays tokenized. RR27 pins the M3 echo
    # silence (no executed content in the drift detail). ----
    ("RR21-abs-operand", "SKIPPED-UNSAFE", "outside-docdir", ["```", "wc -l /etc/hosts", "9 /etc/hosts", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {}),                                           # doc3c carrier shape; fence deleted → it runs (and in v2.0 leaked the first line of /etc/hosts into stdout)
    ("RR22-dotdot-operand", "SKIPPED-UNSAFE", "outside-docdir", ["```", "wc -l ../../etc/hosts", "9 x", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {}),                                           # .. traversal segment — refused before resolve even matters
    ("RR23-symlink-escape", "SKIPPED-UNSAFE", "outside-docdir", ["```", "wc -l link.txt", "10 link.txt", "```", "本批共 10 行。"],
     {"link.txt": "LINK:/etc/hosts", "data.txt": _D10}, {}),            # the sneaky shape: relative name, in-tree link, realpath OUT — only containment-after-resolve catches it
    ("RR24-in-tree-operand", "OK", None, ["```", "grep -c root hosts.txt", "1", "```", "本批共 10 行。"],
     {"hosts.txt": "root:x\nnobody:x\n"}, {}),                          # legitimate relative executes and compares (RR11's grep twin under the fence)
    ("RR25-doc68-shape", "OK", None, ["复核：`grep -cE '^\\*\\*(真裁|知悉)' 本节` → 2 —— 表头各计", "本批共 2 行。"],
     {"本节": "**真裁（3 项）**：a\n**知悉（不需回复）**：b\nplain\n"}, {}),  # the 68号:136 command verbatim: `\*` inside SINGLE QUOTES survives normalization (regex, not markdown), the quoted | stays one token, operand 本节 is in-tree — executes and matches 2 (quote-aware split deleted: unbalanced-quote SKIP; in-quote unescape mutant: grep dies on '^**' → MISMATCH)
    ("RR26-quoted-semi", "OK", None, ["复核：`grep -c 'a;b' data.txt` → 1 —— 引号内分号", "本批共 2 行。"],
     {"data.txt": "a;b\nx\ny\n"}, {}),                                  # the ; twin of RR18: quoted semicolon no longer forges an empty segment
    ("RR27-no-actual-echo", "SKIPPED-UNSAFE", "git-replay-disabled", ["```", "git rev-parse --short HEAD", "deadbeef", "```", "本批共 10 行。"],
     {}, {"repo": True, "git": {}, "no-echo-head": True}),              # v2.1 M3 echo ban: the drift line must carry actual-sha256=…, NEVER the executed short-sha itself (RR08 pins the verdict, this pins the silence)
    # ---- RR28–RR29: v2.2 F-1 flag-value pins (the inner-audit oob_doc
    # probes, promoted to permanent). v2.1's fence SKIPPED every '-'-leading
    # token, so a path hidden INSIDE a flag ran: -fPATH concatenated and
    # --file=PATH equals executed a docdir-external pattern table (a 1-bit
    # read oracle, oracle_hit.md OK / oracle_miss.md rc≠0 under v2.1 —
    # measured this round). Deleting the v2.2 rule flips BOTH pins red (the
    # command runs — SKIPPED can never come back); the space-separated twin
    # needs no new rule (its value is a plain token the pre-existing /
    # -leading non-flag rule already refuses — verified live: atkD). ----
    ("RR28-concat-flag-path", "SKIPPED-UNSAFE", "flag-path-form", ["```", "grep -c -f/etc/passwd data.txt", "1", "```", "本批共 1 行。"],
     {"data.txt": _GREP2}, {}),                                         # -f/PATH concatenated form → flag-path-form, refuse WITHOUT executing
    ("RR29-equals-flag-path", "SKIPPED-UNSAFE", "flag-path-form", ["```", "grep -c --file=/etc/passwd data.txt", "1", "```", "本批共 1 行。"],
     {"data.txt": _GREP2}, {}),                                         # --file=PATH equals form → same refusal (RR24/RR09 prove pure path-free flags stay executable — regression self-proof)
    ("RR50-files0-from", "SKIPPED-UNSAFE", "non-allowlist-flag", ["```", "wc -l --files0-from targets", "1", "```", "本批共 1 行。"],
     {"data.txt": _GREP2, "targets": "/etc/passwd\0data.txt\0"}, {}),  # v2.4 P1-1 (review 4550440277), RENUMBERED from RR37-files0-from in v2.5 F9 (the shorthand collided with RR37-rerun-arrow-twin): operand IN docdir whose CONTENT points outside — argv scope checks cannot see through it; grammar refuses before execution
    ("RR51-ext-diff-p", "SKIPPED-UNSAFE", "git-replay-disabled", ["```", "git log -1 --oneline --ext-diff -p", "x", "```", "本批共 1 行。"],
     {"data.txt": _GREP2}, {}),                                          # v2.4 P1-2 (review 4550440291), was RR38-ext-diff-p (v2.5 F9): execution-extending git options refused by grammar; _git_harden env hygiene is the second layer (marker never created — RR69 and the PRB tier are its holders)
    ("RR52-cluster-positive", "OK", None, ["```", "grep -cE foo data.txt", "2", "```", "共 2 处。"],
     {"data.txt": "foo foo\nbar\nfoo\n"}, {}),                          # was RR39-cluster-positive (v2.5 F9): positive control — a legitimate -cE cluster STILL executes and verifies, the grammar must not become refuse-everything (acceptance rule 2)
    # ---- RR30/RR31: per-component pins for the flag-path rule (inner
    # re-review m-A: the rule tests /, ~, .. but only / had coverage —
    # deleting the ~ or .. branch left all pins green). Both forms are
    # SKIPPED under v2.2 (verified live); deleting the respective component
    # flips exactly these red. ----
    ("RR30-tilde-flag-path", "SKIPPED-UNSAFE", "flag-path-form", ["```", "grep -c -f~ data.txt", "1", "```", "本批共 1 行。"],
     {"data.txt": _GREP2}, {}),                                         # ~-in-flag → flag-path-form
    ("RR31-dotdot-flag-path", "SKIPPED-UNSAFE", "flag-path-form", ["```", "grep -c -e.. data.txt", "1", "```", "本批共 1 行。"],
     {"data.txt": _GREP2}, {}),                                         # ..-in-flag → flag-path-form
    # ---- RR19/RR20: added DURING implementation (S3 发现=补行, matrix
    # addendum): the whole-dir rerun face showed prose command MENTIONS
    # (bare "wc -l", bare "git log") reaching the executor. ----
    ("RR19-no-operand", "SKIPPED-UNSAFE", "no-operand", ["```", "wc -l", "10", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {}),                                           # count tool without a target reads stdin — a mention, not evidence (delete the operand gate: it executes and mismatches)
    ("RR20-log-unbounded", "SKIPPED-UNSAFE", "git-replay-disabled", ["```", "git log", "…", "```", "本批共 10 行。"],
     {}, {"repo": True, "git": {}}),                                    # bare `git log` mention (35号:3 carrier) — bounded-shape gate (RR09 pins the accepted shape)
    # ---- v2.3 收编 pins (outer-r2 镜二② 13-face list, face # → case; the
    # fixR-A brief's core four are RR32/RR37-rerun-arrow-twin/
    # RR38-unclosed-fence/RR48 (the v2.4 trio left this list when v2.5 F9
    # renumbered it to RR50/51/52) — unbalanced-quote
    # (M-2), the rerun-side #→ twin, the unclosed-fence extension, and the
    # markdown-unescape OUTSIDE-quote half (M-1, the P4 identity mutant's
    # zero-red face). Faces the brief deems not separately pinnable are
    # 并入 AXIS3 at the census site; every face below was mutation-checked
    # OUT OF TREE before landing (see fix report). ----
    ("RR32-unbalanced-quote", "SKIPPED-UNSAFE", "unbalanced-quote",
     ["```", "wc -l 'unclosed data.txt", "10 data.txt", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {}),                                           # face#2/M-2: unterminated quote refuses via the state machine — the face L411 had claimed an RR14 lineage for while RR14 pins skip-precedence. Rename/delete of the unbalanced-quote label flips this (AXIS3 member).
    ("RR33-empty-segment", "SKIPPED-UNSAFE", "empty-segment",
     ["```", "wc -l data.txt |", "10 data.txt", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {}),                                           # face#6: trailing separator forges an empty segment (outer-r2 said suite-unconstructible — the trailing-| form constructs it)
    ("RR34-missing-required-flag", "SKIPPED-UNSAFE", "missing-required-flag",
     ["```", "grep x data.txt", "2", "```", "本批共 10 行。"],
     {"data.txt": _GREP2}, {}),                                          # face#3: grep WITHOUT -c — the count-tool flag gate refuses a shape whose paste could be anything
    ("RR35-long-flag-twin", "RERUN-MISMATCH", None,
     ["```", "grep --count x data.txt", "999", "```", "本批共 10 行。"],
     {"data.txt": _GREP2}, {}),                                          # face#4: _LONG_FLAG map — with the map the token counts as the flag and the command REACHES the executor (BSD grep: cmd-failed, GNU grep: value-drift — the verdict is MISMATCH either way); deleting the map turns it into a missing-required-flag SKIP → flips. Verdict-stable across platforms by construction.
    ("RR36-nonflag-tilde", "SKIPPED-UNSAFE", "outside-docdir",
     ["```", "wc -l ~", "1 ~", "```", "本批共 10 行。"],
     {"~": "a\n"}, {}),                                                  # face#9 (m-9): a real in-tree file NAMED ~ — argv never tilde-expands, so ONLY the prefix check refuses it; containment is NOT redundant here (deleting the check executes wc against the planted file → flips to OK). The / and ..-prefix twins were measured redundant and deleted (Simplicity ③): RR21/22 now refuse at the realpath site.
    ("RR37-rerun-arrow-twin", None, None,
     ["```", "wc -l data.txt", "10 data.txt", "# → 本批共 10 行。", "```"],
     {"data.txt": _D10}, {"viol": False}),                               # face#11 (m-14): the rerun-side #→ exemption is declared "the SAME rule as violations()" — asserting BOTH no-verdict AND no-violation pins both copies; deleting either exemption flips this in its own layer (the v1.5 rule's v2.0 copy shipped unpinned for two generations).
    ("RR38-unclosed-fence", "OK", None,
     ["```", "wc -l data.txt", "10 data.txt", "", "本批共 10 行。"],
     {"data.txt": _D10}, {}),                                            # face#7: unclosed fence extends to EOF (desync-safe) — the paste below stays in span; deleting the extension drops the block → no-pasted-output-block flips it (the (start,i) pairing routes are pinned by SELFTEST 600/601/604 via _fence_at; this pins the extend line itself).
    ("RR39-block-stop-hash", "OK", None,
     ["```", "wc -l data.txt", "10 data.txt", "# trailing note", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {}),                                            # face#12 stop-branch (bare #): the block stops at a prose # line; removing the stop appends it → elines>alines flips to MISMATCH
    ("RR40-block-stop-cmd", "OK", None,
     ["```", "wc -l a.txt", "10 a.txt", "wc -l b.txt", "```", "本批共 10 行。"],
     {"a.txt": _D10, "b.txt": "y\n"}, {}),                               # face#12 stop-branch (CMD): the next command line is NOT paste; removing the stop poisons the block → flips
    ("RR41-block-arrow-join", "OK", None,
     ["```", "wc -l data.txt", "# → 10 data.txt", "junk beyond", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {}),                                            # face#12 stop-branch (arrow): the arrow line IS the paste — its annotation is taken and the scan BREAKS; removing the break lets "junk beyond" join → flips (removing only the append flips too, via empty block)
    ("RR42-no-fence-no-block", "RERUN-MISMATCH", None,
     ["wc -l data.txt", "10 data.txt", "", "本批共 10 行。"],
     {"data.txt": _D10}, {}),                                            # face#12 medium rule: OUTSIDE a fence there is no block medium (RR03's prose face) — deleting `if not span: return None` scans past the blank and FINDS a block → flips to OK
    ("RR43-inline-tail-hash", "OK", None,
     ["```", "wc -l data.txt #tail", "10 data.txt", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {}),                                            # face#12 _split_inline: a trailing non-arrow # comment is cut from the command (it is not a value); deleting the cut hands "#tail" to wc → cmd-failed flips
    ("RR44-paren-suffix-cut", "OK", None,
     ["```", "wc -l data.txt", "10 data.txt（含文件名）", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {}),                                            # face#13: 「（…）」 annotation suffix is cut before token comparison; deleting the cut mismatches → flips
    ("RR45-ellipsis-stem", "SKIPPED-UNSAFE", "git-replay-disabled",
     ["git rev-parse --short HEAD → {V1}…", "", "本批共 10 行。"],
     {}, {"repo": True, "git": {"V1": "git rev-parse --short HEAD"}}),   # face#13: 「…」 truncated stem must PREFIX the actual token (64号 sha form); deleting the stem branch mismatches → flips (this is the rule P19's flip-to-MISMATCH depends on)
    ("RR46-exec-unavailable", "SKIPPED-UNSAFE", "exec-unavailable",
     ["```", "wc -l data.txt", "10 data.txt", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {"bare-env": True}),                            # face#6 twin: tool vanished from PATH mid-rerun → OSError → SKIP, never error (collision R2) — the branch outer-r2 called unconstructible; bare-env constructs it
    ("RR47-md-escaped-pipe", "OK", None,
     ["```", "wc -l a\\|b.txt", "10 a|b.txt", "```", "本批共 10 行。"],
     {"a|b.txt": _D10}, {}),                                             # face#1 (M-1, \`-half is invisible: the raw backtick already trips shell-metachar either way — 并入 the shell-metachar pin RR04e): the state machine's OUTSIDE-quote backslash-pair keeps \| in one token and shlex lands a literal-pipe filename (planted) → executes; deleting the pair branch splits the segment (and the orphan trailing backslash now refuses at the machine — no crash, no silent widen) → flips
    ("RR48-md-escaped-dquote", "SKIPPED-UNSAFE", "unbalanced-quote",
     ["```", 'wc -l foo\\"bar.txt', "1 x", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {}),                                            # face#1 (M-1 core): OUTSIDE quotes the markdown \" unescapes to a REAL quote before tokenizing — the display-escape intent becomes shell state (header face v2.1(3) recorded this tokenization change; it had no probe). Identity-mutating _md_unescape keeps the backslash, shlex eats the pair, the literal-name operand EXECUTES (cmd-failed MISMATCH) → this is the P4 zero-red face, now RED on mutation.
    # ---- v2.5 pins (outer-v24 fix list). RR53–RR68 are the F2 half: the
    # coverage battery found 15/22 grammar entries with a REAL accept-face
    # effect and ZERO pins (delete or typo one of them and the whole suite
    # stayed green while every pasted `wc --lines` / `ls-tree -r` doc silently
    # flipped to refused) — each entry now has a positive carrier, and census
    # AXIS4 blanks each entry on its own table line and demands a flip.
    # RR69 (F5) pins the EXECUTED-ENVIRONMENT layer with an observable effect
    # (marker sentinel), RR70 (F1) the fail-closed grammar label, RR71 (F6)
    # the PATH-resolution face. ----
    ("RR53-wc-lines-long", "RERUN-MISMATCH", None,
     ["```", "wc --lines data.txt", "99 data.txt", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {}),                                            # F2 wc/--lines — platform-stable like RR35: BSD wc has no --lines (cmd-failed), GNU prints 10 data.txt (value-drift); EITHER WAY the shape REACHES the executor, which is what the table entry grants. Blank "--lines" ⇒ SKIPPED ⇒ flips.
    ("RR54-grep-ere-long", "OK", None,
     ["```", "grep --extended-regexp -c x data.txt", "2", "```", "本批共 2 行。"],
     {"data.txt": _GREP2}, {}),                                          # F2 grep/--extended-regexp long twin executes (GNU/ugrep/BSD all spell it)
    ("RR55-revparse-abbrev-ref", "SKIPPED-UNSAFE", "git-replay-disabled",
     ["```", "git rev-parse --abbrev-ref HEAD", "{V1}", "```", "本批共 10 行。"],
     {}, {"repo": True, "git": {"V1": "git rev-parse --abbrev-ref HEAD"}}),
    ("RR56-revparse-sq", "SKIPPED-UNSAFE", "git-replay-disabled",
     ["```", "git rev-parse --sq HEAD", "{V1}", "```", "本批共 10 行。"],
     {}, {"repo": True, "git": {"V1": "git rev-parse --sq HEAD"}}),
    ("RR57-revparse-verify", "SKIPPED-UNSAFE", "git-replay-disabled",
     ["```", "git rev-parse --verify HEAD", "{V1}", "```", "本批共 10 行。"],
     {}, {"repo": True, "git": {"V1": "git rev-parse --verify HEAD"}}),
    ("RR58-lstree-r", "SKIPPED-UNSAFE", "git-replay-disabled",
     ["```", "git ls-tree -r HEAD", "{V1}", "```", "本批共 10 行。"],
     {}, {"repo": True, "git": {"V1": "git ls-tree -r HEAD"}}),
    ("RR59-lstree-l", "SKIPPED-UNSAFE", "git-replay-disabled",
     ["```", "git ls-tree -l HEAD", "{V1}", "```", "本批共 10 行。"],
     {}, {"repo": True, "git": {"V1": "git ls-tree -l HEAD"}}),
    ("RR60-lstree-t", "SKIPPED-UNSAFE", "git-replay-disabled",
     ["```", "git ls-tree -t HEAD", "{V1}", "```", "本批共 10 行。"],
     {}, {"repo": True, "git": {"V1": "git ls-tree -t HEAD"}}),
    ("RR61-lstree-name-only", "SKIPPED-UNSAFE", "git-replay-disabled",
     ["```", "git ls-tree --name-only HEAD", "{V1}", "```", "本批共 10 行。"],
     {}, {"repo": True, "git": {"V1": "git ls-tree --name-only HEAD"}}),
    ("RR62-lstree-long", "SKIPPED-UNSAFE", "git-replay-disabled",
     ["```", "git ls-tree --long HEAD", "{V1}", "```", "本批共 10 行。"],
     {}, {"repo": True, "git": {"V1": "git ls-tree --long HEAD"}}),
    ("RR63-lstree-full-name", "SKIPPED-UNSAFE", "git-replay-disabled",
     ["```", "git ls-tree --full-name HEAD", "{V1}", "```", "本批共 10 行。"],
     {}, {"repo": True, "git": {"V1": "git ls-tree --full-name HEAD"}}),
    ("RR64-log-decorate", "SKIPPED-UNSAFE", "git-replay-disabled",
     ["```", "git log --decorate --oneline -1", "{V1}", "```", "本批共 10 行。"],
     {}, {"repo": True, "git": {"V1": "git log --decorate --oneline -1"}}),
    ("RR65-log-nocolor", "SKIPPED-UNSAFE", "git-replay-disabled",
     ["```", "git log --no-color --oneline -1", "{V1}", "```", "本批共 10 行。"],
     {}, {"repo": True, "git": {"V1": "git log --no-color --oneline -1"}}),
    ("RR66-log-noextdiff", "SKIPPED-UNSAFE", "git-replay-disabled",
     ["```", "git log --no-ext-diff --oneline -1", "{V1}", "```", "本批共 10 行。"],
     {}, {"repo": True, "git": {"V1": "git log --no-ext-diff --oneline -1"}}),
    ("RR67-log-format-h", "SKIPPED-UNSAFE", "git-replay-disabled",
     ["```", "git log --format=%h -1", "{V1}", "```", "本批共 10 行。"],
     {}, {"repo": True, "git": {"V1": "git log --format=%h -1"}}),      # F2 log regex --format=.*
    ("RR68-log-pretty-eq", "SKIPPED-UNSAFE", "git-replay-disabled",
     ["```", "git log --pretty=format:%h -1", "{V1}", "```", "本批共 10 行。"],
     {}, {"repo": True, "git": {"V1": "git log --pretty=format:%h -1"}}),  # F2/F3: the ONLY holder of the --pretty regex now that the code special case is gone — the battery had called the entry "NO EFFECT (dead)" because the double write shadowed it; blanking "--pretty.*" flips THIS
    ("RR69-showsig-poison", "SKIPPED-UNSAFE", "git-replay-disabled",
     ["```", "git log --oneline", "{V1}", "```", "本批共 10 行。"],
     {}, {"repo": True, "gpg-poison": True, "sentinel": "MARKER",
          "git": {"V1": "git -c log.showSignature=false log --oneline"}}),
    # F5, the observable-effect pin the v2.4 round never had (M9: every pin
    # stayed green with the whole env-sanitisation block reverted to v2.3).
    # Construction = outer q3d A: gpgsig header on the commit object +
    # log.showSignature=true + gpg.program=<script that touches MARKER>. The
    # whitelist shape here is the POSITIVE one acceptance 2 insists must keep
    # working, so the refusal cannot be bought by rejecting everything: the
    # verdict must be OK AND the marker must not exist. Delete the
    # log.showSignature=false -c pair and gpg runs — RR69 flips red on the
    # sentinel, not on a string.
    ("RR70-no-grammar-table", "SKIPPED-UNSAFE", "git-replay-disabled",
     ["```", "git show --stat HEAD", "1", "```", "本批共 1 行。"],
     {}, {"no-table-member": True}),                                     # F1: the outer's fail-open experiment (whitelist member without a grammar table) as a permanent want_reason holder — refuses WITHOUT executing; renaming the label flips this, and the ENV-grammar-invariant pin makes the member itself impossible to add silently.
    ("RR72-color-ui-poison", "SKIPPED-UNSAFE", "git-replay-disabled",
     ["```", "git log --oneline", "{V1}", "```", "本批共 10 行。"],
     {}, {"repo": True, "color-poison": True,
          "git": {"V1": "git -c color.ui=never log --oneline"}}),
    # exit25 M-ANSI (outer round 2, 2026-11-13): a victim repo whose own
    # .git/config sets color.ui=always injects ANSI into the whitelisted
    # POSITIVE shape `git log --oneline`; a faithful plain-text paste then
    # fails value-drift (rc=1, false refusal of the acceptance-2 shape).
    # Same threat model as F5 (repo-config write access), same one-line
    # recipe (-c on the command line outranks repo-local AND include.path —
    # precedence live-tested by the outer). Not exec-capable, so it is NOT
    # part of the 105 strategy handoff set. Delete the color.ui=never pair
    # and the executor's paste goes coloured — RR72 flips red on the drift
    # itself, an observable effect, not a string.
    ("RR71-path-relative-poison", "OK", None,
     ["```", "wc -l data.txt", "10 data.txt", "```", "本批共 10 行。"],
     {"data.txt": _D10}, {"path-poison": True, "sentinel": "MARKER"}),  # F6: PATH=".:<real>" + an executable ./wc planted in the doc dir that touches MARKER — v2.4 searched the CWD and ran the plant (OK verdict from the attacker, marker=True); v2.5 drops the relative component, the REAL wc answers, and the sentinel stays absent.
]

_RERUN_GIT_ENV = {
    "GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_SYSTEM": "/dev/null",
    "GIT_AUTHOR_NAME": "gate", "GIT_AUTHOR_EMAIL": "gate@local",
    "GIT_COMMITTER_NAME": "gate", "GIT_COMMITTER_EMAIL": "gate@local",
    "GIT_AUTHOR_DATE": "2026-11-11T00:00:00", "GIT_COMMITTER_DATE": "2026-11-11T00:00:00",
}


def _plant_files(d, files):
    """Materialize a case's planted face: normal text files, FIFOs (the
    timeout branch) and v2.1 'LINK:<target>' symlinks (the M3 in-tree-
    symlink-points-out shape; the target may be outside the temp dir or
    even nonexistent — realpath containment rejects either)."""
    import os
    for name, content in files.items():
        p = os.path.join(d, name)
        if content == "FIFO":
            os.mkfifo(p)
        elif content.startswith("LINK:"):
            os.symlink(content[5:], p)
        else:
            open(p, "w", encoding="utf-8").write(content)


def _poison_repo(d, env):
    """v2.5 F5 — the RR69 construction, ported VERBATIM from the outer's q3d
    A segment (the run that first made the marker appear under a passing
    verdict): rewrite the single commit so it CARRIES a gpgsig header, point
    HEAD at it, then set log.showSignature=true and gpg.program=<script that
    touches MARKER> in the REPO-LOCAL config. The document lives in this repo,
    so the whitelisted positive shape `git log --oneline` executes exactly
    where the poison is — which is the whole point: the guard's grammar and
    the operand fence are correct about THIS shape (it must keep executing —
    acceptance 2), only the executed environment can be wrong. MARKER is
    removed on entry so no stale state can fake a pass or a fail."""
    import os
    import subprocess
    marker = os.path.join(d, "MARKER")
    if os.path.exists(marker):
        os.remove(marker)
    evil = os.path.join(d, "evil-gpg.sh")
    open(evil, "w", encoding="utf-8").write(f"#!/bin/sh\ntouch {marker}\nexit 2\n")
    os.chmod(evil, 0o755)
    raw = subprocess.run(["git", "cat-file", "-p", "HEAD"], cwd=d, env=env,
                         capture_output=True, text=True).stdout
    hdr, _, body = raw.partition("\n\n")
    obj = (hdr + "\ngpgsig -----BEGIN PGP SIGNATURE-----\n\n zz\n"
           "-----END PGP SIGNATURE-----\n\n" + body)
    sha = subprocess.run(["git", "hash-object", "-t", "commit", "-w", "--stdin"],
                         cwd=d, env=env, capture_output=True, text=True,
                         input=obj).stdout.strip()
    subprocess.run(["git", "update-ref", "refs/heads/gatepoison", sha], cwd=d,
                   env=env, capture_output=True)
    subprocess.run(["git", "symbolic-ref", "HEAD", "refs/heads/gatepoison"],
                   cwd=d, env=env, capture_output=True)
    subprocess.run(["git", "config", "log.showSignature", "true"], cwd=d,
                   env=env, capture_output=True)
    subprocess.run(["git", "config", "gpg.program", evil], cwd=d, env=env,
                   capture_output=True)
    if os.path.exists(marker):          # the construction itself must be inert
        os.remove(marker)
    return evil


def _rerun_selftest() -> int:
    """Executes RERUN_CASES in throwaway temp dirs (hermetic: planted files,
    a temp git repo, a fifo for the timeout branch — never the real corpus).
    Returns the number of drifted cases; prints selftest ok/FAIL lines so
    --census flip counting covers this tier too.
    v2.5: three new opts (gpg-poison / no-table-member / path-poison) each
    mutate the EXECUTION CONTEXT of one case only — the PATH and the
    RERUN_READONLY globals are restored in the same finally that guards the
    rerun call, so no later case (or tier) can observe them."""
    global RERUN_READONLY
    import os
    import shutil
    import subprocess
    import tempfile
    bad = 0
    for cid, want, want_reason, doc, files, opts in RERUN_CASES:
        d = tempfile.mkdtemp()
        headsha = ""
        try:
            env = {**os.environ, **_RERUN_GIT_ENV}
            if opts.get("repo"):
                subprocess.run(["git", "init", "-q", "."], cwd=d, env=env,
                               capture_output=True, text=True, check=True)
                open(os.path.join(d, "f.txt"), "w").write("a\n")
                subprocess.run(["git", "add", "f.txt"], cwd=d, env=env, check=True,
                               capture_output=True)
                subprocess.run(["git", "commit", "-qm", "init"], cwd=d, env=env, check=True,
                               capture_output=True)
                if opts.get("gpg-poison"):      # v2.5 F5: RR69's poisoned repo
                    _poison_repo(d, env)
                if opts.get("color-poison"):    # exit25 M-ANSI: RR72's vector —
                    subprocess.run(["git", "config", "color.ui", "always"],
                                   cwd=d, env=env, check=True, capture_output=True)
                if opts.get("no-echo-head"):    # content the executor knows but stdout must never show
                    headsha = subprocess.run(["git", "rev-parse", "--short", "HEAD"], cwd=d, env=env,
                                             capture_output=True, text=True).stdout.strip()
                _plant_files(d, files)
                text = "\n".join(doc)
                for key, cmd in (opts.get("git") or {}).items():
                    r = subprocess.run(shlex.split(cmd), cwd=d, env=env,
                                       capture_output=True, text=True)
                    text = text.replace("{" + key + "}", r.stdout.strip())
                path = os.path.join(d, "doc.md")
                open(path, "w", encoding="utf-8").write(text + "\n")
            else:
                _plant_files(d, files)
                path = os.path.join(d, "doc.md")
                open(path, "w", encoding="utf-8").write("\n".join(doc) + "\n")
            _old_path = None
            if opts.get("bare-env") or opts.get("path-poison"):
                _old_path = os.environ.get("PATH")
                if opts.get("bare-env"):      # v2.3 RR46: the whitelisted tool is ABSENT at exec time
                    os.environ["PATH"] = ""
                else:                         # v2.5 F6 RR71: PATH=".:<real>" — a RELATIVE
                    # component means "search the document's own directory", and that
                    # directory now carries an executable literally named wc. With the
                    # v2.5 filter the component is dropped and the REAL wc answers; with
                    # the filter deleted the plant runs, answers "10 data.txt" (so the
                    # VERDICT would still look fine) and the sentinel is what catches it.
                    os.environ["PATH"] = "." + os.pathsep + _old_path
                    open(os.path.join(d, "wc"), "w", encoding="utf-8").write(
                        '#!/bin/sh\ntouch MARKER\necho "10 data.txt"\n')
                    os.chmod(os.path.join(d, "wc"), 0o755)
            _member_saved = None
            if opts.get("no-table-member"):   # v2.5 F1 RR70: the outer's fail-open
                _member_saved = RERUN_READONLY    # experiment as a permanent pin —
                RERUN_READONLY = RERUN_READONLY + (("git", "show"),)
            try:
                got_list = rerun_verdicts(path, timeout=opts.get("timeout"))
            finally:
                if _old_path is not None:
                    os.environ["PATH"] = _old_path
                if _member_saved is not None:
                    RERUN_READONLY = _member_saved
            got = got_list[0][2] if len(got_list) == 1 else (None if not got_list else "MULTI")
            ok = (got == want)
            if ok and want_reason is not None:  # v2.3: the refusal LABEL is pinned, not just the verdict —
                ok = bool(got_list) and got_list[0][3].startswith(f"({want_reason})")
                # this is the field that makes census AXIS3 meaningful
            if not want:                       # expect NO verdict for this line
                ok = (got is None) and (bool(violations(path)) == opts.get("viol", True))
                # RR15b default: no verdict ∧ still a DEFAULT violation (window miss).
                # RR37 {"viol": False}: no verdict ∧ no violation — both #→ exemption
                # layers must agree (the twin assertion of the "same rule" claim).
            if ok and opts.get("sentinel") and os.path.exists(os.path.join(d, opts["sentinel"])):
                ok = False                     # the SKIP claim is a lie — it ran and wrote
            if ok and opts.get("commits"):
                n = subprocess.run(["git", "rev-list", "--count", "HEAD"], cwd=d, env=env,
                                   capture_output=True, text=True).stdout.strip()
                ok = (n == str(opts["commits"]))
            full_det = got_list[0][3] if got_list else ""
            if ok and headsha and headsha in full_det:
                ok = False                     # v2.1 M3 echo ban broken — the detail leaked executed content
            detail = full_det[:60] if got_list else "no verdict"
            bad += 0 if ok else 1
            print(f"selftest {'ok ' if ok else 'FAIL'} want_rerun={want!r} want_reason={want_reason!r} "
                  f"got={got!r} :: {cid} :: {detail}")
        except Exception as e:                              # a broken harness must show up as a FAIL line, never a traceback
            bad += 1
            print(f"selftest FAIL want_rerun={want!r} want_reason={want_reason!r} got=EXC :: {cid} :: {e!r}")
        finally:
            shutil.rmtree(d, ignore_errors=True)
    return bad


# v2.1 N4: frozen census expectations. These are a DELIBERATE second source
# of truth (header face v2.1(4)): a units-set or whitelist edit must ALSO
# touch the expectation here, or the axis goes red and names the missing/
# unexpected member — the v2.0 disease was that the axis iterated over the
# derived list, so deleting a member shrank the axis itself and it reported
# "ALL 4 pinned" GREEN (and deleting |PASS) mis-attributed the failure to
# 提交 via the head/tail special cases). Keep the flat-string forms below:
# they must never match a live anchor token of their own (no `|处|`, no
# indented `("wc", "-l"),` line).
_EXPECTED_UNITS = ("处", "件", "条", "项", "次", "类", "组", "波", "枚", "篇", "包",
                   "个", "轮", "行", "测试", "函数", "删除", "新增", "新提交", "提交", "PASS",
                   "样本", "用例")  # v2.7 (adjudication-chamber MR item 1): +样本/用例 — corpus vocabulary from a second campaign
_EXPECTED_CMD = ("grep -c", "wc -l")
# v2.6: the three git members retired with the face (see RERUN_READONLY);
# the retired trio is registered in _RETIRED_GIT_MEMBERS — revival flows
# through re-adding to RERUN_READONLY, which the census diff catches here.
# v2.3 AXIS3 (outer-r2 singularity prescription, executed in this round —
# the "钉要钉到 reason" lesson): the refusal-reason strings emitted by the
# rerun guard are a CLOSED SET of literals too, so they get the same
# census treatment as units and the whitelist. The set is DERIVED from the
# source (a `return [None, ]"label"` statement — the line-end anchor keeps
# _run_cmd's status strings "timeout"/"unavailable"/"failed"/"ok", which are
# NOT refusal labels, out of the axis) plus the exec-unavailable label site
# in rerun_verdicts, then diffed against the frozen tuple below — both
# directions, member count included (the AXIS2 lesson: an axis may never
# iterate itself smaller into a green; a removal is NAMED as AXIS3 missing).
_EXPECTED_REASONS = ("empty-segment", "exec-unavailable", "flag-path-form",
                     "git-replay-disabled",
                     "missing-required-flag", "multi-segment",
                     "no-operand", "non-allowlist-flag",
                     "not-whitelisted",
                     "outside-docdir",
                     "path-disguise", "shell-metachar", "unbalanced-quote")
# v2.6: "git-replay-disabled" (13th member) retires the git replay face —
# RR74/RR74b + the 26 flipped legacy pins hold its want_reason. The three
# git-only special-case reasons (log-unbounded-shape, no-grammar-table,
# write-flag) died WITH the face: their generation sites sat behind the
# retirement branch, census AXIS3 flagged all three unpinned, and dead
# labels are removed (retired-with-the-face, same doctrine as the whitelist
# trio). wc/grep still hit write-flag-shaped inputs via non-allowlist-flag.
# v2.5 F1: "no-grammar-table" is the 15th member (the fail-closed refuse —
# RR70 holds its want_reason, ENV-grammar-invariant makes it unreachable by
# accident). F2: the grammar TABLES are now a closed set too, axis four: the
# derived entry list comes from the live _GRAMMAR/_GIT_GRAMMAR objects (never
# a hand-copied mirror), diffed against this frozen 22-member register, then
# mutated blank-on-its-own-line — an entry may be deleted or typo-fixed only
# over the flip of its RR53–RR68 positive carrier.
_EXPECTED_GRAMMAR = ("grep/exact/--count", "grep/exact/--extended-regexp",
                     "grep/regex/-[cE]+", "wc/exact/--lines", "wc/exact/-l")
# v2.6: the 17 git grammar entries retired with the face (unreachable past
# the _guard retirement; census flagged every one unpinned). The five
# wc/grep entries survive with their carriers.


def _derive_grammar():
    """AXIS4 derivation from the LIVE dicts (P-Ⅰ law: a hand-maintained mirror
    drifts silently green). Naming is "<table-key>/<kind>/<token>" — the
    table-key is the dict's own key (tool name for _GRAMMAR, subcommand for
    _GIT_GRAMMAR) and doubles as the anchor that locates the entry's SOURCE
    LINE, which is what makes the per-line blanking mutation unambiguous."""
    entries = []
    for key, (exact, regexes) in list(_GRAMMAR.items()) + list(_GIT_GRAMMAR.items()):
        entries += [f"{key}/exact/{t}" for t in exact]
        entries += [f"{key}/regex/{rx}" for rx in regexes]
    return tuple(sorted(entries))


def _derive_reasons(src: str):
    """REFUSE_REASONS derived: refusal-label returns + the exec-unavailable
    f-string label. Sorted tuple — the register of the open set that the
    outer-r2 singularity section identified (closed sets 100% census-covered,
    guard-branch×reason ~60% pinned: the census had no third axis)."""
    found = set(re.findall(r'return (?:None, )?"([a-z][a-z-]*)"[ \t]*$', src, re.M))
    found |= set(re.findall(r'"SKIPPED-UNSAFE", f"\(([a-z][a-z-]*)\)', src))
    return tuple(sorted(found))


def _run_mutant_selftest(base: str, mut: str) -> int:
    """Write a mutated copy of the gate, run --selftest on it, count FAILs."""
    import os
    import subprocess
    import tempfile
    with tempfile.NamedTemporaryFile("w", suffix=".py", delete=False) as f:
        f.write(mut)
        path = f.name
    r = subprocess.run([sys.executable, path, "--selftest"], capture_output=True, text=True)
    os.unlink(path)
    return sum(1 for l in r.stdout.splitlines() if l.startswith("selftest FAIL"))


def _census() -> int:
    """Pin census (durable form of the ㉜ discipline): delete each live unit
    from COUNTING, run --selftest, count drift lines. Every unit must have
    >=1 pin flip; exit 1 otherwise. Rerunnable: the census claim in the
    runbook points HERE, not at a pasted blob in /tmp.
    v2.0 adds the second axis (collision-file §二.3): the --rerun execution
    whitelist is a NEW closed set, so each of its members gets the same
    delete-and-count treatment (anchor-uniqueness asserted first, P-Ⅱ —
    a token appearing twice would mutate the wrong site and measure nothing).
    v2.1 N4: both axes diff the derived list against the frozen expectation
    FIRST (shrink/extend named and red — an axis can no longer iterate
    itself smaller into a green), every line is labelled AXIS1(units)/
    AXIS2(whitelist) so an anchor failure points at its own axis, and unit
    anchors are located by a (|u) boundary regex instead of the
    "(处|"/"|PASS)"/"|u|" position special cases (whose first/last handling
    produced the mis-attribution outer-r1 m21/N4-A recorded).
    v2.3 AXIS3 (outer-r2 singularity prescription): the guard's refusal
    REASONS are the third closed set — derived from this file's own source
    by _derive_reasons, diffed against _EXPECTED_REASONS both ways, then
    mutated member-by-member (every emission site of a label renamed); a
    rename MUST flip >=1 want_reason holder or the label prints UNPINNED
    and names AXIS3. This extends "census is DURABLE" from the two closed
    sets to the third, at zero new methodology — and its limits are stated
    in the header (OPEN faces v2.3 (2): labels, not branch semantics).
    v2.5 AXIS4(grammar) (outer-v24 F2): the fourth closed set — the option
    tables the v2.4 round shipped WITHOUT an axis, which is exactly why 15
    of their 22 entries could be deleted or typo-fixed in silence. Derived
    from the live dicts, diffed against _EXPECTED_GRAMMAR both ways, then
    blanked entry-by-entry on its own table line; every entry must flip a
    positive carrier. Methodology unchanged, only the set gets named."""
    base = Path(__file__).read_text(encoding="utf-8")
    rc = 0
    unpinned = []
    missing = [u for u in _EXPECTED_UNITS if u not in UNITS]
    extra = [u for u in UNITS if u not in _EXPECTED_UNITS]
    axis1_countfail = bool(missing or extra)
    if axis1_countfail:
        rc = 1
        print(f"census: AXIS1(units) FAIL derived {len(UNITS)} != pinned {len(_EXPECTED_UNITS)}; "
              f"missing={missing!r} unexpected={extra!r} — set edits require re-pinning, "
              f"a removed member must never shrink the axis into green")
    # mutation site = the DERIVED alternation inside this file's source (the
    # exact `(u|…|u)` text COUNTING was built from). Anchoring the search at
    # that region, not at the file's first occurrence, keeps prose that
    # quotes an anchor token (the v2.0 header nearly did) from poisoning the
    # census: a wrong-site mutation would flip 0 and false-red; a derived-
    # site mutation measures the real branch.
    alt = "(" + "|".join(UNITS) + ")"
    a0 = base.find(alt)
    if a0 < 0:
        rc = 1
        print("census: AXIS1(units) FAIL — derived alternation not found in source "
              "(pattern site poisoned or reformatted; anchors cannot be located)")
        return rc
    for u in UNITS:
        m = re.search(rf"(?P<lead>[(|]){re.escape(u)}[|)]", base[a0:])
        if not m:
            print(f"census: AXIS1(units) anchor not found for unit {u!r}")
            unpinned.append(u)
            continue
        repl = ")" if m.group(0).endswith(")") else m.group("lead")
        s, e = a0 + m.start(), a0 + m.end()
        mut = base[:s] + repl + base[e:]
        flips = _run_mutant_selftest(base, mut)
        print(f"census: AXIS1(units) delete {u!r} -> {flips} pin-case flip(s)")
        if flips == 0:
            unpinned.append(u)
    if unpinned:
        rc = 1
        print(f"census: AXIS1(units) UNPINNED: {unpinned!r}")
    elif axis1_countfail:
        print("census: AXIS1(units) per-member flip scan incomplete — count FAIL above is the verdict")
    else:
        print(f"census: AXIS1(units) ALL {len(_EXPECTED_UNITS)} live units pinned")
    uncmd = []
    names = [f"{t} {m}" for t, m in RERUN_READONLY]
    missing2 = [x for x in _EXPECTED_CMD if x not in names]
    extra2 = [x for x in names if x not in _EXPECTED_CMD]
    if missing2 or extra2:
        rc = 1
        print(f"census: AXIS2(whitelist) FAIL derived {len(names)} != pinned {len(_EXPECTED_CMD)}; "
              f"missing={missing2!r} unexpected={extra2!r} — member count is a hard pin (v2.6: 2 live + 3 retired) "
              f"(N4-B: deleting a member may not report a shortened axis green)")
    for t, m in RERUN_READONLY:
        token = f'    ("{t}", "{m}"),\n'
        n = base.count(token)
        if n != 1:
            print(f"census: AXIS2(whitelist) member {t} {m!r} anchor count {n} (want 1)")
            uncmd.append(f"{t} {m}")
            continue
        mut = base.replace(token, "", 1)
        flips = _run_mutant_selftest(base, mut)
        print(f"census: AXIS2(whitelist) delete ({t}, {m!r}) -> {flips} pin-case flip(s)")
        if flips == 0:
            uncmd.append(f"{t} {m}")
    if uncmd:
        rc = 1
        print(f"census: AXIS2(whitelist) UNPINNED CMDS: {uncmd!r}")
    else:
        print(f"census: AXIS2(whitelist) ALL {len(_EXPECTED_CMD)} execution-whitelist members pinned")
    # ---- AXIS3 (v2.3, outer-r2 singularity prescription): refusal-reason ×
    # pin. Mutation mode = RENAME the emitted label at every anchor site
    # (deleting a branch instead would change behaviour beyond the label and
    # is caught transitively: the label a fall-through branch would then
    # emit differs from some want_reason — see RR04d's path-disguise note).
    # A rename flips >=1 pin BECAUSE RERUN_CASES assert want_reason — the
    # field that closed the "reason-string hostage-less" root cause.
    # Missing/unexpected members name AXIS3 (never a neighbouring axis). ----
    reasons = _derive_reasons(base)
    missing3 = [x for x in _EXPECTED_REASONS if x not in reasons]
    extra3 = [x for x in reasons if x not in _EXPECTED_REASONS]
    axis3_countfail = bool(missing3 or extra3)
    if axis3_countfail:
        rc = 1
        print(f"census: AXIS3(reasons) FAIL derived {len(reasons)} != pinned {len(_EXPECTED_REASONS)}; "
              f"missing={missing3!r} unexpected={extra3!r} — a refusal site that disappears or a new "
              f"unlabelled refuse may not shrink the axis into green (N4-B lesson, third axis)")
    unreason = []
    for r in reasons:
        mut = base
        n = 0
        for p in (f'return None, "{r}"', f'return "{r}"'):
            n += base.count(p)
            mut = mut.replace(p, p[:-1] + "-AXIS3MUT\"")
        if r == "exec-unavailable":
            n += base.count('f"(exec-unavailable) ')
            mut = mut.replace('f"(exec-unavailable) ', 'f"(exec-unavailable-AXIS3MUT) ')
        if n == 0:
            print(f"census: AXIS3(reasons) anchor not found for reason {r!r}")
            unreason.append(r)
            continue
        flips = _run_mutant_selftest(base, mut)
        print(f"census: AXIS3(reasons) rename-label {r!r} ({n} site(s)) -> {flips} pin-case flip(s)")
        if flips == 0:
            unreason.append(r)
    if unreason:
        rc = 1
        print(f"census: AXIS3(reasons) UNPINNED REASONS: {unreason!r} — every refusal label "
              f"must have a want_reason holder")
    elif axis3_countfail:
        print("census: AXIS3(reasons) per-member flip scan incomplete — count FAIL above is the verdict")
    else:
        print(f"census: AXIS3(reasons) ALL {len(_EXPECTED_REASONS)} refusal reasons pinned via want_reason")
    # ---- AXIS4 (v2.5 F2, outer-v24 fix item 5): the grammar TABLES were the
    # last closed set without an axis — and the coverage battery proved what
    # that meant: 15 of 22 entries had a real ACCEPT-face effect and zero
    # pins, so deleting or typo-ing one shipped green while every pasted
    # `wc --lines` / `git ls-tree -r` doc silently flipped to refused (the
    # mirror image of the M11 anti-cheat: refuse-everything is caught,
    # accept-nothing is not). Structure = AXIS1-3 again: derive from the LIVE
    # dicts, diff against the frozen register BOTH ways with the member count
    # pinned, then mutate each entry — anchor = the entry's quoted token ON
    # ITS OWN TABLE LINE (the same token may live on another line: "-l" is
    # both a wc and an ls-tree entry, so a whole-file replace would mutate
    # the wrong site and measure nothing; the per-line anchor is asserted
    # unique the way AXIS2 asserts its token). Mutation blanks the token to
    # the empty string, which behaves as REMOVED (a token can never equal ""
    # because the loop only inspects '-'-leading ones, and fullmatch("",…)
    # never matches) — so a blanked entry is a deleted entry, and every
    # entry must flip at least one of the RR53–RR68 positive carriers. ----
    gram = _derive_grammar()
    missing4 = [x for x in _EXPECTED_GRAMMAR if x not in gram]
    extra4 = [x for x in gram if x not in _EXPECTED_GRAMMAR]
    axis4_countfail = bool(missing4 or extra4)
    if axis4_countfail:
        rc = 1
        print(f"census: AXIS4(grammar) FAIL derived {len(gram)} != pinned {len(_EXPECTED_GRAMMAR)}; "
              f"missing={missing4!r} unexpected={extra4!r} — the option grammar is a closed set "
              f"since v2.5: an entry added or removed must re-pin its positive carrier (N4-B lesson, "
              f"fourth axis)")
    ungram = []
    for entry in gram:
        key, kind, tok = entry.split("/", 2)
        lines = base.split("\n")
        hits = [i for i, l in enumerate(lines) if l.strip().startswith(f'"{key}":')]
        if len(hits) != 1:
            print(f"census: AXIS4(grammar) table-line anchor for {key!r} count {len(hits)} (want 1)")
            ungram.append(entry)
            continue
        i = hits[0]
        qtok = f'"{tok}"'
        if lines[i].count(qtok) != 1:
            print(f"census: AXIS4(grammar) token anchor for {entry!r} count "
                  f"{lines[i].count(qtok)} (want 1)")
            ungram.append(entry)
            continue
        mut = "\n".join(lines[:i] + [lines[i].replace(qtok, '""', 1)] + lines[i + 1:])
        flips = _run_mutant_selftest(base, mut)
        print(f"census: AXIS4(grammar) blank {entry!r} -> {flips} pin-case flip(s)")
        if flips == 0:
            ungram.append(entry)
    if ungram:
        rc = 1
        print(f"census: AXIS4(grammar) UNPINNED ENTRIES: {ungram!r} — every accepted option "
              f"shape needs a positive carrier that dies when the entry dies")
    elif axis4_countfail:
        print("census: AXIS4(grammar) per-member flip scan incomplete — count FAIL above is the verdict")
    else:
        print(f"census: AXIS4(grammar) ALL {len(_EXPECTED_GRAMMAR)} grammar entries pinned "
              f"via positive carriers")
    return rc


def _mainline_selftest() -> int:
    """v2.1 CLI-face pins — the PRINT paths themselves, not just verdicts.
    M1: the zero-hit clean line must be byte-equal to the v1.8 output
    ("claim gate: clean (N file(s))", rc=0; --check stays silent), because
    the clean line is the only falsifiable evidence for a 0-hit outbound
    claim (outer-r1 M1 consumer ②). N5: the --rerun summary must expose
    executed=/SKIPPED= so a zero-execution pass is visible without any rc
    change. rc semantics themselves are FROZEN (0/1 exactly as v1.8/v2.0)."""
    import os
    import shutil
    import subprocess
    import tempfile
    bad = 0
    d = tempfile.mkdtemp()
    try:
        clean = os.path.join(d, "clean.md")
        open(clean, "w", encoding="utf-8").write("# t\n\nno counts here at all.\n")
        r = subprocess.run([sys.executable, "-I", __file__, clean], capture_output=True,
                           text=True, stdin=subprocess.DEVNULL, timeout=60)
        ok = r.stdout == "claim gate: clean (1 file(s))\n" and r.returncode == 0
        bad += 0 if ok else 1
        print(f"selftest {'ok ' if ok else 'FAIL'} want_cli='clean line byte-equal v1.8, rc=0' "
              f":: M1-clean-line :: {r.stdout[:60]!r} rc={r.returncode}")
        q = subprocess.run([sys.executable, "-I", __file__, "--check", clean], capture_output=True,
                           text=True, stdin=subprocess.DEVNULL, timeout=60)
        ok = q.stdout == "" and q.returncode == 0
        bad += 0 if ok else 1
        print(f"selftest {'ok ' if ok else 'FAIL'} want_cli='--check silent, rc=0 (v1.8 parity)' "
              f":: M1-check-quiet :: {q.stdout[:40]!r} rc={q.returncode}")
        vac = os.path.join(d, "vac.md")
        open(vac, "w", encoding="utf-8").write("计数：共十行。\n\n```\nwc -l /etc/hosts → 33\n```\n")
        v = subprocess.run([sys.executable, "-I", __file__, "--rerun", vac], capture_output=True,
                           text=True, stdin=subprocess.DEVNULL, timeout=60)
        ok = (v.returncode == 0 and "executed=0/SKIPPED=1" in v.stdout
              and "SKIPPED-UNSAFE" in v.stdout and "(outside-docdir)" in v.stdout
              and "value-drift" not in v.stdout and "cmd-failed" not in v.stdout)
              # the fence refuses WITHOUT executing: the cmd= echo of the
              # document's own text is allowed, an executed-output verdict
              # (value-drift/cmd-failed) is not — doc3c's v2.0 failure mode
        bad += 0 if ok else 1
        print(f"selftest {'ok ' if ok else 'FAIL'} want_cli='rc=0 ∧ executed=0/SKIPPED=1 visible ∧ no "
              f"outside operand executed' :: N5-vacuous-visible :: {v.stdout[:60]!r} rc={v.returncode}")
    except Exception as e:
        bad += 1
        print(f"selftest FAIL want_cli=… got=EXC :: M1/N5-harness :: {e!r}")
    finally:
        shutil.rmtree(d, ignore_errors=True)
    return bad


def _env_selftest() -> int:
    """v2.5 (F1/F5/F6/F7) — the faces RERUN_CASES cannot express, because
    they are STRUCTURAL: an invariant over two data sources, and the exact
    argv the executor builds. Each check prints its own selftest line, so a
    census mutation that breaks one of them is counted like any pin flip.

    Why these four are not prose: the v2.4 round SHIPPED a dead argv edit
    (argv[2] is always '-c' after a 5-element insert, so the ext-diff flag
    never appeared — M2: deleting it flipped 0 pins) and shipped a whitelist
    layer with no table check (F1: add a member, forget the table ⇒ 0 FAIL,
    unconstrained execution). A sentence about either would have been the
    next 81 §九 fidelity finding; an assert cannot drift that way."""
    import os
    bad = 0

    def line(ok, want, got, cid, detail):
        nonlocal bad
        bad += 0 if ok else 1
        print(f"selftest {'ok ' if ok else 'FAIL'} want_env={want!r} got={got!r} "
              f":: {cid} :: {detail}")

    # ENV-grammar-invariant (F1): every execution-whitelist member must have
    # a grammar table — the fail-closed refuse makes an unmemored member SAFE
    # but this pin makes it LOUD, so nobody can extend the whitelist and
    # silently run the new tool unconstrained until some doc hits the refuse.
    noftable = [f"{t} {m}" for t, m in RERUN_READONLY
                if not ((m in _GIT_GRAMMAR) if t == "git" else (t in _GRAMMAR))]
    line(not noftable, "every RERUN_READONLY member has a grammar table",
         "ALL-HAVE" if not noftable else "MISSING", "ENV-grammar-invariant",
         f"{len(RERUN_READONLY)} members checked against _GRAMMAR/_GIT_GRAMMAR, "
         f"offenders={noftable!r}")

    # ENV-harden-argv-log (F5 + F7): the prefix, the showSignature pair and
    # v2.6: the two ENV-harden-argv pins are RETIRED with _git_harden
    # (their subject no longer exists; RR74/RR74b hold the retirement).

    # ENV-path-clean (F6 unit half): empty and relative components dropped,
    # order preserved, absolute kept. (RR71 is the EXEC half.)
    probe = os.pathsep.join([".", "/usr/bin", "", "rel/dir", "/bin"])
    clean = _path_clean(probe)
    line(clean == ["/usr/bin", "/bin"] and _path_clean("") == [],
         "['/usr/bin', '/bin'] and []", str(clean), "ENV-path-clean",
         "cwd-search components cannot reach the executor")
    return bad


def _probes_selftest() -> int:
    """v2.5 F8 — the marker/leak sentinels are no longer /tmp-only evidence.

    review_guard_probes.py is the mainline's own verbatim consumer file (邢's
    PR#22 review code, 逐字 taken from review 5874479780); the outer round
    showed it HAS teeth — at v2.3 the external-diff case really wrote the
    marker — yet neither --selftest nor --census ever called it, so the whole
    env-sanitisation layer was being vouched for by a script in /tmp
    (conservation-law violation: a permanent claim resting on an ephemeral
    probe). This tier runs it and asserts the two JSON lines say
    accepted=False with no marker and no outside read.

    A copy that is NOT in the repo layout (census mutant copies live in /tmp,
    so they never see the file) prints a VISIBLE ABSENT line — skipped loudly,
    never silently — so the tier's absence is itself auditable in the output
    rather than a missing line someone must notice. The in-tree run is the
    consumer that matters (OPEN faces v2.5 (5))."""
    import json
    import subprocess
    root = Path(__file__).resolve().parents[2]
    probes = root / "review_guard_probes.py"
    if not probes.exists():
        print(f"selftest ok  want_probe='(absent file is LOUD, not silent)' "
              f"got=ABSENT :: PRB-file-absent :: {probes} not in this layout — tier skipped")
        return 0
    want = {"indirect-file-escape": ("read_outside_synthetic_file", "accepted"),
            "external-diff-execution": ("synthetic_marker_written", "accepted")}
    bad = 0
    try:
        r = subprocess.run([sys.executable, "-I", str(probes)], capture_output=True,
                           text=True, timeout=120)
        rows = {}
        for l in r.stdout.splitlines():
            try:
                j = json.loads(l)
            except ValueError:
                continue
            if isinstance(j, dict) and "case" in j:
                rows[j["case"]] = j
        for case, (leak_field, acc_field) in want.items():
            j = rows.get(case)
            ok = (j is not None and r.returncode == 0
                  and j.get(acc_field) is False and j.get(leak_field) is False)
            bad += 0 if ok else 1
            print(f"selftest {'ok ' if ok else 'FAIL'} want_probe='accepted=False ∧ "
                  f"{leak_field}=False' got={j!r} :: PRB-{case} :: rc={r.returncode}")
    except Exception as e:
        print(f"selftest FAIL want_probe='…' got=EXC :: PRB-harness :: {e!r}")
        bad += 1
    return bad


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
    n_rerun_bad = _rerun_selftest()
    n_env_bad = _env_selftest()
    n_prb_bad = _probes_selftest()
    n_cli_bad = _mainline_selftest()
    if bad or n_rerun_bad or n_env_bad or n_prb_bad or n_cli_bad:
        print(f"selftest: {len(bad)} boundary + {n_rerun_bad} rerun + {n_env_bad} env + "
              f"{n_prb_bad} probes + {n_cli_bad} cli case(s) drifted from the boundary spec")
        return 1
    print("selftest: all boundary cases hold")
    return 0


def main() -> int:
    if "--selftest" in sys.argv[1:]:
        return _selftest()
    if "--census" in sys.argv[1:]:
        return _census()
    rerun = "--rerun" in sys.argv[1:]
    args = [a for a in sys.argv[1:] if a not in ("--check", "--rerun")]
    quiet = "--check" in sys.argv[1:]
    hits = []
    for a in args:
        hits += violations(a)
    if hits:
        if not quiet:
            print(f"claim gate: {len(hits)} counting claim(s) lack nearby command evidence:")
        for h in hits:
            print(h)
    elif not quiet:
        # v2.1 M1: v1.8's clean line, byte-equal — a 0-hit verdict without
        # output is an unfalsifiable claim (the 79号 outbound case).
        print(f"claim gate: clean ({len(args)} file(s))")
    if not rerun:
        return 1 if hits else 0
    # tier 1 (v2.0): default verdicts printed above are byte-identical to the
    # default path; what follows is the command-vs-paste fidelity pass.
    tally = {"OK": 0, "RERUN-MISMATCH": 0, "SKIPPED-UNSAFE": 0, "SKIPPED-TIMEOUT": 0}
    for a in args:
        for ln, claim, ver, det in rerun_verdicts(a):
            tally[ver] = tally.get(ver, 0) + 1
            print(f"rerun: {a}:{ln}: {ver} claim {claim!r} {det}")
    mis = tally["RERUN-MISMATCH"]
    if not quiet or rerun:
        # R2 m5: --check suppressed the N5 summary line too, so "vacuous
        # pass" detection (executed=0 machine-visible) silently died under
        # the --check --rerun combination. The per-file verdict lines above
        # still print in that combination (they are the rerun evidence);
        # the summary must as well — quiet gates PROSE, not the contract.
        n = sum(tally.values())
        executed = tally["OK"] + mis
        skipped = tally["SKIPPED-UNSAFE"] + tally["SKIPPED-TIMEOUT"]
        # v2.1 N5: rc semantics FROZEN (see header) — the vacuity is made
        # visible instead: "OK+MISMATCH==0" alone never proved anything ran;
        # executed=0 alongside evidence lines in scope is the machine-
        # detectable false-pass shape (n5-skip, MR !2 consumer contract).
        print(f"rerun: {n} claim line(s) carried an evidence command in scope: "
              f"{tally['OK']} OK, {mis} RERUN-MISMATCH, {tally['SKIPPED-UNSAFE']} SKIPPED-UNSAFE, "
              f"{tally['SKIPPED-TIMEOUT']} SKIPPED-TIMEOUT | executed={executed}/SKIPPED={skipped}")
    return 1 if (hits or mis) else 0


if __name__ == "__main__":
    raise SystemExit(main())
