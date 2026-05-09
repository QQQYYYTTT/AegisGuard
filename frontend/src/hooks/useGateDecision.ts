import { ref, computed, onMounted, onUnmounted } from "vue";
import { useGateStoreHook } from "@/store/modules/gate";
import type { GateEvaluateRequest } from "@/api/gate";

export function useGateDecision() {
  const gateStore = useGateStoreHook();
  const loading = ref(false);
  let pollTimer: ReturnType<typeof setInterval> | null = null;

  async function loadOverview() {
    loading.value = true;
    try {
      await gateStore.fetchOverview();
    } finally {
      loading.value = false;
    }
  }

  async function loadDecisions(params?: object) {
    return await gateStore.fetchDecisions(params);
  }

  async function evaluate(payload?: GateEvaluateRequest) {
    return await gateStore.evaluate(payload);
  }

  function startPolling(intervalMs = 5000) {
    stopPolling();
    pollTimer = setInterval(() => {
      gateStore.fetchOverview();
    }, intervalMs);
  }

  function stopPolling() {
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  }

  const riskScoreColor = (score: number) => {
    if (score >= 80) return "var(--risk-critical, #ff4d4f)";
    if (score >= 60) return "var(--risk-high, #f56c6c)";
    if (score >= 40) return "var(--risk-medium, #e6a23c)";
    return "var(--risk-low, #67c23a)";
  };

  onMounted(() => {
    loadOverview();
  });

  onUnmounted(() => {
    stopPolling();
  });

  return {
    overview: computed(() => gateStore.overview),
    decisions: computed(() => gateStore.decisions),
    loading,
    loadOverview,
    loadDecisions,
    evaluate,
    riskScoreColor,
    startPolling,
    stopPolling
  };
}
