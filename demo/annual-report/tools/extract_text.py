# Extract each page's text of the annual report to text/pNNN.txt (1-based PDF page index).
import sys, pathlib, pdfplumber
src, out = pathlib.Path(sys.argv[1]), pathlib.Path(sys.argv[2])
with pdfplumber.open(src) as pdf:
    print("pages", len(pdf.pages))
    for i, page in enumerate(pdf.pages, 1):
        (out / f"p{i:03d}.txt").write_text(page.extract_text() or "", encoding="utf-8")
