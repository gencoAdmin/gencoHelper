// One-off (re-runnable) generator that rewrites src/style.css:
//  - swaps hard-coded colors for CSS variables with light + dark values
//  - recolors the green palette to vivid tangerine orange
//  - adds a soft-black dark theme under [data-theme="dark"] + prefers-color-scheme
//  - lifts tiny font-sizes to a readable floor and relaxes a little spacing
// Run from frontend/: node scripts/retheme.mjs
// It reads style.src.css (the original design) and writes style.css.
import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

const dir = path.dirname(fileURLToPath(import.meta.url))
const SRC = path.join(dir, '../src/style.src.css')
const OUT = path.join(dir, '../src/style.css')

// ---------- color math ----------
function parseHex(hex) {
  let h = hex.slice(1), a = null
  if (h.length === 3) h = h.split('').map(c => c + c).join('')
  else if (h.length === 4) { a = h[3] + h[3]; h = h.slice(0, 3).split('').map(c => c + c).join('') }
  else if (h.length === 8) { a = h.slice(6, 8); h = h.slice(0, 6) }
  return { r: parseInt(h.slice(0, 2), 16), g: parseInt(h.slice(2, 4), 16), b: parseInt(h.slice(4, 6), 16), a }
}
function toHsl({ r, g, b }) {
  r /= 255; g /= 255; b /= 255
  const mx = Math.max(r, g, b), mn = Math.min(r, g, b)
  let h, s, l = (mx + mn) / 2
  if (mx === mn) { h = s = 0 } else {
    const d = mx - mn
    s = l > 0.5 ? d / (2 - mx - mn) : d / (mx + mn)
    switch (mx) { case r: h = (g - b) / d + (g < b ? 6 : 0); break; case g: h = (b - r) / d + 2; break; default: h = (r - g) / d + 4 }
    h /= 6
  }
  return { h: h * 360, s: s * 100, l: l * 100 }
}
function hslToHex(h, s, l, a) {
  h = ((h % 360) + 360) % 360 / 360; s = Math.max(0, Math.min(100, s)) / 100; l = Math.max(0, Math.min(100, l)) / 100
  let r, g, b
  if (s === 0) { r = g = b = l } else {
    const q = l < 0.5 ? l * (1 + s) : l + s - l * s, p = 2 * l - q
    const f = t => { if (t < 0) t++; if (t > 1) t--; if (t < 1 / 6) return p + (q - p) * 6 * t; if (t < 1 / 2) return q; if (t < 2 / 3) return p + (q - p) * (2 / 3 - t) * 6; return p }
    r = f(h + 1 / 3); g = f(h); b = f(h - 1 / 3)
  }
  const H = x => Math.round(x * 255).toString(16).padStart(2, '0')
  return '#' + H(r) + H(g) + H(b) + (a ? a : '')
}
const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v))

// ---------- role + mapping ----------
// role: 'fg' = text/icon/outline/accent (lighten in dark); 'surf' = background/border (darken in dark)
const GREEN_LO = 72, GREEN_HI = 163        // green hue band in the source palette
const ORANGE_LO = 20, ORANGE_HI = 42       // target orange band
const PRIMARY = '#f2740c'                  // vivid tangerine (buttons/hero/brand)

// Explicit overrides for the darkest brand greens used as solid fills / strong accents.
// key: source hex (lowercased, 6-digit) -> { light, dark } final values.
const BRAND = {
  '#183c32': { light: '#f2740c', dark: '#f2740c' }, // --green-dark: primary fill
  '#244e3d': { light: '#e26a09', dark: '#f2740c' }, // --green: accent
  '#244c3b': { light: '#e26a09', dark: '#f2740c' },
  '#2d5c43': { light: '#d85f08', dark: '#ff8a2b' }, // primary hover
  '#284c37': { light: '#c75f10', dark: '#ff8a2b' }, // nav-current text
}

