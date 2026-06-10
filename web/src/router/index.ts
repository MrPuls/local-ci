import { createRouter, createWebHashHistory, type RouteRecordRaw } from 'vue-router';
import PipelineView from '@/views/PipelineView.vue';
import ConfigView from '@/views/ConfigView.vue';
import HistoryView from '@/views/HistoryView.vue';

// Hash history keeps the SPA portable: no server rewrites needed when served
// statically or packaged in a Tauri shell.
const routes: RouteRecordRaw[] = [
  { path: '/', name: 'pipeline', component: PipelineView },
  { path: '/runs/:id', name: 'run', component: PipelineView },
  { path: '/config', name: 'config', component: ConfigView },
  { path: '/history', name: 'history', component: HistoryView },
  { path: '/:pathMatch(.*)*', redirect: '/' },
];

export const router = createRouter({
  history: createWebHashHistory(),
  routes,
});
