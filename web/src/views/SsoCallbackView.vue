<template>
  <div class="sso-callback">
    <span v-if="error" class="error">{{ error }}</span>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const error = ref('')

onMounted(async () => {
  try {
    await auth.loginSSO()
    const next = (route.query.next as string) || '/user/me'
    router.replace(next)
  } catch (e: any) {
    error.value = e?.response?.data?.error ?? 'SSO login failed'
  }
})
</script>

<style scoped>
.sso-callback {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100vh;
  font-size: 1rem;
  color: var(--el-text-color-secondary);
}
.error {
  color: var(--el-color-danger);
}
</style>
