// Package excel 基于 excelize 的极简表格读写封装：只保留脚手架用得到的两个方向。
//
//   - Build：二维数据 → .xlsx 字节流（带表头样式、列宽自适应、冻结首行）
//   - Read：.xlsx 字节流 → 二维字符串（跳过表头，容忍尾部空行）
package excel

import (
	"bytes"
	"io"

	"github.com/xuri/excelize/v2"
)

// DefaultSheet 默认工作表名。
const DefaultSheet = "Sheet1"

// Build 生成表格：headers 为表头，rows 为数据行（值会按 Excel 类型写入）。
func Build(sheet string, headers []string, rows [][]any) ([]byte, error) {
	if sheet == "" {
		sheet = DefaultSheet
	}
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	// excelize 默认第一个 sheet 名为 Sheet1，这里改名以贴合业务
	if err := f.SetSheetName(f.GetSheetName(0), sheet); err != nil {
		return nil, err
	}

	bold, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"4A6CF7"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err != nil {
		return nil, err
	}

	for i, h := range headers {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return nil, err
		}
	}
	if err := f.SetRowHeight(sheet, 1, 22); err != nil {
		return nil, err
	}
	headerCells, err := cellRange(1, 1, len(headers), 1)
	if err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, firstCell(headerCells), lastCell(headerCells), bold); err != nil {
		return nil, err
	}

	for r, row := range rows {
		for c, v := range row {
			cell, err := excelize.CoordinatesToCellName(c+1, r+2)
			if err != nil {
				return nil, err
			}
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				return nil, err
			}
		}
	}

	// 列宽：按表头长度给一个稳妥宽度，避免中文被截断
	for i, h := range headers {
		width := float64(len([]rune(h))*2 + 8)
		if width < 12 {
			width = 12
		}
		col, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return nil, err
		}
		if err := f.SetColWidth(sheet, col, col, width); err != nil {
			return nil, err
		}
	}
	// 冻结首行，方便大表浏览
	if err := f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return nil, err
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Read 读取第一个工作表的全部内容（含表头行）。空行会被跳过。
func Read(r io.Reader, maxRows int) ([][]string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, nil
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, err
	}
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		out = append(out, row)
		if maxRows > 0 && len(out) > maxRows {
			break
		}
	}
	return out, nil
}

func isEmptyRow(row []string) bool {
	for _, v := range row {
		if len(bytes.TrimSpace([]byte(v))) > 0 {
			return false
		}
	}
	return true
}

// cellRange 生成 "A1:K1" 形式的区域（仅行内范围）。
func cellRange(colFrom, rowFrom, colTo, rowTo int) ([]string, error) {
	from, err := excelize.CoordinatesToCellName(colFrom, rowFrom)
	if err != nil {
		return nil, err
	}
	to, err := excelize.CoordinatesToCellName(colTo, rowTo)
	if err != nil {
		return nil, err
	}
	return []string{from, to}, nil
}

func firstCell(r []string) string { return r[0] }
func lastCell(r []string) string  { return r[len(r)-1] }
