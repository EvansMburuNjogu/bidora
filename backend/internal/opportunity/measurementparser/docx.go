package measurementparser

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"path"
	"sort"
	"strings"
	"unicode/utf8"

	"bidora/backend/internal/opportunity/application"
	"bidora/backend/internal/opportunity/measurementprofile"
)

const (
	wordprocessingNamespace = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	relationshipsNamespace  = "http://schemas.openxmlformats.org/package/2006/relationships"
	contentTypesNamespace   = "http://schemas.openxmlformats.org/package/2006/content-types"
	wordMainContentType     = "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"
)

var oleCompoundSignature = []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}

type packageRelationship struct {
	Type, Target, TargetMode string
}

func measureDOCX(ctx context.Context, data []byte, limits application.MeasurementLimits) Result {
	result := Result{Unit: application.DOCXExtractedCharacters, Semantics: measurementprofile.DOCXSemantics}
	if bytes.HasPrefix(data, oleCompoundSignature) {
		result.FailureCode = application.MeasurementEncryptedDocument
		return result
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		result.FailureCode = application.MeasurementUnsupportedFormat
		return result
	}
	if int64(len(reader.File)) > limits.MaxArchiveEntries {
		result.FailureCode = application.MeasurementArchiveLimitExceeded
		return result
	}

	files, code := readPackage(ctx, reader, limits)
	if code.Valid() {
		result.FailureCode = code
		return result
	}
	if code = validateContentTypes(ctx, files["[Content_Types].xml"], limits.MaxXMLDepth); code.Valid() {
		result.FailureCode = code
		return result
	}

	groups := map[string][]string{"header": {}, "footer": {}, "footnotes": {}, "endnotes": {}}
	for name, content := range files {
		if !strings.HasSuffix(name, ".rels") {
			continue
		}
		relationships, relationCode := parseRelationships(ctx, content, limits.MaxXMLDepth)
		if relationCode.Valid() {
			result.FailureCode = relationCode
			return result
		}
		for _, relationship := range relationships {
			if strings.EqualFold(relationship.TargetMode, "External") || strings.Contains(relationship.Target, "://") {
				result.FailureCode = application.MeasurementArchiveLimitExceeded
				return result
			}
			if unsafeRelationshipTarget(relationship.Target) {
				result.FailureCode = application.MeasurementArchiveLimitExceeded
				return result
			}
			if name != "word/_rels/document.xml.rels" {
				continue
			}
			kind := supportingPartKind(relationship.Type)
			if kind == "" {
				continue
			}
			target := path.Clean(path.Join("word", relationship.Target))
			if !strings.HasPrefix(target, "word/") || target == "word" {
				result.FailureCode = application.MeasurementArchiveLimitExceeded
				return result
			}
			if _, exists := files[target]; !exists {
				result.FailureCode = application.MeasurementCorruptDocument
				return result
			}
			groups[kind] = append(groups[kind], target)
		}
	}

	if _, exists := files["word/document.xml"]; !exists {
		result.FailureCode = application.MeasurementCorruptDocument
		return result
	}
	parts := []string{"word/document.xml"}
	seen := map[string]bool{"word/document.xml": true}
	expectedRoots := map[string]string{"word/document.xml": "document"}
	for _, kind := range []string{"header", "footer", "footnotes", "endnotes"} {
		sort.Strings(groups[kind])
		for _, part := range groups[kind] {
			if !seen[part] {
				parts = append(parts, part)
				seen[part] = true
				expectedRoots[part] = supportingRoot(kind)
			}
		}
	}

	var total int64
	for _, part := range parts {
		count, textCode := countWordText(ctx, files[part], expectedRoots[part], limits.MaxXMLDepth, limits.MaxDOCXCharacters-total)
		if textCode.Valid() {
			result.FailureCode = textCode
			return result
		}
		total += count
		if total > limits.MaxDOCXCharacters {
			result.FailureCode = application.MeasurementCharacterLimitExceeded
			return result
		}
	}
	if total < 1 {
		result.FailureCode = application.MeasurementCorruptDocument
		return result
	}
	result.MeasuredUnits = total
	return result
}

