<script setup lang="ts">
import DecimalInput from '../components/DecimalInput.vue'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { api, json, ApiError, canonicalJSON } from '../api'
import { type Product, type EvaluationRevision, type ImportRow } from '../types'
import { useApp } from '../stores/app'
import { useResearchDrafts, type ResearchDraft } from '../lib/researchDrafts'
import Icon from '../components/Icon.vue'
import CategorySelect from '../components/CategorySelect.vue'
import RateEditor from '../components/RateEditor.vue'
import ResearchFields from '../components/ResearchFields.vue'
import Results from '../components/Results.vue'
import ResearchInsights from '../components/ResearchInsights.vue'
import ImportProducts from '../components/ImportProducts.vue'
const { t } = useI18n(); const route = useRoute(); const router = useRouter(); const app = useApp()
const { drafts, activeId, active, add, importRows, initializeMargin } = useResearchDrafts()
const input = computed({ get: () => active.value.input, set: value => { active.value.input = value } })
const competitors = computed(() => active.value.competitors)
const files = computed(() => active.value.files)
const removedImages = computed(() => active.value.removedImages)
const visibleImages = computed(() => active.value.existingImages.filter(im => !removedImages.value.includes(im.id)))
const evaluation = computed(() => active.value.evaluation)
const calculationError = computed(() => active.value.calculationError)
const localFields = computed(() => active.value.localFields)
const editing = computed(() => typeof route.params.id === 'string')
const step = ref(0); const calculating = ref(false); const saving = ref(false); const initialized = ref(false)
const reference = ref<{ url: string; name: string }>()
const showImport = ref(!editing.value && route.query.mode === 'bulk')
const showSave = ref(false); const selectedDrafts = ref<number[]>([])
const unsaved = computed(() => drafts.value.filter(d => !d.saved))
const pricingFields = ['price','cost','packaging'] as const
const fresh = computed(() => evaluation.value && canonicalJSON(evaluation.value.input) === canonicalJSON(input.value))
let timer: ReturnType<typeof setTimeout> | undefined; let sequence = 0
async function calculate() {
 clearTimeout(timer); const token = ++sequence; const draft = active.value; calculating.value = true
 if (draft.saved) { calculating.value = false; return }
 try {
  const res = await api<EvaluationRevision>('/calculate', json('POST', draft.input))
  if (token === sequence) { draft.evaluation = res; draft.calculationError = ''; draft.localFields = {} }
 } catch(e) {
  if (token === sequence) { draft.calculationError = e instanceof Error ? e.message : String(e); draft.localFields = e instanceof ApiError ? e.fields : {}; draft.evaluation = undefined }
 } finally { if (token === sequence) calculating.value = false }
}
watch(input, () => {
 if (!initialized.value) return
 clearTimeout(timer); sequence++; calculating.value = true; timer = setTimeout(calculate, 350)
}, { deep: true, flush: 'sync' })
watch(activeId, () => { reference.value = undefined; app.clear(); if (initialized.value) void calculate() }, { flush: 'sync' })
onMounted(async () => {
 try {
  if (!app.settings) await app.loadSettings()
  if (editing.value) {
   const p = await api<Product>(`/products/${route.params.id}`)
   input.value = { ...p.input, overrides: p.input.overrides || {} }
   active.value.competitors = p.competitors || []; active.value.existingImages = p.images; active.value.version = p.version; active.value.productId = p.id
   active.value.channel = p.revisions[p.revisions.length - 1]?.selectedChannel || ''
  } else initializeMargin(app.settings?.targetMargin || '15')
  initialized.value = true; await calculate()
 } catch(e) { app.report(e) }
})
onUnmounted(() => { sequence++; clearTimeout(timer); drafts.value.forEach(d => d.files.forEach(f => URL.revokeObjectURL(f.url))) })
function addProduct() { add(app.settings?.targetMargin || '15'); step.value = 0 }
function receiveImport(rows: ImportRow[]) { importRows(rows); showImport.value = false; step.value = 0 }
function chooseImages(event: Event) {
 const chosen = Array.from((event.target as HTMLInputElement).files || [])
 if (chosen.length + files.value.length + visibleImages.value.length > 5 || chosen.some(f => f.size > 5 * 1024 * 1024 || !['image/jpeg','image/png','image/webp'].includes(f.type))) { app.report(new Error(t('invalidImage'))); return }
 chosen.forEach(file => files.value.push({ file, url: URL.createObjectURL(file) })); (event.target as HTMLInputElement).value = ''
}
function removeFile(i: number) { if (reference.value?.url === files.value[i].url) reference.value = undefined; URL.revokeObjectURL(files.value[i].url); files.value.splice(i,1) }
function openSave() {
 if (drafts.value.length === 1) { selectedDrafts.value = [activeId.value]; void saveSelected(); return }
 selectedDrafts.value = unsaved.value.map(d => d.id); showSave.value = true
}
function selectAll(event: Event) { selectedDrafts.value = (event.target as HTMLInputElement).checked ? unsaved.value.map(d => d.id) : [] }
async function saveDraft(draft: ResearchDraft) {
 // Remember successful creation immediately so retries after an image failure
 // update the existing product rather than creating another evaluation.
 let p = await api<Product>(draft.productId ? `/products/${draft.productId}` : '/products', json(draft.productId ? 'PUT' : 'POST', { version: draft.version, input: draft.input, competitors: draft.competitors, channel: draft.channel }))
 draft.productId = p.id; draft.version = p.version
 while (draft.removedImages.length) {
  p = await api<Product>(`/products/${p.id}/images/${draft.removedImages[0]}`, json('DELETE', { version: p.version }))
  draft.version = p.version; draft.removedImages.shift()
 }
 while (draft.files.length) {
  const item = draft.files[0]; const form = new FormData(); form.set('file', item.file); form.set('version', String(p.version))
  p = await api<Product>(`/products/${p.id}/images`, { method: 'POST', body: form })
  draft.version = p.version; URL.revokeObjectURL(item.url); draft.files.shift()
 }
 draft.existingImages = p.images; draft.evaluation = p.revisions[p.revisions.length - 1]; draft.saved = true
 return p
}
async function saveSelected() {
 if (saving.value) return
 const chosen = drafts.value.filter(d => selectedDrafts.value.includes(d.id) && !d.saved)
 if (!chosen.length) return
 // Validate all selected inputs before the first write, including products
 // whose analysis has not yet been opened in the dropdown.
 saving.value = true; clearTimeout(timer); sequence++; calculating.value = false; app.clear(); let currentDraft = chosen[0]
 try {
  const skus = new Set<string>()
  for (const draft of chosen) {
   currentDraft = draft
   if (!draft.input.name.trim()) throw new ApiError(422, t('basicError'), { name: t('required') })
   const sku = draft.input.sku.trim()
   if (sku && skus.has(sku)) throw new ApiError(422, t('duplicateDraftSKU'), { sku: t('duplicateDraftSKU') })
   if (sku) skus.add(sku)
   draft.evaluation = await api<EvaluationRevision>('/calculate', json('POST', draft.input))
  }
  let last: Product | undefined
  for (const draft of chosen) { currentDraft = draft; last = await saveDraft(draft); selectedDrafts.value = selectedDrafts.value.filter(id => id !== draft.id) }
  showSave.value = false
  if (drafts.value.length === 1 && last) await router.push(`/products/${last.id}`)
  else if (unsaved.value.length) activeId.value = unsaved.value[0].id
  app.success(t('savedSuccess'))
 } catch(e) {
  activeId.value = currentDraft.id; showSave.value = false
  currentDraft.localFields = e instanceof ApiError ? e.fields : {}
  step.value = Object.keys(currentDraft.localFields).some(key => key.startsWith('competitors.')) ? 1 : 0
  app.report(e)
 } finally { saving.value = false }
}
function reloadPage() { window.location.reload() }
</script>
<template><div class="calculator-page"><div class="page-heading"><div><span class="eyebrow"><span class="eyebrow-line"></span>{{ t('research') }}</span><h1>{{ t('researchTitle') }}</h1><p>{{ t('researchSub') }}</p></div><RouterLink to="/" class="btn secondary"><Icon name="back" />{{ t('products') }}</RouterLink></div>
 <section v-if="!editing" class="panel draft-toolbar"><div class="field"><label for="research-product">{{ t('researchProduct') }}</label><select id="research-product" v-model="activeId" :disabled="saving"><option v-for="(draft,i) in drafts" :key="draft.id" :value="draft.id">{{ draft.input.name || `${t('product')} ${i+1}` }}{{ draft.input.sku ? ` · ${draft.input.sku}` : '' }}{{ draft.saved ? ` · ${t('saved')}` : '' }}</option></select></div><div class="heading-actions"><button class="btn secondary" :disabled="!initialized || saving" @click="addProduct"><Icon name="plus" />{{ t('addProduct') }}</button><button class="btn secondary" :disabled="!initialized || saving" @click="showImport = true"><Icon name="upload" />{{ t('bulkResearch') }}</button></div><p class="muted small-text">{{ t('draftResearchHint') }}</p></section>
 <div :key="activeId">
 <div class="research-toolbar"><div class="segment-control" role="tablist" :aria-label="t('research')"><button id="product-tab" role="tab" :aria-selected="step === 0" aria-controls="product-inputs" :class="{ chosen: step === 0 }" @click="step = 0">{{ t('financials') }}</button><button id="market-tab" role="tab" :aria-selected="step === 1" aria-controls="market-inputs" :class="{ chosen: step === 1 }" @click="step = 1">{{ t('market') }} <span v-if="competitors.length">({{ competitors.length }})</span></button></div><a class="text-link" href="#live-results">{{ t('viewResults') }}<Icon name="arrow" :size="16" /></a></div>
 <fieldset class="research-fieldset" :disabled="!initialized || saving || active.saved"><div class="calculator-layout research-layout"><div class="calculator-form"><div id="product-inputs" v-show="step === 0" role="tabpanel" aria-labelledby="product-tab">
 <section class="panel form-panel">
  <div class="section-title"><span class="section-icon"><Icon name="box" /></span><div><h3>{{ t('commercial') }}</h3><p>{{ t('commercialSub') }}</p></div></div>
  <div class="form-grid">
   <div class="field span-2"><label for="name">{{ t('name') }} <span class="required-dot">*</span></label><input id="name" v-model="input.name" maxlength="200" :placeholder="t('namePlaceholder')" :class="{ invalid: app.fields.name || localFields.name }" /><small v-if="app.fields.name || localFields.name" class="negative">{{ app.fields.name || localFields.name }}</small></div>
   <div class="field"><label for="sku">{{ t('sku') }} <span>{{ t('optional') }}</span></label><input id="sku" v-model="input.sku" :class="{ invalid: app.fields.sku || localFields.sku }" maxlength="100" :placeholder="t('skuPlaceholder')" /><small v-if="app.fields.sku || localFields.sku" class="negative">{{ app.fields.sku || localFields.sku }}</small></div>
   <div class="field"><label for="category">{{ t('category') }}</label><CategorySelect id="category" v-model="input.category" /></div>
  </div>
 </section>
 <section class="panel form-panel">
  <div class="section-title"><span class="section-icon"><Icon name="calculator" /></span><div><h3>{{ t('pricingCosts') }}</h3><p>{{ t('pricingCostsHint') }}</p></div></div>
  <div class="form-grid">
   <div v-for="key in pricingFields" :key="key" class="field" :class="{ 'span-2': key === 'packaging' }">
    <label :for="key">{{ t(key) }}</label><div class="input-unit" :class="{ invalid: localFields[key] }"><span>Rp</span><DecimalInput :id="key" v-model="input[key]" min="0" step="any" /></div>
    <small v-if="localFields[key]" class="negative">{{ localFields[key] }}</small>
   </div>
  </div>
 </section>
 <section class="panel form-panel">
  <div class="section-title"><span class="section-icon"><Icon name="trend" /></span><div><h3>{{ t('monthlyPlan') }}</h3><p>{{ t('monthlyPlanHint') }}</p></div></div>
  <div class="form-grid">
   <div class="field"><label for="orders">{{ t('orders') }}</label><DecimalInput id="orders" v-model="input.orders" min="1" max="10000000" step="1" :class="{ invalid: localFields.orders }" /><small v-if="localFields.orders" class="negative">{{ localFields.orders }}</small></div>
   <div class="field"><label for="targetMargin">{{ t('targetMargin') }}</label><div class="input-unit suffix" :class="{ invalid: localFields.targetMargin }"><DecimalInput id="targetMargin" v-model="input.targetMargin" min="0" max="100" step="any" /><span>%</span></div><small v-if="localFields.targetMargin" class="negative">{{ localFields.targetMargin }}</small></div>
   <div class="field span-2"><label for="fixed">{{ t('fixed') }}</label><div class="input-unit" :class="{ invalid: localFields.fixed }"><span>Rp</span><DecimalInput id="fixed" v-model="input.fixed" min="0" step="any" /></div><small>{{ t('fixedHint') }}</small><small v-if="localFields.fixed" class="negative">{{ localFields.fixed }}</small></div>
  </div>
 </section>
 <details class="panel calculator-notes">
  <summary>{{ t('notes') }} <span class="muted">{{ t('optional') }}</span></summary>
  <div class="field"><label for="notes">{{ t('notes') }}</label><textarea id="notes" v-model="input.notes" maxlength="10000" rows="3" :placeholder="t('notesPlaceholder')"></textarea></div>
 </details>
 <RateEditor :input="input" /><button class="btn secondary full" @click="step = 1">{{ t('addMarketEvidence') }}<Icon name="arrow" :size="16" /></button></div>
 <div id="market-inputs" v-show="step === 1" role="tabpanel" aria-labelledby="market-tab"><ResearchFields :input="input" :competitors="competitors" /><section class="panel form-panel"><div class="section-title"><span class="section-icon"><Icon name="image" /></span><div><h3>{{ t('screenshots') }}</h3><p>{{ t('evidenceHint') }}</p><p>{{ t('screenshotsHint') }}</p></div></div><div class="image-grid"><div v-for="im in visibleImages" :key="im.id" class="image-card"><button class="image-preview" :aria-label="im.name" @click="reference = { url: im.url, name: im.name }"><img :src="im.thumbnail" :alt="im.name" /></button><button class="image-delete icon-btn" :aria-label="`${t('remove')} ${im.name}`" @click="removedImages.push(im.id); reference = undefined"><Icon name="close" :size="16" /></button></div><div v-for="(file,i) in files" :key="file.url" class="image-card"><button class="image-preview" :aria-label="file.file.name" @click="reference = { url: file.url, name: file.file.name }"><img :src="file.url" :alt="file.file.name" /></button><button class="image-delete icon-btn" :aria-label="t('remove')" @click="removeFile(i)"><Icon name="close" :size="16" /></button></div><label v-if="files.length + visibleImages.length < 5" class="image-upload"><Icon name="plus" /><span>{{ t('addImages') }}</span><input type="file" accept="image/jpeg,image/png,image/webp" multiple @change="chooseImages" /></label></div><div v-if="reference" class="evidence-reference"><div><strong>{{ reference.name }}</strong><button class="icon-btn" :aria-label="t('close')" @click="reference = undefined"><Icon name="close" /></button></div><img :src="reference.url" :alt="reference.name" /><a :href="reference.url" target="_blank" rel="noopener noreferrer" class="text-link">{{ t('openEvidence') }} ↗</a></div></section></div>
 </div><div id="live-results" class="calculator-results" :aria-busy="!fresh || calculating"><div v-if="!initialized" class="loading-state"><span class="spinner"></span>{{ t('loading') }}</div><div v-else-if="!evaluation" class="panel empty-state"><Icon name="calculator" :size="40" /><p>{{ calculationError }}</p><p v-for="(message,key) in localFields" :key="key">{{ t(key) }}: {{ message }}</p></div><template v-else><p class="live-label"><i></i>{{ t(!fresh || calculating ? 'updatingResults' : 'liveCalc') }}</p><div :class="{ 'results-updating': !fresh || calculating }"><ResearchInsights :key="activeId" v-model:channel="active.channel" :evaluation="evaluation" :competitors="competitors" :editable="Boolean(fresh) && !calculating && !saving" @price="input.price = $event" @cost="input.cost = $event" /><Results :key="activeId" v-model:channel="active.channel" :evaluation="evaluation" /></div></template></div></div>
 </fieldset>
 <div class="form-actions research-save"><span class="muted small-text">{{ t('saveResearchHint') }}</span><button class="btn primary" :disabled="saving || (drafts.length === 1 && (calculating || !fresh)) || !unsaved.length" @click="openSave"><Icon name="check" />{{ t(saving ? 'saving' : drafts.length > 1 ? 'chooseEvaluations' : 'save') }}</button></div><button v-if="app.conflict" class="btn secondary" @click="reloadPage">{{ t('reload') }}</button>
 </div>
 <div v-if="showImport" class="modal-backdrop" @click.self="showImport = false"><section class="modal import-modal" role="dialog" aria-modal="true" :aria-label="t('bulkResearch')"><button class="icon-btn modal-close" :aria-label="t('close')" @click="showImport = false"><Icon name="close" /></button><h2>{{ t('bulkResearch') }}</h2><ImportProducts @imported="receiveImport" /></section></div>
 <div v-if="showSave" class="modal-backdrop" @click.self="!saving && (showSave = false)"><section class="modal" role="dialog" aria-modal="true" :aria-label="t('chooseEvaluations')"><button class="icon-btn modal-close" :disabled="saving" :aria-label="t('close')" @click="showSave = false"><Icon name="close" /></button><h2>{{ t('chooseEvaluations') }}</h2><p class="muted">{{ t('selectDraftHint') }}</p><label class="draft-select-all"><input type="checkbox" :checked="selectedDrafts.length === unsaved.length && unsaved.length > 0" :disabled="saving" @change="selectAll" />{{ t('selectAll') }}</label><div class="draft-save-list"><div v-for="(draft,i) in drafts" :key="draft.id" class="draft-save-item"><label><input v-model="selectedDrafts" type="checkbox" :value="draft.id" :disabled="saving || draft.saved" /><span><strong>{{ draft.input.name || `${t('product')} ${i+1}` }}</strong><small>{{ draft.input.sku || '—' }} · {{ draft.saved ? t('saved') : t('unsavedDraft') }}</small></span></label><button type="button" class="text-link" :disabled="saving" @click="activeId = draft.id; showSave = false">{{ t('edit') }}</button></div></div><div class="form-actions"><button class="btn secondary" :disabled="saving" @click="showSave = false">{{ t('cancel') }}</button><button class="btn primary" :disabled="saving || !selectedDrafts.length" @click="saveSelected">{{ t(saving ? 'saving' : 'saveSelectedEvaluations') }} ({{ selectedDrafts.length }})</button></div></section></div>
