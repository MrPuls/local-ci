<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import Icon from '@/components/Icon.vue';
import PipelineGraph from '@/components/PipelineGraph.vue';
import { useConfig } from '@/composables/useConfig';
import { useConfigRaw } from '@/composables/useConfigRaw';
import { useRunStatus } from '@/composables/useRunStatus';
import { useToast } from '@/composables/useToast';
import { highlightYamlLine } from '@/lib/yaml';
import { lintConfig } from '@/lib/lint';
import { compactDiff, diffLines } from '@/lib/diff';
import { mergePipeline } from '@/lib/pipeline';
import { baseName } from '@/lib/format';
import { getRawConfig } from '@/lib/api';

// CONFIG view — YAML editor (left) + saved-state pipeline preview (right).
// The editor is a transparent <textarea> over a highlighted <pre>: same font
// metrics, the textarea owns input + caret, the pre owns the colors.

const { config, loading: graphLoading } = useConfig();
const { text, dirty, stale, loading, saving, error, validation, load, ensureLoaded, save } =
  useConfigRaw();
const { reset } = useRunStatus();
const { push } = useToast();

onMounted(() => {
  reset(); // no run "on screen" while editing
  ensureLoaded();
});

// A selector switch while the buffer is clean just follows the new file.
watch(stale, (s) => {
  if (s && !dirty.value) load();
});

const filename = computed(() => baseName(config.value?.path));
const lineCount = computed(() => text.value.split('\n').length);

// --- client-side lint: catch the classics before the file is even saved ---
const issues = computed(() => (text.value ? lintConfig(text.value) : []));
const issueByLine = computed(() => {
  const m = new Map<number, string>();
  for (const i of issues.value) {
    m.set(i.line, m.has(i.line) ? `${m.get(i.line)}; ${i.message}` : i.message);
  }
  return m;
});

const highlighted = computed(() => {
  const map = issueByLine.value;
  return (
    text.value
      .split('\n')
      .map((l, i) => {
        const html = highlightYamlLine(l);
        return map.has(i + 1) ? `<span class="lint-bad">${html || ' '}</span>` : html;
      })
      .join('\n') + '\n'
  );
});

/** Move the caret to a 1-based line (from the LINT panel). */
function goToLine(line: number): void {
  const el = ta.value;
  if (!el) return;
  const offsets = text.value.split('\n').slice(0, line - 1).join('\n').length;
  const pos = line === 1 ? 0 : offsets + 1;
  el.focus();
  el.setSelectionRange(pos, pos);
  const lineHeight = el.scrollHeight / Math.max(1, lineCount.value);
  el.scrollTop = Math.max(0, (line - 4) * lineHeight);
  syncScroll();
  updateCaret();
}

// Validation chip: unsaved edits beat whatever the last save/load reported.
const valState = computed<{ label: string; cls: string }>(() => {
  if (dirty.value) return { label: 'UNSAVED', cls: 'accent' };
  const v = validation.value ?? (config.value ? { valid: config.value.valid } : null);
  if (!v) return { label: 'UNKNOWN', cls: 'dim' };
  return v.valid ? { label: 'VALID', cls: 'glow-strong' } : { label: 'INVALID', cls: 'error' };
});
const valErrors = computed(() => validation.value?.errors ?? config.value?.errors ?? []);

// Preview renders the *saved* config (the graph endpoint reads from disk).
const previewStages = computed(() => mergePipeline(config.value, null));

// --- editing -------------------------------------------------------------
const ta = ref<HTMLTextAreaElement | null>(null);
const hl = ref<HTMLElement | null>(null);
const gutter = ref<HTMLElement | null>(null);
const caret = ref({ line: 1, col: 1 });

function syncScroll(): void {
  if (!ta.value) return;
  if (hl.value) {
    hl.value.scrollTop = ta.value.scrollTop;
    hl.value.scrollLeft = ta.value.scrollLeft;
  }
  if (gutter.value) gutter.value.scrollTop = ta.value.scrollTop;
}

function updateCaret(): void {
  const el = ta.value;
  if (!el) return;
  const before = text.value.slice(0, el.selectionStart);
  const line = (before.match(/\n/g)?.length ?? 0) + 1;
  caret.value = { line, col: before.length - before.lastIndexOf('\n') };
}

function onKeydown(e: KeyboardEvent): void {
  if (e.key === 'Tab') {
    e.preventDefault();
    ta.value?.setRangeText('  ', ta.value.selectionStart, ta.value.selectionEnd, 'end');
    text.value = ta.value?.value ?? text.value;
  } else if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's') {
    e.preventDefault();
    void onSave();
  }
}

