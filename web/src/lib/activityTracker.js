import { postActivity, postDemoActivity } from 'src/services/api'
import { useAuth } from 'src/services/auth'

const INPUT_IDLE_MS = 5000

let started = false
let pageEnteredAt = 0
let currentPath = ''
let visibleAccumMs = 0
let engagedAccumMs = 0
let visibleSegmentStart = 0
let engagedSegmentStart = 0
let lastInputAt = 0
let visible = true
let engaged = false
let flushTimer = null

function now() {
  return typeof performance !== 'undefined' && performance.now ? performance.now() : Date.now()
}

function isDocumentVisible() {
  if (typeof document === 'undefined') return true
  return document.visibilityState !== 'hidden'
}

function isWindowFocused() {
  if (typeof document === 'undefined') return true
  return typeof document.hasFocus !== 'function' || document.hasFocus()
}

function foregroundActive() {
  return isDocumentVisible() && isWindowFocused()
}

function closeVisibleSegment() {
  if (visibleSegmentStart > 0) {
    visibleAccumMs += Math.max(0, now() - visibleSegmentStart)
    visibleSegmentStart = 0
  }
}

function closeEngagedSegment() {
  if (engagedSegmentStart > 0) {
    engagedAccumMs += Math.max(0, now() - engagedSegmentStart)
    engagedSegmentStart = 0
  }
  engaged = false
}

function syncVisibility() {
  const fg = foregroundActive()
  if (fg && !visible) {
    visible = true
    visibleSegmentStart = now()
    // Re-enter engaged only if recent input.
    if (now() - lastInputAt < INPUT_IDLE_MS) {
      engaged = true
      engagedSegmentStart = now()
    }
  } else if (!fg && visible) {
    closeEngagedSegment()
    closeVisibleSegment()
    visible = false
  }
}

function onInput() {
  lastInputAt = now()
  if (!foregroundActive()) return
  if (!engaged) {
    engaged = true
    engagedSegmentStart = now()
  }
}

function tickEngagementIdle() {
  if (!engaged) return
  if (now() - lastInputAt >= INPUT_IDLE_MS) {
    closeEngagedSegment()
  }
}

function snapshotAndReset() {
  syncVisibility()
  closeEngagedSegment()
  closeVisibleSegment()
  const dwellMs = pageEnteredAt > 0 ? Math.round(Math.max(0, now() - pageEnteredAt)) : 0
  const visibleMs = Math.round(Math.max(0, visibleAccumMs))
  const engagedMs = Math.round(Math.max(0, engagedAccumMs))
  visibleAccumMs = 0
  engagedAccumMs = 0
  if (foregroundActive()) {
    visible = true
    visibleSegmentStart = now()
  } else {
    visible = false
    visibleSegmentStart = 0
  }
  engaged = false
  engagedSegmentStart = 0
  pageEnteredAt = now()
  return { dwellMs, visibleMs, engagedMs }
}

async function flushNavigate(path, metrics) {
  const auth = useAuth()
  if (!auth.isAuthenticated.value) return
  const payload = {
    type: 'navigate',
    path,
    dwellMs: metrics.dwellMs,
    visibleMs: metrics.visibleMs,
    engagedMs: metrics.engagedMs,
    demo: !!auth.isDemo?.value,
  }
  try {
    if (auth.isDemo?.value) {
      await postDemoActivity(payload)
    } else {
      await postActivity(payload)
    }
  } catch {
    // Tracking must never break navigation.
  }
}

function onRouteChange(toPath) {
  const next = toPath || '/'
  if (currentPath && currentPath !== next) {
    const metrics = snapshotAndReset()
    void flushNavigate(currentPath, metrics)
  } else if (!currentPath) {
    pageEnteredAt = now()
    visibleAccumMs = 0
    engagedAccumMs = 0
    if (foregroundActive()) {
      visible = true
      visibleSegmentStart = now()
    }
  }
  currentPath = next
}

function onPageHide() {
  if (!currentPath) return
  const metrics = snapshotAndReset()
  // keepalive-style best effort via fetch; axios may abort on unload
  const auth = useAuth()
  if (!auth.isAuthenticated.value) return
  const body = JSON.stringify({
    type: 'navigate',
    path: currentPath,
    dwellMs: metrics.dwellMs,
    visibleMs: metrics.visibleMs,
    engagedMs: metrics.engagedMs,
  })
  try {
    const base = (typeof import.meta !== 'undefined' && import.meta.env?.VITE_API_BASE_URL) || '/api/v1'
    if (navigator.sendBeacon) {
      const blob = new Blob([body], { type: 'application/json' })
      navigator.sendBeacon(`${base}/activity`, blob)
    } else {
      void flushNavigate(currentPath, metrics)
    }
  } catch {
    /* ignore */
  }
}

/**
 * Install lightweight page dwell / engagement tracking for all authenticated users.
 * Call once after the router is created.
 */
export function installActivityTracker(router) {
  if (started || !router) return
  started = true

  if (typeof window !== 'undefined') {
    window.addEventListener('focus', syncVisibility)
    window.addEventListener('blur', syncVisibility)
    document.addEventListener('visibilitychange', syncVisibility)
    ;['mousemove', 'scroll', 'keydown', 'touchstart', 'click'].forEach((evt) => {
      window.addEventListener(evt, onInput, { passive: true })
    })
    window.addEventListener('pagehide', onPageHide)
    flushTimer = setInterval(tickEngagementIdle, 1000)
  }

  router.afterEach((to) => {
    if (to.meta?.public) {
      if (currentPath) {
        const metrics = snapshotAndReset()
        void flushNavigate(currentPath, metrics)
        currentPath = ''
      }
      return
    }
    onRouteChange(to.fullPath || to.path)
  })
}

export function _resetActivityTrackerForTests() {
  started = false
  currentPath = ''
  if (flushTimer) clearInterval(flushTimer)
  flushTimer = null
}