</div></template>

<style>
.calculator-page {
 --calculator-gap: 24px;
 --calculator-padding: 24px;
}
.calculator-page .draft-toolbar {
 display: grid;
 grid-template-columns: minmax(0, 1fr) auto;
 gap: 12px 24px;
 padding: var(--calculator-padding);
 margin-bottom: var(--calculator-gap);
 align-items: end;
}
.calculator-page .draft-toolbar > .field { min-width: 0; }
.calculator-page .draft-toolbar > p { grid-column: 1 / -1; }
.calculator-page .heading-actions { flex-wrap: wrap; }
.calculator-page .research-toolbar { margin-bottom: var(--calculator-gap); }
.calculator-page .research-toolbar .segment-control { gap: 4px; }
.calculator-page .research-toolbar .segment-control button { padding: 10px 16px; }
.calculator-page .research-layout {
 grid-template-columns: minmax(380px, .9fr) minmax(0, 1.25fr);
 gap: var(--calculator-gap);
}
.calculator-page .calculator-form [role="tabpanel"] {
 display: flex;
 flex-direction: column;
 gap: var(--calculator-gap);
}
.calculator-page .calculator-form .form-panel {
 padding: var(--calculator-padding);
 margin: 0;
 overflow: visible;
}
.calculator-page .calculator-form .section-title {
 margin-bottom: 24px;
 gap: 12px;
 align-items: flex-start;
}
.calculator-page .calculator-form .section-title > div { min-width: 0; }
.calculator-page .calculator-form .section-title h3 { font-size: 15px; }
.calculator-page .calculator-form .section-title p { font-size: 12.5px; }
.calculator-page .form-grid {
 grid-template-columns: repeat(2, minmax(0, 1fr));
 gap: 20px;
 align-items: start;
}
.calculator-page .field { min-width: 0; gap: 8px; }
.calculator-page .field label { font-size: 12.5px; line-height: 1.5; color: var(--ink); }
.calculator-page label[for="name"] { justify-content: flex-start; gap: 4px; }
.calculator-page .field label > span:not(.required-dot) { font-size: 11.5px; color: var(--muted); font-weight: 400; }
.calculator-page .field input,
.calculator-page .field select,
.calculator-page .category-trigger { min-height: 44px; font-size: 13px; }
.calculator-page .field small { line-height: 1.6; }
.calculator-page .calculator-form .rate-editor { margin-bottom: 0; }
.calculator-page .calculator-form .disclosure-heading { padding: var(--calculator-padding); }
.calculator-page .calculator-form .rates-grid { padding: 0 var(--calculator-padding) var(--calculator-padding); }
.calculator-page .calculator-form .rate-editor > .channel-tabs { padding: 0 var(--calculator-padding); }
.calculator-page .calculator-form .reset-link { margin: 0 var(--calculator-padding) var(--calculator-padding); }
.calculator-page .calculator-notes > summary {
 padding: 18px var(--calculator-padding);
 font-size: 13px;
 font-weight: 500;
 cursor: pointer;
}
.calculator-page .calculator-notes > summary span { margin-left: 8px; font-size: 11.5px; font-weight: 400; }
.calculator-page .calculator-notes > summary:focus-visible { outline: 2px solid var(--green); outline-offset: -4px; }
.calculator-page .calculator-notes > .field { padding: 0 var(--calculator-padding) var(--calculator-padding); }
.calculator-page .competitor-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
.calculator-page .competitor-form { padding: 20px 0; }
.calculator-page .competitor-number { display: none; }
.calculator-page .competitor-summary { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 20px; }
.calculator-page .research-save { position: static; margin-top: var(--calculator-gap); padding: 16px var(--calculator-padding); gap: 16px; }
.calculator-page .research-save > .small-text { max-width: none; flex: 1; line-height: 1.6; }

