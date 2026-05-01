import { defineStore } from "pinia";
import { store } from "../utils";
import {
  type AuditEvent,
  type AttackChain,
  type AuditStats,
  getAuditLogs,
  getAttackChains,
  getAuditStats
} from "@/api/audit";

export type AuditState = {
  events: AuditEvent[];
  attackChains: AttackChain[];
  stats: AuditStats | null;
  loading: boolean;
};

export const useAuditStore = defineStore("aegis-audit", {
  state: (): AuditState => ({
    events: [],
    attackChains: [],
    stats: null,
    loading: false
  }),
  actions: {
    async fetchLogs(params?: object) {
      this.loading = true;
      try {
        const res = await getAuditLogs(params);
        this.events = res.data;
      } finally {
        this.loading = false;
      }
    },
    async fetchAttackChains(params?: object) {
      const res = await getAttackChains(params);
      this.attackChains = res.data;
      return res.data;
    },
    async fetchStats() {
      const res = await getAuditStats();
      this.stats = res.data;
      return res.data;
    }
  }
});

export function useAuditStoreHook() {
  return useAuditStore(store);
}