async function onSave(): Promise<void> {
  if (saving.value || !dirty.value) return;
  try {
    const res = await save();
    push(
      res.valid ? '> CONFIG_SAVED_' : `> SAVED_WITH_${res.errors.length}_ERROR${res.errors.length === 1 ? '' : 'S'}_`,
      res.valid ? 'accent' : 'error',
    );
  } catch (e) {
    push(`ERROR: ${e instanceof Error ? e.message : String(e)}`, 'error');
  }
}

// Reload from disk — two-step confirm with a diff preview when it would
// discard edits (disk on the left of the comparison, your buffer additions
// shown as +).
const reloadPending = ref(false);
const diskText = ref<string | null>(null);

async function onReload(): Promise<void> {
  if (dirty.value && !reloadPending.value) {
    reloadPending.value = true;
    try {
      diskText.value = await getRawConfig(); // diff against what's really on disk
    } catch {
      diskText.value = null;
    }
    return;
  }
  cancelReload();
  await load();
  push('> CONFIG_RELOADED_', 'accent');
}

function cancelReload(): void {
  reloadPending.value = false;
  diskText.value = null;
}

const pendingDiff = computed(() =>
  diskText.value === null ? [] : compactDiff(diffLines(diskText.value, text.value)),
);

// Starter pipeline for a config file that doesn't exist yet.
const TEMPLATE = `stages:
  - build
  - test

build:
  stage: build
  image: alpine:3.21
  script:
    - echo "building..."

test:
  stage: test
  image: alpine:3.21
  script:
    - echo "testing..."
`;
function insertTemplate(): void {
  text.value = TEMPLATE;
  push('> TEMPLATE_INSERTED — SAVE TO CREATE FILE_', 'accent');
}

// Unsaved edits should survive an accidental tab close.
function beforeUnload(e: BeforeUnloadEvent): void {
  if (dirty.value) e.preventDefault();
}
onMounted(() => window.addEventListener('beforeunload', beforeUnload));
onUnmounted(() => window.removeEventListener('beforeunload', beforeUnload));
</script>

