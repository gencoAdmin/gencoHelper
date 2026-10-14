import { defineStore } from 'pinia'
import { ref } from 'vue'

export type ThemeChoice = 'light' | 'dark' | 'system'
const KEY = 'genco-theme'
const mql = typeof window !== 'undefined' && window.matchMedia
  ? window.matchMedia('(prefers-color-scheme: dark)')
  : null

// Resolve the actual light/dark to paint for a given choice.
function resolved(choice: ThemeChoice): 'light' | 'dark' {
  if (choice === 'system') return mql?.matches ? 'dark' : 'light'
  return choice
}

// Apply the choice to <html> (data-theme) and the browser chrome color.
// Defensive: no-ops under SSR / non-DOM stubs that lack these methods.
export function applyTheme(choice: ThemeChoice) {
  if (typeof document === 'undefined') return
  const root = document.documentElement as HTMLElement | undefined
  if (!root || typeof root.setAttribute !== 'function') return
  const mode = resolved(choice)
  // 'system' leaves data-theme off so the prefers-color-scheme media query drives it.
  if (choice === 'system') root.removeAttribute('data-theme')
  else root.setAttribute('data-theme', choice)
  if (typeof document.querySelector !== 'function') return
  const meta = document.querySelector('meta[name="theme-color"]:not([media])')
    || document.querySelector('meta[name="theme-color"]')
  if (meta) meta.setAttribute('content', mode === 'dark' ? '#141210' : '#f2740c')
}

export function initThemeEarly(): ThemeChoice {
  let choice: ThemeChoice = 'system'
  try {
    const saved = localStorage.getItem(KEY) as ThemeChoice | null
    if (saved === 'light' || saved === 'dark' || saved === 'system') choice = saved
  } catch { /* storage may be unavailable */ }
  applyTheme(choice)
  return choice
}

export const useTheme = defineStore('theme', () => {
  const choice = ref<ThemeChoice>(initThemeEarly())

  function set(next: ThemeChoice) {
    choice.value = next
    applyTheme(next)
    try { localStorage.setItem(KEY, next) } catch { /* ignore */ }
  }

  // Cycle used by the topbar button: light -> dark -> system -> light.
  function cycle() {
    set(choice.value === 'light' ? 'dark' : choice.value === 'dark' ? 'system' : 'light')
  }

  // Repaint when the OS theme changes while on 'system'.
  if (mql) {
    const onChange = () => { if (choice.value === 'system') applyTheme('system') }
    mql.addEventListener ? mql.addEventListener('change', onChange) : mql.addListener(onChange)
  }

  return { choice, set, cycle }
})
