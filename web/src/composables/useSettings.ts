import { reactive, watch } from 'vue';

// Phosphor theme selection, shared by every consumer via a single module-level
// reactive object. Persisted to localStorage and applied to <html data-theme>.
// (CRT effects are always on now — no longer user-configurable.)

export type Theme = 'amber' | 'cyan' | 'mono';
export const THEMES: Theme[] = ['amber', 'cyan', 'mono'];

export interface Settings {
  theme: Theme;
  /** Fire a desktop notification when a run finishes in a hidden tab. */
  notify: boolean;
}

const STORAGE_KEY = 'local-ci.settings';

const DEFAULTS: Settings = { theme: 'amber', notify: false };

function load(): Settings {
  const out = { ...DEFAULTS };
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw) as Partial<Settings>;
      if (parsed.theme && THEMES.includes(parsed.theme)) out.theme = parsed.theme;
      if (typeof parsed.notify === 'boolean') out.notify = parsed.notify;
    }
  } catch {
    // ignore malformed/blocked storage — fall back to defaults
  }
  return out;
}

const settings = reactive<Settings>(load());

function applyTheme(theme: Theme): void {
  if (typeof document !== 'undefined') {
    document.documentElement.setAttribute('data-theme', theme);
  }
}

let started = false;

/** Returns the shared settings object plus the theme cycler. Idempotent: the
 *  watcher and initial theme application run only once across all callers. */
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

  /** Toggle run-finished notifications, requesting permission on first enable.
   *  Resolves to the resulting on/off state (off when permission is denied). */
  async function toggleNotify(): Promise<boolean> {
    if (settings.notify) {
      settings.notify = false;
      return false;
    }
    if (!('Notification' in window)) return false;
    let perm = Notification.permission;
    if (perm === 'default') perm = await Notification.requestPermission();
    settings.notify = perm === 'granted';
    return settings.notify;
  }

  return { settings, cycleTheme, toggleNotify };
}
