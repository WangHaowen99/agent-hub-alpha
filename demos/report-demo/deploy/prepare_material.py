"""Verify the fixed W01 PDF and extract source pages, never acceptance answers."""
from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path

import pdfplumber


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--pdf", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--manifest", type=Path, default=Path(__file__).resolve().parents[1] / "scenes/annual-report/source.json")
    args = parser.parse_args()
    manifest = json.loads(args.manifest.read_text(encoding="utf-8"))
    raw = args.pdf.read_bytes()
    digest = hashlib.sha256(raw).hexdigest()
    if digest != manifest["sha256"] or len(raw) != manifest["bytes"]:
        raise SystemExit("Fixed PDF digest/size mismatch; refusing different source")
    pages = []
    with pdfplumber.open(args.pdf) as document:
        if len(document.pages) != manifest["pages"]:
            raise SystemExit("PDF page count differs")
        for pdf_page in [14, 15, 16]:
            text = document.pages[pdf_page - 1].extract_text() or ""
            if not text:
                raise SystemExit("Source page extraction is empty")
            pages.append({"pdf_page": pdf_page, "printed_page": str(pdf_page - 1), "text": text,
                          "digest": "sha256:" + hashlib.sha256(text.encode("utf-8")).hexdigest()})
    projection = json.dumps(pages, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8")
    result = {"id": manifest["id"], "title": manifest["title"], "digest": "sha256:" + digest,
              "projection_digest": "sha256:" + hashlib.sha256(projection).hexdigest(),
              "pages": pages, "verification": "source_digest_verified; independent_reference_verification_not_asserted"}
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"output": str(args.output), "pages": len(pages), "source_digest_verified": True}))


if __name__ == "__main__":
    main()
