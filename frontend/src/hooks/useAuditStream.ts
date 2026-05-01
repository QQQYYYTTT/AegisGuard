import { ref, computed, onMounted, onUnmounted } from "vue";
import { useAuditStoreHook } from "@/store/modules/audit";
import type { AuditEvent } from "@/api/audit";

export function useAuditStream() {
  const auditStore = useAuditStoreHook();
  const loading = ref(false);
  const streamBuffer = ref<AuditEvent[]>([]);
  let pollTimer: ReturnType<typeof setInterval> | null = null;

  async function loadLogs(params?: object) {
    loading.value = true;
    try {
      await auditStore.fetchLogs(params);
    } finally {
      loading.value = false;
    }
  }

  async function loadChains(params?: object) {
    return await auditStore.fetchAttackChains(params);
  }

  async function loadStats() {
    return await auditStore.fetchStats();
  }

  function startPolling(intervalMs = 5000) {
    stopPolling();
    pollTimer = setInterval(() => {
      auditStore.fetchLogs();
    }, intervalMs);
  }

  function stopPolling() {
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  }

  function pushEvent(event: AuditEvent) {
    streamBuffer.value.unshift(event);
    if (streamBuffer.value.length > 100) {
      streamBuffer.value.pop();
    }
  }

  onMounted(() => {
    loadLogs();
    loadChains();
    loadStats();
  });

  onUnmounted(() => {
    stopPolling();
  });

  return {
    events: computed(() => auditStore.events),
    attackChains: computed(() => auditStore.attackChains),
    stats: computed(() => auditStore.stats),
    streamBuffer,
    loading,
    loadLogs,
    loadChains,
    loadStats,
    startPolling,
    stopPolling,
    pushEvent
  };
}
