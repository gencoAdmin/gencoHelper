package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"genco/finance/internal/finance"
	"genco/finance/internal/model"
	"genco/finance/internal/store"
	"genco/finance/internal/workbook"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// BlobStore is the object-storage surface the API needs. It is satisfied by
// *gcs.Store in production and by a local-disk fake in tests.
type BlobStore interface {
	Upload(ctx context.Context, name string, r io.Reader, contentType string) error
	Open(ctx context.Context, name string) (io.ReadCloser, string, error)
	Download(ctx context.Context, name, path string, perm os.FileMode) error
	Created(ctx context.Context, name string) (time.Time, error)
	Delete(ctx context.Context, name string) error
}

type Server struct {
	Store    *store.Store
	Blobs    BlobStore
	Dir      string
	BaseURL  string
	Frontend string
	Log      *slog.Logger
}
type APIError struct {
	Status  int
	Code    string
	Message string
	Fields  map[string]string
}

func (e *APIError) Error() string { return e.Message }
func bad(msg string) error        { return &APIError{Status: 400, Code: "invalid_request", Message: msg} }
func conflict(msg string) error   { return &APIError{Status: 409, Code: "conflict", Message: msg} }
func versionConflict(msg string) error {
	return &APIError{Status: 409, Code: "version_conflict", Message: msg}
}
func duplicateSKU(msg string) error {
	return &APIError{Status: 409, Code: "duplicate_sku", Message: msg, Fields: map[string]string{"sku": msg}}
}
func missing() error { return &APIError{Status: 404, Code: "not_found", Message: "Record not found"} }
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, err error) {
	var a *APIError
	var v finance.Validation
	if errors.As(err, &a) {
		respond(w, a.Status, map[string]any{"code": a.Code, "message": a.Message, "fields": a.Fields})
		return
	}
	if errors.As(err, &v) {
		respond(w, 422, map[string]any{"code": "validation", "message": v.Error(), "fields": v})
		return
	}
	respond(w, 500, map[string]string{"code": "internal", "message": "The request could not be completed"})
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return bad("Invalid JSON: " + err.Error())
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return bad("Request must contain one JSON object")
	}
	return nil
}
func (s *Server) endpoint(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			if _, ok := err.(*APIError); !ok {
				s.Log.Error("request failed", "request_id", w.Header().Get("X-Request-ID"), "error", err)
			}
			fail(w, err)
		}
	}
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health/live", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /api/v1/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Store.Ready(r.Context()); err != nil {
			respond(w, 503, map[string]string{"status": "unavailable"})
			return
		}
		respond(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/v1/calculate", s.endpoint(s.calculate))
	mux.HandleFunc("GET /api/v1/settings", s.endpoint(s.settings))
	mux.HandleFunc("PUT /api/v1/settings", s.endpoint(s.settings))
	mux.HandleFunc("GET /api/v1/settings/versions", s.endpoint(s.settingVersions))
	mux.HandleFunc("POST /api/v1/categories", s.endpoint(s.addCategory))
	mux.HandleFunc("GET /api/v1/products", s.endpoint(s.list))
	mux.HandleFunc("POST /api/v1/products", s.endpoint(s.save))
	mux.HandleFunc("GET /api/v1/products/{id}", s.endpoint(s.product))
	mux.HandleFunc("PUT /api/v1/products/{id}", s.endpoint(s.save))
	mux.HandleFunc("GET /api/v1/products/{id}/revisions", s.endpoint(s.history))
	mux.HandleFunc("POST /api/v1/products/{id}/{action}", s.endpoint(s.action))
	mux.HandleFunc("DELETE /api/v1/products/{id}/images/{image}", s.endpoint(s.deleteImage))
	mux.HandleFunc("GET /api/v1/images/{image}", s.endpoint(s.serveImage))
	mux.HandleFunc("GET /api/v1/imports/template", s.endpoint(s.template))
	mux.HandleFunc("POST /api/v1/imports/upload", s.endpoint(s.uploadWorkbook))
	mux.HandleFunc("POST /api/v1/imports/preview", s.endpoint(s.preview))
	mux.HandleFunc("POST /api/v1/imports/commit", s.endpoint(s.commit))
	mux.HandleFunc("POST /api/v1/exports", s.endpoint(s.export))
	mux.HandleFunc("GET /api/v1/jobs/{id}", s.endpoint(s.job))
	mux.HandleFunc("GET /api/v1/jobs/{id}/download", s.endpoint(s.download))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { fail(w, missing()) })
	fs := http.FileServer(http.Dir(s.Frontend))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		if _, err := os.Stat(filepath.Join(s.Frontend, filepath.Clean("/"+r.URL.Path))); err != nil {
			http.ServeFile(w, r, filepath.Join(s.Frontend, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		w.Header().Set("X-Request-ID", store.ID())
		w.Header().Set("X-Content-Type-Options", "nosniff")
		defer func() {
			if e := recover(); e != nil {
				s.Log.Error("panic", "error", e)
				fail(w, errors.New("panic"))
			}
			s.Log.Info("request", "method", r.Method, "path", r.URL.Path, "request_id", w.Header().Get("X-Request-ID"), "duration_ms", time.Since(started).Milliseconds())
		}()
		mux.ServeHTTP(w, r)
	})
}
func current(st model.State) model.Settings { return st.Settings[len(st.Settings)-1] }
func (s *Server) calculate(w http.ResponseWriter, r *http.Request) error {
	var in model.Input
	if err := decode(w, r, &in); err != nil {
		return err
	}
	cur := current(s.Store.Read())
	if e := finance.Validate(in, false, cur.Categories); len(e) > 0 {
		return e
	}
	respond(w, 200, finance.Evaluate(in, cur))
	return nil
}
func (s *Server) settings(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "GET" {
		respond(w, 200, current(s.Store.Read()))
		return nil
	}
	var input model.Settings
	if err := decode(w, r, &input); err != nil {
		return err
	}
	input.Normalize()
	if e := finance.ValidateSettings(input); len(e) > 0 {
		return e
	}
	err := s.Store.Change(r.Context(), func(st *model.State) error {
		if current(*st).Version != input.Version {
			return versionConflict("Settings have changed. Reload before saving.")
		}
		input.Version++
		input.UpdatedAt = time.Now().UTC()
		st.Settings = append(st.Settings, input)
		return nil
	})
	if err != nil {
		return err
	}
	respond(w, 200, input)
	return nil
}

