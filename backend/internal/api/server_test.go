package api

import (
	"bytes"
	"context"
	"encoding/json"
	"genco/finance/internal/model"
	"genco/finance/internal/store"
	"genco/finance/internal/workbook"
	"github.com/xuri/excelize/v2"
	"image"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func testServer(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(context.Background(), dir, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s := &Server{Store: db, Dir: dir, BaseURL: "https://finance.example.com", Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	return s, s.Handler()
}
func request(t *testing.T, h http.Handler, method, path string, body any, want int) []byte {
	t.Helper()
	var b bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&b).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, "/api/v1"+path, &b)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != want {
		t.Fatalf("%s %s: got %d, want %d: %s", method, path, w.Code, want, w.Body.String())
	}
	return w.Body.Bytes()
}
func in() model.Input {
	return model.Input{Name: "Linen 衬衫", SKU: "00123", Category: "general", Cost: "30000", Price: "50000", Orders: "1000", Fixed: "5300000", Packaging: "0", TargetMargin: "15"}
}
func decodeAs[T any](t *testing.T, b []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestSaveValidationCanBeCorrectedWithoutReload(t *testing.T) {
	s, h := testServer(t)
	input := in()
	input.Name = ""
	request(t, h, "POST", "/products", SaveRequest{Input: input}, 422)
	if len(s.Store.Read().Products) != 0 {
		t.Fatal("invalid save created a product")
	}
	input.Name = "Corrected product"
	p := decodeAs[model.Product](t, request(t, h, "POST", "/products", SaveRequest{Input: input}, 201))
	for _, field := range []string{"name", "competitors"} {
		t.Run(field, func(t *testing.T) {
			req := SaveRequest{Version: p.Version, Input: p.Input}
			if field == "name" {
				req.Input.Name = ""
			} else {
				req.Competitors = []model.Competitor{{Platform: "Shopee", Price: "50000", Sales: "10", Rating: "4"}}
			}
			request(t, h, "PUT", "/products/"+p.ID, req, 422)
			unchanged := decodeAs[model.Product](t, request(t, h, "GET", "/products/"+p.ID, nil, 200))
			if !reflect.DeepEqual(p, unchanged) {
				t.Fatal("validation failure changed the saved product")
			}
			req.Input.Name = "Fixed " + field
			if len(req.Competitors) > 0 {
				req.Competitors[0].Shop = "Corrected shop"
			}
			p = decodeAs[model.Product](t, request(t, h, "PUT", "/products/"+p.ID, req, 200))
			if p.Version != req.Version+1 {
				t.Fatal("corrected save did not advance exactly one version")
			}
		})
	}
}

func TestDuplicateSKUAndStaleVersionHaveDistinctErrors(t *testing.T) {
	_, h := testServer(t)
	type errorResponse struct {
		Code   string            `json:"code"`
		Fields map[string]string `json:"fields"`
	}
	req := SaveRequest{Input: in()}
	p := decodeAs[model.Product](t, request(t, h, "POST", "/products", req, 201))
	duplicate := decodeAs[errorResponse](t, request(t, h, "POST", "/products", req, 409))
	if duplicate.Code != "duplicate_sku" || duplicate.Fields["sku"] == "" {
		t.Fatalf("duplicate SKU must identify the field: %+v", duplicate)
	}
	req.Input.SKU = "SECOND"
	second := decodeAs[model.Product](t, request(t, h, "POST", "/products", req, 201))
	req.Version = second.Version
	req.Input.SKU = p.Input.SKU
	duplicate = decodeAs[errorResponse](t, request(t, h, "PUT", "/products/"+second.ID, req, 409))
	if duplicate.Code != "duplicate_sku" || duplicate.Fields["sku"] == "" {
		t.Fatalf("duplicate SKU on edit must identify the field: %+v", duplicate)
	}
	req.Input.SKU = "CORRECTED"
	updated := decodeAs[model.Product](t, request(t, h, "PUT", "/products/"+second.ID, req, 200))
	stale := decodeAs[errorResponse](t, request(t, h, "PUT", "/products/"+second.ID, req, 409))
	if stale.Code != "version_conflict" || len(stale.Fields) != 0 {
		t.Fatalf("stale version must require reloading: %+v", stale)
	}
	unchanged := decodeAs[model.Product](t, request(t, h, "GET", "/products/"+second.ID, nil, 200))
	if !reflect.DeepEqual(updated, unchanged) {
		t.Fatal("stale save overwrote the latest product")
	}
}

func TestProductsRevisionsConcurrencyAndPersistence(t *testing.T) {
	s, h := testServer(t)
	req := SaveRequest{Input: in()}
	p := decodeAs[model.Product](t, request(t, h, "POST", "/products", req, 201))
	saved := p.Latest()
	request(t, h, "POST", "/products", req, 409)
	settings := current(s.Store.Read())
	settings.Channels["shelf"]["commission"] = "1"
	settings = decodeAs[model.Settings](t, request(t, h, "PUT", "/settings", settings, 200))
	settings.Version--
	request(t, h, "PUT", "/settings", settings, 409)
	old := decodeAs[model.Product](t, request(t, h, "GET", "/products/"+p.ID, nil, 200))
	if old.Latest().PresetVersion != 1 || old.Latest().Results[0].MonthlyProfit != "2700000" {
		t.Fatal("presets changed saved evaluations")
	}
	p = decodeAs[model.Product](t, request(t, h, "POST", "/products/"+p.ID+"/recalculate", map[string]int{"version": p.Version}, 200))
	if len(p.Revisions) != 2 || p.Latest().PresetVersion != 2 || p.Revisions[0].ID != saved.ID {
		t.Fatal("revisions")
	}
	request(t, h, "PUT", "/products/"+p.ID, SaveRequest{Input: in(), Version: old.Version}, 409)
	p = decodeAs[model.Product](t, request(t, h, "POST", "/products/"+p.ID+"/archive", map[string]int{"version": p.Version}, 200))
	request(t, h, "POST", "/products", req, 409)
	copy := decodeAs[model.Product](t, request(t, h, "POST", "/products/"+p.ID+"/duplicate", map[string]int{"version": p.Version}, 200))
	if copy.Input.SKU != "" || copy.Archived {
		t.Fatal("duplicate identity")
	}
	restored, err := store.Open(context.Background(), s.Dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if len(restored.Read().Products) != 2 || len(restored.Read().Products[p.ID].Revisions) != 2 {
		t.Fatal("restart lost data")
	}
}
func TestImportIdempotenceExportFrozenAndRestartRecovery(t *testing.T) {
	s, h := testServer(t)
	id := store.ID()
	path := workbook.UploadPath(s.Dir, id)
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatal(err)
	}
	f := excelize.NewFile()
	headers := []any{"Product name", "SKU", "Product cost", "Selling price", "Monthly orders"}
	_ = f.SetSheetRow("Sheet1", "A1", &headers)
	rows := [][]any{{"Imported 衬衫", "00007", 30000, 50000, 1000}, {"Duplicate", "00007", 30000, 50000, 1000}, {"Formula", "NEW", 30000, 50000, 1000}, {"Invalid", "BAD", -1, 50000, 1000}, {"No SKU", "", 30000, 50000, 1000}}
	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		_ = f.SetSheetRow("Sheet1", cell, &row)
	}
	_ = f.SetCellFormula("Sheet1", "C4", "1+1")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	f.Close()
	mapping := map[string]int{"name": 0, "sku": 1, "cost": 2, "price": 3, "orders": 4}
	p := decodeAs[model.Preview](t, request(t, h, "POST", "/imports/preview", map[string]any{"uploadId": id, "sheet": "Sheet1", "mapping": mapping}, 200))
	if len(s.Store.Read().Products) != 0 || len(p.Rows) != 5 || p.Rows[0].Input.SKU != "00007" || len(p.Rows[1].Errors) == 0 || len(p.Rows[2].Errors) == 0 || len(p.Rows[3].Errors) == 0 {
		t.Fatalf("preview %+v", p)
	}
	for _, row := range p.Rows {
		if len(row.Errors) > 0 {
			if row.Evaluation != nil {
				t.Fatal("invalid rows must not include analysis")
			}
			continue
		}
		if row.Evaluation == nil || len(row.Evaluation.Results) != len(model.Channels) {
			t.Fatal("valid import rows must include every channel's analysis")
		}
		calculated := decodeAs[model.Evaluation](t, request(t, h, "POST", "/calculate", row.Input, 200))
		if !reflect.DeepEqual(row.Evaluation.Results, calculated.Results) || row.Evaluation.Recommended != calculated.Recommended {
			t.Fatal("bulk research must use the same analysis as single-product research")
		}
	}
	// A shared preset edit between reviewing and saving must not change the
	// imported evaluation. The snapshot must also survive a restart.
	settings := s.Store.Read().Settings[0]
	settings.Channels["shelf"]["commission"] = "90"
	request(t, h, "PUT", "/settings", settings, 200)
	commit := map[string]any{"previewId": p.ID, "rows": []int{2, 6}}
	j := decodeAs[model.Job](t, request(t, h, "POST", "/imports/commit", commit, 202))
	j2 := decodeAs[model.Job](t, request(t, h, "POST", "/imports/commit", commit, 202))
	if j.ID != j2.ID {
		t.Fatal("confirmation not idempotent")
	}
	db, err := store.Open(context.Background(), s.Dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s.Store = db
	s.runJob(context.Background(), j.ID)
	st := s.Store.Read()
	if st.Jobs[j.ID].Created != 2 || len(st.Products) != 2 {
		t.Fatal("restart recovery failed")
	}
	var product model.Product
	for _, v := range st.Products {
		if v.Input.SKU == "00007" {
			product = v
		}
	}
	if product.Latest().Results[0].ContributionPerOrder != "8000" {
		t.Fatal("import defaults not applied")
	}
	if !reflect.DeepEqual(product.Latest().Results, p.Rows[0].Evaluation.Results) || product.Latest().PresetVersion != p.Rows[0].Evaluation.PresetVersion {
		t.Fatal("saved import changed the reviewed analysis after settings edit and restart")
	}
	exp := decodeAs[model.Job](t, request(t, h, "POST", "/exports", map[string]any{"ids": []string{product.ID}, "format": "xlsx", "locale": "zh"}, 202))
	latest := product.Latest().ID
	newIn := product.Input
	newIn.Price = "70000"
	request(t, h, "PUT", "/products/"+product.ID, SaveRequest{Version: product.Version, Input: newIn}, 200)
	s.runJob(context.Background(), exp.ID)
	artifact := s.Store.Read().Jobs[exp.ID].Artifact
	wb, err := excelize.OpenFile(filepath.Join(s.Dir, "reports", artifact))
	if err != nil {
		t.Fatal(err)
	}
	defer wb.Close()
	if len(wb.GetSheetList()) != 5 {
		t.Fatal("missing sheets")
	}
	rev, _ := wb.GetCellValue("Summary", "K2")
	price, _ := wb.GetCellValue("Summary", "E2")
	if rev != latest || price != "50000" {
		t.Fatal("export did not freeze revision")
	}
	typ, _ := wb.GetCellType("Summary", "E2")
	if typ != excelize.CellTypeNumber && typ != excelize.CellTypeUnset {
		t.Fatal("financial cells are not numeric")
	}
}
func TestSharedScreenshotsAndLimits(t *testing.T) {
	s, h := testServer(t)
	p := decodeAs[model.Product](t, request(t, h, "POST", "/products", SaveRequest{Input: in()}, 201))
	var img bytes.Buffer
	_ = png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 50, 40)))
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	_ = mw.WriteField("version", "1")
	part, _ := mw.CreateFormFile("file", "research.png")
	_, _ = part.Write(img.Bytes())
	mw.Close()
	req := httptest.NewRequest("POST", "/api/v1/products/"+p.ID+"/images", &b)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("upload %d: %s", w.Code, w.Body.String())
	}
	p = decodeAs[model.Product](t, w.Body.Bytes())
	if len(p.Images) != 1 {
		t.Fatal("image missing")
	}
	second := httptest.NewRecorder()
	h.ServeHTTP(second, httptest.NewRequest("GET", p.Images[0].URL, nil))
	if second.Code != 200 || second.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatal("normalized image not shared")
	}
	request(t, h, "DELETE", "/products/"+p.ID+"/images/"+p.Images[0].ID, map[string]int{"version": p.Version}, 200)
	request(t, h, "GET", strings.TrimPrefix(p.Images[0].URL, "/api/v1"), nil, 404)
	badInput := in()
	badInput.Orders = "0"
	request(t, h, "POST", "/calculate", badInput, 422)
	_ = s
}