function lightValue(hex) {
  const low = hex.toLowerCase()
  const base = low.length === 4 ? '#' + low.slice(1).split('').map(c => c + c).join('') : (low.length === 7 || low.length === 9 ? low.slice(0, 7) : low)
  if (BRAND[base]) return BRAND[base].light + (parseHex(hex).a ? parseHex(hex).a : '')
  const p = parseHex(hex); const { h, s, l } = toHsl(p)
  if (s < 9) return hex // neutral gray -> unchanged in light mode
  if (h >= GREEN_LO && h <= GREEN_HI) {
    const t = (h - GREEN_LO) / (GREEN_HI - GREEN_LO)
    const nh = ORANGE_LO + t * (ORANGE_HI - ORANGE_LO)
    let ns = s
    if (l < 72 && l > 26) ns = clamp(s * 1.5 + 8, 0, 100) // make mid accents read vividly orange
    return hslToHex(nh, ns, l, p.a)
  }
  return hex // red / purple / teal / blue stay as-is
}

function darkValue(hex, role) {
  const lv = lightValue(hex)
  const p = parseHex(lv); let { h, s, l } = toHsl(p)
  if (s < 9) { // neutral
    const nl = role === 'fg' ? clamp(100 - l + 6, 70, 95) : clamp(100 - l - 2, 7, 24)
    return hslToHex(h, s, nl, p.a)
  }
  if (role === 'fg') { // colored text/icon -> lift for contrast on dark
    const nl = l < 55 ? clamp(l + 34, 60, 86) : clamp(l, 60, 90)
    return hslToHex(h, clamp(s + 8, 0, 92), nl, p.a)
  }
  // colored surface/border tint -> dark warm equivalent
  const nl = l > 80 ? 15 : l > 60 ? 21 : l > 40 ? 27 : clamp(l, 12, 34)
  return hslToHex(h, clamp(s, 0, 60), nl, p.a)
}

// Explicit text overrides so body copy is crisp near-black (light) / near-white (dark)
// instead of being tinted orange by the green->orange rotation.
const TEXT = {
  '#253a30': { light: '#26211c', dark: '#ece7e2' }, // --ink / body text
  '#56634e': { light: '#4a433a', dark: '#d7d0c7' },
  '#56644b': { light: '#4a433a', dark: '#d7d0c7' },
  '#5c6a53': { light: '#4a433a', dark: '#cfc8bf' },
}

// Structural neutrals anchored to the chosen soft-black ramp (dark only).
// warm hue 28, low saturation so panels feel warm-black not flat gray.
const SURF_OVERRIDE = {
  '#ffffff': '#1e1a17', // panels / cards
  '#f7f8f4': '#141210', // page background (--cream)
  '#fafbf8': '#1b1714', // table head / subtle fills
  '#e4e8df': '#2a2420', // --border
}
// Order-preserving dark surface lightness: lighter source -> lighter (more elevated) dark surface.
function surfDark(hex) {
  const base = (() => { let h = hex.toLowerCase(); if (h.length === 4) h = '#' + h.slice(1).split('').map(c => c + c).join(''); return h.length >= 7 ? h.slice(0, 7) : h })()
  const alpha = parseHex(hex).a || ''
  if (SURF_OVERRIDE[base]) return SURF_OVERRIDE[base] + alpha
  const lv = lightValue(hex); const p = parseHex(lv); let { h, s, l } = toHsl(p)
  let dl
  if (l >= 78) dl = 13 + (l - 78) / 22 * 6            // light tints (badges/hover/sections) -> 13..19 (above panel)
  else if (l >= 55) dl = 19 + (78 - l) / 23 * 5       // borders / mid tints -> 19..24
  else dl = clamp(l, 14, 34)                          // already-dark accents stay
  const nh = s < 9 ? 28 : h                           // warm up neutrals
  const ns = s < 9 ? 6 : clamp(s, 0, 20)
  return hslToHex(nh, ns, dl, p.a)
}
function fgDark(hex) {
  const lv = lightValue(hex); const p = parseHex(lv); let { h, s, l } = toHsl(p)
  if (s < 9) { const nl = l > 82 ? clamp(l - 6, 78, 94) : clamp(118 - l, 66, 92); return hslToHex(h, s, nl, p.a) }
  const nl = l < 60 ? clamp(l + 32, 62, 86) : clamp(l, 62, 90)
  return hslToHex(h, clamp(s + 6, 0, 90), nl, p.a)
}
function shadowValue(hex) { return lightValue(hex) } // shadows/glows: same in both themes