// addCategory appends a new category to the shared settings and saves it as a
// new preset version, so the category list is durable and shared across the
// workspace. Existing evaluations keep their frozen preset version.
func (s *Server) addCategory(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Name string `json:"name"`
	}
	if err := decode(w, r, &req); err != nil {
		return err
	}
	name := strings.TrimSpace(req.Name)
	if !finance.ValidCategory(name) {
		return &APIError{Status: 422, Code: "validation", Message: "Enter a valid category name", Fields: map[string]string{"name": "Use up to 24 letters, numbers, spaces, or & ' . / -"}}
	}
	var out model.Settings
	err := s.Store.Change(r.Context(), func(st *model.State) error {
		cur := current(*st)
		for _, c := range cur.Categories {
			if strings.EqualFold(c, name) {
				return conflict("This category already exists.")
			}
		}
		next := cur
		next.Categories = append(append([]string{}, cur.Categories...), name)
		next.CategoryReturns = map[string]string{}
		for k, v := range cur.CategoryReturns {
			next.CategoryReturns[k] = v
		}
		next.CategoryReturns[name] = ""
		next.Version++
		next.UpdatedAt = time.Now().UTC()
		st.Settings = append(st.Settings, next)
		out = next
		return nil
	})
	if err != nil {
		return err
	}
	respond(w, 201, out)
	return nil
}
func (s *Server) settingVersions(w http.ResponseWriter, r *http.Request) error {
	respond(w, 200, s.Store.Read().Settings)
	return nil
}
func summary(p model.Product) map[string]any {
	ev := p.Latest()
	var best model.Result
	for _, r := range ev.Results {
		if r.Channel == ev.Recommended {
			best = r
		}
	}
	return map[string]any{"id": p.ID, "version": p.Version, "archived": p.Archived, "input": p.Input, "createdAt": p.CreatedAt, "updatedAt": p.UpdatedAt, "evaluation": ev, "headline": best, "imageCount": len(p.Images), "competitorCount": len(p.Competitors)}
}
func filtered(st model.State, q url.Values) []model.Product {
	out := []model.Product{}
	search := strings.ToLower(strings.TrimSpace(q.Get("search")))
	for _, p := range st.Products {
		if q.Get("archived") != "all" && p.Archived != (q.Get("archived") == "true") {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(p.Input.Name+" "+p.Input.SKU), search) {
			continue
		}
		if q.Get("category") != "" && q.Get("category") != p.Input.Category {
			continue
		}
		ev := p.Latest()
		if q.Get("channel") != "" && q.Get("channel") != ev.Recommended {
			continue
		}
		best := model.Result{}
		for _, res := range ev.Results {
			if res.Channel == ev.Recommended {
				best = res
			}
		}
		if q.Get("verdict") != "" && q.Get("verdict") != best.Verdict {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		get := func(p model.Product, field string) string {
			ev := p.Latest()
			for _, r := range ev.Results {
				if r.Channel == ev.Recommended {
					if field == "margin" {
						return r.Margin
					}
					return r.MonthlyProfit
				}
			}
			return "0"
		}
		var cmp int
		switch q.Get("sort") {
		case "margin", "profit":
			cmp = workbook.DecimalCompare(get(a, q.Get("sort")), get(b, q.Get("sort")))
		case "created":
			cmp = a.CreatedAt.Compare(b.CreatedAt)
		default:
			cmp = a.UpdatedAt.Compare(b.UpdatedAt)
		}
		if cmp == 0 {
			cmp = strings.Compare(a.ID, b.ID)
		}
		if q.Get("direction") == "asc" {
			return cmp < 0
		}
		return cmp > 0
	})
	return out
}
func (s *Server) list(w http.ResponseWriter, r *http.Request) error {
	items := []any{}
	for _, p := range filtered(s.Store.Read(), r.URL.Query()) {
		items = append(items, summary(p))
	}
	respond(w, 200, map[string]any{"items": items, "total": len(items)})
	return nil
}
func (s *Server) product(w http.ResponseWriter, r *http.Request) error {
	p, ok := s.Store.Read().Products[r.PathValue("id")]
	if !ok {
		return missing()
	}
	respond(w, 200, p)
	return nil
}
func (s *Server) history(w http.ResponseWriter, r *http.Request) error {
	p, ok := s.Store.Read().Products[r.PathValue("id")]
	if !ok {
		return missing()
	}
	respond(w, 200, p.Revisions)
	return nil
}
func validateCompetitors(cs []model.Competitor) error {
	if len(cs) > 8 {
		return bad("Maximum eight competitors")
	}
	e := finance.Validation{}
	for i, c := range cs {
		prefix := fmt.Sprintf("competitors.%d.", i)
		if strings.TrimSpace(c.Shop) == "" || len(c.Shop) > 200 {
			e[prefix+"shop"] = "Shop name is required, maximum 200 bytes"
		}
		if c.Platform == "" || len(c.Platform) > 100 {
			e[prefix+"platform"] = "Platform is required"
		}
		for k, v := range map[string]string{"price": c.Price, "sales": c.Sales, "rating": c.Rating} {
			max := "1000000000000"
			if k == "rating" {
				max = "5"
			}
			in := model.Input{Name: "test", Category: "general", Cost: "0", Price: "1", Orders: "1", Fixed: "0", Packaging: "0", TargetMargin: "0"}
			in.Cost = v
			if len(finance.Validate(in, true, []string{"general"})) > 0 || workbook.DecimalCompare(v, max) > 0 {
				e[prefix+k] = "Enter a valid non-negative decimal"
			}
		}
		if c.URL != "" {
			u, err := url.Parse(c.URL)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				e[prefix+"url"] = "Use an http or https URL"
			}
		}
	}
	if len(e) > 0 {
		return e
	}
	return nil
}
func create(st *model.State, in model.Input, cs []model.Competitor) (model.Product, error) {
	if in.Overrides == nil {
		in.Overrides = map[string]model.Rates{}
	}
	in.Name = strings.TrimSpace(in.Name)
	in.SKU = strings.TrimSpace(in.SKU)
	for _, p := range st.Products {
		if in.SKU != "" && p.Input.SKU == in.SKU {
			return model.Product{}, duplicateSKU("This SKU already exists, including archived products.")
		}
	}
	now := time.Now().UTC()
	ev := finance.Evaluate(in, current(*st))
	ev.ID = store.ID()
	for i := range cs {
		cs[i].ID = store.ID()
	}
	p := model.Product{ID: store.ID(), Version: 1, Input: in, Competitors: cs, Images: []model.Image{}, Revisions: []model.Evaluation{ev}, CreatedAt: now, UpdatedAt: now}
	st.Products[p.ID] = p
	return p, nil
}

