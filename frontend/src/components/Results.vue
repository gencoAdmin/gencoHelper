<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { rateKeys, cashRates, type EvaluationRevision, type Metric } from '../types'
import { money, number, percent } from '../api'
import Icon from './Icon.vue'
import Chart from './Chart.vue'
const props = defineProps<{ evaluation: EvaluationRevision }>(); const { t } = useI18n(); const channel = defineModel<string>('channel', { default: '' }); const showRates = ref(false)
const result = computed(() => props.evaluation.results.find(r => r.channel === (channel.value || props.evaluation.recommended)) || props.evaluation.results[0])
const formatMetric = (m: Metric, type: string) => m.value == null ? t(m.status) : type === 'money' ? money(m.value) : type === 'percent' ? percent(m.value) : number(m.value, type === 'roas' ? 2 : 0) + (type === 'roas' ? '×' : '')
const metrics = computed(() => [ ['contributionPerOrder', money(result.value.contributionPerOrder)], ['profitPerOrder', money(result.value.profitPerOrder)], ['successfulSales', number(result.value.successfulSales, 1)], ['breakEvenOrders', formatMetric(result.value.breakEvenOrders, 'number')], ['breakEvenPrice', formatMetric(result.value.breakEvenPrice,'money')], ['maxSupplierCost', formatMetric(result.value.maxSupplierCost,'money')], ['priceBuffer', formatMetric(result.value.priceBuffer,'percent')], ['maxAdvertising', formatMetric(result.value.maxAdvertising,'percent')], ['breakEvenROAS', formatMetric(result.value.breakEvenROAS,'roas')], ['targetPrice', formatMetric(result.value.targetPrice,'money')], ['targetCost', formatMetric(result.value.targetCost,'money')] ])
</script>
<template>
 <section class="result-hero"><div><span class="eyebrow light">{{ t('analysisChannel') }}</span><h2>{{ t(result.channel) }}<Icon name="arrowUp" :size="28" /></h2><p>{{ t('evaluatedPrice') }} · {{ money(result.price) }}</p></div><div class="hero-numbers"><span>{{ t('profit') }}</span><strong>{{ money(result.monthlyProfit) }}</strong><span>{{ percent(result.margin) }} {{ t('margin').toLowerCase() }}</span></div></section>
 <section class="panel comparison-panel"><div class="panel-heading"><div><h2>{{ t('compare') }}</h2><p>{{ t('compareSub') }}</p></div></div><div class="table-wrap"><table class="comparison-table"><thead><tr><th>{{ t('channel') }}</th><th>{{ t('contributionPerOrder') }}</th><th>{{ t('profit') }}</th><th>{{ t('margin') }}</th><th>{{ t('breakEvenOrders') }}</th><th>{{ t('verdict') }}</th></tr></thead><tbody><tr v-for="r in evaluation.results" :key="r.channel" :class="{ recommended: r.channel === evaluation.recommended, focused: r.channel === result.channel }" @click="channel = r.channel"><td><button class="channel-button" type="button" :aria-pressed="r.channel === result.channel" @click.stop="channel = r.channel"><span class="channel-label"><i :class="`channel-${r.channel}`"></i>{{ t(r.channel) }}</span><Icon v-if="r.channel === evaluation.recommended" name="check" :size="16" /></button></td><td>{{ money(r.contributionPerOrder) }}</td><td :class="Number(r.monthlyProfit) < 0 ? 'negative' : 'positive'">{{ money(r.monthlyProfit) }}</td><td>{{ percent(r.margin) }}</td><td>{{ formatMetric(r.breakEvenOrders, 'number') }}</td><td><span class="badge" :class="r.verdict">{{ t(r.verdict) }}</span></td></tr></tbody></table></div></section>
 <div class="result-focus-heading"><h2>{{ t(result.channel) }}</h2><span class="badge" :class="result.verdict">{{ t(result.verdict) }}</span></div><div class="verdict-note" :class="result.verdict"><Icon name="info" /><p>{{ t(`reason_${result.reason}`) }}</p></div>
 <div class="metrics-grid"><div v-for="[key,val] in metrics" :key="key" class="metric-card"><span>{{ t(key) }}</span><strong>{{ val }}</strong></div></div>
 <section class="panel rate-editor"><button class="disclosure-heading" @click="showRates = !showRates"><div><h3>{{ t('assumptions') }} · {{ t(result.channel) }}</h3><p>{{ t('presetVersion') }} {{ evaluation.presetVersion }}</p></div><Icon name="chevron" :class="{ turn: showRates }" /></button><div v-if="showRates" class="form-grid rates-grid"><div v-for="k in rateKeys" :key="k" class="field"><label>{{ t(k) }}</label><strong>{{ cashRates.includes(k) ? money(result.rates[k]) : percent(result.rates[k]) }}</strong><small>{{ t(result.sources[k] === 'product' ? 'override' : result.sources[k] === 'category' ? 'categoryDefault' : 'preset') }}</small></div></div></section>
 <div class="chart-grid"><section class="panel chart-panel"><h3>{{ t('costWaterfall') }}</h3><Chart :result="result" type="waterfall" /></section><section class="panel chart-panel"><h3>{{ t('volumeChart') }}</h3><p>{{ t('volumeHint') }}</p><Chart :result="result" type="volume" /></section></div><p class="calculation-footnote">{{ t('taxNote') }} · {{ t('presetVersion') }} {{ evaluation.presetVersion }} · v{{ evaluation.calculationVersion }}</p>
</template>

<style scoped>
.comparison-table tbody tr { cursor: pointer; }
.comparison-table .focused td { background: var(--lime); box-shadow: inset 0 2px var(--green), inset 0 -2px var(--green); }
.comparison-table .focused td:first-child { box-shadow: inset 3px 0 var(--green), inset 0 2px var(--green), inset 0 -2px var(--green); }
.comparison-table .focused td:last-child { box-shadow: inset -2px 0 var(--green), inset 0 2px var(--green), inset 0 -2px var(--green); }
.channel-button[aria-pressed="true"] { font-weight: 600; }
.channel-button:focus-visible { outline: 2px solid var(--green); outline-offset: 3px; }
</style>