function themed(hex, role) {
  const base = (() => { let h = hex.toLowerCase(); if (h.length === 4) h = '#' + h.slice(1).split('').map(c => c + c).join(''); return h.length >= 7 ? h.slice(0, 7) : h })()
  const alpha = parseHex(hex).a || ''
  if (TEXT[base] && role === 'fg') return { light: TEXT[base].light + alpha, dark: TEXT[base].dark + alpha }
  const light = lightValue(hex)
  let dark
  if (role === 'shadow') dark = shadowValue(hex)
  else if (role === 'fg') dark = fgDark(hex)
  else dark = surfDark(hex)
  return { light, dark }
}

// ---------- role detection by CSS property ----------
function roleOf(prop) {
  prop = prop.trim().toLowerCase()
  if (prop === 'box-shadow' || prop === '-webkit-box-shadow') return 'shadow'
  if (['color', 'stroke', 'fill', 'accent-color', 'outline', 'outline-color', 'caret-color'].includes(prop)) return 'fg'
  return 'surf' // background*, border*, etc.
}

// ---------- font-size floor + base bump ----------
const FONT_FLOOR = { 8: 10.5, 9: 11, 10: 11.5, 11: 12.5, 12: 13, 13: 13.5, 14: 14.5 }
function bumpFont(px, isRoot) {
  if (isRoot && px === 14) return 15
  const f = FONT_FLOOR[px]
  return f && f > px ? f : px
}

// ---------- token registry ----------
const tokens = new Map() // name -> {light, dark}
function tokenName(hex, role) {
  const base = hex.toLowerCase().replace('#', '')
  return `--c-${role}-${base}`
}
function register(hex, role) {
  const name = tokenName(hex, role)
  if (!tokens.has(name)) tokens.set(name, themed(hex, role))
  return name
}

const HEX_RE = /#[0-9a-fA-F]{3,8}\b/g
const WHITE_RE = /(?<![\w-])white(?![\w-])/g

// Replace every color literal in a declaration value with a var() token, by role.
function themeDeclaration(prop, value) {
  const role = roleOf(prop)
  let v = value.replace(HEX_RE, hex => `var(${register(hex, role)})`)
  v = v.replace(WHITE_RE, () => `var(${register('#ffffff', role)})`)
  return v
}

// Split CSS into top-level rules while respecting @media{...} blocks and nested braces.
function processCss(css, isRootPass) {
  let out = '', i = 0
  while (i < css.length) {
    const brace = css.indexOf('{', i)
    if (brace === -1) { out += css.slice(i); break }
    const selector = css.slice(i, brace)
    // find matching close brace
    let depth = 1, j = brace + 1
    while (j < css.length && depth > 0) { if (css[j] === '{') depth++; else if (css[j] === '}') depth--; j++ }
    const body = css.slice(brace + 1, j - 1)
    const sel = selector.trim()
    if (sel.startsWith('@media') || sel.startsWith('@supports')) {
      out += selector + '{' + processCss(body, false) + '}'
    } else if (sel.startsWith('@')) {
      out += selector + '{' + body + '}' // @keyframes etc. - leave untouched
    } else {
      out += selector + '{' + themeBody(body, sel) + '}'
    }
    i = j
  }
  return out
}

function themeBody(body, selector) {
  // split declarations on ; at depth 0 (handles url()/calc() with no nested ; here)
  const decls = []
  let buf = '', depth = 0
  for (const ch of body) {
    if (ch === '(') depth++
    else if (ch === ')') depth--
    if (ch === ';' && depth === 0) { decls.push(buf); buf = '' } else buf += ch
  }
  if (buf.trim()) decls.push(buf)
  const isRoot = selector === ':root'
  return decls.map(d => {
    const c = d.indexOf(':')
    if (c === -1) return d
    const prop = d.slice(0, c)
    let val = d.slice(c + 1)
    // custom properties handled separately in the root pass
    if (prop.trim().startsWith('--')) return d
    // font-size floor
    if (prop.trim().toLowerCase() === 'font-size') {
      val = val.replace(/(\d+(?:\.\d+)?)px/g, (m, n) => {
        const px = parseFloat(n); const nb = bumpFont(px, isRoot)
        return nb + 'px'
      })
    }
    val = themeDeclaration(prop, val)
    return prop + ':' + val
  }).join(';')
}

