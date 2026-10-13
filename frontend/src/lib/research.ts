import { compareDecimals } from '../api'
import type { Competitor } from '../types'

export const validPrice = (value: string) => value.length <= 24 && /^\d+(\.\d{1,8})?$/.test(value) && compareDecimals(value, '0') > 0 && compareDecimals(value, '1000000000000') <= 0
const units = (value: string) => { const [whole, fraction = ''] = value.split('.'); return BigInt(whole) * 100000000n + BigInt(fraction.padEnd(8, '0')) }
const decimal = (value: bigint) => `${value / 100000000n}.${String(value % 100000000n).padStart(8, '0')}`.replace(/\.?0+$/, '')

export function marketSummary(competitors: Competitor[]) {
 const rows = competitors.filter(c => validPrice(c.price))
 const prices = rows.map(c => c.price).sort(compareDecimals)
 const middle = Math.floor(prices.length / 2)
 // Round upwards at the API's eight-decimal precision when averaging.
 const average = (values: string[]) => decimal((values.reduce((sum, value) => sum + units(value), 0n) + BigInt(values.length - 1)) / BigInt(values.length))
 return {
  count: prices.length, min: prices[0] || '', max: prices.at(-1) || '',
  median: prices.length ? prices.length % 2 ? prices[middle] : average(prices.slice(middle - 1, middle + 1)) : '',
  avg: prices.length ? average(prices) : '',
  sales: rows.reduce((sum, c) => sum + (Number(c.sales) || 0), 0),
  top: [...rows].sort((a,b) => (Number(b.sales) || 0) - (Number(a.sales) || 0))[0]?.shop || '—',
 }
}

export function researchLink(value: string) {
 try { const url = new URL(value); return ['http:', 'https:'].includes(url.protocol) ? url.href : undefined } catch { return undefined }
}
