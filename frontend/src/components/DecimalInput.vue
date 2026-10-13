<script setup lang="ts">
// Vue's built-in number-input v-model coerces decimals to binary numbers.
// Keep the input's exact string for the API and authoritative Go engine.
const value = defineModel<string>({ required: true })
function update(event: Event) { value.value = (event.target as HTMLInputElement).value }
</script>
<template><input type="number" :value="value" @input="update" /></template>
