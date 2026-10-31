package proposalexport

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"bidora/backend/internal/opportunity/application"
)

const maxSourceBytes = 5 * 1024 * 1024
const maxOutputBytes = 8 * 1024 * 1024

type Renderer struct{}

func New() *Renderer { return &Renderer{} }
func (*Renderer) Render(format string, p application.ProposalDocument) ([]byte, error) {
	if err := validateProposal(p); err != nil {
		return nil, err
	}
	var out []byte
	var err error
	switch format {
	case "docx":
		out, err = renderDOCX(p)
	case "pdf":
		out, err = renderPDF(p)
	default:
		return nil, application.ErrProposalExportUnsupported
	}
	if err != nil {
		return nil, err
	}
	if len(out) > maxOutputBytes {
		return nil, application.ErrProposalExportTooLarge
	}
	return out, nil
}
func validateProposal(p application.ProposalDocument) error {
	total := len(p.OrganizationLegalName) + len(p.Title) + len(p.Recipient) + len(p.Introduction) + len(p.Closing)
	lineBreaks := strings.Count(p.Introduction, "\n") + strings.Count(p.Closing, "\n")
	for _, line := range p.LetterheadLines {
		total += len(line)
		lineBreaks += strings.Count(line, "\n")
		if !validText(line) {
			return application.ErrProposalExportUnsupported
		}
	}
	for _, r := range p.Responses {
		total += len(r.RequirementText) + len(r.Category) + len(r.Content)
		lineBreaks += strings.Count(r.RequirementText, "\n") + strings.Count(r.Content, "\n")
		if !validText(r.RequirementText) || !validText(r.Category) || !validText(r.Content) {
			return application.ErrProposalExportUnsupported
		}
	}
	if total > maxSourceBytes || lineBreaks > 10000 || len(p.Responses) > 1000 {
		return application.ErrProposalExportTooLarge
	}
	if !validText(p.OrganizationLegalName) || !validText(p.Title) || !validText(p.Recipient) || !validText(p.Introduction) || !validText(p.Closing) {
		return application.ErrProposalExportUnsupported
	}
	return nil
}
func validText(v string) bool {
	if !utf8.ValidString(v) {
		return false
	}
	for _, r := range v {
		if r == 0x9 || r == 0xA || r == 0xD {
			continue
		}
		if r < 0x20 || (r >= 0xD800 && r <= 0xDFFF) || r == 0xFFFE || r == 0xFFFF {
			return false
		}
	}
	return true
}
func renderDOCX(p application.ProposalDocument) ([]byte, error) {
	var document bytes.Buffer
	document.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	docParagraph(&document, p.OrganizationLegalName, true, "center", 28)
	for _, line := range p.LetterheadLines {
		docParagraph(&document, line, false, "center", 18)
	}
	docParagraph(&document, "", false, "left", 18)
	docParagraph(&document, p.Title, true, "center", 32)
	docParagraph(&document, "Prepared for: "+p.Recipient, false, "left", 20)
	docParagraphs(&document, p.Introduction, false, "left", 20)
	for _, response := range p.Responses {
		heading := fmt.Sprintf("%d. %s", response.Ordinal, response.RequirementText)
		docParagraph(&document, heading, true, "left", 22)
		docParagraphs(&document, response.Content, false, "left", 20)
	}
	docParagraphs(&document, p.Closing, false, "left", 20)
	document.WriteString(`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1134" w:right="1134" w:bottom="1134" w:left="1134"/></w:sectPr></w:body></w:document>`)
	files := []struct{ name, data string }{{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`}, {"_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`}, {"word/document.xml", document.String()}}
	var result bytes.Buffer
	zw := zip.NewWriter(&result)
	fixed := time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, file := range files {
		header := &zip.FileHeader{Name: file.name, Method: zip.Store, Modified: fixed}
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return nil, err
		}
		if _, err = writer.Write([]byte(file.data)); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}
func docParagraphs(dst *bytes.Buffer, text string, bold bool, align string, size int) {
	for _, line := range splitLines(text) {
		docParagraph(dst, line, bold, align, size)
	}
}
func docParagraph(dst *bytes.Buffer, text string, bold bool, align string, size int) {
	dst.WriteString(`<w:p><w:pPr><w:jc w:val="` + align + `"/><w:spacing w:after="120"/></w:pPr><w:r><w:rPr>`)
	if bold {
		dst.WriteString(`<w:b/>`)
	}
	dst.WriteString(`<w:sz w:val="` + strconv.Itoa(size) + `"/></w:rPr><w:t xml:space="preserve">`)
	_ = xml.EscapeText(dst, []byte(text))
	dst.WriteString(`</w:t></w:r></w:p>`)
}

type pdfLine struct {
	text         string
	bold, center bool
	size         int
}