// Existing semantic custom props -> role so var(--green) etc. keep working, recolored.
const ROOT_VARS = {
  '--green': 'fg',       // accent (checkbox accent-color, icon-btn hover text)
  '--green-dark': 'surf',// primary fill (buttons, hero, brand icon bg) -> treat as solid surface
  '--ink': 'fg',         // body text
  '--muted': 'fg',       // muted text
  '--border': 'surf',    // borders
  '--cream': 'surf',     // page background
  '--lime': 'surf',      // light accent tint
}
// green-dark is a solid branded fill; force the primary ramp rather than a dark surface.
const ROOT_OVERRIDE = {
  '--green-dark': { light: '#f2740c', dark: '#f2740c' },
  '--green': { light: '#e26a09', dark: '#ff8a2b' },
}

function run() {
  let css = fs.readFileSync(SRC, 'utf8')
  // pull the first :root { ... } (base + custom props) out for bespoke handling
  const rootStart = css.indexOf(':root{')
  const rootOpen = rootStart + ':root'.length
  let depth = 0, k = rootOpen
  do { if (css[k] === '{') depth++; else if (css[k] === '}') depth--; k++ } while (depth > 0)
  const rootBody = css.slice(rootOpen + 1, k - 1)
  const rest = css.slice(k) // everything after the first :root{...}

  // parse root declarations
  const baseDecls = []       // non-color base (font-family etc.)
  const rootVarLight = []     // custom props at light values
  const rootVarDark = []      // custom props dark overrides
  let rootColorLight = '', rootColorDark = '', rootBgLight = '', rootBgDark = '', rootFont = ''
  for (const d of rootBody.split(';')) {
    if (!d.trim()) continue
    const c = d.indexOf(':'); const prop = d.slice(0, c).trim(); const val = d.slice(c + 1).trim()
    if (prop === 'color') { const t = themed(val, 'fg'); rootColorLight = t.light; rootColorDark = t.dark }
    else if (prop === 'background') { const t = themed(val, 'surf'); rootBgLight = t.light; rootBgDark = t.dark }
    else if (prop === 'font-size') { rootFont = bumpFont(parseFloat(val), true) + 'px' }
    else if (prop.startsWith('--')) {
      const role = ROOT_VARS[prop] || 'surf'
      const t = ROOT_OVERRIDE[prop] || themed(val, role)
      rootVarLight.push(`${prop}:${t.light}`); rootVarDark.push(`${prop}:${t.dark}`)
    } else baseDecls.push(d.trim())
  }

  // process the rest of the stylesheet (replaces literals with var() tokens, bumps fonts)
  const themedRest = processCss(rest, false)

  // emit token blocks
  const lightTokens = [...tokens].map(([n, v]) => `${n}:${v.light}`).join(';')
  const darkTokens = [...tokens].map(([n, v]) => `${n}:${v.dark}`).join(';')

  const header =
`:root{color-scheme:light;${baseDecls.join(';')};font-size:${rootFont};color:${rootColorLight};background:${rootBgLight};${rootVarLight.join(';')};${lightTokens}}
:root[data-theme="dark"]{color-scheme:dark;color:${rootColorDark};background:${rootBgDark};${rootVarDark.join(';')};${darkTokens}}
@media(prefers-color-scheme:dark){:root:not([data-theme="light"]){color-scheme:dark;color:${rootColorDark};background:${rootBgDark};${rootVarDark.join(';')};${darkTokens}}}
`
  const spacingNote = '/* generated by scripts/retheme.mjs — edit style.src.css + the script, not this file */\n'
  fs.writeFileSync(OUT, spacingNote + header + themedRest)
  console.log(`wrote ${OUT}`)
  console.log(`tokens: ${tokens.size}`)
  // sample report
  const pick = ['--c-surf-fff', '--c-fg-253a30', '--c-surf-f7f8f4', '--c-surf-edf4e6', '--c-fg-56734c', '--c-fg-789860', '--c-surf-e4e8df']
  for (const [n, v] of tokens) if (pick.includes(n)) console.log(`  ${n}  light=${v.light} dark=${v.dark}`)
  console.log('  --green-dark light=#f2740c dark=#f2740c (primary)')
}
run()



