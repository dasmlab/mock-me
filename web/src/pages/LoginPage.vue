<template>
  <q-layout view="hHh lpR fFf">
    <q-page-container>
      <q-page class="flex flex-center column q-pa-xl">
        <div class="text-h4 q-mb-sm">mock-me</div>
        <p class="text-body2 text-grey-7 q-mb-lg text-center" style="max-width: 420px">
          Sign in with the dasmlab Keycloak realm to author MockUps, or try a labeled demo
          that never deploys to a live node.
        </p>
        <q-btn
          color="primary"
          unelevated
          size="lg"
          icon="login"
          label="Sign in with Keycloak"
          :loading="busy"
          @click="doLogin"
        />
        <q-btn
          outline
          color="secondary"
          size="lg"
          class="q-mt-md"
          icon="science"
          label="Try demo (fake mode)"
          :loading="demoBusy"
          @click="doDemo"
        />
        <p class="text-caption text-grey-7 q-mt-md text-center" style="max-width: 420px">
          Demo / fake mode — scripted orchestration only. Not a live system.
        </p>
        <p v-if="hint" class="text-caption text-negative q-mt-md">{{ hint }}</p>
      </q-page>
    </q-page-container>
  </q-layout>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from 'src/services/auth'
import { useDemoMode } from 'src/composables/useDemoMode'

const auth = useAuth()
const demo = useDemoMode()
const route = useRoute()
const router = useRouter()
const hint = ref('')
const busy = ref(false)
const demoBusy = ref(false)

function doLogin() {
  busy.value = true
  auth.login()
}

async function doDemo() {
  demoBusy.value = true
  hint.value = ''
  try {
    await demo.enter()
    auth.resetReady()
    await auth.init()
    router.push({ name: 'demo' })
  } catch (e) {
    hint.value = e.response?.data?.error || e.message || 'Demo failed'
  } finally {
    demoBusy.value = false
  }
}

onMounted(async () => {
  await auth.init()
  if (!auth.authEnabled.value) {
    router.replace(typeof route.query.returnTo === 'string' ? route.query.returnTo : '/')
    return
  }
  if (auth.isDemo.value) {
    router.replace({ name: 'demo' })
    return
  }
  if (auth.isAdmin.value) {
    router.replace(typeof route.query.returnTo === 'string' ? route.query.returnTo : '/')
  } else if (auth.isAuthenticated.value) {
    hint.value = 'Signed in but missing mock-me client role “admin”. Ask an admin to assign it in Keycloak.'
  }
})
</script>
