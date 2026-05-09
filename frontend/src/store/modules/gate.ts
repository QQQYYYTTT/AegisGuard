import { defineStore } from "pinia";
import { store } from "../utils";
import {
  type GateEvaluateRequest,
  type GateDecision,
  type GateOverview,
  getGateOverview,
  getGateDecisions,
  evaluateGate,
  normalizeGateDecision,
  normalizeGateOverview
} from "@/api/gate";

export type GateState = {
  overview: GateOverview | null;
  decisions: GateDecision[];
  loading: boolean;
};

export const useGateStore = defineStore("aegis-gate", {
  state: (): GateState => ({
    overview: null,
    decisions: [],
    loading: false
  }),
  actions: {
    async fetchOverview() {
      this.loading = true;
      try {
        const res = await getGateOverview();
        this.overview = normalizeGateOverview(res.data);
        this.decisions = this.overview.recent_decisions || [];
      } finally {
        this.loading = false;
      }
    },
    async fetchDecisions(params?: object) {
      const res = await getGateDecisions(params);
      this.decisions = (res.data || []).map(normalizeGateDecision);
      return res.data;
    },
    async evaluate(payload?: GateEvaluateRequest) {
      const res = await evaluateGate(payload);
      const normalized = normalizeGateDecision(res.data);
      this.decisions.unshift(normalized);
      return normalized;
    }
  }
});

export function useGateStoreHook() {
  return useGateStore(store);
}
