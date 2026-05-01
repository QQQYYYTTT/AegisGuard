import { defineStore } from "pinia";
import { store } from "../utils";
import {
  type TokenInfo,
  type AuthStatus,
  getTokenInfo,
  issueToken,
  verifyToken,
  getAuthStatus
} from "@/api/auth";

export type AuthState = {
  currentToken: TokenInfo | null;
  authStatus: AuthStatus | null;
  verificationResult: { valid: boolean; checks: Record<string, boolean> } | null;
  loading: boolean;
};

export const useAuthStore = defineStore("aegis-auth", {
  state: (): AuthState => ({
    currentToken: null,
    authStatus: null,
    verificationResult: null,
    loading: false
  }),
  actions: {
    async fetchToken() {
      this.loading = true;
      try {
        const res = await getTokenInfo();
        this.currentToken = res.data;
      } finally {
        this.loading = false;
      }
    },
    async issueNewToken(payload?: object) {
      this.loading = true;
      try {
        const res = await issueToken(payload);
        this.currentToken = res.data;
        return res.data;
      } finally {
        this.loading = false;
      }
    },
    async verifyCurrentToken() {
      const res = await verifyToken();
      this.verificationResult = res.data;
      return res.data;
    },
    async fetchAuthStatus() {
      const res = await getAuthStatus();
      this.authStatus = res.data;
      return res.data;
    }
  }
});

export function useAuthStoreHook() {
  return useAuthStore(store);
}
