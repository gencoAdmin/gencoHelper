import { readFile, writeFile } from 'node:fs/promises'
import { parse, compileScript, compileTemplate } from '@vue/compiler-sfc'
import ts from 'typescript'
globalThis.localStorage = { getItem: () => null, setItem: () => {} }
// ECharts 4 detects Node through the absence of navigator (Node 22+ adds it).
delete globalThis.navigator
const { run } = await import('../.ssr-test/render.js')
globalThis.document = { documentElement: { lang: '' } }
// Compile the actual input component for the client and exercise its events
// with Vue's custom renderer; no simulated implementation or browser needed.
const source = await readFile(new URL('../src/components/DecimalInput.vue', import.meta.url), 'utf8')
const { descriptor } = parse(source)
const script = compileScript(descriptor, { id: 'decimal-test', genDefaultAs: '_sfc_main' })
const template = compileTemplate({ source: descriptor.template.content, filename: 'DecimalInput.vue', id: 'decimal-test', compilerOptions: { bindingMetadata: script.bindings } })
if (template.errors.length) throw new Error(String(template.errors))
const code = ts.transpileModule(`${script.content}\n${template.code}\n_sfc_main.render = render;\nexport default _sfc_main;`, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } }).outputText
await writeFile(new URL('../.ssr-test/decimal-input.mjs', import.meta.url), code)
const { default: DecimalInput } = await import('../.ssr-test/decimal-input.mjs')
await run(DecimalInput)
