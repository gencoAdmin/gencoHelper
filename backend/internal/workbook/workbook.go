package workbook

import (
	"archive/zip"
	"encoding/csv"
	"errors"
	"fmt"
	"genco/finance/internal/finance"
	"genco/finance/internal/model"
	"github.com/xuri/excelize/v2"
	"io"
	"math/big"
	"path/filepath"
	"strconv"
	"strings"
)

var Fields = []string{"name", "sku", "category", "cost", "price", "orders", "fixed", "packaging", "marketMin", "marketMax", "targetMargin", "notes"}
var Headers = []string{"Product name", "SKU", "Category", "Product cost", "Selling price", "Monthly orders", "Monthly fixed costs", "Packaging cost", "Market minimum", "Market maximum", "Target margin (%)", "Notes"}
var IDHeaders = []string{"Nama produk", "SKU", "Kategori", "Biaya produk", "Harga jual", "Pesanan bulanan", "Biaya tetap bulanan", "Biaya kemasan", "Harga pasar minimum", "Harga pasar maksimum", "Target margin (%)", "Catatan"}
var ZHHeaders = []string{"产品名称", "SKU", "分类", "产品成本", "售价", "月订单量", "月固定成本", "包装成本", "市场最低价", "市场最高价", "目标利润率 (%)", "备注"}

type Sheet struct {
	Name     string   `json:"name"`
	Headers  []string `json:"headers"`
	RowCount int      `json:"rowCount"`
}
type Upload struct {
	ID      string         `json:"id"`
	Sheets  []Sheet        `json:"sheets"`
	Mapping map[string]int `json:"mapping"`
}

