package finance

import (
	"genco/finance/internal/model"
	"math/big"
	"strings"
)

// Exact rational arithmetic avoids binary floats and repeating-decimal drift.
// Values are rounded only at API presentation and whole-Rupiah thresholds.
type dec struct{ *big.Rat }

func d(s string) dec {
	v, ok := new(big.Rat).SetString(s)
	if !ok {
		v = new(big.Rat)
	}
	return dec{v}
}
func (a dec) add(b dec) dec { return dec{new(big.Rat).Add(a.Rat, b.Rat)} }
func (a dec) sub(b dec) dec { return dec{new(big.Rat).Sub(a.Rat, b.Rat)} }
func (a dec) mul(b dec) dec { return dec{new(big.Rat).Mul(a.Rat, b.Rat)} }
func (a dec) div(b dec) dec { return dec{new(big.Rat).Quo(a.Rat, b.Rat)} }
func (a dec) str() string {
	// Finite decimal amounts retain all digits, even for large values and
	// high-precision rates. Repeating ratios are presented to 12 places.
	denominator := new(big.Int).Set(a.Denom())
	digits := 0
	for _, factor := range []int64{2, 5} {
		count := 0
		for new(big.Int).Mod(denominator, big.NewInt(factor)).Sign() == 0 {
			denominator.Quo(denominator, big.NewInt(factor))
			count++
		}
		if count > digits {
			digits = count
		}
	}
	if denominator.Cmp(big.NewInt(1)) != 0 {
		digits = 12
	}
	s := a.FloatString(digits)
	if digits == 0 {
		return s
	}
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "-0" {
		return "0"
	}
	return s
}
func (a dec) ceil() dec {
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(a.Num(), a.Denom(), r)
	if r.Sign() > 0 {
		q.Add(q, big.NewInt(1))
	}
	return dec{new(big.Rat).SetInt(q)}
}
func (a dec) floor() dec {
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(a.Num(), a.Denom(), r)
	if r.Sign() < 0 {
		q.Sub(q, big.NewInt(1))
	}
	return dec{new(big.Rat).SetInt(q)}
}
func value(a dec) model.Metric     { s := a.str(); return model.Metric{Status: "ok", Value: &s} }
func status(s string) model.Metric { return model.Metric{Status: s} }

var zero = d("0")
var one = d("1")
var hundred = d("100")
