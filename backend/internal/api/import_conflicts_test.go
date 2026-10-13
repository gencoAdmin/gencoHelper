package api

import (
	"context"
	"genco/finance/internal/model"
	"genco/finance/internal/store"
	"testing"
	"time"
)

func TestImportRechecksSKUAndDoesNotRepeat(t *testing.T) {
	s, h := testServer(t)
	input := in()
	jobID := store.ID()
	if err := s.Store.Change(context.Background(), func(st *model.State) error {
		st.Jobs[jobID] = model.StoredJob{Job: model.Job{ID: jobID, Kind: "import", Status: "pending", Total: 1, CreatedAt: time.Now().UTC(), LeaseUntil: time.Now().UTC(), Errors: []string{}}, Rows: []model.ImportRow{{Row: 7, Input: input}}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// A manual save can claim the SKU after preview but before the worker commits.
	request(t, h, "POST", "/products", SaveRequest{Input: input}, 201)
	s.runJob(context.Background(), jobID)
	j := s.Store.Read().Jobs[jobID]
	if j.Status != "completed" || j.Created != 0 || j.Skipped != 1 || j.Processed != 1 {
		t.Fatalf("duplicate recheck: %+v", j)
	}
	s.runJob(context.Background(), jobID)
	if len(s.Store.Read().Products) != 1 || s.Store.Read().Jobs[jobID].Skipped != 1 {
		t.Fatal("completed job repeated")
	}
}

func TestOverrideSurvivesRecalculation(t *testing.T) {
	s, h := testServer(t)
	input := in()
	input.Category = "fashion"
	input.Overrides = map[string]model.Rates{"shelf": {"returns": "1", "advertising": "2"}}
	p := decodeAs[model.Product](t, request(t, h, "POST", "/products", SaveRequest{Input: input}, 201))
	settings := current(s.Store.Read())
	settings.CategoryReturns["fashion"] = "30"
	settings.Channels["shelf"]["advertising"] = "20"
	request(t, h, "PUT", "/settings", settings, 200)
	p = decodeAs[model.Product](t, request(t, h, "POST", "/products/"+p.ID+"/recalculate", map[string]int{"version": p.Version}, 200))
	r := p.Latest().Results[0]
	if r.Rates["returns"] != "1" || r.Rates["advertising"] != "2" || r.Sources["returns"] != "product" || p.Latest().PresetVersion != 2 {
		t.Fatalf("override lost: %+v", r)
	}
}
