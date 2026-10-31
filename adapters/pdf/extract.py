"""Local PDF adapter. JSON request on stdin; versioned JSONL events on stdout."""

import hashlib
import json
import logging
import platform
import sys

import pypdf

VERSION = 1


def info():
    return {"adapter_version": "1", "library": "pypdf", "library_version": pypdf.__version__,
            "python_version": platform.python_version()}


def extract(request, emit):
    required = {"schema_version", "source_id", "source_sha256", "path", "mode"}
    if set(request) != required or request["schema_version"] != VERSION:
        raise ValueError("invalid request contract")
    if request["mode"] not in ("plain", "layout"):
        raise ValueError("invalid extraction mode")
    if pypdf.__version__ != "6.20.1":
        raise ValueError("pypdf version must be 6.20.1")
    with open(request["path"], "rb") as stream:
        if hashlib.file_digest(stream, "sha256").hexdigest() != request["source_sha256"]:
            raise ValueError("source checksum mismatch")

    common = {"schema_version": VERSION, "source_id": request["source_id"],
              "source_sha256": request["source_sha256"]}
    counts = {"units": 0, "diagnostics": 0, "errors": 0, "sections": 0}
    pending = []
    current_page = None

    def event(kind, **fields):
        emit({**common, "type": kind, **fields})

    def diagnostic(code, message, severity="warning", page=None):
        counts["diagnostics"] += 1
        counts["errors"] += severity == "error"
        payload = {"code": code, "message": str(message)[:1024], "severity": severity}
        if page is not None:
            payload["page"] = page
        event("diagnostic", diagnostic=payload)

    class Capture(logging.Handler):
        def emit(self, record):
            # Buffer warnings until document_start or the current page completes.
            if len(pending) < 100:
                pending.append(("pdf_warning", record.getMessage(), current_page))
            elif len(pending) == 100:
                pending.append(("warnings_truncated", "more than 100 warnings in this operation", current_page))

    def flush():
        for code, message, page in pending:
            diagnostic(code, message, page=page)
        pending.clear()

    logger = logging.getLogger("pypdf")
    capture = Capture()
    logger.addHandler(capture)
    try:
        reader = pypdf.PdfReader(request["path"])
        if reader.is_encrypted:
            raise ValueError("encrypted PDF: explicit credential support is not implemented")
        pages = len(reader.pages)
        event("document_start", page_count=pages, engine=info(), mode=request["mode"])
        flush()
        if pages == 0:
            diagnostic("no_pages", "PDF has no pages", "error")

        def outlines(items, depth=0):
            for item in items:
                if isinstance(item, list):
                    outlines(item, depth + 1)
                    continue
                page = reader.get_destination_page_number(item)
                title = item.title
                if page is None or not 0 <= page < pages or not isinstance(title, str) or not title.strip():
                    diagnostic("invalid_outline", "outline destination or title unavailable")
                    continue
                title.encode("utf-8", errors="strict")
                event("section", section={"title": title, "page": page + 1, "depth": depth})
                counts["sections"] += 1

        try:
            outline = reader.outline
            if outline:
                outlines(outline)
            else:
                diagnostic("structure_unavailable", "No reliable outline supplied by the PDF")
        except Exception as exc:
            diagnostic("outline_error", type(exc).__name__ + ": " + str(exc))
        flush()

        nonempty_pages = 0
        for number, page in enumerate(reader.pages, 1):
            current_page = number
            try:
                options = {"extraction_mode": request["mode"]}
                if request["mode"] == "layout":
                    options["layout_mode_strip_rotated"] = False
                text = (page.extract_text(**options) or "") if page.get_contents() is not None else ""
                text.encode("utf-8", errors="strict")
            except Exception as exc:
                if request["mode"] == "layout":
                    try:
                        text = page.extract_text(extraction_mode="plain") or ""
                        text.encode("utf-8", errors="strict")
                        diagnostic("layout_fallback", "Layout extraction failed; plain text used for this page: " + type(exc).__name__, page=number)
                    except Exception as fallback_error:
                        text = ""
                        diagnostic("page_error", type(fallback_error).__name__ + ": " + str(fallback_error), "error", number)
                else:
                    text = ""
                    diagnostic("page_error", type(exc).__name__ + ": " + str(exc), "error", number)
            flush()
            if not text.strip():
                diagnostic("empty_page", "No text extracted; blank/image/OCR status is unknown", page=number)
            else:
                nonempty_pages += 1
            event("unit", unit={"id": f"page-{number:06d}", "order": number, "text": text,
                               "locator": {"kind": "pdf_page", "page": number,
                                           "representation": "unit_text_utf8", "start_byte": 0,
                                           "end_byte": len(text.encode("utf-8"))}})
            counts["units"] += 1
        current_page = None
        flush()
        if pages and not nonempty_pages:
            diagnostic("no_text_extracted", "No text was extracted from any page; document content is unverified", "error")
        event("document_end", end={**counts, "status": "partial" if counts["errors"] else "complete"})
    finally:
        logger.removeHandler(capture)


def main():
    if sys.argv[1:] == ["--info"]:
        print(json.dumps(info(), ensure_ascii=False, allow_nan=False))
        return 0
    if sys.argv[1:]:
        raise ValueError("unexpected arguments")
    request = json.load(sys.stdin)
    extract(request, lambda record: print(json.dumps(record, ensure_ascii=False, allow_nan=False), flush=True))
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception as error:
        # No stdout pollution, extracted text or absolute paths in operational logs.
        print("PDF adapter failed: " + type(error).__name__, file=sys.stderr)
        sys.exit(1)
