package workbook

// Column labels are localized while sheet names remain stable for consumers.
func resultHeaders(locale string) []any {
	rows := [][3]string{
		{"Contribution / order", "Kontribusi / pesanan", "每单贡献利润"},
		{"Successful sales", "Penjualan berhasil", "成功销售量"},
		{"Break-even orders", "Pesanan impas", "盈亏平衡订单量"},
		{"Break-even price", "Harga impas", "保本售价"},
		{"Max supplier cost", "Biaya pemasok maksimum", "最高供货成本"},
		{"Target price", "Harga target", "目标售价"},
		{"Target cost", "Biaya target", "目标成本"},
		{"Max advertising (%)", "Iklan maksimum (%)", "最高广告比例 (%)"},
		{"Break-even ROAS", "ROAS impas", "保本 ROAS"},
		{"Price buffer (%)", "Buffer harga (%)", "价格安全垫 (%)"},
		{"Gross revenue", "Pendapatan bruto", "毛收入"},
		{"Returned revenue", "Pendapatan retur", "退货收入扣除"},
		{"Shipping income", "Pendapatan ongkir", "运费收入"},
		{"Commission", "Komisi", "佣金"},
		{"Affiliate", "Afiliasi", "达人佣金"},
		{"Promotion", "Promosi", "促销"},
		{"Payment", "Pembayaran", "支付费用"},
		{"Advertising", "Iklan", "广告"},
		{"Product cost", "Biaya produk", "产品成本"},
		{"Logistics", "Logistik", "物流"},
		{"Settlement", "Penyelesaian", "结算"},
		{"Packaging", "Kemasan", "包装"},
		{"Fixed costs", "Biaya tetap", "固定成本"},
	}
	index := 0
	if locale == "id" {
		index = 1
	}
	if locale == "zh" {
		index = 2
	}
	out := []any{}
	for _, row := range rows {
		out = append(out, row[index])
	}
	return out
}
