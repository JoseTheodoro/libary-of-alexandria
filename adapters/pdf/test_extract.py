"""Synthetic fixtures only; no corpus files required."""
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

from pypdf import PdfWriter
from pypdf.generic import DictionaryObject, NameObject, DecodedStreamObject

import extract


def write_pdf(path, texts=None, encrypted=False):
    writer = PdfWriter()
    for text in texts if texts is not None else ["Fonte sintética — ação / evidence\nfunc ReadFile() string {\n    return value\n}", ""]:
        page = writer.add_blank_page(width=612, height=792)
        if not text:
            continue
        font = DictionaryObject({NameObject("/Type"): NameObject("/Font"), NameObject("/Subtype"): NameObject("/Type1"),
                                 NameObject("/BaseFont"): NameObject("/Courier"), NameObject("/Encoding"): NameObject("/WinAnsiEncoding")})
        page[NameObject("/Resources")] = DictionaryObject({NameObject("/Font"): DictionaryObject({NameObject("/F1"): font})})
        commands = [b"BT /F1 12 Tf 16 TL 40 740 Td"]
        for line in text.splitlines():
            commands.append(b"<" + line.encode("cp1252").hex().encode() + b"> Tj T*")
        commands.append(b"ET")
        content = DecodedStreamObject()
        content.set_data(b"\n".join(commands))
        page[NameObject("/Contents")] = writer._add_object(content)
    writer.add_outline_item("Synthetic section", 0)
    if encrypted:
        writer.encrypt("test-only-password")
    writer.write(str(path))


class ExtractTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.path = Path(self.temp.name) / "synthetic.pdf"
        write_pdf(self.path)

    def request(self, **changes):
        result = {"schema_version": 1, "source_id": "synthetic", "source_sha256": hashlib.sha256(self.path.read_bytes()).hexdigest(),
                  "path": str(self.path), "mode": "layout"}
        return {**result, **changes}

    def test_text_structure_and_empty_page(self):
        for mode in ["plain", "layout"]:
            records = []
            extract.extract(self.request(mode=mode), records.append)
            self.assertEqual(records[0]["type"], "document_start")
            units = [r["unit"] for r in records if r["type"] == "unit"]
            self.assertEqual(len(units), 2)
            self.assertIn("ação", units[0]["text"])
            self.assertIn("ReadFile", units[0]["text"])
            self.assertIn("    return value", units[0]["text"])
            self.assertEqual(units[0]["locator"]["end_byte"], len(units[0]["text"].encode("utf-8")))
            self.assertTrue(any(r["type"] == "section" for r in records))
            self.assertTrue(any(r.get("diagnostic", {}).get("code") == "empty_page" for r in records))
            self.assertEqual(records[-1]["end"]["status"], "complete")

    def test_repeatable(self):
        a, b = [], []
        extract.extract(self.request(), a.append)
        extract.extract(self.request(), b.append)
        self.assertEqual(a, b)

    def test_request_and_hash_errors(self):
        for changes in [{"schema_version": 2}, {"source_sha256": "0" * 64}, {"mode": "unknown"}, {"unknown": True}]:
            with self.assertRaises(ValueError):
                extract.extract(self.request(**changes), lambda _: None)

    def test_layout_page_failure_falls_back_and_records_diagnostic(self):
        from pypdf._page import PageObject
        original = PageObject.extract_text
        def failing_layout(page, *args, **kwargs):
            if kwargs.get("extraction_mode") == "layout":
                raise ValueError("synthetic layout failure")
            return original(page, *args, **kwargs)
        records = []
        with patch.object(PageObject, "extract_text", failing_layout):
            extract.extract(self.request(), records.append)
        self.assertTrue(any(r.get("diagnostic", {}).get("code") == "layout_fallback" for r in records))
        self.assertIn("ReadFile", next(r["unit"]["text"] for r in records if r["type"] == "unit"))
        self.assertEqual(records[-1]["end"]["status"], "complete")

    def test_encrypted_pdf_is_explicit_failure(self):
        write_pdf(self.path, encrypted=True)
        with self.assertRaisesRegex(ValueError, "encrypted"):
            extract.extract(self.request(), lambda _: None)

    def test_document_with_no_extracted_text_is_partial(self):
        write_pdf(self.path, texts=["", ""])
        records = []
        extract.extract(self.request(), records.append)
        self.assertEqual(records[-1]["end"]["status"], "partial")
        self.assertTrue(any(r.get("diagnostic", {}).get("code") == "no_text_extracted" for r in records))

    def test_cli_protocol_and_invalid_pdf(self):
        command = [sys.executable, str(Path(extract.__file__))]
        ok = subprocess.run(command, input=json.dumps(self.request()), capture_output=True, text=True)
        self.assertEqual(ok.returncode, 0, ok.stderr)
        self.assertEqual(json.loads(ok.stdout.splitlines()[-1])["type"], "document_end")
        self.path.write_bytes(b"not a PDF")
        bad = subprocess.run(command, input=json.dumps(self.request()), capture_output=True, text=True)
        self.assertEqual(bad.returncode, 1)
        self.assertEqual(bad.stdout, "")
        self.assertNotIn(str(self.path), bad.stderr)


if __name__ == "__main__":
    unittest.main()
