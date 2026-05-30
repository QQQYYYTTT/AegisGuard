import { defineStore } from "pinia";
import { store } from "../utils";
import {
  type SandboxContext,
  type TransferRecord,
  getSandboxContext,
  getTransferRecords,
  isolateContext
} from "@/api/sandbox";

export type SandboxState = {
  context: SandboxContext | null;
  transfers: TransferRecord[];
  loading: boolean;
};

export const useSandboxStore = defineStore("aegis-sandbox", {
  state: (): SandboxState => ({
    context: null,
    transfers: [],
    loading: false
  }),
  actions: {
    async fetchContext() {
      this.loading = true;
      try {
        const res = await getSandboxContext();
        this.context = res.data;
      } finally {
        this.loading = false;
      }
    },
    async fetchTransfers(params?: object) {
      const res = await getTransferRecords(params);
      this.transfers = res.data || [];
      return res.data;
    },
    async isolate(payload?: object) {
      const res = await isolateContext(payload);
      this.context = res.data;
      return res.data;
    }
  }
});

export function useSandboxStoreHook() {
  return useSandboxStore(store);
}
