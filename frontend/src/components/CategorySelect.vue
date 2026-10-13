<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useApp } from '../stores/app'
import Icon from './Icon.vue'
const props = defineProps<{ modelValue: string; id?: string }>()
const emit = defineEmits<{ 'update:modelValue': [string] }>()
const { t } = useI18n(); const app = useApp()
const open = ref(false); const adding = ref(false); const draft = ref(''); const busy = ref(false); const addError = ref('')
const root = ref<HTMLElement>(); const draftInput = ref<HTMLInputElement>()
const label = (c: string) => t(c)
const current = computed(() => props.modelValue)
function toggle() { open.value ? close() : openMenu() }
function openMenu() { open.value = true; document.addEventListener('pointerdown', onOutside, true) }
function close() { open.value = false; adding.value = false; draft.value = ''; addError.value = ''; document.removeEventListener('pointerdown', onOutside, true) }
function onOutside(e: PointerEvent) { if (root.value && !root.value.contains(e.target as Node)) close() }
function choose(c: string) { emit('update:modelValue', c); close() }
async function startAdd() { adding.value = true; addError.value = ''; await nextTick(); draftInput.value?.focus() }
async function confirmAdd() {
 const name = draft.value.trim(); addError.value = ''
 if (!name) { addError.value = t('categoryNameRequired'); return }
 if (app.categories.some(c => c.toLowerCase() === name.toLowerCase())) { addError.value = t('categoryExists'); return }
 busy.value = true
 try { await app.addCategory(name); emit('update:modelValue', name); close() }
 catch (e) { addError.value = e instanceof Error ? e.message : String(e) }
 finally { busy.value = false }
}
</script>
<template>
 <div ref="root" class="category-select">
  <button :id="id" type="button" class="category-trigger" :class="{ open }" aria-haspopup="listbox" :aria-expanded="open" @click="toggle">
   <span>{{ label(current) }}</span><Icon name="chevron" :size="16" />
  </button>
  <div v-if="open" class="category-menu" role="listbox">
   <button v-for="c in app.categories" :key="c" type="button" role="option" :aria-selected="c === current" class="category-option" :class="{ chosen: c === current }" @click="choose(c)">
    <span>{{ label(c) }}</span><Icon v-if="c === current" name="check" :size="15" />
   </button>
   <div class="category-add">
    <button v-if="!adding" type="button" class="category-add-btn" @click="startAdd"><Icon name="plus" :size="15" />{{ t('addCategory') }}</button>
    <div v-else class="category-add-form">
     <input ref="draftInput" v-model="draft" maxlength="24" :placeholder="t('categoryNamePlaceholder')" :aria-label="t('addCategory')" @keydown.enter.prevent="confirmAdd" @keydown.esc.prevent="adding = false" />
     <button type="button" class="icon-btn" :disabled="busy" :aria-label="t('addCategory')" @click="confirmAdd"><Icon name="check" :size="16" /></button>
     <button type="button" class="icon-btn" :aria-label="t('cancel')" @click="adding = false"><Icon name="close" :size="16" /></button>
    </div>
    <small v-if="addError" class="negative">{{ addError }}</small>
   </div>
  </div>
 </div>
</template>
