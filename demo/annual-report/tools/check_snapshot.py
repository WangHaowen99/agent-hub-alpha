# Check that a local copy of the report is the snapshot named in source.json.
# Usage: python3 -I tools/check_snapshot.py <report.pdf>
import hashlib, json, pathlib, sys

here = pathlib.Path(__file__).resolve().parent.parent
src = json.loads((here / "source.json").read_text(encoding="utf-8"))
pdf = pathlib.Path(sys.argv[1])
data = pdf.read_bytes()
problems = []
if len(data) != src["bytes"]:
    problems.append(f"size {len(data)} != {src['bytes']}")
digest = hashlib.sha256(data).hexdigest()
if digest != src["sha256"]:
    problems.append(f"sha256 {digest} != {src['sha256']}")
try:
    import pypdf
    pages = len(pypdf.PdfReader(str(pdf)).pages)
    if pages != src["pages"]:
        problems.append(f"pages {pages} != {src['pages']}")
except ImportError:
    pass
if problems:
    sys.exit(f"{pdf} is not snapshot {src['id']}: " + "; ".join(problems))
print(f"ok {src['id']} {digest}")
