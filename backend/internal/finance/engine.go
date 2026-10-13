package finance

import (
	"fmt"
	"genco/finance/internal/model"
	"regexp"
	"strings"
	"time"
)

var decimalPattern = regexp.MustCompile(`^\d+(\.\d{1,8})?$`)

type Validation map[string]string

func (v Validation) Error() string { return "Please check the highlighted fields" }
func valid(s string, max string) bool {
	return len(s) <= 24 && decimalPattern.MatchString(s) && d(s).Cmp(d(max).Rat) <= 0
}
func Validate(in model.Input, identity bool) Validation {
	e := Validation{}
	if identity && strings.TrimSpace(in.Name) == "" {
		e["name"] = "Product name is required"
	}
	if len([]rune(in.Name)) > 200 {
		e["name"] = "Maximum 200 characters"
	}
	if len([]rune(in.SKU)) > 100 {
		e["sku"] = "Maximum 100 characters"
	}
	if len(in.Notes) > 10000 {
		e["notes"] = "Maximum 10,000 bytes"
	}
	if _, ok := model.Defaults().CategoryReturns[in.Category]; !ok {
		e["category"] = "Choose a valid category"
	}
	for k, v := range map[string]string{"cost": in.Cost, "price": in.Price, "fixed": in.Fixed, "packaging": in.Packaging} {
		if !valid(v, "1000000000000") {
			e[k] = "Enter a non-negative decimal, up to 1 trillion"
		}
	}
	if valid(in.Price, "1000000000000") && d(in.Price).Sign() == 0 {
		e["price"] = "Selling price must be greater than zero"
	}
	if !valid(in.Orders, "10000000") || d(in.Orders).Sign() <= 0 || !d(in.Orders).IsInt() {
		e["orders"] = "Enter 1–10,000,000 whole orders"
	}
	if !valid(in.TargetMargin, "100") {
		e["targetMargin"] = "Enter a percentage from 0 to 100"
	}
	for k, v := range map[string]string{"marketMin": in.MarketMin, "marketMax": in.MarketMax} {
		if v != "" && (!valid(v, "1000000000000") || d(v).Sign() <= 0) {
			e[k] = "Enter a positive market price"
		}
	}
	if in.MarketMin != "" && in.MarketMax != "" && d(in.MarketMin).Cmp(d(in.MarketMax).Rat) > 0 {
		e["marketMax"] = "Maximum must be at least the minimum"
	}
	for c, r := range in.Overrides {
		if !channel(c) {
			e["overrides."+c] = "Unknown channel"
		}
		validateRates(e, "overrides."+c+".", r, false)
	}
	return e
}
func channel(c string) bool {
	for _, x := range model.Channels {
		if x == c {
			return true
		}
	}
	return false
}
func validateRates(e Validation, prefix string, r model.Rates, required bool) {
	for k, v := range r {
		known := false
		for _, x := range model.RateKeys {
			if k == x {
				known = true
			}
		}
		if !known {
			e[prefix+k] = "Unknown rate"
		}
		max := "100"
		if k == "logistics" || k == "shippingIncome" || k == "settlement" {
			max = "1000000000000"
		}
		if !valid(v, max) {
			e[prefix+k] = "Enter a valid non-negative rate"
		}
	}
	if required {
		for _, k := range model.RateKeys {
			if _, ok := r[k]; !ok {
				e[prefix+k] = "Rate is required"
			}
		}
	}
}
func ValidateSettings(s model.Settings) Validation {
	e := Validation{}
	for _, c := range model.Channels {
		validateRates(e, "channels."+c+".", s.Channels[c], true)
	}
	for c, r := range s.CategoryReturns {
		if _, ok := model.Defaults().CategoryReturns[c]; !ok {
			e["categoryReturns."+c] = "Unknown category"
		}
		if r != "" && !valid(r, "100") {
			e["categoryReturns."+c] = "Enter 0–100, or blank to use the channel preset"
		}
	}
	for c := range model.Defaults().CategoryReturns {
		if _, ok := s.CategoryReturns[c]; !ok {
			e["categoryReturns."+c] = "Category is required"
		}
	}
	for k, v := range map[string]string{"targetMargin": s.TargetMargin, "healthyMargin": s.HealthyMargin, "healthyBuffer": s.HealthyBuffer, "viableMargin": s.ViableMargin} {
		if !valid(v, "100") {
			e[k] = "Enter 0–100"
		}
	}
	if d(s.ViableMargin).Cmp(d(s.HealthyMargin).Rat) > 0 {
		e["viableMargin"] = "Viable threshold cannot exceed healthy threshold"
	}
	return e
}
func Effective(in model.Input, s model.Settings, c string) (model.Rates, map[string]string) {
	r := model.Rates{}
	src := map[string]string{}
	for k, v := range s.Channels[c] {
		r[k] = v
		src[k] = "preset"
	}
	if v := s.CategoryReturns[in.Category]; v != "" {
		r["returns"] = v
		src["returns"] = "category"
	}
	for k, v := range in.Overrides[c] {
		r[k] = v
		src[k] = "product"
	}
	return r, src
}
func Evaluate(in model.Input, s model.Settings) model.Evaluation {
	if in.Overrides == nil {
		in.Overrides = map[string]model.Rates{}
	}
	ev := model.Evaluation{Input: in, PresetVersion: s.Version, CalculationVersion: "1.0.0", CreatedAt: time.Now().UTC()}
	price := in.Price
	if in.MarketMin != "" {
		price = in.MarketMin
	}
	best := zero
	for i, c := range model.Channels {
		r, src := Effective(in, s, c)
		res := Calculate(in, r, src, c, price, s)
		ev.Results = append(ev.Results, res)
		if i == 0 || d(res.MonthlyProfit).Cmp(best.Rat) > 0 {
			best = d(res.MonthlyProfit)
			ev.Recommended = c
		}
	}
	if in.MarketMax != "" {
		r, src := Effective(in, s, ev.Recommended)
		res := Calculate(in, r, src, ev.Recommended, in.MarketMax, s)
		ev.MaximumScenario = &res
	}
	return ev
}
func Calculate(in model.Input, r model.Rates, src map[string]string, c, price string, settings model.Settings) model.Result {
	p, q, cost, fixed, pack := d(price), d(in.Orders), d(in.Cost), d(in.Fixed), d(in.Packaging)
	pct := func(k string) dec { return d(r[k]).div(hundred) }
	s := one.sub(pct("returns"))
	successful := q.mul(s)
	gmv := q.mul(p)
	revenue := successful.mul(p)
	shipping := successful.mul(d(r["shippingIncome"]))
	// contribution/order = A*P+B, with costs applied on their exact bases.
	a := s.mul(one.sub(pct("commission")).sub(pct("payment")).sub(pct("affiliate")).sub(pct("promotion"))).sub(pct("advertising")).sub(pct("logisticsRate"))
	b := s.mul(d(r["shippingIncome"])).mul(one.sub(pct("payment"))).sub(s.mul(cost)).sub(d(r["logistics"])).sub(d(r["settlement"])).sub(pack)
	cpu := a.mul(p).add(b)
	contribution := cpu.mul(q)
	profit := contribution.sub(fixed)
	fpo := fixed.div(q)
	margin := profit.div(gmv).mul(hundred)
	out := model.Result{Channel: c, Rates: r, Sources: src, Price: price, SuccessfulSales: successful.str(), Revenue: revenue.str(), ShippingIncome: shipping.str(), Contribution: contribution.str(), ContributionPerOrder: cpu.str(), MonthlyProfit: profit.str(), ProfitPerOrder: profit.div(q).str(), Margin: margin.str(), BreakEvenOrders: status("unattainable"), BreakEvenPrice: status("unattainable"), MaxSupplierCost: status("undefined"), PriceBuffer: status("undefined"), MaxAdvertising: status("unattainable"), BreakEvenROAS: status("unattainable"), TargetPrice: status("unattainable"), TargetCost: status("undefined")}
	var buffer *dec
	if cpu.Sign() > 0 {
		out.BreakEvenOrders = value(fixed.div(cpu).ceil())
	} else if cpu.Sign() == 0 && fixed.Sign() == 0 {
		out.BreakEvenOrders = value(zero)
	}
	if a.Sign() > 0 {
		be := fpo.sub(b).div(a)
		if be.Sign() < 0 {
			be = zero
		}
		out.BreakEvenPrice = value(be.ceil())
		exactBuffer := p.sub(be).div(p).mul(hundred)
		buffer = &exactBuffer
		out.PriceBuffer = value(exactBuffer)
	} else if b.Cmp(fpo.Rat) >= 0 && a.Sign() == 0 {
		out.BreakEvenPrice = value(zero)
		out.PriceBuffer = value(hundred)
		buffer = &hundred
	}
	target := d(in.TargetMargin).div(hundred)
	if a.sub(target).Sign() > 0 {
		tp := fpo.sub(b).div(a.sub(target))
		if tp.Sign() < 0 {
			tp = zero
		}
		out.TargetPrice = value(tp.ceil())
	}
	if s.Sign() > 0 {
		ceiling := a.mul(p).add(b).add(s.mul(cost)).sub(fpo).div(s)
		if ceiling.Sign() >= 0 {
			out.MaxSupplierCost = value(ceiling.floor())
		} else {
			out.MaxSupplierCost = status("unattainable")
		}
		tc := ceiling.sub(target.mul(p).div(s))
		if tc.Sign() >= 0 {
			out.TargetCost = value(tc.floor())
		} else {
			out.TargetCost = status("unattainable")
		}
	}
	maxAds := pct("advertising").add(cpu.sub(fpo).div(p))
	if maxAds.Sign() >= 0 {
		out.MaxAdvertising = value(maxAds.mul(hundred))
		if maxAds.Sign() > 0 {
			out.BreakEvenROAS = value(one.div(maxAds))
		}
	}
	out.Verdict = "thin"
	out.Reason = "thin"
	if profit.Sign() < 0 {
		out.Verdict = "unprofitable"
		if cpu.Sign() <= 0 {
			out.Reason = "negative_contribution"
		} else {
			out.Reason = "insufficient_volume"
		}
	} else if profit.Sign() == 0 {
		out.Reason = "break_even"
	} else if margin.Cmp(d(settings.HealthyMargin).Rat) >= 0 && buffer != nil && buffer.Cmp(d(settings.HealthyBuffer).Rat) >= 0 {
		out.Verdict = "healthy"
		out.Reason = "healthy"
	} else if margin.Cmp(d(settings.ViableMargin).Rat) >= 0 {
		out.Verdict = "viable"
		out.Reason = "viable"
	}
	add := func(k string, v dec) { out.Breakdown = append(out.Breakdown, model.Breakdown{Key: k, Value: v.str()}) }
	add("grossRevenue", gmv)
	add("returnedRevenue", revenue.sub(gmv))
	add("shippingIncome", shipping)
	for _, k := range []string{"commission", "affiliate", "promotion"} {
		add(k, zero.sub(revenue.mul(pct(k))))
	}
	add("payment", zero.sub(revenue.add(shipping).mul(pct("payment"))))
	add("advertising", zero.sub(gmv.mul(pct("advertising"))))
	add("productCost", zero.sub(successful.mul(cost)))
	add("logistics", zero.sub(q.mul(d(r["logistics"])).add(gmv.mul(pct("logisticsRate")))))
	add("settlement", zero.sub(q.mul(d(r["settlement"]))))
	add("packaging", zero.sub(q.mul(pack)))
	add("fixed", zero.sub(fixed))
	for i := 0; i <= 10; i++ {
		v := q.mul(d(fmt.Sprint(i))).div(d("5"))
		out.Volume = append(out.Volume, model.Point{Orders: v.str(), Profit: v.mul(cpu).sub(fixed).str()})
	}
	return out
}