func readPackage(ctx context.Context, reader *zip.Reader, limits application.MeasurementLimits) (map[string][]byte, application.MeasurementFailureCode) {
	files := make(map[string][]byte, len(reader.File))
	var declaredTotal int64
	for _, file := range reader.File {
		if err := ctx.Err(); err != nil {
			return nil, timeoutCode(err)
		}
		name, directory, ok := normalizedPartName(file)
		if !ok {
			return nil, application.MeasurementArchiveLimitExceeded
		}
		if _, duplicate := files[name]; duplicate {
			return nil, application.MeasurementArchiveLimitExceeded
		}
		files[name] = nil
		if directory {
			continue
		}
		if file.UncompressedSize64 > uint64(limits.MaxExpandedBytes) || declaredTotal > limits.MaxExpandedBytes-int64(file.UncompressedSize64) {
			return nil, application.MeasurementArchiveLimitExceeded
		}
		declaredTotal += int64(file.UncompressedSize64)
	}

	var expanded int64
	for _, file := range reader.File {
		name, directory, _ := normalizedPartName(file)
		if directory {
			continue
		}
		stream, err := file.Open()
		if err != nil {
			return nil, application.MeasurementCorruptDocument
		}
		content, readErr := io.ReadAll(io.LimitReader(&contextReader{ctx: ctx, reader: stream}, limits.MaxExpandedBytes-expanded+1))
		closeErr := stream.Close()
		if readErr != nil {
			if ctx.Err() != nil {
				return nil, timeoutCode(ctx.Err())
			}
			return nil, application.MeasurementCorruptDocument
		}
		if closeErr != nil || uint64(len(content)) != file.UncompressedSize64 {
			return nil, application.MeasurementCorruptDocument
		}
		expanded += int64(len(content))
		if expanded > limits.MaxExpandedBytes {
			return nil, application.MeasurementArchiveLimitExceeded
		}
		if isZIPSignature(content) {
			return nil, application.MeasurementArchiveLimitExceeded
		}
		files[name] = content
	}
	return files, ""
}

func normalizedPartName(file *zip.File) (string, bool, bool) {
	name := file.Name
	if name == "" || strings.Contains(name, "\\") || strings.ContainsRune(name, 0) || strings.HasPrefix(name, "/") {
		return "", false, false
	}
	directory := file.FileInfo().IsDir()
	if directory {
		name = strings.TrimSuffix(name, "/")
	}
	if name == "" || path.Clean(name) != name || name == ".." || strings.HasPrefix(name, "../") {
		return "", false, false
	}
	if !directory && !file.Mode().IsRegular() {
		return "", false, false
	}
	return name, directory, true
}

func isZIPSignature(content []byte) bool {
	if len(content) < 4 {
		return false
	}
	return bytes.Equal(content[:4], []byte{'P', 'K', 3, 4}) || bytes.Equal(content[:4], []byte{'P', 'K', 5, 6}) || bytes.Equal(content[:4], []byte{'P', 'K', 7, 8})
}

func validateContentTypes(ctx context.Context, content []byte, maxDepth int32) application.MeasurementFailureCode {
	if len(content) == 0 {
		return application.MeasurementUnsupportedFormat
	}
	decoder := xml.NewDecoder(bytes.NewReader(content))
	decoder.Strict = true
	var depth int32
	mainFound := false
	for {
		if err := ctx.Err(); err != nil {
			return timeoutCode(err)
		}
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return application.MeasurementCorruptDocument
		}
		switch value := token.(type) {
		case xml.Directive:
			return application.MeasurementXMLLimitExceeded
		case xml.StartElement:
			depth++
			if depth > maxDepth {
				return application.MeasurementXMLLimitExceeded
			}
			partName, contentType := "", ""
			for _, attribute := range value.Attr {
				switch attribute.Name.Local {
				case "PartName":
					partName = attribute.Value
				case "ContentType":
					contentType = attribute.Value
				}
			}
			if strings.Contains(strings.ToLower(contentType), "macroenabled") {
				return application.MeasurementUnsupportedFormat
			}
			if value.Name.Space == contentTypesNamespace && value.Name.Local == "Override" && partName == "/word/document.xml" && contentType == wordMainContentType {
				mainFound = true
			}
		case xml.EndElement:
			depth--
		}
	}
	if !mainFound {
		return application.MeasurementUnsupportedFormat
	}
	return ""
}