func TestAddCategoryPersistsAndValidates(t *testing.T) {
	s, h := testServer(t)
	// A new category is appended, bumps the preset version, and gets a blank return entry.
	before := current(s.Store.Read())
	out := decodeAs[model.Settings](t, request(t, h, "POST", "/categories", map[string]string{"name": "Toys & Games"}, 201))
	if out.Version != before.Version+1 {
		t.Fatalf("version not bumped: %d -> %d", before.Version, out.Version)
	}
	if !out.HasCategory("Toys & Games") {
		t.Fatal("category not added")
	}
	if _, ok := out.CategoryReturns["Toys & Games"]; !ok {
		t.Fatal("category return entry missing")
	}
	// Case-insensitive duplicates are rejected.
	request(t, h, "POST", "/categories", map[string]string{"name": "toys & games"}, 409)
	// Invalid names are rejected before any write.
	request(t, h, "POST", "/categories", map[string]string{"name": "   "}, 422)
	request(t, h, "POST", "/categories", map[string]string{"name": strings.Repeat("x", 25)}, 422)
	// A product can now use the new category.
	prod := in()
	prod.SKU = "TOY-1"
	prod.Category = "Toys & Games"
	request(t, h, "POST", "/products", SaveRequest{Input: prod}, 201)
	// An unknown category is still rejected.
	bad := in()
	bad.SKU = "BAD-1"
	bad.Category = "nonexistent"
	request(t, h, "POST", "/products", SaveRequest{Input: bad}, 422)
}

func TestSettingsNormalizeBackfillsCategories(t *testing.T) {
	// A settings record saved before Categories existed is backfilled from CategoryReturns.
	legacy := model.Settings{CategoryReturns: map[string]string{"general": "", "fashion": "10"}}
	legacy.Normalize()
	if !legacy.HasCategory("general") || !legacy.HasCategory("fashion") {
		t.Fatalf("categories not backfilled: %+v", legacy.Categories)
	}
	// A category without a return entry gets a blank one.
	s := model.Settings{Categories: []string{"general", "custom"}, CategoryReturns: map[string]string{"general": ""}}
	s.Normalize()
	if v, ok := s.CategoryReturns["custom"]; !ok || v != "" {
		t.Fatal("missing return entry not backfilled")
	}
}
