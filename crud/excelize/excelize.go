package excelize

import (
	"bytes"
	"io"
	"strings"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/crud"
	xlsx "github.com/xuri/excelize/v2"
)

func init() {
	crud.RegisterExportFormat("xlsx", Encode)
	crud.RegisterImportFormat("xlsx", Decode)
}

// Encode writes an XLSX workbook from an export table.
func Encode(table crud.ExportTable) ([]byte, string, error) {
	file := xlsx.NewFile()
	defer file.Close()
	sheet := file.GetSheetName(0)
	for index, header := range table.Headers {
		cell, err := xlsx.CoordinatesToCellName(index+1, 1)
		if err != nil {
			return nil, "", err
		}
		if err := file.SetCellValue(sheet, cell, crud.SafeExportValue(header)); err != nil {
			return nil, "", err
		}
	}
	for rowIndex, row := range table.Rows {
		for colIndex, value := range row {
			cell, err := xlsx.CoordinatesToCellName(colIndex+1, rowIndex+2)
			if err != nil {
				return nil, "", err
			}
			if err := file.SetCellValue(sheet, cell, crud.SafeExportValue(value)); err != nil {
				return nil, "", err
			}
		}
	}
	var buf bytes.Buffer
	if err := file.Write(&buf); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
}

// Decode reads import rows from an XLSX workbook.
func Decode(reader io.Reader) ([]*crud.ImportRow, error) {
	file, err := xlsx.OpenReader(reader)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	sheet := file.GetSheetName(0)
	rawRows, err := file.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	if len(rawRows) == 0 {
		return nil, nil
	}
	headers := rawRows[0]
	rows := make([]*crud.ImportRow, 0, len(rawRows)-1)
	for rowIndex, record := range rawRows[1:] {
		data := make(core.Map)
		for i, header := range headers {
			if i < len(record) {
				data[strings.TrimSpace(header)] = record[i]
			}
		}
		rows = append(rows, crud.NewImportRow(rowIndex+2, data))
	}
	return rows, nil
}
