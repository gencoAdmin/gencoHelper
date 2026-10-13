<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import * as echarts from 'echarts'
import type { ChannelResult } from '../types'
import { money, number } from '../api'
const props = defineProps<{ result: ChannelResult; type: string }>(); const { t, locale } = useI18n(); const el = ref<HTMLElement>(); let chart: echarts.ChartInstance | undefined; let observer: ResizeObserver | undefined
function draw() {
 if (!chart) return
 const common = { textStyle: { fontFamily: 'Arial, sans-serif', fontSize: 11, color: '#718078' }, grid: { left: 65, right: 20, top: 18, bottom: props.type === 'waterfall' ? 115 : 40 }, tooltip: { trigger: 'axis' as const, valueFormatter: (value: unknown) => money(String(value)) }, yAxis: { type: 'value' as const, splitLine: { lineStyle: { color: '#edf0e9' } }, axisLabel: { formatter: (v: number) => Math.abs(v) >= 1e6 ? `${number(v / 1e6,1)}M` : `${number(v / 1000,0)}k` } } }
 if (props.type === 'volume') chart.setOption({ ...common, xAxis: { type: 'category', data: props.result.volume.map(p => number(p.orders)), boundaryGap: false, axisLine: { lineStyle: { color: '#e1e6de' } } }, series: [{ type: 'line', smooth: false, data: props.result.volume.map(p => Number(p.profit)), symbolSize: 5, lineStyle: { color: '#3d785b', width: 3 }, itemStyle: { color: '#3d785b' }, areaStyle: { color: '#eff4e7', opacity: .8 }, markLine: { symbol: 'none', data: [{ yAxis: 0 }], lineStyle: { color: '#abaf9c', type: 'dashed' }, label: { show: false } } }] }, true)
 else {
  const breakdown = props.result.breakdown.filter(b => Number(b.value) !== 0); let sum = 0
  const ranges = breakdown.map(b => { const start = sum; const change = Number(b.value); sum += change; return { start, end: sum, color: change >= 0 ? '#719572' : '#d7ba93' } })
  ranges.push({ start: 0, end: sum, color: sum >= 0 ? '#214e3d' : '#ad6154' })
  // ECharts stacks positive and negative bars separately. Split intervals at
  // zero so losses and steps crossing zero keep their correct vertical range.
  const positiveBase = ranges.map(r => Math.max(0, Math.min(r.start, r.end)))
  const negativeBase = ranges.map(r => Math.min(0, Math.max(r.start, r.end)))
  const positive = ranges.map((r,i) => ({ value: Math.max(0, Math.max(r.start,r.end)) - positiveBase[i], itemStyle: { color: r.color } }))
  const negative = ranges.map((r,i) => ({ value: Math.min(0, Math.min(r.start,r.end)) - negativeBase[i], itemStyle: { color: r.color } }))
  const invisible = { type: 'bar', stack: 'total', silent: true, itemStyle: { color: 'transparent' }, emphasis: { itemStyle: { color: 'transparent' } } }
  chart.setOption({ ...common, tooltip: { trigger: 'axis', formatter: (params: unknown) => { const p = (params as {dataIndex: number}[])[0]; if (!p) return ''; const b = breakdown[p.dataIndex]; return `${t(b?.key || 'profit')}: ${money(b?.value || sum)}` } }, xAxis: { type: 'category', data: [...breakdown.map(b => t(b.key)), t('profit')], axisLabel: { interval: 0, rotate: 45, fontSize: 10 }, axisLine: { lineStyle: { color: '#e1e6de' } } }, series: [{ ...invisible, data: positiveBase }, { ...invisible, data: negativeBase }, { type: 'bar', stack: 'total', data: positive, barMaxWidth: 27, itemStyle: { borderRadius: 3 } }, { type: 'bar', stack: 'total', data: negative, barMaxWidth: 27, itemStyle: { borderRadius: 3 } }] }, true)
 }
}
watch([() => props.result, locale], draw, { deep: true }); onMounted(() => { if (el.value) { chart = echarts.init(el.value); observer = new ResizeObserver(() => chart?.resize()); observer.observe(el.value); draw() } }); onUnmounted(() => { observer?.disconnect(); chart?.dispose() })
</script>
<template><div ref="el" class="chart" role="img" :aria-label="t(type === 'volume' ? 'volumeChart' : 'costWaterfall')"></div></template>
