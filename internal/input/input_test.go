package input

import (
	"strings"
	"testing"
)

const sampleCSV = `order_id,customer,old_karat,old_weight,new_product_code,new_weight,gold_price
A001,张三,999,10.0,G001,8.0,768.50
A002,李四,990,5.5,G002,6.0,768.50
`

func TestParseValid(t *testing.T) {
	orders, err := Parse(strings.NewReader(sampleCSV))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(orders) != 2 {
		t.Fatalf("len = %d", len(orders))
	}
	if orders[0].LineNumber != 2 {
		t.Errorf("first line = %d, want 2", orders[0].LineNumber)
	}
	if orders[0].OrderID != "A001" {
		t.Errorf("order = %v", orders[0])
	}
	if orders[1].OldKarat != 990 {
		t.Errorf("karat = %v", orders[1].OldKarat)
	}
	if orders[1].GoldPrice != 768.5 {
		t.Errorf("gold_price = %v", orders[1].GoldPrice)
	}
}

func TestParseBadHeader(t *testing.T) {
	if _, err := Parse(strings.NewReader("a,b,c\n")); err == nil {
		t.Fatal("expected header mismatch error")
	}
}

func TestParseBadField(t *testing.T) {
	bad := `order_id,customer,old_karat,old_weight,new_product_code,new_weight,gold_price
A001,张三,xxx,10,G001,8,768.5
`
	if _, err := Parse(strings.NewReader(bad)); err == nil {
		t.Fatal("expected field parse error")
	}
}

func TestParseWrongFieldCount(t *testing.T) {
	bad := `order_id,customer,old_karat,old_weight,new_product_code,new_weight,gold_price
A001,张三,999,10,G001,8
`
	_, err := Parse(strings.NewReader(bad))
	if err == nil {
		t.Fatal("expected a parse error")
	}
	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if pe.LineNumber != 2 {
		t.Errorf("line = %d, want 2", pe.LineNumber)
	}
}

func TestParseEmpty(t *testing.T) {
	if _, err := Parse(strings.NewReader("")); err == nil {
		t.Fatal("expected empty input error")
	}
	headerOnly := `order_id,customer,old_karat,old_weight,new_product_code,new_weight,gold_price
`
	if _, err := Parse(strings.NewReader(headerOnly)); err == nil {
		t.Fatal("expected no-records error")
	}
}
