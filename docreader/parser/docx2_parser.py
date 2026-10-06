import logging

from docreader.models.document import Document
from docreader.parser.chain_parser import FirstParser
from docreader.parser.docx_parser import DocxParser, extract_header_footer_lines
from docreader.parser.markitdown_parser import MarkitdownParser

logger = logging.getLogger(__name__)


class Docx2Parser(FirstParser):
    _parser_cls = (MarkitdownParser, DocxParser)

    def parse_into_text(self, content: bytes) -> Document:
        """Parse DOCX, then prepend section headers/footers (#3849).

        Both chained engines (markitdown, python-docx body walk) ignore the
        header/footer parts, so collect them here once from the raw package
        and prefix the winning parser's markdown with marker lines.
        """
        document = super().parse_into_text(content)
        if not document.is_valid():
            return document
        try:
            marker_lines = extract_header_footer_lines(content)
        except Exception:
            logger.warning("Failed to extract DOCX headers/footers", exc_info=True)
            return document
        if marker_lines:
            document.content = "\n\n".join(marker_lines) + "\n\n" + document.content
        return document


if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)

    your_file = "/path/to/your/file.docx"
    parser = Docx2Parser(separators=[".", "?", "!", "。", "？", "！"])
    with open(your_file, "rb") as f:
        content = f.read()

        document = parser.parse(content)
        for cc in document.chunks:
            logger.info(f"chunk: {cc}")

        # document = parser.parse_into_text(content)
        # logger.info(f"docx content: {document.content}")
        # logger.info(f"find images {document.images.keys()}")
