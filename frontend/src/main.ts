import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import { i18n } from './locales'
import './style.css'
const router = createRouter({ history: createWebHistory(), routes: [
 { path: '/', component: () => import('./pages/Products.vue') }, { path: '/calculator', redirect: '/research' },
 { path: '/research', component: () => import('./pages/Calculator.vue') },
 { path: '/products/:id', component: () => import('./pages/ProductDetail.vue') }, { path: '/products/:id/edit', component: () => import('./pages/Calculator.vue') },
 { path: '/settings', component: () => import('./pages/Settings.vue') }, { path: '/import', redirect: '/research?mode=bulk' },
 { path: '/:pathMatch(.*)*', redirect: '/' },
] })
createApp(App).use(createPinia()).use(router).use(i18n).mount('#app')