func parseRelationships(ctx context.Context, content []byte, maxDepth int32) ([]packageRelationship, application.MeasurementFailureCode) {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	decoder.Strict = true
	var relationships []packageRelationship
	var depth int32
	for {
		if err := ctx.Err(); err != nil {
			return nil, timeoutCode(err)
		}
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, application.MeasurementCorruptDocument
		}
		switch value := token.(type) {
		case xml.Directive:
			return nil, application.MeasurementXMLLimitExceeded
		case xml.StartElement:
			depth++
			if depth > maxDepth {
				return nil, application.MeasurementXMLLimitExceeded
			}
			if value.Name.Space != relationshipsNamespace || value.Name.Local != "Relationship" {
				continue
			}
			relationship := packageRelationship{}
			for _, attribute := range value.Attr {
				switch attribute.Name.Local {
				case "Type":
					relationship.Type = attribute.Value
				case "Target":
					relationship.Target = attribute.Value
				case "TargetMode":
					relationship.TargetMode = attribute.Value
				}
			}
			if relationship.Type == "" || relationship.Target == "" {
				return nil, application.MeasurementCorruptDocument
			}
			relationships = append(relationships, relationship)
		case xml.EndElement:
			depth--
		}
	}
	return relationships, ""
}

func unsafeRelationshipTarget(target string) bool {
	if target == "" || strings.Contains(target, "\\") || strings.ContainsRune(target, 0) || strings.HasPrefix(target, "/") {
		return true
	}
	for _, component := range strings.Split(target, "/") {
		if component == ".." || component == "" {
			return true
		}
	}
	return false
}

func supportingPartKind(relationshipType string) string {
	for _, kind := range []string{"header", "footer", "footnotes", "endnotes"} {
		if strings.HasSuffix(relationshipType, "/"+kind) {
			return kind
		}
	}
	return ""
}

func supportingRoot(kind string) string {
	switch kind {
	case "header":
		return "hdr"
	case "footer":
		return "ftr"
	default:
		return kind
	}
}

func countWordText(ctx context.Context, content []byte, expectedRoot string, maxDepth int32, remaining int64) (int64, application.MeasurementFailureCode) {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	decoder.Strict = true
	var depth, deleted int32
	inText, rootSeen := false, false
	var count int64
	for {
		if err := ctx.Err(); err != nil {
			return 0, timeoutCode(err)
		}
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, application.MeasurementCorruptDocument
		}
		switch value := token.(type) {
		case xml.Directive:
			return 0, application.MeasurementXMLLimitExceeded
		case xml.StartElement:
			depth++
			if depth > maxDepth {
				return 0, application.MeasurementXMLLimitExceeded
			}
			if depth == 1 {
				if value.Name.Space != wordprocessingNamespace || value.Name.Local != expectedRoot {
					return 0, application.MeasurementCorruptDocument
				}
				rootSeen = true
			}
			if value.Name.Space != wordprocessingNamespace {
				continue
			}
			if value.Name.Local == "del" {
				deleted++
			}
			if deleted == 0 {
				switch value.Name.Local {
				case "t":
					inText = true
				case "tab", "br", "cr":
					count++
				}
			}
		case xml.CharData:
			if deleted == 0 && inText {
				if !utf8.Valid(value) {
					return 0, application.MeasurementCorruptDocument
				}
				count += int64(utf8.RuneCount(value))
			}
		case xml.EndElement:
			if value.Name.Space == wordprocessingNamespace {
				if deleted == 0 {
					if value.Name.Local == "t" {
						inText = false
					}
					if value.Name.Local == "p" {
						count++
					}
				}
				if value.Name.Local == "del" {
					deleted--
				}
			}
			depth--
		}
		if count > remaining {
			return 0, application.MeasurementCharacterLimitExceeded
		}
	}
	if !rootSeen {
		return 0, application.MeasurementCorruptDocument
	}
	return count, ""
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}
