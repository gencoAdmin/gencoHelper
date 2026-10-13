package finance

import (
	"genco/finance/internal/model"
	"testing"
)

func fixture() model.Input {
	return model.Input{Name: "Original example", Category: "general", Cost: "30000", Price: "50000", Orders: "1000", Fixed: "5300000", Packaging: "0", TargetMargin: "15"}
}
func result(t *testing.T, c string) model.Result {
	t.Helper()
	for _, r := range Evaluate(fixture(), model.Defaults()).Results {
		if r.Channel == c {
			return r
		}
	}
	t.Fatal("missing channel")
	return model.Result{}
}
func TestOriginalFixtures(t *testing.T) {
	r := result(t, "shelf")
	if r.ContributionPerOrder != "8000" || r.MonthlyProfit != "2700000" || r.Margin != "5.4" || *r.BreakEvenOrders.Value != "663" {
		t.Fatalf("marketplace fixture: %+v", r)
	}
	r = result(t, "live")
	if r.MonthlyProfit != "-4125000" || *r.BreakEvenPrice.Value != "56951" {
		t.Fatalf("live fixture: %+v", r)
	}
}

func TestMarketRangeDoesNotReplaceSellingPrice(t *testing.T) {
	in := fixture()
	in.MarketMin, in.MarketMax = "40000", "70000"
	s := model.Defaults()
	ev := Evaluate(in, s)
	base := Evaluate(fixture(), s)
	if ev.Recommended != base.Recommended || ev.CalculationVersion != "1.1.0" {
		t.Fatal("market context changed the selected-price recommendation")
	}
	for i, r := range ev.Results {
		if r.Price != in.Price || r.MonthlyProfit != base.Results[i].MonthlyProfit {
			t.Fatalf("market range replaced the selling price: %+v", r)
		}
	}
	if ev.MaximumScenario == nil || ev.MaximumScenario.Channel != ev.Recommended || ev.MaximumScenario.Price != in.MarketMax {
		t.Fatal("market maximum must compare the same channel")
	}
	// Testing a competitor price is an explicit new scenario with the same costs.
	in.Price = in.MarketMin
	low := Evaluate(in, s)
	if low.Results[0].Price != "40000" || d(low.Results[0].MonthlyProfit).Cmp(d(ev.Results[0].MonthlyProfit).Rat) >= 0 {
		t.Fatal("explicit price scenario did not change profit")
	}
}
func TestThresholdsAndReconciliation(t *testing.T) {
	for _, category := range []string{"general", "fashion", "beauty", "electronics", "home"} {
		in := fixture()
		in.Category = category
		in.Packaging = "1357.25"
		ev := Evaluate(in, model.Defaults())
		for _, r := range ev.Results {
			sum := zero
			for _, b := range r.Breakdown {
				sum = sum.add(d(b.Value))
			}
			if sum.sub(d(r.MonthlyProfit)).Sign() != 0 {
				t.Fatalf("waterfall does not reconcile %s", r.Channel)
			}
			check := func(m model.Metric, price, cost string, margin string) {
				if m.Value == nil {
					return
				}
				v := in
				v.Price = price
				v.Cost = cost
				rr := Calculate(v, r.Rates, r.Sources, r.Channel, price, model.Defaults())
				if d(rr.Margin).Cmp(d(margin).Rat) < 0 {
					t.Fatalf("threshold fails %s: %+v", r.Channel, rr)
				}
			}
			if r.BreakEvenPrice.Value != nil {
				check(r.BreakEvenPrice, *r.BreakEvenPrice.Value, in.Cost, "0")
			}
			if r.TargetPrice.Value != nil {
				check(r.TargetPrice, *r.TargetPrice.Value, in.Cost, in.TargetMargin)
			}
			if r.MaxSupplierCost.Value != nil {
				check(r.MaxSupplierCost, in.Price, *r.MaxSupplierCost.Value, "0")
			}
			if r.TargetCost.Value != nil {
				check(r.TargetCost, in.Price, *r.TargetCost.Value, in.TargetMargin)
			}
		}
	}
}
func TestBoundariesAndPrecedence(t *testing.T) {
	s := model.Defaults()
	in := fixture()
	in.Category = "fashion"
	in.Overrides = map[string]model.Rates{"live": {"returns": "100", "advertising": "0"}}
	r, src := Effective(in, s, "live")
	if r["returns"] != "100" || src["returns"] != "product" {
		t.Fatal("precedence")
	}
	in.TargetMargin = "100"
	in.Fixed = "0"
	out := Calculate(in, r, src, "live", in.Price, s)
	if out.SuccessfulSales != "0" || out.MaxSupplierCost.Status != "undefined" || out.TargetPrice.Status != "unattainable" {
		t.Fatalf("boundary %+v", out)
	}
	in = fixture()
	in.Fixed = "0"
	in.Cost = "38000"
	r, src = Effective(in, s, "shelf")
	out = Calculate(in, r, src, "shelf", in.Price, s)
	if out.MonthlyProfit != "0" || *out.BreakEvenOrders.Value != "0" || out.Reason != "break_even" {
		t.Fatal("exact break-even")
	}
	r["commission"] = "100"
	r["payment"] = "100"
	out = Calculate(in, r, src, "shelf", in.Price, s)
	if out.BreakEvenPrice.Status != "unattainable" {
		t.Fatal("excessive fees")
	}
}

func TestHighPrecisionWaterfall(t *testing.T) {
	in := fixture()
	in.Price = "999999999999.12345678"
	in.Cost = "123456789012.87654321"
	in.Orders = "9999991"
	in.Packaging = "1234.12345678"
	in.Overrides = map[string]model.Rates{"live": {"returns": "3.12345678", "commission": "12.12345678", "payment": "1.12345678"}}
	for _, r := range Evaluate(in, model.Defaults()).Results {
		sum := zero
		for _, b := range r.Breakdown {
			sum = sum.add(d(b.Value))
		}
		if sum.Cmp(d(r.MonthlyProfit).Rat) != 0 {
			t.Fatalf("high precision amounts did not reconcile: %s", r.Channel)
		}
	}
}
