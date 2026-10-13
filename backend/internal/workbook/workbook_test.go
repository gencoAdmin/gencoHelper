package workbook

import (
	"bytes"
	"encoding/csv"
	"genco/finance/internal/finance"
	"genco/finance/internal/model"
	"github.com/xuri/excelize/v2"
	"strings"
	"testing"
)

func TestCSVFormulaProtectionAndTextSKU(t *testing.T) {
	in := model.Input{Name: " =HYPERLINK(\"evil\")", SKU: "00001", Category: "general", Cost: "30", Price: "50", Orders: "10", Fixed: "0", Packaging: "0", TargetMargin: "15"}
	p := model.Product{Input: in, Revisions: []model.Evaluation{finance.Evaluate(in, model.Defaults())}}
	var buf bytes.Buffer
	if err := Export(&buf, []model.Product{p}, "csv", "en", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(buf.String(), "\ufeff")))
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rows[1][0], "'") || rows[1][1] != "00001" {
		t.Fatal("CSV unsafe or SKU lost")
	}
	buf.Reset()
	if err := Export(&buf, []model.Product{p}, "xlsx", "id", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	formula, _ := f.GetCellFormula("Summary", "A2")
	sku, _ := f.GetCellValue("Summary", "B2")
	if formula != "" || sku != "00001" {
		t.Fatal("Excel user text evaluated")
	}
}