<template>
  <div class="col" data-test-id="config-view">
    <!-- toolbar -->
    <div class="panel controls" data-test-id="config-toolbar">
      <span class="dim">SOURCE:</span>
      <span class="filename" data-test-id="editor-filename">{{ filename }}</span>
      <span v-if="dirty" class="accent soft-pulse" data-test-id="editor-dirty">* MODIFIED</span>
      <span class="dim">|</span>
      <span class="dim">SYNTAX:</span>
      <span :class="valState.cls" data-test-id="editor-validation">{{ valState.label }}_</span>
      <span class="grow"></span>
      <button
        class="btn btn-accent"
        data-test-id="editor-save"
        :disabled="!dirty || saving"
        title="WRITE FILE (CTRL+S)"
        @click="onSave"
      >
        <Icon :name="saving ? 'spinner' : 'download'" :spin="saving" />
        {{ saving ? 'WRITING...' : 'SAVE' }}
      </button>
      <button
        v-if="!reloadPending"
        class="btn btn-sq"
        data-test-id="editor-reload"
        title="RE-READ FILE FROM DISK"
        @click="onReload"
      >
        <Icon name="refresh" /> RELOAD
      </button>
      <template v-else>
        <button
          class="btn btn-sq btn-error"
          data-test-id="editor-reload-confirm"
          title="Discard edits and re-read from disk"
          @click="onReload"
        >
          <Icon name="warning" /> DISCARD_EDITS?
        </button>
        <button
          class="btn btn-sq"
          data-test-id="editor-reload-cancel"
          title="Keep editing"
          @click="cancelReload"
        >
          KEEP_EDITING
        </button>
      </template>
    </div>

    <!-- discard preview: what reload would throw away -->
    <section
      v-if="reloadPending && pendingDiff.length"
      class="panel panel-error diff-panel"
      data-test-id="reload-diff"
    >
      <div class="panel-hd">
        <span>UNSAVED_CHANGES</span>
        <span class="dim" style="font-weight: normal">DISK vs BUFFER — DISCARD LOSES THE + LINES</span>
      </div>
      <div class="diff-body">
        <template v-for="(d, i) in pendingDiff" :key="i">
          <div v-if="d.kind === 'gap'" class="dim diff-gap">··· {{ d.count }} UNCHANGED LINE{{ d.count === 1 ? '' : 'S' }} ···</div>
          <div v-else :class="['diff-line', d.kind]">
            <span class="sign">{{ d.kind === 'add' ? '+' : d.kind === 'del' ? '-' : ' ' }}</span>{{ d.text || ' ' }}
          </div>
        </template>
      </div>
    </section>

    <div v-if="stale && dirty" class="banner" data-test-id="editor-stale">
      <span class="accent">CONFIG SOURCE CHANGED — RELOAD TO LOAD {{ filename }} (DISCARDS EDITS)_</span>
    </div>
    <div v-if="error" class="banner" data-test-id="editor-error">
      <span class="error">ERROR: {{ error }}</span>
    </div>

    <div class="cfg-work">
      <!-- editor -->
      <section class="panel editor-panel" data-test-id="yaml-editor">
        <div class="panel-hd">
          <span>YAML_EDITOR</span>
          <span class="dim" style="font-weight: normal">{{ lineCount }} LINES</span>
          <span style="flex: 1"></span>
          <button
            v-if="!text && !loading"
            class="log-ctl"
            data-test-id="editor-template"
            title="Insert a starter pipeline"
            @click="insertTemplate"
          >
            <Icon name="plus" /> INSERT_TEMPLATE
          </button>
        </div>

        <div v-if="loading" class="empty">
          <span class="dim">&gt; READING_FILE_</span><span class="blink accent">_</span>
        </div>

        <div v-else class="editor">
          <div ref="gutter" class="gutter">
            <div
              v-for="n in lineCount"
              :key="n"
              class="ln"
              :class="{ 'ln-bad': issueByLine.has(n) }"
              :title="issueByLine.get(n) ?? ''"
            >
              {{ n }}
            </div>
          </div>
          <div class="code-wrap">
            <!-- eslint-disable-next-line vue/no-v-html — own highlighter output, input is escaped -->
            <pre ref="hl" class="hl" aria-hidden="true" v-html="highlighted"></pre>
            <textarea
              ref="ta"
              v-model="text"
              class="src"
              data-test-id="editor-textarea"
              spellcheck="false"
              autocapitalize="off"
              autocomplete="off"
              autocorrect="off"
              wrap="off"
              @scroll="syncScroll"
              @keydown="onKeydown"
              @keyup="updateCaret"
              @click="updateCaret"
              @input="updateCaret"
            ></textarea>
          </div>
        </div>

        <div class="editor-status dim">
          <span>LN {{ caret.line }} · COL {{ caret.col }}</span>
          <span v-if="issues.length" class="error" data-test-id="lint-count">
            {{ issues.length }} LINT ISSUE{{ issues.length === 1 ? '' : 'S' }}
          </span>
          <span style="flex: 1"></span>
          <span>TAB = 2 SPACES · CTRL+S = SAVE</span>
        </div>
      </section>

      <!-- visualizer -->
      <div class="col" style="min-width: 0">
        <PipelineGraph
          :stages="previewStages"
          :focused-job="null"
          :loading="graphLoading"
          :error="null"
        />
        <section v-if="valErrors.length" class="panel panel-error" data-test-id="config-errors">
          <div class="panel-hd"><span>VALIDATION</span></div>
          <div v-for="(err, i) in valErrors" :key="i" class="error val-err">! {{ err }}</div>
        </section>
        <section v-if="issues.length" class="panel" data-test-id="lint-panel">
          <div class="panel-hd">
            <span>LINT</span>
            <span class="dim" style="font-weight: normal">LIVE · BEFORE_SAVE</span>
          </div>
          <button
            v-for="(issue, i) in issues"
            :key="i"
            class="lint-row"
            :data-test-id="`lint-issue-${i}`"
            :title="`Jump to line ${issue.line}`"
            @click="goToLine(issue.line)"
          >
            <span class="accent">LN {{ issue.line }}</span>
            <span class="lint-msg">{{ issue.message }}</span>
          </button>
        </section>
        <section class="panel" data-test-id="config-meta">
          <div class="panel-hd"><span>SOURCE_INFO</span></div>
          <div class="kv">
            <span class="k">PATH</span>
            <span class="v path">{{ config?.path ?? '—' }}</span>
            <span class="k">STAGES</span>
            <span class="v">{{ config?.stages?.length ?? 0 }}</span>
            <span class="k">JOBS</span>
            <span class="v">{{ config?.jobs?.length ?? 0 }}</span>
            <template v-if="config?.includes?.length">
              <span class="k">INCLUDES</span>
              <span class="v path">{{ config.includes.join(', ') }}</span>
            </template>
          </div>
          <div class="dim" style="margin-top: 0.6rem; font-size: var(--fs-small)">
            PREVIEW REFLECTS THE SAVED FILE — SAVE TO UPDATE_
          </div>
        </section>
      </div>
    </div>
  </div>
