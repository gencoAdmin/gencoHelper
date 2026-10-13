<script setup lang="ts">
import { onMounted, watch, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useApp } from './stores/app'
import Icon from './components/Icon.vue'
const { t, locale } = useI18n(); const app = useApp(); const route = useRoute(); const mobileOpen = ref(false)
const reload = () => window.location.reload()
const nav = [{ path: '/', key: 'products', icon: 'grid' }, { path: '/research', key: 'research', icon: 'research' }, { path: '/import', key: 'imports', icon: 'upload' }]
watch(locale, value => { localStorage.setItem('genco-language', value); document.documentElement.lang = value }, { immediate: true })
watch(() => route.path, () => { mobileOpen.value = false; app.clear() })
onMounted(() => app.loadSettings().catch(app.report))
</script>
<template>
 <div class="app-shell">
  <aside class="sidebar" :class="{ open: mobileOpen }">
   <RouterLink to="/" class="brand"><span class="brand-icon"><Icon name="leaf" :size="27" /></span><span>genco<span class="brand-dot">.</span></span></RouterLink>
   <div class="workspace-name"><span class="tiny-label">{{ t('workspaceLabel') }}</span><strong>{{ t('workspace') }}</strong><span class="workspace-status"><i></i> Genco Finance</span></div>
   <nav><RouterLink v-for="item in nav" :key="item.path" :to="item.path" class="nav-link" :class="{ current: item.path === '/' ? route.path === '/' || route.path.startsWith('/products') : route.path === item.path }"><Icon :name="item.icon" /><span>{{ t(item.key) }}</span><span v-if="item.path === '/research'" class="nav-plus">+</span></RouterLink></nav>
   <div class="sidebar-bottom"><RouterLink to="/settings" class="nav-link" :class="{ current: route.path === '/settings' }"><Icon name="settings" /><span>{{ t('settings') }}</span></RouterLink><div class="sidebar-note"><span class="note-orbit"><Icon name="trend" /></span><p>{{ t('footer') }}</p></div><div class="sidebar-user"><span class="avatar">G</span><div><strong>Genco Team</strong><span>{{ t('workspaceLabel') }}</span></div><span class="online-dot"></span></div></div>
  </aside>
  <div v-if="mobileOpen" class="sidebar-overlay" @click="mobileOpen = false"></div>
  <div class="main-shell">
   <header class="topbar"><div class="topbar-context"><button class="icon-btn mobile-menu" :aria-label="t('products')" @click="mobileOpen = !mobileOpen"><Icon name="menu" /></button><span>Genco Finance</span><span class="breadcrumb-slash">/</span><strong>{{ t(route.path === '/' || route.path.startsWith('/products') ? 'products' : (route.path === '/import' ? 'imports' : route.path.slice(1))) }}</strong></div><div class="topbar-right"><Icon name="globe" :size="17" /><select v-model="locale" class="language-select" :aria-label="t('language')"><option value="en">English</option><option value="id">Indonesia</option><option value="zh">简体中文</option></select><span class="topbar-divider"></span><span class="avatar small">G</span></div></header>
   <main>
    <div v-if="app.error" role="alert" class="alert error-alert"><Icon name="info" /><div><strong>{{ t('error') }}</strong><p>{{ app.conflict ? t('conflict') : app.error }}</p><ul v-if="Object.keys(app.fields).length"><li v-for="(message, field) in app.fields" :key="field">{{ t(String(field).split('.').at(-1) || String(field)) }}: {{ message }}</li></ul><button v-if="app.conflict" class="btn secondary compact" @click="reload">{{ t('reload') }}</button></div><button class="icon-btn" :aria-label="t('close')" @click="app.clear()"><Icon name="close" /></button></div>
    <div v-if="app.notice" role="status" class="alert success-alert"><Icon name="check" /><span>{{ app.notice }}</span><button class="icon-btn" :aria-label="t('close')" @click="app.clear()"><Icon name="close" /></button></div>
    <RouterView />
   </main>
   <footer class="footer"><span>© {{ new Date().getFullYear() }} Genco</span><span>{{ t('taxNote') }}</span></footer>
  </div>
 </div>
</template>
