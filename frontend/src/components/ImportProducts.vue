<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { api, json } from '../api'
import type { Preview, ImportRow } from '../types'
import type { ImportUpload } from '../generated-api'
import { useApp } from '../stores/app'
import Icon from './Icon.vue'
const emit = defineEmits<{ imported: [rows: ImportRow[]] }>()
const { t, locale } = useI18n()
const app = useApp()
const step = ref(0)
const upload = ref<ImportUpload>()
const sheet = ref('')
const mapping = ref<Record<string, number>>({})
const preview = ref<Preview>()
const selected = ref<number[]>([])
const busy = ref(false)
const filename = ref('')
const displayLimit = ref(100)
const fields = ['name','sku','category','cost','price','orders','fixed','packaging','marketMin','marketMax','targetMargin','notes']
const required = ['name','cost','price','orders']
const headers = computed(() => upload.value?.sheets.find(s => s.name === sheet.value)?.headers || [])
const ready = computed(() => required.every(k => mapping.value[k] !== undefined))
const validRows = computed(() => preview.value?.rows.filter(r => !r.errors.length && r.evaluation) || [])
async function choose(event: Event) {
 const element = event.target as HTMLInputElement
 const file = element.files?.[0]
 if (!file || busy.value) return
 app.clear(); busy.value = true
 const form = new FormData(); form.set('file', file)
 try {
  upload.value = await api<ImportUpload>('/imports/upload', { method: 'POST', body: form })
  filename.value = file.name; sheet.value = upload.value.sheets[0].name; mapping.value = upload.value.mapping; step.value = 1
 } catch(e) { app.report(e) } finally { busy.value = false; element.value = '' }
}
function map(k: string, event: Event) {
 const v = (event.target as HTMLSelectElement).value
 if (v === '') delete mapping.value[k]; else mapping.value[k] = Number(v)
}
function changeSheet() {
 const h = headers.value; mapping.value = {}
 fields.forEach(k => { const i = h.findIndex(v => v.toLowerCase() === k.toLowerCase() || v === t(k)); if (i >= 0) mapping.value[k] = i })
}
async function makePreview() {
 if (!upload.value || busy.value) return
 busy.value = true; app.clear()
 try {
  preview.value = await api<Preview>('/imports/preview', json('POST', { uploadId: upload.value.id, sheet: sheet.value, mapping: mapping.value }))
  selected.value = validRows.value.map(r => r.row); displayLimit.value = 100; step.value = 2
 } catch(e) { app.report(e) } finally { busy.value = false }
}
function importRows() { emit('imported', validRows.value.filter(r => selected.value.includes(r.row))) }
function toggle(event: Event) { selected.value = (event.target as HTMLInputElement).checked ? validRows.value.map(r => r.row) : [] }
</script>
<template>
 <div class="bulk-intro"><p class="muted small-text">{{ t('bulkResearchHint') }}</p><a class="btn secondary compact" :href="`/api/v1/imports/template?locale=${locale}`"><Icon name="download" :size="16" />{{ t('template') }}</a></div>
 <div class="stepper"><button v-for="(key,i) in ['upload','mapping','preview']" :key="key" :class="{ current:step===i, done:step>i }" :disabled="busy || i > step" @click="step=i"><span><Icon v-if="step>i" name="check" :size="16" /><template v-else>{{ i+1 }}</template></span>{{ t(key) }}</button></div>
 <section v-if="step === 0" class="panel upload-panel">
  <div class="upload-orbit"><Icon name="file" :size="42" /><span><Icon name="upload" :size="18" /></span></div><h2>{{ t('uploadTitle') }}</h2><p>{{ t('uploadHint') }}</p>
  <label class="btn primary file-label"><Icon name="upload" />{{ t(busy ? 'loading' : 'chooseFile') }}<input type="file" accept=".xlsx" :disabled="busy" @change="choose" /></label>
  <a class="text-link" :href="`/api/v1/imports/template?locale=${locale}`">{{ t('template') }}<Icon name="arrow" :size="16" /></a>
 </section>
 <section v-if="step === 1 && upload" class="panel form-panel">
  <div class="section-title"><span class="section-icon"><Icon name="file" /></span><div><h3>{{ filename }}</h3><p>{{ t('mapping') }}</p></div></div>
  <div class="field worksheet-field"><label for="worksheet">{{ t('worksheet') }}</label><select id="worksheet" v-model="sheet" :disabled="busy" @change="changeSheet"><option v-for="s in upload.sheets" :key="s.name" :value="s.name">{{ s.name }} · {{ s.rowCount }} {{ t('row') }}</option></select></div>
  <div class="mapping-grid"><div v-for="k in fields" :key="k" class="mapping-row"><label :for="`map-${k}`">{{ t(k) }}<span v-if="required.includes(k)" class="required-label">{{ t('required') }}</span></label><Icon name="arrow" :size="16" /><select :id="`map-${k}`" :value="mapping[k] ?? ''" :disabled="busy" @change="map(k,$event)"><option value="">{{ t('skipColumn') }}</option><option v-for="(h,i) in headers" :key="i" :value="i">{{ String.fromCharCode(65+i) }} · {{ h }}</option></select></div></div>
  <div class="form-actions"><button class="btn secondary" :disabled="busy" @click="step=0">{{ t('previous') }}</button><button class="btn primary" :disabled="busy || !ready" @click="makePreview">{{ t(busy ? 'loading' : 'preview') }}<Icon name="arrow" /></button></div>
 </section>
 <section v-if="step === 2 && preview" class="panel">
  <div class="panel-heading"><div><h2>{{ t('preview') }}</h2><p>{{ t('importDraftHint') }}</p></div></div>
  <div class="table-wrap"><table class="import-table"><thead><tr><th><input type="checkbox" :checked="selected.length === validRows.length && validRows.length > 0" :aria-label="t('selectAll')" @change="toggle" /></th><th>{{ t('row') }}</th><th>{{ t('product') }}</th><th>{{ t('sku') }}</th><th>{{ t('validation') }}</th></tr></thead><tbody><tr v-for="r in preview.rows.slice(0,displayLimit)" :key="r.row"><td><input v-model="selected" type="checkbox" :value="r.row" :disabled="r.errors.length > 0" :aria-label="`${t('row')} ${r.row}`" /></td><td>{{ r.row }}</td><td>{{ r.input.name }}</td><td>{{ r.input.sku || '—' }}</td><td><span v-if="!r.errors.length" class="badge healthy">{{ t('valid') }}</span><ul v-else class="row-errors"><li v-for="e in r.errors" :key="e">{{ e }}</li></ul></td></tr></tbody></table></div>
  <div v-if="displayLimit < preview.rows.length" class="load-more"><button class="btn secondary compact" @click="displayLimit+=100">+ 100</button></div>
  <p v-if="!validRows.length" class="padded muted">{{ t('noValidImportRows') }}</p>
  <div class="form-actions padded"><button class="btn secondary" @click="step=1">{{ t('previous') }}</button><button class="btn primary" :disabled="!selected.length" @click="importRows">{{ t('addToResearch') }} ({{ selected.length }})<Icon name="arrow" /></button></div>
 </section>
</template>
