<template>
  <q-page padding class="q-gutter-md">
    <q-banner class="bg-warning text-dark" rounded>
      <template #avatar><q-icon name="science" /></template>
      Demo / fake mode — not a live system. This timeline never deploys to a real node.
    </q-banner>

    <div class="text-h5">Mock-Me demo deploy</div>
    <p class="text-body2 text-grey-8">
      Click simulate to walk the same assembly-line stages as a real deploy — using fixtures only.
    </p>

    <div class="row q-gutter-sm">
      <q-btn color="primary" unelevated icon="play_arrow" label="Simulate deploy" :loading="busy" @click="run" />
      <q-btn flat color="primary" label="Exit demo" @click="leave" />
    </div>

    <q-list v-if="job" bordered class="rounded-borders">
      <q-item v-for="stage in job.stages" :key="stage.id">
        <q-item-section avatar>
          <q-icon :name="stage.icon || 'check_circle'" :color="stage.status === 'ok' ? 'positive' : 'grey'" />
        </q-item-section>
        <q-item-section>
          <q-item-label>{{ stage.label }}</q-item-label>
          <q-item-label caption>{{ stage.detail }} — {{ stage.message }}</q-item-label>
        </q-item-section>
        <q-item-section side>
          <q-badge :color="stage.status === 'ok' ? 'positive' : 'grey'">{{ stage.status }}</q-badge>
        </q-item-section>
      </q-item>
    </q-list>

    <q-card v-if="job?.console?.length" flat bordered>
      <q-card-section class="text-caption text-mono">
        <div v-for="(line, i) in job.console" :key="i">{{ line.text }}</div>
      </q-card-section>
    </q-card>
  </q-page>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { simulateDemoDeploy } from 'src/services/api'
import { useDemoMode } from 'src/composables/useDemoMode'
import { useAuth } from 'src/services/auth'

const router = useRouter()
const { enter, exit, refreshDemo, isDemo } = useDemoMode()
const auth = useAuth()
const job = ref(null)
const busy = ref(false)

onMounted(async () => {
  if (!isDemo.value) {
    await enter()
    auth.resetReady?.()
    await auth.init()
  } else {
    await refreshDemo()
  }
})

async function run() {
  busy.value = true
  try {
    job.value = await simulateDemoDeploy()
  } finally {
    busy.value = false
  }
}

async function leave() {
  await exit()
  auth.resetReady?.()
  await auth.init()
  router.push({ name: 'login' })
}
</script>

<style scoped>
.text-mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  white-space: pre-wrap;
}
</style>