type SaveRequest struct {
	Version     int                `json:"version"`
	Input       model.Input        `json:"input"`
	Competitors []model.Competitor `json:"competitors"`
}

func (s *Server) save(w http.ResponseWriter, r *http.Request) error {
	var req SaveRequest
	if err := decode(w, r, &req); err != nil {
		return err
	}
	if req.Input.Overrides == nil {
		req.Input.Overrides = map[string]model.Rates{}
	}
	req.Input.Name = strings.TrimSpace(req.Input.Name)
	req.Input.SKU = strings.TrimSpace(req.Input.SKU)
	if e := finance.Validate(req.Input, true, current(s.Store.Read()).Categories); len(e) > 0 {
		return e
	}
	if err := validateCompetitors(req.Competitors); err != nil {
		return err
	}
	var p model.Product
	err := s.Store.Change(r.Context(), func(st *model.State) error {
		if r.Method == "POST" {
			var err error
			p, err = create(st, req.Input, req.Competitors)
			return err
		}
		old, ok := st.Products[r.PathValue("id")]
		if !ok {
			return missing()
		}
		if old.Version != req.Version {
			return versionConflict("This product has changed. Reload before saving.")
		}
		for id, other := range st.Products {
			if id != old.ID && req.Input.SKU != "" && other.Input.SKU == req.Input.SKU {
				return duplicateSKU("This SKU already exists.")
			}
		}
		p = old
		before, _ := json.Marshal(old.Input)
		after, _ := json.Marshal(req.Input)
		p.Input = req.Input
		for i := range req.Competitors {
			req.Competitors[i].ID = store.ID()
		}
		p.Competitors = req.Competitors
		p.Version++
		p.UpdatedAt = time.Now().UTC()
		if string(before) != string(after) {
			ev := finance.Evaluate(req.Input, current(*st))
			ev.ID = store.ID()
			p.Revisions = append(p.Revisions, ev)
		}
		st.Products[p.ID] = p
		return nil
	})
	if err != nil {
		return err
	}
	code := 200
	if r.Method == "POST" {
		code = 201
	}
	respond(w, code, p)
	return nil
}
func (s *Server) action(w http.ResponseWriter, r *http.Request) error {
	if r.PathValue("action") == "images" {
		return s.uploadImage(w, r)
	}
	var req struct {
		Version int `json:"version"`
	}
	if err := decode(w, r, &req); err != nil {
		return err
	}
	var p model.Product
	err := s.Store.Change(r.Context(), func(st *model.State) error {
		old, ok := st.Products[r.PathValue("id")]
		if !ok {
			return missing()
		}
		if old.Version != req.Version {
			return versionConflict("This product has changed. Reload before continuing.")
		}
		p = old
		switch r.PathValue("action") {
		case "duplicate":
			in := old.Input
			in.SKU = ""
			in.Name += " (copy)"
			if len([]rune(in.Name)) > 200 {
				in.Name = string([]rune(in.Name)[:200])
			}
			var err error
			p, err = create(st, in, append([]model.Competitor{}, old.Competitors...))
			return err
		case "archive":
			p.Archived = true
		case "restore":
			p.Archived = false
		case "recalculate":
			ev := finance.Evaluate(old.Input, current(*st))
			ev.ID = store.ID()
			p.Revisions = append(p.Revisions, ev)
		default:
			return missing()
		}
		p.Version++
		p.UpdatedAt = time.Now().UTC()
		st.Products[p.ID] = p
		return nil
	})
	if err != nil {
		return err
	}
	respond(w, 200, p)
	return nil
}
func (s *Server) template(w http.ResponseWriter, r *http.Request) error {
	b, err := workbook.Template(r.URL.Query().Get("locale"))
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="genco-import-template.xlsx"`)
	_, err = w.Write(b)
	return err
}
func (s *Server) uploadWorkbook(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 11<<20)
	if err := r.ParseMultipartForm(11 << 20); err != nil {
		return bad("Upload limit is 10 MB")
	}
	defer r.MultipartForm.RemoveAll()
	f, h, err := r.FormFile("file")
	if err != nil {
		return bad("Choose a workbook")
	}
	defer f.Close()
	if h.Size > 10<<20 || strings.ToLower(filepath.Ext(h.Filename)) != ".xlsx" {
		return bad("Choose an .xlsx workbook up to 10 MB")
	}
	id := store.ID()
	// The Excel reader needs a real file, so stage the upload in a temp file,
	// inspect it, then push it to GCS for the later preview step.
	tmp, err := os.CreateTemp("", "genco-upload-*.xlsx")
	if err != nil {
		return err
	}
	path := tmp.Name()
	defer os.Remove(path)
	_, err = io.Copy(tmp, f)
	ce := tmp.Close()
	if err != nil || ce != nil {
		return errors.Join(err, ce)
	}
	u, err := workbook.Inspect(path, id)
	if err != nil {
		return bad(err.Error())
	}
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()
	if err = s.Blobs.Upload(r.Context(), workbook.UploadKey(id), src, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"); err != nil {
		return err
	}
	respond(w, 201, u)
	return nil
}
func validID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, c := range id {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}
func (s *Server) preview(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		UploadID string         `json:"uploadId"`
		Sheet    string         `json:"sheet"`
		Mapping  map[string]int `json:"mapping"`
	}
	if err := decode(w, r, &req); err != nil {
		return err
	}
	if !validID(req.UploadID) {
		return bad("Invalid upload ID")
	}
	key := workbook.UploadKey(req.UploadID)
	created, err := s.Blobs.Created(r.Context(), key)
	if err != nil || time.Since(created) > 24*time.Hour {
		return bad("Upload expired. Upload the file again.")
	}
	tmp, err := os.CreateTemp("", "genco-preview-*.xlsx")
	if err != nil {
		return err
	}
	path := tmp.Name()
	_ = tmp.Close()
	_ = os.Remove(path)
	if err = s.Blobs.Download(r.Context(), key, path, 0600); err != nil {
		return bad("Upload expired. Upload the file again.")
	}
	defer os.Remove(path)
	st := s.Store.Read()
	rows, err := workbook.Preview(path, req.Sheet, req.Mapping, current(st), st.Products)
	if err != nil {
		return bad(err.Error())
	}
	for i := range rows {
		if len(rows[i].Errors) == 0 {
			ev := finance.Evaluate(rows[i].Input, current(st))
			rows[i].Evaluation = &ev
		}
	}
	p := model.Preview{ID: store.ID(), Rows: rows, CreatedAt: time.Now().UTC()}
	if err = s.Store.Change(r.Context(), func(st *model.State) error { st.Previews[p.ID] = p; return nil }); err != nil {
		return err
	}
	respond(w, 200, p)
	return nil
}
func (s *Server) commit(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		PreviewID string `json:"previewId"`
		Rows      []int  `json:"rows"`
	}
	if err := decode(w, r, &req); err != nil {
		return err
	}
	var job model.StoredJob
	err := s.Store.Change(r.Context(), func(st *model.State) error {
		p, ok := st.Previews[req.PreviewID]
		if !ok || time.Since(p.CreatedAt) > 24*time.Hour {
			return bad("Preview expired. Preview again.")
		}
		if p.JobID != "" {
			job = st.Jobs[p.JobID]
			return nil
		}
		selected := map[int]bool{}
		for _, i := range req.Rows {
			selected[i] = true
		}
		rows := []model.ImportRow{}
		for _, row := range p.Rows {
			if selected[row.Row] {
				if len(row.Errors) > 0 {
					return bad("Only valid rows may be selected")
				}
				rows = append(rows, row)
				delete(selected, row.Row)
			}
		}
		if len(selected) > 0 {
			return bad("Unknown source row")
		}
		if len(rows) == 0 {
			return bad("Select at least one valid row")
		}
		job = model.StoredJob{Job: model.Job{ID: store.ID(), Kind: "import", Status: "pending", Total: len(rows), Errors: []string{}, CreatedAt: time.Now().UTC(), LeaseUntil: time.Now().UTC()}, Rows: rows}
		st.Jobs[job.ID] = job
		p.JobID = job.ID
		st.Previews[p.ID] = p
		return nil
	})
	if err != nil {
		return err
	}
	respond(w, 202, job.Job)
	return nil
}
func (s *Server) export(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		IDs     []string          `json:"ids"`
		Filters map[string]string `json:"filters"`
		Format  string            `json:"format"`
		Locale  string            `json:"locale"`
	}
	if err := decode(w, r, &req); err != nil {
		return err
	}
	if req.Format != "xlsx" && req.Format != "csv" {
		return bad("Choose Excel or CSV")
	}
	if req.Locale != "en" && req.Locale != "id" && req.Locale != "zh" {
		return bad("Choose a supported language")
	}
	var job model.StoredJob
	err := s.Store.Change(r.Context(), func(st *model.State) error {
		products := []model.Product{}
		if len(req.IDs) > 0 {
			seen := map[string]bool{}
			for _, id := range req.IDs {
				if seen[id] {
					continue
				}
				p, ok := st.Products[id]
				if !ok {
					return missing()
				}
				seen[id] = true
				products = append(products, p)
			}
		} else {
			q := url.Values{}
			for k, v := range req.Filters {
				q.Set(k, v)
			}
			products = filtered(*st, q)
		}
		if len(products) == 0 {
			return bad("No products to export")
		}
		job = model.StoredJob{Job: model.Job{ID: store.ID(), Kind: "export", Status: "pending", Total: len(products), Format: req.Format, Locale: req.Locale, Errors: []string{}, CreatedAt: time.Now().UTC(), LeaseUntil: time.Now().UTC()}, Snapshots: products}
		st.Jobs[job.ID] = job
		return nil
	})
	if err != nil {
		return err
	}
	respond(w, 202, job.Job)
	return nil
}
func (s *Server) job(w http.ResponseWriter, r *http.Request) error {
	j, ok := s.Store.Read().Jobs[r.PathValue("id")]
	if !ok {
		return missing()
	}
	respond(w, 200, j.Job)
	return nil
}
func (s *Server) download(w http.ResponseWriter, r *http.Request) error {
	j, ok := s.Store.Read().Jobs[r.PathValue("id")]
	if !ok {
		return missing()
	}
	if j.Status != "completed" || j.Artifact == "" || time.Since(j.CreatedAt) > 24*time.Hour {
		return bad("Report is not available or has expired")
	}
	path := filepath.Join(s.Dir, "reports", j.Artifact)
	if _, err := os.Stat(path); err != nil {
		return missing()
	}
	w.Header().Set("Content-Disposition", `attachment; filename="genco-evaluations.`+j.Format+`"`)
	if j.Format == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	}
	http.ServeFile(w, r, path)
	return nil
}
func version(r *http.Request) (int, error) {
	n, err := strconv.Atoi(r.FormValue("version"))
	if err != nil {
		return 0, bad("Product version is required")
	}
	return n, nil
}
func (s *Server) Worker(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	s.cleanup(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			st := s.Store.Read()
			for _, job := range st.Jobs {
				if job.Status == "pending" || (job.Status == "running" && time.Now().After(job.LeaseUntil)) {
					s.runJob(ctx, job.ID)
				}
			}
			s.cleanup(ctx)
		}
	}
}
func (s *Server) runJob(ctx context.Context, id string) {
	started := time.Now()
	var job model.StoredJob
	err := s.Store.Change(ctx, func(st *model.State) error {
		j := st.Jobs[id]
		if j.Status != "pending" && !(j.Status == "running" && time.Now().After(j.LeaseUntil)) {
			return errors.New("job already leased")
		}
		j.Status = "running"
		j.LeaseUntil = time.Now().UTC().Add(2 * time.Minute)
		st.Jobs[id] = j
		job = j
		return nil
	})
	if err != nil {
		return
	}
	if job.Kind == "import" {
		for _, row := range job.Rows[job.Processed:] {
			if ctx.Err() != nil {
				return
			}
			err = s.Store.Change(ctx, func(st *model.State) error {
				j := st.Jobs[id]
				if e := finance.Validate(row.Input, true, current(*st).Categories); len(e) > 0 {
					j.Failed++
					j.Errors = append(j.Errors, fmt.Sprintf("Row %d: invalid inputs", row.Row))
				} else if p, err := create(st, row.Input, nil); err != nil {
					j.Skipped++
					j.Errors = append(j.Errors, fmt.Sprintf("Row %d: %s", row.Row, err.Error()))
				} else {
					// Keep the analysis reviewed in the preview, even if shared
					// settings changed before saving. Older jobs have no snapshot.
					if row.Evaluation != nil {
						ev := *row.Evaluation
						ev.ID = p.Latest().ID
						ev.CreatedAt = p.Latest().CreatedAt
						p.Revisions = []model.Evaluation{ev}
						st.Products[p.ID] = p
					}
					j.Created++
				}
				j.Processed++
				j.LeaseUntil = time.Now().UTC().Add(2 * time.Minute)
				st.Jobs[id] = j
				return nil
			})
			if err != nil {
				break
			}
		}
	} else {
		dir := filepath.Join(s.Dir, "reports")
		err = os.MkdirAll(dir, 0750)
		if err == nil {
			path := filepath.Join(dir, id+"."+job.Format)
			var f *os.File
			f, err = os.OpenFile(path+".tmp", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
			if err == nil {
				err = workbook.Export(f, job.Snapshots, job.Format, job.Locale, s.BaseURL)
				closeErr := f.Close()
				err = errors.Join(err, closeErr)
				if err == nil {
					err = os.Rename(path+".tmp", path)
				}
				if err == nil {
					job.Artifact = id + "." + job.Format
				}
			}
		}
	}
	finishErr := s.Store.Change(ctx, func(st *model.State) error {
		j := st.Jobs[id]
		if err != nil {
			j.Status = "failed"
			j.Errors = append(j.Errors, "Job failed. Check server logs.")
			j.Failed++
		} else {
			j.Status = "completed"
			if j.Kind == "export" {
				j.Artifact = job.Artifact
				j.Processed = j.Total
			}
		}
		// Durable inputs are only needed until the job reaches a terminal state.
		j.Rows = nil
		j.Snapshots = nil
		st.Jobs[id] = j
		return nil
	})
	s.Log.Info("job finished", "id", id, "kind", job.Kind, "duration_ms", time.Since(started).Milliseconds(), "error", errors.Join(err, finishErr))
}
func (s *Server) cleanup(ctx context.Context) {
	st := s.Store.Read()
	expired := false
	for _, p := range st.Previews {
		if time.Since(p.CreatedAt) > 24*time.Hour {
			expired = true
		}
	}
	if expired {
		_ = s.Store.Change(ctx, func(st *model.State) error {
			for id, p := range st.Previews {
				if time.Since(p.CreatedAt) > 24*time.Hour {
					if j, ok := st.Jobs[p.JobID]; ok && (j.Status == "pending" || j.Status == "running") {
						continue
					}
					delete(st.Previews, id)
				}
			}
			return nil
		})
	}
	// Reports are server-generated artifacts still kept on local disk; staged
	// import workbooks now live in blob storage and are removed on commit (a
	// bucket lifecycle rule on the uploads/ prefix handles abandoned uploads).
	entries, _ := os.ReadDir(filepath.Join(s.Dir, "reports"))
	for _, e := range entries {
		info, err := e.Info()
		if err == nil && !e.IsDir() && time.Since(info.ModTime()) > 24*time.Hour {
			_ = os.Remove(filepath.Join(s.Dir, "reports", e.Name()))
		}
	}
}
