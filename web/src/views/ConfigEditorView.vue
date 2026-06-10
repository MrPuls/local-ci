<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import Icon from '@/components/Icon.vue';
import PipelineGraph from '@/components/PipelineGraph.vue';
import { useConfig } from '@/composables/useConfig';
import { useConfigRaw } from '@/composables/useConfigRaw';
import { useRunStatus } from '@/composables/useRunStatus';
import { useToast } from '@/composables/useToast';
import { highlightYaml } from '@/lib/yaml';
import { mergePipeline } from '@/lib/pipeline';
import { baseName } from '@/lib/format';

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
const highlighted = computed(() => highlightYaml(text.value) + '\n');

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

// Reload from disk — two-step confirm when it would discard edits.
const reloadPending = ref(false);
async function onReload(): Promise<void> {
  if (dirty.value && !reloadPending.value) {
    reloadPending.value = true;
    return;
  }
  reloadPending.value = false;
  await load();
  push('> CONFIG_RELOADED_', 'accent');
}

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
      <button
        v-else
        class="btn btn-sq btn-error"
        data-test-id="editor-reload-confirm"
        title="Discard edits and re-read from disk"
        @click="onReload"
      >
        <Icon name="warning" /> DISCARD_EDITS?
      </button>
    </div>

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
          <div ref="gutter" class="gutter" aria-hidden="true">
            <div v-for="n in lineCount" :key="n" class="ln">{{ n }}</div>
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
</style>
