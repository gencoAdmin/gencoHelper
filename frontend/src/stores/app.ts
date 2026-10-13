import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, ApiError } from '../api'
import type { Settings } from '../types'
export const useApp = defineStore('app', () => {
 const settings = ref<Settings>(); const notice = ref(''); const error = ref(''); const fields = ref<Record<string, string>>({}); const conflict = ref(false)
 async function loadSettings() { settings.value = await api<Settings>('/settings') }
 function clear() { notice.value = ''; error.value = ''; fields.value = {}; conflict.value = false }
 function report(e: unknown) { error.value = e instanceof Error ? e.message : String(e); if (e instanceof ApiError) { fields.value = e.fields; conflict.value = e.status === 409 } }
 function success(message: string) { clear(); notice.value = message }
 return { settings, notice, error, fields, conflict, loadSettings, report, success, clear }
})
