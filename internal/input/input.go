// Package input parses the POS trade-in export (CSV) into domain orders.
package input

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"goldbar/internal/model"
)

// expectedHeader defines the required CSV column order.
var expectedHeader = []string{
	"order_id", "customer", "old_karat", "old_weight",
	"new_product_code", "new_weight", "gold_price",
}

// ParseError reports a CSV or field parsing failure tied to a source line.
type ParseError struct {
	LineNumber int
	Message    string
}

// Error implements the error interface.
func (e *ParseError) Error() string {
	return fmt.Sprintf("第 %d 行: %s", e.LineNumber, e.Message)
}

// Parse reads the CSV stream and returns the parsed orders. The header row is
// required and must match expectedHeader exactly; data rows must have the same
// number of fields. An empty input or a header-only input is an error.
func Parse(r io.Reader) ([]model.Order, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	cr.FieldsPerRecord = -1

	header, err := cr.Read()
	if err == io.EOF {
		return nil, fmt.Errorf("输入为空")
	}
	if err != nil {
		return nil, fmt.Errorf("读取表头失败: %w", err)
	}
	if !headerMatches(header) {
		return nil, fmt.Errorf("表头不匹配: 期望 %s, 实际 %s",
			strings.Join(expectedHeader, ","), strings.Join(header, ","))
	}
	cr.FieldsPerRecord = len(expectedHeader)

	orders := make([]model.Order, 0, 16)
	lineNo := 1 // header occupies line 1
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		lineNo++
		if err != nil {
			return nil, &ParseError{LineNumber: lineNo, Message: err.Error()}
		}
		o, perr := parseRow(lineNo, rec)
		if perr != nil {
			return nil, perr
		}
		orders = append(orders, o)
	}
	if len(orders) == 0 {
		return nil, fmt.Errorf("输入不含任何订单记录")
	}
	return orders, nil
}

func headerMatches(header []string) bool {
	if len(header) != len(expectedHeader) {
		return false
	}
	for i, h := range expectedHeader {
		if strings.TrimSpace(header[i]) != h {
			return false
		}
	}
	return true
}

func parseRow(lineNo int, rec []string) (model.Order, error) {
	oldKarat, err := strconv.Atoi(strings.TrimSpace(rec[2]))
	if err != nil {
		return model.Order{}, &ParseError{LineNumber: lineNo, Message: fmt.Sprintf("old_karat 不是整数: %q", rec[2])}
	}
	oldWeight, err := strconv.ParseFloat(strings.TrimSpace(rec[3]), 64)
	if err != nil {
		return model.Order{}, &ParseError{LineNumber: lineNo, Message: fmt.Sprintf("old_weight 不是数值: %q", rec[3])}
	}
	newWeight, err := strconv.ParseFloat(strings.TrimSpace(rec[5]), 64)
	if err != nil {
		return model.Order{}, &ParseError{LineNumber: lineNo, Message: fmt.Sprintf("new_weight 不是数值: %q", rec[5])}
	}
	goldPrice, err := strconv.ParseFloat(strings.TrimSpace(rec[6]), 64)
	if err != nil {
		return model.Order{}, &ParseError{LineNumber: lineNo, Message: fmt.Sprintf("gold_price 不是数值: %q", rec[6])}
	}
	return model.Order{
		LineNumber:     lineNo,
		OrderID:        strings.TrimSpace(rec[0]),
		Customer:       strings.TrimSpace(rec[1]),
		OldKarat:       oldKarat,
		OldWeight:      oldWeight,
		NewProductCode: strings.TrimSpace(rec[4]),
		NewWeight:      newWeight,
		GoldPrice:      goldPrice,
	}, nil
}
