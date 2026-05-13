<script setup lang="ts">
import { useTheme } from '@/composables/useTheme'

const { initTheme } = useTheme()
initTheme()
</script>

<template>
  <router-view />
</template>

<style>
html,
body,
#app {
  height: 100%;
  margin: 0;
  background: var(--main-bg);
}
</style>