func renderPDF(p application.ProposalDocument) ([]byte, error) {
	lines := []pdfLine{{p.OrganizationLegalName, true, true, 14}}
	for _, v := range p.LetterheadLines {
		lines = appendWrapped(lines, v, false, true, 9, 95)
	}
	lines = append(lines, pdfLine{"", false, false, 10})
	lines = appendWrapped(lines, p.Title, true, true, 16, 70)
	lines = appendWrapped(lines, "Prepared for: "+p.Recipient, false, false, 10, 95)
	lines = append(lines, pdfLine{"", false, false, 10})
	lines = appendWrapped(lines, p.Introduction, false, false, 10, 95)
	for _, r := range p.Responses {
		lines = appendWrapped(lines, fmt.Sprintf("%d. %s", r.Ordinal, r.RequirementText), true, false, 11, 85)
		lines = appendWrapped(lines, r.Content, false, false, 10, 95)
	}
	lines = appendWrapped(lines, p.Closing, false, false, 10, 95)
	for _, line := range lines {
		if _, err := winANSI(line.text); err != nil {
			return nil, application.ErrProposalExportUnsupported
		}
	}
	const pageLines = 55
	pageCount := (len(lines) + pageLines - 1) / pageLines
	if pageCount < 1 {
		pageCount = 1
	}
	objects := make([][]byte, 4+pageCount*2)
	objects[0] = []byte(`<< /Type /Catalog /Pages 2 0 R >>`)
	kids := make([]string, pageCount)
	for i := 0; i < pageCount; i++ {
		kids[i] = fmt.Sprintf("%d 0 R", 5+i*2)
	}
	objects[1] = []byte(`<< /Type /Pages /Kids [` + strings.Join(kids, " ") + `] /Count ` + strconv.Itoa(pageCount) + ` >>`)
	objects[2] = []byte(`<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>`)
	objects[3] = []byte(`<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>`)
	for page := 0; page < pageCount; page++ {
		start := page * pageLines
		end := start + pageLines
		if end > len(lines) {
			end = len(lines)
		}
		var content bytes.Buffer
		y := 800
		for _, line := range lines[start:end] {
			encoded, _ := winANSI(line.text)
			font := "F1"
			if line.bold {
				font = "F2"
			}
			x := 50
			if line.center {
				x = 297 - (len(encoded)*line.size)/4
				if x < 50 {
					x = 50
				}
			}
			fmt.Fprintf(&content, "BT /%s %d Tf %d %d Td (%s) Tj ET\n", font, line.size, x, y, pdfEscape(encoded))
			y -= 14
		}
		pageObject := 4 + page*2
		contentObject := pageObject + 1
		objects[pageObject] = []byte(fmt.Sprintf(`<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 3 0 R /F2 4 0 R >> >> /Contents %d 0 R >>`, contentObject+1))
		stream := content.Bytes()
		objects[contentObject] = []byte(fmt.Sprintf("<< /Length %d >>\nstream\n", len(stream)) + string(stream) + "endstream")
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.7\n%\xE2\xE3\xCF\xD3\n")
	offsets := make([]int, len(objects)+1)
	for i, obj := range objects {
		offsets[i+1] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n", i+1)
		out.Write(obj)
		out.WriteString("\nendobj\n")
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for i := 1; i < len(offsets); i++ {
		fmt.Fprintf(&out, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return out.Bytes(), nil
}
func appendWrapped(dst []pdfLine, text string, bold, center bool, size, width int) []pdfLine {
	for _, paragraph := range splitLines(text) {
		runes := []rune(paragraph)
		if len(runes) == 0 {
			dst = append(dst, pdfLine{"", bold, center, size})
			continue
		}
		for len(runes) > width {
			cut := width
			for cut > width/2 && runes[cut] != ' ' {
				cut--
			}
			if cut <= width/2 {
				cut = width
			}
			dst = append(dst, pdfLine{strings.TrimSpace(string(runes[:cut])), bold, center, size})
			runes = []rune(strings.TrimSpace(string(runes[cut:])))
		}
		dst = append(dst, pdfLine{string(runes), bold, center, size})
	}
	return dst
}
func splitLines(v string) []string {
	return strings.Split(strings.ReplaceAll(strings.ReplaceAll(v, "\r\n", "\n"), "\r", "\n"), "\n")
}
func winANSI(v string) ([]byte, error) {
	mapping := map[rune]byte{0x20AC: 0x80, 0x201A: 0x82, 0x0192: 0x83, 0x201E: 0x84, 0x2026: 0x85, 0x2020: 0x86, 0x2021: 0x87, 0x02C6: 0x88, 0x2030: 0x89, 0x0160: 0x8A, 0x2039: 0x8B, 0x0152: 0x8C, 0x017D: 0x8E, 0x2018: 0x91, 0x2019: 0x92, 0x201C: 0x93, 0x201D: 0x94, 0x2022: 0x95, 0x2013: 0x96, 0x2014: 0x97, 0x02DC: 0x98, 0x2122: 0x99, 0x0161: 0x9A, 0x203A: 0x9B, 0x0153: 0x9C, 0x017E: 0x9E, 0x0178: 0x9F}
	out := make([]byte, 0, len(v))
	for _, r := range v {
		if r >= 0x20 && r <= 0x7E {
			out = append(out, byte(r))
			continue
		}
		if r >= 0xA0 && r <= 0xFF {
			out = append(out, byte(r))
			continue
		}
		if b, ok := mapping[r]; ok {
			out = append(out, b)
			continue
		}
		return nil, application.ErrProposalExportUnsupported
	}
	return out, nil
}
func pdfEscape(v []byte) string {
	var out strings.Builder
	for _, b := range v {
		switch b {
		case '(', ')', '\\':
			out.WriteByte('\\')
			out.WriteByte(b)
		default:
			if b < 0x20 || b >= 0x7F {
				fmt.Fprintf(&out, "\\%03o", b)
			} else {
				out.WriteByte(b)
			}
		}
	}
	return out.String()
}
