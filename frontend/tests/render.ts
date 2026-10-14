import assert from 'node:assert/strict'
import { createSSRApp, createRenderer, h, ref, nextTick, watch } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import App from '../src/App.vue'
import Products from '../src/pages/Products.vue'
import Calculator from '../src/pages/Calculator.vue'
import Settings from '../src/pages/Settings.vue'
import Results from '../src/components/Results.vue'
import { i18n } from '../src/locales'
import { useApp } from '../src/stores/app'
import { emptyInput, type EvaluationRevision, type ChannelResult } from '../src/types'
import { api, ApiError, canonicalJSON } from '../src/api'
import { marketSummary, validPrice, researchLink } from '../src/lib/research'
import ResearchInsights from '../src/components/ResearchInsights.vue'
import { useResearchDrafts } from '../src/lib/researchDrafts'

const expected: Record<string,string[]> = {
 en: ['Your next great product starts here.', 'Research the market. Test the profit.', 'Research the market. Test the profit.', 'Research the market. Test the profit.', 'Better assumptions. Better decisions.'],
 id: ['Produk hebat berikutnya dimulai di sini.', 'Riset pasar. Uji laba.', 'Riset pasar. Uji laba.', 'Riset pasar. Uji laba.', 'Asumsi lebih baik. Keputusan lebih baik.'],
 zh: ['从这里发现下一款好产品。', '研究市场，测试利润。', '研究市场，测试利润。', '研究市场，测试利润。', '更好的假设，更好的决策。'],
}
const m = { status: 'ok', value: '10' } as const
const result: ChannelResult = { channel:'shelf',rates:{},sources:{},price:'50000',successfulSales:'1000',revenue:'50000000',shippingIncome:'0',contribution:'8000000',contributionPerOrder:'8000',profitPerOrder:'2700',monthlyProfit:'2700000',margin:'5.4',breakEvenOrders:{status:'ok',value:'663'},breakEvenPrice:{status:'ok',value:'46448'},maxSupplierCost:m,priceBuffer:m,maxAdvertising:m,breakEvenROAS:m,targetPrice:m,targetCost:m,verdict:'thin',reason:'thin',breakdown:[],volume:[] }
const evaluation: EvaluationRevision = { id:'fixture',input:emptyInput(),presetVersion:1,calculationVersion:'1.0.0',results:[result],recommended:'shelf',createdAt:'2026-10-12T00:00:00Z' }
export async function run(DecimalInput: any) {
 const drafts = useResearchDrafts()
 const stopDraftWatch = watch(() => drafts.active.value.input, () => { assert.ok(drafts.active.value.input,'Import replacement must never expose an empty active form to synchronous watchers') }, {flush:'sync'})
 drafts.initializeMargin('20')
 drafts.importRows([{row:2,input:{...emptyInput(),name:'Imported shirt',sku:'00007',price:'65000'},errors:[],evaluation}])
 assert.equal(drafts.drafts.value.length,1,'Import should replace an untouched placeholder even with a custom shared margin')
 const shirtId = drafts.activeId.value
 drafts.active.value.input.price='72000'
 drafts.active.value.channel='live'
 drafts.active.value.competitors.push({id:'',platform:'Marketplace',shop:'Shirt shop',price:'75000',sales:'10',rating:'5',url:''})
 drafts.active.value.removedImages.push('shirt-image')
 drafts.active.value.files.push({file:{} as File,url:'shirt-file'})
 drafts.add('20')
 const manualId = drafts.activeId.value
 drafts.active.value.input.name='Manual lamp'
 drafts.active.value.input.price='90000'
 assert.equal(drafts.active.value.competitors.length,0)
 assert.equal(drafts.active.value.files.length,0)
 assert.equal(drafts.active.value.removedImages.length,0)
 drafts.activeId.value=shirtId
 assert.equal(drafts.active.value.input.price,'72000')
 assert.equal(drafts.active.value.channel,'live','Each draft must retain its analysis channel')
 assert.equal(drafts.active.value.competitors[0].shop,'Shirt shop')
 assert.equal(drafts.active.value.files[0].url,'shirt-file')
 assert.equal(drafts.active.value.removedImages[0],'shirt-image')
 drafts.activeId.value=manualId
 assert.equal(drafts.active.value.input.price,'90000')
 assert.equal(drafts.active.value.channel,'','New drafts must use their own automatic recommendation')
 drafts.importRows([{row:3,input:{...emptyInput(),name:'Imported stool'},errors:[]}])
 assert.equal(drafts.drafts.value.length,3,'Import must preserve existing manual and imported drafts')
 assert.equal(drafts.drafts.value[0].input.sku,'00007','Leading-zero SKUs must survive draft switching')
 const changedPlaceholder = useResearchDrafts()
 changedPlaceholder.active.value.input.cost='40000'
 changedPlaceholder.importRows([{row:2,input:{...emptyInput(),name:'Imported item'},errors:[]}])
 assert.equal(changedPlaceholder.drafts.value.length,2,'Import must not discard financial edits to an unnamed draft')
 const emptyImport = useResearchDrafts(); emptyImport.importRows([])
 assert.equal(emptyImport.drafts.value.length,1,'Empty imports must keep a valid active form')
 stopDraftWatch()
 const appStore = useApp(createPinia())
 const originalFetch = globalThis.fetch
 try {
  for (const code of ['version_conflict', 'duplicate_sku', 'conflict']) {
   globalThis.fetch = async () => new Response(JSON.stringify({code, message:'Save failed', fields: code === 'duplicate_sku' ? {sku:'This SKU already exists.'} : {}}), {status:409})
   await assert.rejects(api('/products'), (error: unknown) => {
    assert.ok(error instanceof ApiError)
    assert.equal(error.code,code)
    appStore.report(error)
    assert.equal(appStore.conflict,code === 'version_conflict','Only a stale version should ask the user to reload')
    assert.deepEqual(appStore.fields,code === 'duplicate_sku' ? {sku:'This SKU already exists.'} : {})
    return true
   })
  }
 } finally { globalThis.fetch = originalFetch }
 appStore.report(new ApiError(409,'Stale record',{sku:'Old field error'},'version_conflict'))
 appStore.report(new Error('Product name is required'))
 assert.equal(appStore.error,'Product name is required')
 assert.equal(appStore.conflict,false,'Local validation must clear a previous stale-version error')
 assert.deepEqual(appStore.fields,{})
 appStore.report(new ApiError(422,'Please check the highlighted fields',{name:'Required'},'validation'))
 assert.equal(appStore.conflict,false)
 assert.deepEqual(appStore.fields,{name:'Required'})
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
 const liveResult = {...result,channel:'live',monthlyProfit:'1234567',targetPrice:{status:'ok' as const,value:'61000'}}
 const selectedEvaluation = {...evaluation,results:[result,liveResult]}
 const selectedDecision = await renderToString(createSSRApp(ResearchInsights,{evaluation:selectedEvaluation,competitors:[],channel:'live'}).use(i18n))
 assert.ok(selectedDecision.includes('TikTok Live'))
 assert.ok(selectedDecision.includes('Rp 61,000'),'Decision analysis must use the selected channel target price')
 const selectedResults = await renderToString(createSSRApp(Results,{evaluation:selectedEvaluation,channel:'live'}).use(i18n))
 assert.ok(selectedResults.includes('Rp 1,234,567'),'Profit summary must use the selected channel')
 assert.match(selectedResults,/aria-pressed="true"[^>]*><span[^>]*><i[^>]*channel-live/,'Selected row must expose its pressed state')

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
   const router = createRouter({ history:createMemoryHistory(),routes:[{path:'/',component:Products},{path:'/calculator',redirect:'/research'},{path:'/research',component:Calculator},{path:'/import',redirect:'/research?mode=bulk'},{path:'/settings',component:Settings}] })
   await router.push(path); await router.isReady()
   const pinia=createPinia();const app=createSSRApp(App).use(pinia).use(router).use(i18n);useApp(pinia).settings={version:1,channels:{},categories:['general'],categoryReturns:{general:''},targetMargin:'15',healthyMargin:'15',healthyBuffer:'10',viableMargin:'8',updatedAt:''}
   const html=await renderToString(app);assert.ok(html.includes(expected[lang][index]),`${lang} ${path}: missing page title`);assert.ok(!html.includes('undefined'),`${lang} ${path}: undefined text`); assert.ok(!html.includes('to="/calculator"'), 'Navigation must use the unified workspace')
  }
  const html = await renderToString(createSSRApp(Results,{evaluation}).use(i18n))
  assert.ok(html.includes('Rp 2,700,000'),`${lang}: result profit missing`)
  assert.ok(html.includes('663'),`${lang}: break-even orders missing`)
 }
 console.log('Passed: independent research drafts, imported and manual edits, leading-zero SKUs, competitor and image isolation, exact decimal input event handling, 15 page renders across all three languages, financial result rendering, exact market summaries, safe research links, and market decision guidance.')
}
