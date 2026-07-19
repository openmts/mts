import { ref, watch } from 'vue'

const THEME_KEY = 'mts_theme'
type Theme = 'light' | 'dark'

function readTheme(): Theme {
  try {
    const v = localStorage.getItem(THEME_KEY)
    if (v === 'dark' || v === 'light') return v
  } catch { /* ignore */ }
  return 'light'
}

const theme = ref<Theme>(readTheme())

function applyTheme(t: Theme) {
  const root = document.documentElement
  if (t === 'dark') root.classList.add('dark')
  else root.classList.remove('dark')
}

applyTheme(theme.value)

watch(theme, (t) => {
  applyTheme(t)
  try { localStorage.setItem(THEME_KEY, t) } catch { /* ignore */ }
})

export function useTheme() {
  function toggleTheme() {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
  }
  function setTheme(t: Theme) {
    theme.value = t
  }
  return { theme, toggleTheme, setTheme }
}
