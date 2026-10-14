import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { api, json, ApiError } from '../api'
import { defaultCategories, type Settings } from '../types'
export const useApp = defineStore('app', () => {
 const settings = ref<Settings>(); const notice = ref(''); const error = ref(''); const fields = ref<Record<string, string>>({}); const conflict = ref(false)
 const categories = computed(() => settings.value?.categories?.length ? settings.value.categories : defaultCategories)
 async function loadSettings() { settings.value = await api<Settings>('/settings') }
 // Appends a shared category and keeps local settings in sync with the new version.
 async function addCategory(name: string) { const next = await api<Settings>('/categories', json('POST', { name })); settings.value = next; return next }
 function clear() { notice.value = ''; error.value = ''; fields.value = {}; conflict.value = false }
 function report(e: unknown) { clear(); error.value = e instanceof Error ? e.message : String(e); if (e instanceof ApiError) { fields.value = e.fields; conflict.value = e.status === 409 && e.code === 'version_conflict' } }
 function success(message: string) { clear(); notice.value = message }
 return { settings, notice, error, fields, conflict, categories, loadSettings, addCategory, report, success, clear }
})
