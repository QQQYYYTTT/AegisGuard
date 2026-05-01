import { defineStore } from "pinia";
import { store } from "../utils";
import {
  type ExperimentSummary,
  type ExperimentRecord,
  type ThreeGateResult,
  type AttackFamilyStats,
  getExperimentSummaries,
  getExperimentSummary,
  getExperimentRecords,
  getThreeGateResult,
  getAttackFamilyStats
} from "@/api/experiment";

export type ExperimentState = {
  summaries: ExperimentSummary[];
  currentSummary: Record<string, any> | null;
  records: ExperimentRecord[];
  threeGateResult: ThreeGateResult | null;
  attackFamilyStats: AttackFamilyStats[];
  loading: boolean;
  selectedRunId: string | null;
};

export const useExperimentStore = defineStore("aegis-experiment", {
  state: (): ExperimentState => ({
    summaries: [],
    currentSummary: null,
    records: [],
    threeGateResult: null,
    attackFamilyStats: [],
    loading: false,
    selectedRunId: null
  }),
  actions: {
    async fetchSummaries() {
      this.loading = true;
      try {
        const res = await getExperimentSummaries();
        this.summaries = res.data;
      } finally {
        this.loading = false;
      }
    },
    async fetchSummary(runId: string) {
      this.loading = true;
      try {
        const res = await getExperimentSummary(runId);
        this.currentSummary = res.data;
        this.selectedRunId = runId;
      } finally {
        this.loading = false;
      }
    },
    async fetchRecords(runId: string) {
      this.loading = true;
      try {
        const res = await getExperimentRecords(runId);
        this.records = res.data;
        this.selectedRunId = runId;
      } finally {
        this.loading = false;
      }
    },
    async fetchThreeGateResult() {
      this.loading = true;
      try {
        const res = await getThreeGateResult();
        this.threeGateResult = res.data;
      } finally {
        this.loading = false;
      }
    },
    async fetchAttackFamilyStats() {
      this.loading = true;
      try {
        const res = await getAttackFamilyStats();
        this.attackFamilyStats = res.data;
      } finally {
        this.loading = false;
      }
    }
  }
});

export function useExperimentStoreHook() {
  return useExperimentStore(store);
}
