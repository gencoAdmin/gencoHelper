<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { api, json, canonicalJSON, compareDecimals, money, percent } from '../api'
import type { Competitor, EvaluationRevision, ChannelResult } from '../types'
import { marketSummary, researchLink, validPrice } from '../lib/research'
import Icon from './Icon.vue'
const props = defineProps<{ evaluation: EvaluationRevision; competitors: Competitor[]; editable?: boolean }>()
const emit = defineEmits<{ price: [value: string]; cost: [value: string] }>()
const { t } = useI18n()
const summary = computed(() => marketSummary(props.competitors))
const result = computed(() => props.evaluation.results.find(r => r.channel === props.evaluation.recommended)!)
const bounds = computed(() => ({ min: props.evaluation.input.marketMin || summary.value.min, max: props.evaluation.input.marketMax || summary.value.max }))
const fit = computed(() => {
 const target = result.value.targetPrice.value
 if (!target) return 'targetImpossible'
 if (!validPrice(bounds.value.min) || !validPrice(bounds.value.max)) return 'marketMissing'
 return compareDecimals(target, bounds.value.max) > 0 ? 'targetAboveMarket' : compareDecimals(target, bounds.value.min) <= 0 ? 'targetWithinMarket' : 'targetNeedsPremium'
})
const scenarios = ref<Record<string, ChannelResult>>({}); const loading = ref(false); const failed = ref(false)
let sequence = 0; let timer: ReturnType<typeof setTimeout> | undefined
watch(() => canonicalJSON({ input: props.evaluation.input, version: props.evaluation.presetVersion, competitors: props.competitors.map(c => c.price) }), () => {
 const token = ++sequence; clearTimeout(timer); scenarios.value = {}; failed.value = false
 const prices = [...new Set(props.competitors.map(c => c.price).filter(validPrice))]
 loading.value = prices.length > 0
 if (!prices.length) return
 timer = setTimeout(async () => {
  const results = await Promise.allSettled(prices.map(async price => {
   // Use the saved rates so the comparison also works for a frozen evaluation.
   const overrides = Object.fromEntries(props.evaluation.results.map(r => [r.channel, r.rates]))
   const ev = await api<EvaluationRevision>('/calculate', json('POST', { ...props.evaluation.input, price, marketMin: '', marketMax: '', overrides }))
   return [price, ev.results.find(r => r.channel === props.evaluation.recommended)!] as const
  }))
  if (token !== sequence) return
  scenarios.value = Object.fromEntries(results.flatMap(r => r.status === 'fulfilled' ? [r.value] : []))
  failed.value = results.some(r => r.status === 'rejected'); loading.value = false
 }, 450)
}, { immediate: true })
onUnmounted(() => { sequence++; clearTimeout(timer) })
</script>
<template>
 <section class="panel research-insights">
  <div class="panel-heading"><div><h2>{{ t('decisionTitle') }}</h2><p>{{ t('decisionSub') }} · {{ t(evaluation.recommended) }}</p></div><Icon name="research" /></div>
  <div class="decision-body">
   <div class="decision-metrics"><div><span>{{ t('evaluatedPrice') }}</span><strong>{{ money(result.price) }}</strong></div><div><span>{{ t('targetPrice') }}</span><strong>{{ result.targetPrice.value == null ? t(result.targetPrice.status) : money(result.targetPrice.value) }}</strong></div><div><span>{{ t('competitorRange') }}</span><strong>{{ bounds.min && bounds.max ? `${money(bounds.min)} – ${money(bounds.max)}` : '—' }}</strong></div></div>
   <p class="decision-note" :class="{ negative: fit === 'targetAboveMarket' || fit === 'targetImpossible' }">{{ t(fit) }}</p>
   <div v-if="editable" class="research-actions"><button v-if="result.targetPrice.value && validPrice(result.targetPrice.value)" class="btn secondary compact" @click="emit('price', result.targetPrice.value)">{{ t('testTargetPrice') }}</button><button v-if="result.targetCost.value != null && compareDecimals(result.targetCost.value, evaluation.input.cost) < 0" class="btn secondary compact" @click="emit('cost', result.targetCost.value)">{{ t('testTargetCost') }} · {{ money(result.targetCost.value) }}</button></div>
   <p class="small-text muted">{{ t('marketEvidenceHint') }}</p>
  </div>
  <template v-if="summary.count">
   <div class="panel-heading"><div><h3>{{ t('competitorTestTitle') }}</h3><p>{{ t('competitorTestHint') }}</p></div></div>
   <p v-if="loading" class="scenario-status" role="status">{{ t('loading') }}</p><p v-if="failed" class="scenario-status negative" role="alert">{{ t('scenarioFailed') }}</p>
   <div class="table-wrap"><table class="comparison-table"><thead><tr><th>{{ t('shop') }}</th><th>{{ t('price') }}</th><th>{{ t('profit') }}</th><th>{{ t('margin') }}</th><th v-if="editable"></th></tr></thead><tbody><tr v-for="(c,i) in competitors.filter(c => validPrice(c.price))" :key="i"><td><a v-if="researchLink(c.url)" :href="researchLink(c.url)" target="_blank" rel="noopener noreferrer" class="text-link">{{ c.shop || c.platform }} ↗</a><span v-else>{{ c.shop || c.platform }}</span></td><td>{{ money(c.price) }}</td><td :class="{ negative: Number(scenarios[c.price]?.monthlyProfit) < 0 }">{{ money(scenarios[c.price]?.monthlyProfit) }}</td><td>{{ percent(scenarios[c.price]?.margin) }}</td><td v-if="editable"><button class="text-link" @click="emit('price', c.price)">{{ t('testPrice') }}</button></td></tr></tbody></table></div>
  </template>
 </section>
</template>
