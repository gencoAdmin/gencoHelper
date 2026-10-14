<script setup lang="ts">
defineProps<{ name: string; size?: number }>()
const paths: Record<string, string> = {
 grid: 'M3 3h7v7H3z M14 3h7v7h-7z M3 14h7v7H3z M14 14h7v7h-7z',
 calculator: 'M6 2h12a2 2 0 0 1 2 2v16a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2z M8 6h8 M8 11h1 M15 11h1 M8 15h1 M15 15h1 M8 19h1 M15 19h1',
 research: 'M10 18a8 8 0 1 0 0-16 8 8 0 0 0 0 16z M16 16l6 6 M6 10h8 M10 6v8',
 upload: 'M12 16V3 M7 8l5-5 5 5 M4 16v4a1 1 0 0 0 1 1h14a1 1 0 0 0 1-1v-4',
 settings: 'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8z M9 2h6l1 4 3 1 3 5-3 3-1 4-6 3-3-3-4-1-3-5 3-3 1-4z',
 plus: 'M12 5v14 M5 12h14', arrow: 'M4 12h16 M14 6l6 6-6 6', back: 'M20 12H4 M10 6l-6 6 6 6',
 download: 'M12 3v13 M7 11l5 5 5-5 M4 17v4h16v-4', search: 'M10 18a8 8 0 1 0 0-16 8 8 0 0 0 0 16z M16 16l6 6',
 leaf: 'M20 3C8 1 1 8 5 16c8 5 15-1 15-13z M5 20L16 8', check: 'M5 12l4 4L19 6', chevron: 'M9 5l7 7-7 7',
 trend: 'M3 17l6-6 4 4 8-11 M15 4h6v6', box: 'M12 2L2 7v10l10 5 10-5V7z M2 7l10 5 10-5 M12 12v10 M7 4.5l10 5',
 layers: 'M12 3L2 8l10 5 10-5z M2 12l10 5 10-5 M2 16l10 5 10-5', clock: 'M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20z M12 6v6l4 2',
 close: 'M6 6l12 12 M6 18L18 6', archive: 'M3 3h18v5H3z M5 8v13h14V8 M10 12h4', copy: 'M9 9h12v12H9z M15 9V3H3v12h6',
 image: 'M3 3h18v18H3z M3 17l6-6 4 4 3-3 5 5 M15 7h.01', globe: 'M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20z M2 12h20 M12 2c-6 6-6 14 0 20 6-6 6-14 0-20',
 edit: 'M15 3l6 6 M3 21l2-7L17 2l5 5-12 12z', info: 'M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20z M12 11v6 M12 7h.01',
 file: 'M14 2H4v20h16V8z M14 2v6h6 M8 12h8 M8 16h8', heart: 'M20 4c-3-3-6-1-8 1-2-2-5-4-8-1-4 4 0 9 8 16 8-7 12-12 8-16z',
 refresh: 'M20 7a9 9 0 1 0 1 9 M20 2v5h-5', arrowUp: 'M7 17L17 7 M7 7h10v10', menu: 'M3 6h18 M3 12h18 M3 18h18',
 sun: 'M12 17a5 5 0 1 0 0-10 5 5 0 0 0 0 10z M12 1v3 M12 20v3 M4.2 4.2l2.1 2.1 M17.7 17.7l2.1 2.1 M1 12h3 M20 12h3 M4.2 19.8l2.1-2.1 M17.7 6.3l2.1-2.1',
 moon: 'M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z',
 display: 'M3 4h18v12H3z M8 20h8 M12 16v4',
}
</script>
<template><svg :width="size || 20" :height="size || 20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path :d="paths[name] || paths.box" /></svg></template>
