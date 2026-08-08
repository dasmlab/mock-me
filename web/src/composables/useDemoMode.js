import { computed, ref } from 'vue'
import { enterDemo, exitDemo, getDemoMe } from 'src/services/api'

const demoUser = ref(null)

export function useDemoMode() {
  const isDemo = computed(() => !!demoUser.value?.demo)

  async function refreshDemo() {
    try {
      demoUser.value = await getDemoMe()
    } catch {
      demoUser.value = null
    }
    return isDemo.value
  }

  async function enter() {
    await enterDemo()
    await refreshDemo()
  }

  async function exit() {
    try {
      await exitDemo()
    } finally {
      demoUser.value = null
    }
  }

  return { isDemo, demoUser, enter, exit, refreshDemo }
}
