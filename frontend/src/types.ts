// Decimal API values remain strings until display or charting.
export type { Rates, CalculationInput, Settings, Metric, ChannelResult, EvaluationRevision, Competitor, ProductImage, Product, ProductSummary, ImportRow, Preview, JobStatus } from "./generated-api"
import type { CalculationInput } from "./generated-api"
export const channels = ['shelf', 'live', 'cod', 'self', 'consignment']
// Fallback only, used before shared settings load. The source of truth is Settings.categories.
export const defaultCategories = ['general', 'fashion', 'beauty', 'electronics', 'home']
export const rateKeys = ['commission', 'payment', 'affiliate', 'advertising', 'promotion', 'logisticsRate', 'logistics', 'shippingIncome', 'settlement', 'returns']
export const cashRates = ['logistics', 'shippingIncome', 'settlement']
export const emptyInput = (): CalculationInput => ({ name: '', sku: '', category: 'general', notes: '', cost: '30000', price: '50000', orders: '1000', fixed: '5300000', packaging: '0', targetMargin: '15', marketMin: '', marketMax: '', overrides: {} })
