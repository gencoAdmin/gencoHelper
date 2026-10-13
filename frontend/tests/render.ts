import assert from 'node:assert/strict'
import { createSSRApp, createRenderer, h, ref, nextTick } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import App from '../src/App.vue'
import Products from '../src/pages/Products.vue'
import Calculator from '../src/pages/Calculator.vue'
import Import from '../src/pages/Import.vue'
import Settings from '../src/pages/Settings.vue'
import Results from '../src/components/Results.vue'
import { i18n } from '../src/locales'
import { useApp } from '../src/stores/app'
import { emptyInput, type EvaluationRevision, type ChannelResult } from '../src/types'
import { canonicalJSON } from '../src/api'
import { marketSummary, validPrice, researchLink } from '../src/lib/research'
import ResearchInsights from '../src/components/ResearchInsights.vue'

const expected: Record<string,string[]> = {
 en: ['Your next great product starts here.', 'Research the market. Test the profit.', 'Research the market. Test the profit.', 'From spreadsheet to insight.', 'Better assumptions. Better decisions.'],
 id: ['Produk hebat berikutnya dimulai di sini.', 'Riset pasar. Uji laba.', 'Riset pasar. Uji laba.', 'Dari spreadsheet ke wawasan.', 'Asumsi lebih baik. Keputusan lebih baik.'],
 zh: ['从这里发现下一款好产品。', '研究市场，测试利润。', '研究市场，测试利润。', '从表格到洞察。', '更好的假设，更好的决策。'],
}
const m = { status: 'ok', value: '10' } as const
const result: ChannelResult = { channel:'shelf',rates:{},sources:{},price:'50000',successfulSales:'1000',revenue:'50000000',shippingIncome:'0',contribution:'8000000',contributionPerOrder:'8000',profitPerOrder:'2700',monthlyProfit:'2700000',margin:'5.4',breakEvenOrders:{status:'ok',value:'663'},breakEvenPrice:{status:'ok',value:'46448'},maxSupplierCost:m,priceBuffer:m,maxAdvertising:m,breakEvenROAS:m,targetPrice:m,targetCost:m,verdict:'thin',reason:'thin',breakdown:[],volume:[] }
const evaluation: EvaluationRevision = { id:'fixture',input:emptyInput(),presetVersion:1,calculationVersion:'1.0.0',results:[result],recommended:'shelf',createdAt:'2026-10-12T00:00:00Z' }
export async function run(DecimalInput: any) {
 const competitor = (price: string) => ({ id:'', shop:'Test shop', platform:'Shopee', price, sales:'50', rating:'4.8', url:'' })
 const summary = marketSummary(['0','bad','50000.00000001','70000.00000002'].map(competitor))
 assert.equal(summary.count,2)
 assert.equal(summary.median,'60000.00000002','Median must preserve exact decimal precision')
 assert.equal(summary.sales,100,'Zero and invalid prices must be excluded from market evidence')
 assert.equal(marketSummary(['30000','90000','50000'].map(competitor)).median,'50000')
 assert.equal(marketSummary([]).median,'')
 assert.equal(validPrice('1000000000000.00000001'),false)
 assert.equal(validPrice('0.00000001'),true)
 assert.equal(researchLink('javascript:alert(1)'),undefined)
 assert.equal(researchLink('https://example.com/item'),'https://example.com/item')
 const decision = async (target: string | null, min: string, max: string) => renderToString(createSSRApp(ResearchInsights,{evaluation:{...evaluation,input:{...emptyInput(),marketMin:min,marketMax:max},results:[{...result,targetPrice:{status:target ? 'ok' : 'unattainable',value:target}}]},competitors:[]}).use(i18n))
 i18n.global.locale.value='en'
 assert.ok((await decision('60000','40000','50000')).includes('above the observed market ceiling'))
 assert.ok((await decision('40000','40000','50000')).includes('reached at the observed market floor'))
 assert.ok((await decision('45000','40000','50000')).includes('within the observed range'))
 assert.ok((await decision(null,'40000','50000')).includes('unattainable with these rates'))
 assert.ok((await decision('45000','','')).includes('Add comparable competitor prices'))

 // Go sorts JSON map keys; user edits can insert overrides in any order.
 const inputA={...emptyInput(),overrides:{shelf:{commission:'11',advertising:'4'}}}
 const inputB={...emptyInput(),overrides:{shelf:{advertising:'4',commission:'11'}}}
 assert.equal(canonicalJSON(inputA),canonicalJSON(inputB),'Map order must not disable saving a current calculation')
 // Exercise the real component's input handler without browser sockets.
 const renderer = createRenderer<any, any>({
  createElement: (tag) => ({ tag, props: {}, children: [] }), createText: (text) => ({ text }), createComment: (text) => ({ text }),
  setText: (node, text) => { node.text = text }, setElementText: (node, text) => { node.text = text },
  parentNode: () => null, nextSibling: () => null, remove: () => {},
  insert: (node, parent) => { parent.children.push(node) }, patchProp: (node, key, _old, value) => { node.props[key] = value },
 })
 const amount = ref('30000');const root = { children: [] as any[] }
 const modelApp = renderer.createApp({ setup: () => () => h(DecimalInput,{modelValue:amount.value,'onUpdate:modelValue':(value: string) => { amount.value=value }}) })
 modelApp.mount(root);root.children[0].props.onInput({target:{value:'999999999999.12345678'}});await nextTick()
 assert.equal(amount.value,'999999999999.12345678');assert.equal(typeof amount.value,'string');modelApp.unmount()
 for (const lang of ['en','id','zh']) {
  i18n.global.locale.value = lang
  for (const [index,path] of ['/','/calculator','/research','/import','/settings'].entries()) {
   const router = createRouter({ history:createMemoryHistory(),routes:[{path:'/',component:Products},{path:'/calculator',redirect:'/research'},{path:'/research',component:Calculator},{path:'/import',component:Import},{path:'/settings',component:Settings}] })
   await router.push(path); await router.isReady()
   const pinia=createPinia();const app=createSSRApp(App).use(pinia).use(router).use(i18n);useApp(pinia).settings={version:1,channels:{},categories:['general'],categoryReturns:{general:''},targetMargin:'15',healthyMargin:'15',healthyBuffer:'10',viableMargin:'8',updatedAt:''}
   const html=await renderToString(app);assert.ok(html.includes(expected[lang][index]),`${lang} ${path}: missing page title`);assert.ok(!html.includes('undefined'),`${lang} ${path}: undefined text`); assert.ok(!html.includes('to="/calculator"'), 'Navigation must use the unified workspace')
  }
  const html = await renderToString(createSSRApp(Results,{evaluation}).use(i18n))
  assert.ok(html.includes('Rp 2,700,000'),`${lang}: result profit missing`)
  assert.ok(html.includes('663'),`${lang}: break-even orders missing`)
 }
 console.log('Passed: exact decimal input event handling, 15 page renders across all three languages, financial result rendering, exact market summaries, safe research links, and market decision guidance.')
}
