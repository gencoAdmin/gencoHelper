package model

import "time"

type Rates map[string]string

var Channels = []string{"shelf", "live", "cod", "self", "consignment"}
var RateKeys = []string{"commission", "payment", "affiliate", "advertising", "promotion", "logisticsRate", "logistics", "shippingIncome", "settlement", "returns"}

type Settings struct {
	Version         int               `json:"version"`
	Channels        map[string]Rates  `json:"channels"`
	CategoryReturns map[string]string `json:"categoryReturns"`
	TargetMargin    string            `json:"targetMargin"`
	HealthyMargin   string            `json:"healthyMargin"`
	HealthyBuffer   string            `json:"healthyBuffer"`
	ViableMargin    string            `json:"viableMargin"`
	UpdatedAt       time.Time         `json:"updatedAt"`
}

func Defaults() Settings {
	rows := [][]string{
		{"11", "0", "5", "4", "0", "4", "0", "0", "0", "0"},
		{"16", "1", "10", "10", "0", "0", "0", "0", "0", "5"},
		{"3", "0", "26.6", "0", "0", "0", "9000", "0", "0", "0"},
		{"0", "0.7", "0", "0", "31", "0", "9000", "9000", "500", "0"},
		{"25", "0", "0", "0", "0", "0", "0", "0", "0", "0"},
	}
	s := Settings{Version: 1, Channels: map[string]Rates{}, CategoryReturns: map[string]string{"general": "", "fashion": "10", "beauty": "3", "electronics": "5", "home": "3"}, TargetMargin: "15", HealthyMargin: "15", HealthyBuffer: "10", ViableMargin: "8", UpdatedAt: time.Now().UTC()}
	for i, c := range Channels {
		s.Channels[c] = Rates{}
		for j, k := range RateKeys {
			s.Channels[c][k] = rows[i][j]
		}
	}
	return s
}

type Input struct {
	Name         string           `json:"name"`
	SKU          string           `json:"sku"`
	Category     string           `json:"category"`
	Notes        string           `json:"notes"`
	Cost         string           `json:"cost"`
	Price        string           `json:"price"`
	Orders       string           `json:"orders"`
	Fixed        string           `json:"fixed"`
	Packaging    string           `json:"packaging"`
	TargetMargin string           `json:"targetMargin"`
	MarketMin    string           `json:"marketMin"`
	MarketMax    string           `json:"marketMax"`
	Overrides    map[string]Rates `json:"overrides"`
}
type Metric struct {
	Status string  `json:"status"`
	Value  *string `json:"value"`
}
type Breakdown struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
type Point struct {
	Orders string `json:"orders"`
	Profit string `json:"profit"`
}
type Result struct {
	Channel              string            `json:"channel"`
	Rates                Rates             `json:"rates"`
	Sources              map[string]string `json:"sources"`
	Price                string            `json:"price"`
	SuccessfulSales      string            `json:"successfulSales"`
	Revenue              string            `json:"revenue"`
	ShippingIncome       string            `json:"shippingIncome"`
	Contribution         string            `json:"contribution"`
	ContributionPerOrder string            `json:"contributionPerOrder"`
	ProfitPerOrder       string            `json:"profitPerOrder"`
	MonthlyProfit        string            `json:"monthlyProfit"`
	Margin               string            `json:"margin"`
	BreakEvenOrders      Metric            `json:"breakEvenOrders"`
	BreakEvenPrice       Metric            `json:"breakEvenPrice"`
	MaxSupplierCost      Metric            `json:"maxSupplierCost"`
	PriceBuffer          Metric            `json:"priceBuffer"`
	MaxAdvertising       Metric            `json:"maxAdvertising"`
	BreakEvenROAS        Metric            `json:"breakEvenROAS"`
	TargetPrice          Metric            `json:"targetPrice"`
	TargetCost           Metric            `json:"targetCost"`
	Verdict              string            `json:"verdict"`
	Reason               string            `json:"reason"`
	Breakdown            []Breakdown       `json:"breakdown"`
	Volume               []Point           `json:"volume"`
}
type Evaluation struct {
	ID                 string    `json:"id"`
	Input              Input     `json:"input"`
	PresetVersion      int       `json:"presetVersion"`
	CalculationVersion string    `json:"calculationVersion"`
	Results            []Result  `json:"results"`
	Recommended        string    `json:"recommended"`
	MaximumScenario    *Result   `json:"maximumScenario,omitempty"`
	CreatedAt          time.Time `json:"createdAt"`
}
type Competitor struct {
	ID       string `json:"id"`
	Platform string `json:"platform"`
	Shop     string `json:"shop"`
	Price    string `json:"price"`
	Sales    string `json:"sales"`
	Rating   string `json:"rating"`
	URL      string `json:"url"`
}
type Image struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Thumbnail string    `json:"thumbnail"`
	CreatedAt time.Time `json:"createdAt"`
}
type Product struct {
	ID          string       `json:"id"`
	Version     int          `json:"version"`
	Archived    bool         `json:"archived"`
	Input       Input        `json:"input"`
	Competitors []Competitor `json:"competitors"`
	Images      []Image      `json:"images"`
	Revisions   []Evaluation `json:"revisions"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

func (p Product) Latest() Evaluation { return p.Revisions[len(p.Revisions)-1] }

type ImportRow struct {
	Row    int      `json:"row"`
	Input  Input    `json:"input"`
	Errors []string `json:"errors"`
}
type Preview struct {
	ID        string      `json:"id"`
	Rows      []ImportRow `json:"rows"`
	CreatedAt time.Time   `json:"createdAt"`
	JobID     string      `json:"jobId"`
}
type Job struct {
	ID         string      `json:"id"`
	Kind       string      `json:"kind"`
	Status     string      `json:"status"`
	Total      int         `json:"total"`
	Processed  int         `json:"processed"`
	Created    int         `json:"created"`
	Skipped    int         `json:"skipped"`
	Failed     int         `json:"failed"`
	Errors     []string    `json:"errors"`
	CreatedAt  time.Time   `json:"createdAt"`
	LeaseUntil time.Time   `json:"leaseUntil"`
	Artifact   string      `json:"artifact,omitempty"`
	Format     string      `json:"format,omitempty"`
	Locale     string      `json:"locale,omitempty"`
	ImportRows []ImportRow `json:"-"`
	Products   []Product   `json:"-"`
}

// StoredJob includes private job inputs, which must survive a service restart.
type StoredJob struct {
	Job
	Rows      []ImportRow `json:"storedRows,omitempty"`
	Snapshots []Product   `json:"snapshots,omitempty"`
}
type State struct {
	Products map[string]Product   `json:"products"`
	Settings []Settings           `json:"settings"`
	Previews map[string]Preview   `json:"previews"`
	Jobs     map[string]StoredJob `json:"jobs"`
}

func NewState() State {
	return State{Products: map[string]Product{}, Settings: []Settings{Defaults()}, Previews: map[string]Preview{}, Jobs: map[string]StoredJob{}}
}
