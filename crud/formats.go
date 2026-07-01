package crud

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/duxweb/runa/core"
)

// ExportTable stores normalized rows for export encoders.
type ExportTable struct {
	Headers []string
	Rows    [][]any
}

// ExportEncoder writes an export table and returns bytes plus content type.
type ExportEncoder func(table ExportTable) ([]byte, string, error)

// ImportDecoder reads import rows from a stream.
type ImportDecoder func(reader io.Reader) ([]*ImportRow, error)

var formatRegistry = struct {
	sync.RWMutex
	exporters map[string]ExportEncoder
	importers map[string]ImportDecoder
}{
	exporters: map[string]ExportEncoder{},
	importers: map[string]ImportDecoder{},
}

func init() {
	RegisterExportFormat("csv", encodeCSV)
	RegisterImportFormat("csv", decodeCSVRows)
}

// RegisterExportFormat registers an export encoder.
func RegisterExportFormat(name string, encoder ExportEncoder) {
	name = normalizeFormat(name)
	if name == "" || encoder == nil {
		return
	}
	formatRegistry.Lock()
	formatRegistry.exporters[name] = encoder
	formatRegistry.Unlock()
}

// RegisterImportFormat registers an import decoder.
func RegisterImportFormat(name string, decoder ImportDecoder) {
	name = normalizeFormat(name)
	if name == "" || decoder == nil {
		return
	}
	formatRegistry.Lock()
	formatRegistry.importers[name] = decoder
	formatRegistry.Unlock()
}

// NewImportRow creates one import row for custom import decoders.
func NewImportRow(index int, data core.Map) *ImportRow {
	if data == nil {
		data = make(core.Map)
	}
	return &ImportRow{index: index, data: data}
}

func exportEncoder(name string) (ExportEncoder, bool) {
	formatRegistry.RLock()
	defer formatRegistry.RUnlock()
	encoder, ok := formatRegistry.exporters[normalizeFormat(name)]
	return encoder, ok
}

func importDecoder(name string) (ImportDecoder, bool) {
	formatRegistry.RLock()
	defer formatRegistry.RUnlock()
	decoder, ok := formatRegistry.importers[normalizeFormat(name)]
	return decoder, ok
}

func defaultExportFormats() []string {
	return defaultFormats(formatRegistry.exporters)
}

func defaultImportFormats() []string {
	return defaultFormats(formatRegistry.importers)
}

func defaultFormats[T any](items map[string]T) []string {
	formatRegistry.RLock()
	defer formatRegistry.RUnlock()
	formats := make([]string, 0, len(items))
	if _, ok := items["csv"]; ok {
		formats = append(formats, "csv")
	}
	for name := range items {
		if name != "csv" {
			formats = append(formats, name)
		}
	}
	if len(formats) > 1 {
		sort.Strings(formats[1:])
	}
	return formats
}

func normalizeFormat(name string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(name)), ".")
}

func formatAllowed(formats []string, name string) bool {
	name = normalizeFormat(name)
	if name == "" {
		return false
	}
	if len(formats) == 0 {
		return true
	}
	for _, format := range formats {
		if normalizeFormat(format) == name {
			return true
		}
	}
	return false
}

func encodeCSV(table ExportTable) ([]byte, string, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	headers := make([]string, 0, len(table.Headers))
	for _, header := range table.Headers {
		headers = append(headers, SafeExportText(header))
	}
	if err := writer.Write(headers); err != nil {
		return nil, "", err
	}
	for _, row := range table.Rows {
		record := make([]string, 0, len(row))
		for _, value := range row {
			record = append(record, SafeExportText(value))
		}
		if err := writer.Write(record); err != nil {
			return nil, "", err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "text/csv; charset=utf-8", nil
}

func decodeCSVRows(reader io.Reader) ([]*ImportRow, error) {
	csvReader := csv.NewReader(reader)
	headers, err := csvReader.Read()
	if err != nil {
		return nil, err
	}
	rows := make([]*ImportRow, 0)
	index := 1
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		index++
		data := make(core.Map)
		for i, header := range headers {
			if i < len(record) {
				data[strings.TrimSpace(header)] = record[i]
			}
		}
		rows = append(rows, NewImportRow(index, data))
	}
	return rows, nil
}

func unsupportedExportFormat(name string) error {
	return fmt.Errorf("crud export format %s is not registered", name)
}

func unsupportedImportFormat(name string) error {
	return fmt.Errorf("crud import format %s is not registered", name)
}
