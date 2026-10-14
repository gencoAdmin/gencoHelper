export class ApiError extends Error { constructor(public status: number, message: string, public fields: Record<string, string> = {}, public code = '') { super(message) } }
export async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
 const response = await fetch(`/api/v1${path}`, { ...options, headers: { ...(options.body instanceof FormData ? {} : { 'Content-Type': 'application/json' }), ...options.headers } })
 if (!response.ok) { const error = await response.json().catch(() => ({ message: response.statusText })); throw new ApiError(response.status, error.message, error.fields || {}, error.code || '') }
 return response.json()
}
export const json = (method: string, body: unknown): RequestInit => ({ method, body: JSON.stringify(body) })
export const money = (value: string | number | null | undefined) => value == null ? '—' : `Rp ${new Intl.NumberFormat('en-US', { maximumFractionDigits: 0 }).format(Number(value))}`
export const number = (value: string | number | null | undefined, digits = 0) => value == null ? '—' : new Intl.NumberFormat('en-US', { maximumFractionDigits: digits }).format(Number(value))
export const percent = (value: string | number | null | undefined) => value == null ? '—' : `${number(value, 1)}%`
export const canonicalJSON = (value: unknown) => JSON.stringify(value, (_key, item) => item && typeof item === 'object' && !Array.isArray(item) ? Object.fromEntries(Object.entries(item).sort(([a], [b]) => a.localeCompare(b))) : item)
export function compareDecimals(a: string, b: string) {
 const parse = (value: string) => { const [whole, fraction = ''] = value.split('.'); return { whole: whole || '0', fraction } }
 const x = parse(a), y = parse(b), digits = Math.max(x.fraction.length, y.fraction.length)
 const scale = (v: typeof x) => BigInt(v.whole) * (10n ** BigInt(digits)) + BigInt(v.fraction.padEnd(digits, '0') || '0')
 const first = scale(x), second = scale(y)
 return first < second ? -1 : first > second ? 1 : 0
}
