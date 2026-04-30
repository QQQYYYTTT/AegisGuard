import { ref, computed, onMounted } from "vue";
import { useGateStoreHook } from "@/store/modules/gate";

export function useGateDecision() {
  const gateStore = useGateStoreHook();
  const loading = ref(false);

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

  async function evaluate(payload?: object) {
    return await gateStore.evaluate(payload);
  }

  const riskScoreColor = (score: number) => {
    if (score >= 0.8) return "var(--risk-critical, #ff4d4f)";
    if (score >= 0.6) return "var(--risk-high, #f56c6c)";
    if (score >= 0.4) return "var(--risk-medium, #e6a23c)";
    return "var(--risk-low, #67c23a)";
  };

  onMounted(() => {
    loadOverview();
  });

  return {
    overview: computed(() => gateStore.overview),
    decisions: computed(() => gateStore.decisions),
    loading,
    loadOverview,
    loadDecisions,
    evaluate,
    riskScoreColor
  };
}