@media (max-width: 1280px) {
 .calculator-page .research-layout { grid-template-columns: minmax(0, 1fr); }
 .calculator-page .calculator-layout .metrics-grid { grid-template-columns: repeat(4, minmax(0, 1fr)); }
}
@media (max-width: 700px) {
 .calculator-page { --calculator-gap: 20px; --calculator-padding: 20px; }
 .calculator-page .draft-toolbar { grid-template-columns: minmax(0, 1fr); gap: 16px; }
 .calculator-page .draft-toolbar .heading-actions { width: 100%; gap: 10px; }
 .calculator-page .draft-toolbar .heading-actions .btn { flex: 1; }
 .calculator-page .research-toolbar { flex-direction: row; align-items: center; gap: 10px; flex-wrap: wrap; }
 .calculator-page .research-toolbar .segment-control button { padding: 9px 12px; }
 .calculator-page .form-grid { grid-template-columns: minmax(0, 1fr); gap: 20px; }
 .calculator-page .form-grid > .span-2 { grid-column: 1 / -1; }
 .calculator-page .calculator-form .section-title { flex-wrap: wrap; margin-bottom: 20px; }
 .calculator-page .calculator-form .section-title > div { flex: 1; }
 .calculator-page .calculator-form .section-title > button { margin-left: 0; width: 100%; }
 .calculator-page .calculator-layout .metrics-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
 .calculator-page .research-save { position: static; flex-direction: column; align-items: stretch; }
 .calculator-page .research-save > .btn { width: 100%; }
}
</style>
