import { defineStore } from "pinia";
import { store } from "../utils";
import {
  type GateDecision,
  type GateOverview,
  getGateOverview,
  getGateDecisions,
  evaluateGate
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
        this.overview = res.data;
        this.decisions = res.data.recent_decisions || [];
      } finally {
        this.loading = false;
      }
    },
    async fetchDecisions(params?: object) {
      const res = await getGateDecisions(params);
      this.decisions = res.data;
      return res.data;
    },
    async evaluate(payload?: object) {
      const res = await evaluateGate(payload);
      this.decisions.unshift(res.data);
      return res.data;
    }
  }
});

export function useGateStoreHook() {
  return useGateStore(store);
}