func Open(path string) (*excelize.File, error) {
	z, err := zip.OpenReader(path)
	if err != nil {
		return nil, errors.New("Upload a valid .xlsx workbook")
	}
	defer z.Close()
	var size uint64
	if len(z.File) > 2000 {
		return nil, errors.New("Too many workbook ZIP entries")
	}
	for _, f := range z.File {
		size += f.UncompressedSize64
		if size > 64<<20 || f.UncompressedSize64 > 32<<20 {
			return nil, errors.New("Workbook decompression limit exceeded")
		}
	}
	return excelize.OpenFile(path, excelize.Options{UnzipSizeLimit: 64 << 20, UnzipXMLSizeLimit: 16 << 20, RawCellValue: true})
}
func readSheet(f *excelize.File, sheet string) ([][]string, error) {
	exists := false
	for _, s := range f.GetSheetList() {
		if s == sheet {
			exists = true
		}
	}
	if !exists {
		return nil, errors.New("Worksheet does not exist")
	}
	it, err := f.Rows(sheet)
	if err != nil {
		return nil, err
	}
	defer it.Close()
	rows := [][]string{}
	cells := 0
	for it.Next() {
		if len(rows) >= 5001 {
			return nil, errors.New("Maximum 5,000 data rows per worksheet")
		}
		r, err := it.Columns(excelize.Options{RawCellValue: true})
		if err != nil {
			return nil, err
		}
		cells += len(r)
		if len(r) > 200 || cells > 1000000 {
			return nil, errors.New("Worksheet cell limit exceeded")
		}
		for _, v := range r {
			if len(v) > 10000 {
				return nil, errors.New("Cell text limit exceeded")
			}
		}
		rows = append(rows, r)
	}
	if err := it.Error(); err != nil {
		return nil, err
	}
	return rows, nil
}
func Inspect(path, id string) (Upload, error) {
	f, err := Open(path)
	if err != nil {
		return Upload{}, err
	}
	defer f.Close()
	u := Upload{ID: id, Mapping: map[string]int{}}
	if len(f.GetSheetList()) > 20 {
		return u, errors.New("Maximum 20 worksheets")
	}
	for i, name := range f.GetSheetList() {
		rows, err := readSheet(f, name)
		if err != nil {
			return u, err
		}
		if len(rows) == 0 {
			continue
		}
		u.Sheets = append(u.Sheets, Sheet{Name: name, Headers: rows[0], RowCount: len(rows) - 1})
		if i == 0 {
			u.Mapping = AutoMap(rows[0])
		}
	}
	if len(u.Sheets) == 0 {
		return u, errors.New("Workbook has no data")
	}
	return u, nil
}
func AutoMap(headers []string) map[string]int {
	m := map[string]int{}
	norm := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
	for i, h := range headers {
		for j, k := range Fields {
			if norm(h) == norm(Headers[j]) || norm(h) == norm(IDHeaders[j]) || norm(h) == norm(ZHHeaders[j]) || norm(h) == norm(k) {
				if _, ok := m[k]; !ok {
					m[k] = i
				}
			}
		}
	}
	return m
}
func Preview(path, sheet string, mapping map[string]int, s model.Settings, products map[string]model.Product) ([]model.ImportRow, error) {
	for _, k := range []string{"name", "cost", "price", "orders"} {
		if _, ok := mapping[k]; !ok {
			return nil, fmt.Errorf("Map required column: %s", k)
		}
	}
	used := map[int]bool{}
	for k, i := range mapping {
		known := false
		for _, v := range Fields {
			if k == v {
				known = true
			}
		}
		if !known || i < 0 || i >= 200 {
			return nil, errors.New("Invalid column mapping")
		}
		if used[i] {
			return nil, errors.New("Each source column can only be mapped once")
		}
		used[i] = true
	}
	f, err := Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rows, err := readSheet(f, sheet)
	if err != nil {
		return nil, err
	}
	skus := map[string]bool{}
	for _, p := range products {
		if p.Input.SKU != "" {
			skus[p.Input.SKU] = true
		}
	}
	out := []model.ImportRow{}
	for i, row := range rows {
		if i == 0 {
			continue
		}
		get := func(k string) string {
			idx, ok := mapping[k]
			if !ok || idx >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[idx])
		}
		blank := true
		for _, v := range row {
			if strings.TrimSpace(v) != "" {
				blank = false
			}
		}
		if blank {
			continue
		}
		in := model.Input{Name: get("name"), SKU: get("sku"), Category: get("category"), Notes: get("notes"), Cost: get("cost"), Price: get("price"), Orders: get("orders"), Fixed: get("fixed"), Packaging: get("packaging"), TargetMargin: get("targetMargin"), MarketMin: get("marketMin"), MarketMax: get("marketMax")}
		// Preserve displayed leading zeros for numeric SKU cells with a zero format.
		if col, ok := mapping["sku"]; ok {
			cell, _ := excelize.CoordinatesToCellName(col+1, i+1)
			formatted, err := f.GetCellValue(sheet, cell, excelize.Options{RawCellValue: false})
			if err != nil {
				return nil, err
			}
			in.SKU = strings.TrimSpace(formatted)
		}
		if in.Category == "" {
			in.Category = "general"
		}
		if in.Fixed == "" {
			in.Fixed = "0"
		}
		if in.Packaging == "" {
			in.Packaging = "0"
		}
		if in.TargetMargin == "" {
			in.TargetMargin = s.TargetMargin
		}
		r := model.ImportRow{Row: i + 1, Input: in, Errors: []string{}}
		for k, col := range mapping {
			cell, _ := excelize.CoordinatesToCellName(col+1, i+1)
			formula, err := f.GetCellFormula(sheet, cell)
			if err != nil {
				return nil, err
			}
			if formula != "" {
				r.Errors = append(r.Errors, k+": formulas are not allowed")
			}
		}
		for k, err := range finance.Validate(in, true) {
			r.Errors = append(r.Errors, k+": "+err)
		}
		if in.SKU != "" && skus[in.SKU] {
			r.Errors = append(r.Errors, "sku: duplicate SKU")
		}
		if len(r.Errors) == 0 && in.SKU != "" {
			skus[in.SKU] = true
		}
		out = append(out, r)
	}
	return out, nil
}
func Template(locale string) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetSheetName("Sheet1", "Products")
	headers := Headers
	if locale == "id" {
		headers = IDHeaders
	}
	if locale == "zh" {
		headers = ZHHeaders
	}
	r := make([]any, len(headers))
	for i, h := range headers {
		r[i] = h
	}
	if err := f.SetSheetRow("Products", "A1", &r); err != nil {
		return nil, err
	}
	style, _ := f.NewStyle(&excelize.Style{NumFmt: 49})
	_ = f.SetColStyle("Products", "B", style)
	_ = f.SetColWidth("Products", "A", "L", 23)
	_ = f.SetPanes("Products", &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2"})
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
func headings(locale string) map[string]string {
	en := map[string]string{"product": "Product", "sku": "SKU", "category": "Category", "channel": "Channel", "price": "Evaluated price", "cost": "Product cost", "orders": "Monthly orders", "margin": "Margin (%)", "profit": "Monthly profit before tax", "verdict": "Verdict", "revision": "Evaluation ID", "preset": "Preset version", "calculation": "Calculation version", "field": "Field", "value": "Value", "source": "Assumption source", "platform": "Platform", "shop": "Shop", "sales": "Monthly sales", "rating": "Rating", "url": "URL", "image": "Image URL", "thumbnail": "Thumbnail URL"}
	id := []string{"Produk", "SKU", "Kategori", "Saluran", "Harga evaluasi", "Biaya produk", "Pesanan bulanan", "Margin (%)", "Laba bulanan sebelum pajak", "Penilaian", "ID evaluasi", "Versi preset", "Versi perhitungan", "Kolom", "Nilai", "Sumber asumsi", "Platform", "Toko", "Penjualan bulanan", "Penilaian toko", "URL", "URL gambar", "URL thumbnail"}
	zh := []string{"产品", "SKU", "分类", "渠道", "测算价格", "产品成本", "月订单量", "利润率 (%)", "税前月营业利润", "结论", "测算 ID", "预设版本", "计算版本", "字段", "值", "假设来源", "平台", "店铺", "月销量", "评分", "URL", "图片 URL", "缩略图 URL"}
	keys := []string{"product", "sku", "category", "channel", "price", "cost", "orders", "margin", "profit", "verdict", "revision", "preset", "calculation", "field", "value", "source", "platform", "shop", "sales", "rating", "url", "image", "thumbnail"}
	for i, k := range keys {
		if locale == "id" {
			en[k] = id[i]
		}
		if locale == "zh" {
			en[k] = zh[i]
		}
	}
	return en
}
func labels(locale, key string) string {
	dict := map[string][3]string{
		"shelf": {"Marketplace", "Marketplace", "货架电商"}, "live": {"TikTok Live", "TikTok Live", "TikTok 直播"}, "cod": {"COD distribution", "Distribusi COD", "COD 分销"}, "self": {"Own store", "Toko sendiri", "自营商城"}, "consignment": {"Consignment", "Konsinyasi", "寄售"},
		"healthy": {"Healthy", "Sehat", "健康"}, "viable": {"Viable", "Layak", "可行"}, "thin": {"Thin margin", "Margin tipis", "利润微薄"}, "unprofitable": {"Unprofitable", "Merugi", "亏损"},
		"general": {"General", "Umum", "通用"}, "fashion": {"Fashion", "Fesyen", "时尚"}, "beauty": {"Beauty", "Kecantikan", "美妆"}, "electronics": {"Electronics", "Elektronik", "电子"}, "home": {"Home", "Rumah tangga", "家居"},
		"preset": {"Channel preset", "Preset saluran", "渠道预设"}, "category": {"Category default", "Default kategori", "品类默认"}, "product": {"Product override", "Override produk", "产品覆盖"},
	}
	for i, k := range Fields {
		dict[k] = [3]string{Headers[i], IDHeaders[i], ZHHeaders[i]}
	}
	rateLabels := map[string][3]string{"commission": {"Commission (%)", "Komisi (%)", "佣金 (%)"}, "payment": {"Payment (%)", "Pembayaran (%)", "支付 (%)"}, "affiliate": {"Affiliate (%)", "Afiliasi (%)", "达人 (%)"}, "advertising": {"Advertising (%)", "Iklan (%)", "广告 (%)"}, "promotion": {"Promotion (%)", "Promosi (%)", "促销 (%)"}, "logisticsRate": {"Logistics (%)", "Logistik (%)", "物流 (%)"}, "logistics": {"Logistics / order", "Logistik / pesanan", "每单物流"}, "shippingIncome": {"Shipping income / sale", "Pendapatan ongkir / penjualan", "每单运费收入"}, "settlement": {"Settlement / order", "Penyelesaian / pesanan", "每单结算"}, "returns": {"Returns (%)", "Retur (%)", "退货 (%)"}}
	for k, v := range rateLabels {
		dict[k] = v
	}
	v, ok := dict[key]
	if !ok {
		return key
	}
	i := 0
	if locale == "id" {
		i = 1
	}
	if locale == "zh" {
		i = 2
	}
	return v[i]
}
func numeric(s string) any {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return s
	}
	return v
}
func safeCSV(s string) string {
	v := strings.TrimLeft(s, " \t\r\n")
	if v != "" && strings.ContainsRune("=+-@", rune(v[0])) {
		return "'" + s
	}
	return s
}
func Export(w io.Writer, products []model.Product, format, locale, base string) error {
	h := headings(locale)
	summaryKeys := []string{"product", "sku", "category", "channel", "price", "cost", "orders", "margin", "profit", "verdict", "revision", "preset", "calculation"}
	headers := []any{}
	for _, k := range summaryKeys {
		headers = append(headers, h[k])
	}
	summaries := [][]any{}
	for _, p := range products {
		ev := p.Latest()
		var r model.Result
		for _, v := range ev.Results {
			if v.Channel == ev.Recommended {
				r = v
			}
		}
		summaries = append(summaries, []any{p.Input.Name, p.Input.SKU, labels(locale, p.Input.Category), labels(locale, ev.Recommended), numeric(r.Price), numeric(ev.Input.Cost), numeric(ev.Input.Orders), numeric(r.Margin), numeric(r.MonthlyProfit), labels(locale, r.Verdict), ev.ID, ev.PresetVersion, ev.CalculationVersion})
	}
	if format == "csv" {
		if _, err := io.WriteString(w, "\ufeff"); err != nil {
			return err
		}
		cw := csv.NewWriter(w)
		str := func(row []any) []string {
			out := []string{}
			for _, v := range row {
				if s, ok := v.(string); ok {
					out = append(out, safeCSV(s))
				} else {
					out = append(out, fmt.Sprint(v))
				}
			}
			return out
		}
		if err := cw.Write(str(headers)); err != nil {
			return err
		}
		for _, r := range summaries {
			if err := cw.Write(str(r)); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	}
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetSheetName("Sheet1", "Summary")
	for _, s := range []string{"Inputs and Rates", "Channel Results", "Competitors", "Images"} {
		_, _ = f.NewSheet(s)
	}
	rows := map[string]int{}
	write := func(sheet string, row []any) error {
		rows[sheet]++
		cell, _ := excelize.CoordinatesToCellName(1, rows[sheet])
		return f.SetSheetRow(sheet, cell, &row)
	}
	if err := write("Summary", headers); err != nil {
		return err
	}
	for _, r := range summaries {
		if err := write("Summary", r); err != nil {
			return err
		}
	}
	for sheet, row := range map[string][]any{"Inputs and Rates": {h["product"], h["sku"], h["revision"], h["preset"], h["calculation"], h["channel"], h["field"], h["value"], h["source"]}, "Channel Results": append([]any{h["product"], h["sku"], h["revision"], h["channel"], h["price"], h["margin"], h["profit"], h["verdict"]}, resultHeaders(locale)...), "Competitors": {h["product"], h["sku"], h["platform"], h["shop"], h["price"], h["sales"], h["rating"], h["url"]}, "Images": {h["product"], h["sku"], h["image"], h["thumbnail"]}} {
		if err := write(sheet, row); err != nil {
			return err
		}
	}
	metric := func(m model.Metric) any {
		if m.Value != nil {
			return numeric(*m.Value)
		}
		return m.Status
	}
	for _, p := range products {
		ev := p.Latest()
		values := []string{ev.Input.Name, ev.Input.SKU, ev.Input.Category, ev.Input.Cost, ev.Input.Price, ev.Input.Orders, ev.Input.Fixed, ev.Input.Packaging, ev.Input.MarketMin, ev.Input.MarketMax, ev.Input.TargetMargin, ev.Input.Notes}
		for i, k := range Fields {
			var val any = values[i]
			if i >= 3 && i <= 10 && values[i] != "" {
				val = numeric(values[i])
			}
			if err := write("Inputs and Rates", []any{p.Input.Name, p.Input.SKU, ev.ID, ev.PresetVersion, ev.CalculationVersion, "", labels(locale, k), val, ""}); err != nil {
				return err
			}
		}
		for _, r := range ev.Results {
			for _, k := range model.RateKeys {
				if err := write("Inputs and Rates", []any{p.Input.Name, p.Input.SKU, ev.ID, ev.PresetVersion, ev.CalculationVersion, labels(locale, r.Channel), labels(locale, k), numeric(r.Rates[k]), labels(locale, r.Sources[k])}); err != nil {
					return err
				}
			}
			row := []any{p.Input.Name, p.Input.SKU, ev.ID, labels(locale, r.Channel), numeric(r.Price), numeric(r.Margin), numeric(r.MonthlyProfit), labels(locale, r.Verdict), numeric(r.ContributionPerOrder), numeric(r.SuccessfulSales), metric(r.BreakEvenOrders), metric(r.BreakEvenPrice), metric(r.MaxSupplierCost), metric(r.TargetPrice), metric(r.TargetCost), metric(r.MaxAdvertising), metric(r.BreakEvenROAS), metric(r.PriceBuffer)}
			for _, b := range r.Breakdown {
				row = append(row, numeric(b.Value))
			}
			if err := write("Channel Results", row); err != nil {
				return err
			}
		}
		for _, c := range p.Competitors {
			if err := write("Competitors", []any{p.Input.Name, p.Input.SKU, c.Platform, c.Shop, numeric(c.Price), numeric(c.Sales), numeric(c.Rating), c.URL}); err != nil {
				return err
			}
		}
		for _, im := range p.Images {
			if err := write("Images", []any{p.Input.Name, p.Input.SKU, base + im.URL, base + im.Thumbnail}); err != nil {
				return err
			}
			for col, url := range map[string]string{"C": base + im.URL, "D": base + im.Thumbnail} {
				_ = f.SetCellHyperLink("Images", col+strconv.Itoa(rows["Images"]), url, "External")
			}
		}
	}
	style, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"183C32"}}})
	for _, sheet := range f.GetSheetList() {
		_ = f.SetCellStyle(sheet, "A1", "AF1", style)
		_ = f.SetColWidth(sheet, "A", "AF", 24)
		_ = f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2"})
	}
	_, err := f.WriteTo(w)
	return err
}
func UploadPath(dir, id string) string { return filepath.Join(dir, "uploads", id+".xlsx") }
func DecimalCompare(a, b string) int {
	x, _ := new(big.Rat).SetString(a)
	y, _ := new(big.Rat).SetString(b)
	if x == nil {
		x = new(big.Rat)
	}
	if y == nil {
		y = new(big.Rat)
	}
	return x.Cmp(y)
}
