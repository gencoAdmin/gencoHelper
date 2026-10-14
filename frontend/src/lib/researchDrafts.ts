import { computed, ref, toRaw } from 'vue'
import { emptyInput, type Competitor, type ProductImage, type ImportRow } from '../types'

export function useResearchDrafts() {
 let nextId = 0
 const makeDraft = (row?: ImportRow) => ({
  id: ++nextId, input: row ? structuredClone(toRaw(row.input)) : emptyInput(),
  competitors: [] as Competitor[], existingImages: [] as ProductImage[],
  files: [] as { file: File; url: string }[], removedImages: [] as string[],
  version: 0, productId: '', saved: false, evaluation: row?.evaluation, channel: '',
  calculationError: '', localFields: {} as Record<string, string>,
 })
 const drafts = ref([makeDraft()])
 const activeId = ref(drafts.value[0].id)
 const active = computed(() => drafts.value.find(d => d.id === activeId.value) || drafts.value[0])
 let initialInput = JSON.stringify(drafts.value[0].input)
 function initializeMargin(targetMargin: string) { drafts.value[0].input.targetMargin = targetMargin; initialInput = JSON.stringify(drafts.value[0].input) }
 function add(targetMargin: string) {
  const draft = makeDraft(); draft.input.targetMargin = targetMargin
  drafts.value.push(draft); activeId.value = draft.id
 }
 function importRows(rows: ImportRow[]) {
  const imported = rows.map(makeDraft)
  // Replace only the untouched initial placeholder; keep any work already entered.
  const replacePlaceholder = imported.length && drafts.value.length === 1 && !drafts.value[0].saved && !drafts.value[0].files.length && !drafts.value[0].competitors.length && JSON.stringify(drafts.value[0].input) === initialInput
  drafts.value = [...(replacePlaceholder ? [] : drafts.value), ...imported]
  if (imported.length) activeId.value = imported[0].id
 }
 return { drafts, activeId, active, add, importRows, initializeMargin }
}
export type ResearchDraft = ReturnType<typeof useResearchDrafts>['active']['value']