</template>

<style scoped>
.filename {
  text-transform: none; /* real filename, case preserved */
}
.cfg-work {
  display: grid;
  grid-template-columns: minmax(0, 1.5fr) minmax(300px, 1fr);
  gap: 1rem;
  min-height: 0;
}
@media (max-width: 1100px) {
  .cfg-work {
    grid-template-columns: 1fr;
  }
}

.editor-panel {
  display: flex;
  flex-direction: column;
}

/* The textarea and the highlight <pre> must share every metric that affects
   glyph position — font, size, line-height, letter-spacing, padding — or the
   caret drifts off the colored text. */
.editor {
  --ed-font-size: 1.1rem;
  --ed-line-height: 1.35;
  display: flex;
  height: 62vh;
  min-height: 320px;
  border: 2px solid var(--term-dim);
  background: rgba(0, 0, 0, 0.45);
}
.gutter {
  flex-shrink: 0;
  overflow: hidden;
  padding: 0.5rem 0;
  border-right: 2px solid var(--term-dim);
  background: rgba(0, 0, 0, 0.35);
  text-align: right;
  user-select: none;
}
.gutter .ln {
  padding: 0 0.55rem;
  color: var(--term-dim);
  font-size: var(--ed-font-size);
  line-height: var(--ed-line-height);
  letter-spacing: 0.5px;
  text-shadow: none;
}
.code-wrap {
  position: relative;
  flex: 1;
  min-width: 0;
}
.hl,
.src {
  position: absolute;
  inset: 0;
  margin: 0;
  border: none;
  padding: 0.5rem 0.7rem;
  font-family: var(--font-mono);
  font-size: var(--ed-font-size);
  line-height: var(--ed-line-height);
  letter-spacing: 0.5px;
  text-transform: none; /* YAML is case-sensitive — defeat the global uppercase */
  white-space: pre;
  word-break: normal;
  overflow-wrap: normal;
  tab-size: 2;
}
.hl {
  overflow: hidden;
  pointer-events: none;
  color: var(--term-fg);
}
.src {
  overflow: auto;
  resize: none;
  background: transparent;
  color: transparent; /* the <pre> behind paints the glyphs */
  caret-color: var(--term-fg);
  outline: none;
  scrollbar-color: var(--term-dim) transparent;
  scrollbar-width: thin;
}
.src::selection {
  background: rgba(255, 176, 0, 0.3);
  color: transparent;
}
.src:focus {
  box-shadow: inset 0 0 14px rgba(0, 0, 0, 0.6), inset 0 0 6px var(--term-glow);
}

.editor-status {
  display: flex;
  gap: 1rem;
  padding-top: 0.4rem;
  font-size: var(--fs-small);
  letter-spacing: 1px;
}
.val-err {
  margin-bottom: 0.4rem;
  word-break: break-word;
}
.kv .v.path {
  text-transform: none;
}

.gutter .ln.ln-bad {
  color: var(--term-error);
  text-shadow: 0 0 6px var(--term-glow-error);
  cursor: help;
}

.lint-row {
  display: flex;
  gap: 0.7rem;
  width: 100%;
  background: transparent;
  border: none;
  border-left: 2px solid var(--term-dim);
  color: var(--term-fg);
  font-family: inherit;
  font-size: var(--fs-small);
  letter-spacing: 0.5px;
  text-align: left;
  padding: 0.15rem 0.5rem;
  cursor: pointer;
}
.lint-row:hover {
  border-left-color: var(--term-accent);
  background: rgba(255, 176, 0, 0.07);
}
.lint-msg {
  text-transform: none; /* messages quote case-sensitive names */
}

.diff-panel .diff-body {
  max-height: 14rem;
  overflow: auto;
  font-size: 1rem;
  line-height: 1.3;
}
.diff-line {
  white-space: pre;
  text-transform: none;
}
.diff-line .sign {
  display: inline-block;
  width: 1.2rem;
}
.diff-line.add {
  color: var(--term-fg);
  background: rgba(51, 255, 0, 0.08);
}
.diff-line.del {
  color: var(--term-error);
  background: rgba(255, 51, 51, 0.08);
}
.diff-line.same {
  color: var(--term-dim);
}
.diff-gap {
  padding: 0.1rem 0;
  letter-spacing: 2px;
}
</style>
