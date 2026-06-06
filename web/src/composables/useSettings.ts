import { reactive, watch } from 'vue';

// User-facing display settings: phosphor theme + CRT effect toggles. A single
// module-level reactive object is shared by every consumer (the production
// equivalent of the design's Tweaks panel). Persisted to localStorage and
// applied to <html data-theme>.

export type Theme = 'amber' | 'cyan' | 'mono';
export const THEMES: Theme[] = ['amber', 'cyan', 'mono'];

export interface Settings {
  theme: Theme;
  scanlines: boolean;
  flicker: boolean;
  glitch: boolean;
  vsync: boolean;
}

const STORAGE_KEY = 'local-ci.settings';

const DEFAULTS: Settings = {
  theme: 'amber',
  scanlines: true,
  flicker: true,
  glitch: true,
  vsync: true,
};

function load(): Settings {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return { ...DEFAULTS, ...(JSON.parse(raw) as Partial<Settings>) };
  } catch {
    // ignore malformed/blocked storage — fall back to defaults
  }
  return { ...DEFAULTS };
}

const settings = reactive<Settings>(load());

function applyTheme(theme: Theme): void {
  if (typeof document !== 'undefined') {
    document.documentElement.setAttribute('data-theme', theme);
  }
}

let started = false;

/** Returns the shared settings object plus mutators. Idempotent: the watcher
 *  and initial theme application run only once across all callers. */
export function useSettings() {
  if (!started) {
    started = true;
    applyTheme(settings.theme);
    watch(
      () => settings.theme,
      (t) => applyTheme(t),
    );
    watch(
      settings,
      (s) => {
        try {
          localStorage.setItem(STORAGE_KEY, JSON.stringify(s));
        } catch {
          // ignore storage failures (private mode, quota)
        }
      },
      { deep: true },
    );
  }

  function cycleTheme(): Theme {
    const i = (THEMES.indexOf(settings.theme) + 1) % THEMES.length;
    settings.theme = THEMES[i];
    return settings.theme;
  }

  function setTheme(theme: Theme): void {
    settings.theme = theme;
  }

  function toggle(key: 'scanlines' | 'flicker' | 'glitch' | 'vsync'): void {
    settings[key] = !settings[key];
  }

  return { settings, cycleTheme, setTheme, toggle };
}
