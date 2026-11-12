# Build the W01 reference answers from the report's own lines: every value is
# parsed from the quoted line on its page (not typed in), checked against the
# expected value, and ratios that can be recomputed from other report lines are.
import json, re, sys, pathlib

text_dir, out = pathlib.Path(sys.argv[1]), pathlib.Path(sys.argv[2])
page = lambda n: (text_dir / f"p{n:03d}.txt").read_text(encoding="utf-8")

def line(pdf_page, startswith):
    hits = [l for l in page(pdf_page).splitlines() if l.startswith(startswith)]
    if len(hits) != 1:
        sys.exit(f"p{pdf_page}: {len(hits)} lines start with {startswith!r}")
    return hits[0]

num = lambda s: float(s.replace(",", ""))

def row(id, name, pdf_page, starts, unit, basis, kind, expect, section):
    l = line(pdf_page, starts)
    cells = l[len(starts):].split()
    v25, v24 = num(cells[0]), num(cells[1])
    if kind == "pct":
        change = {"kind": "pct", "value": num(cells[2])}
    else:
        m = re.match(r"(上升|下降)([0-9.]+)个百分点", cells[2])
        change = {"kind": "pp", "value": (1 if m.group(1) == "上升" else -1) * float(m.group(2))}
    got = (v25, v24, change["value"])
    if got != expect:
        sys.exit(f"{id}: parsed {got}, expected {expect}")
    return {
        "id": id, "name": name, "scope": "本集团", "unit": unit, "basis": basis,
        "values": {"2025": v25, "2024": v24}, "change": change,
        "source": {"pdf_page": pdf_page, "printed_page": pdf_page - 1, "section": section, "line": l},
    }

S21, S22 = "2.1 本集团主要会计数据和财务指标", "2.2 本集团补充财务比率"
ind = [
    row("revenue", "营业收入", 14, "营业收入 ", "人民币百万元", "全年", "pct", (337532, 337488, 0.01), S21),
    row("net_profit_attributable", "归属于本行股东的净利润", 14, "归属于本行股东的净利润 ", "人民币百万元", "全年", "pct", (150181, 148391, 1.21), S21),
    row("net_interest_margin", "净利息收益率", 16, "净利息收益率(2) ", "%", "全年", "pp", (1.87, 1.98, -0.11), S22),
    row("cost_to_income", "成本收入比", 16, "成本收入比(3) ", "%", "全年", "pp", (31.98, 31.89, 0.09), S22),
    row("npl_balance", "不良贷款余额", 15, "不良贷款 ", "人民币百万元", "年末", "pct", (68206, 65610, 3.96), S21),
    row("npl_ratio", "不良贷款率", 16, "不良贷款率 ", "%", "年末", "pp", (0.94, 0.95, -0.01), S22),
    row("provision_coverage", "拨备覆盖率", 16, "拨备覆盖率(1) ", "%", "年末", "pp", (391.79, 411.98, -20.19), S22),
    row("cet1_ratio_advanced", "核心一级资本充足率（高级法）", 16, "核心一级资本充足率 ", "%", "年末", "pp", (14.16, 14.86, -0.70), S22),
]
ind[4]["name_in_report"] = "不良贷款"  # the summary table calls the balance just 不良贷款
ind[7]["trap"] = {"note": "同页注释另列权重法核心一级资本充足率", "line": line(16, "注： 截至报告期末，本集团权重法下")}

# Inputs that let the ratios and growth rates be recomputed.
facts = {
    "loan_loss_allowance": row("loan_loss_allowance", "贷款损失准备", 15, "贷款损失准备(3) ", "人民币百万元", "年末", "pct", (267222, 270301, -1.14), S21),
    "gross_loans": row("gross_loans", "贷款和垫款总额", 15, "贷款和垫款总额(2) ", "人民币百万元", "年末", "pct", (7258058, 6888315, 5.37), S21),
}
by = {r["id"]: r for r in ind}
def check(id, formula, value):
    want = by[id]["values"]["2025"]
    if round(value, 2) != want:
        sys.exit(f"recompute {id}: {value:.4f} != {want}")
    by[id]["recompute"] = {"formula": formula, "value": round(value, 2)}
L, N, G = facts["loan_loss_allowance"]["values"]["2025"], by["npl_balance"]["values"]["2025"], facts["gross_loans"]["values"]["2025"]
check("provision_coverage", "贷款损失准备 / 不良贷款余额 × 100", L / N * 100)
check("npl_ratio", "不良贷款余额 / 贷款和垫款总额 × 100", N / G * 100)
for id in ("revenue", "net_profit_attributable", "npl_balance"):
    v = by[id]["values"]
    g = (v["2025"] - v["2024"]) / v["2024"] * 100
    if round(g, 2) != by[id]["change"]["value"]:
        sys.exit(f"growth {id}: {g:.4f}")
    by[id]["recompute"] = {"formula": "(2025 − 2024) / 2024 × 100", "value": round(g, 2)}
for r in ind:
    if r["change"]["kind"] == "pp":
        d = round(r["values"]["2025"] - r["values"]["2024"], 2)
        if d != r["change"]["value"]:
            sys.exit(f"pp {r['id']}: {d}")

for r in ind:
    r["verification"] = {"status": "pending", "verifier": None, "verified_at": None}
doc = {
    "snapshot": "cmb-2025-annual-report-a",
    "note": "程序抽取并复算，尚未人工核对；verification.status 为 pending 时不得作为参考答案使用。",
    "indicators": ind,
    "facts": list(facts.values()),
}
out.write_text(json.dumps(doc, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
print("ok", len(ind), "indicators,", len(facts), "facts")
