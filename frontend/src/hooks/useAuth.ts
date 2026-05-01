import { ref, computed, onMounted } from "vue";
import { useAuthStoreHook } from "@/store/modules/auth";

export function useAuth() {
  const authStore = useAuthStoreHook();
  const loading = ref(false);

  async function loadToken() {
    loading.value = true;
    try {
      await authStore.fetchToken();
    } finally {
      loading.value = false;
    }
  }

  async function loadStatus() {
    await authStore.fetchAuthStatus();
  }

  async function issueNew(payload?: object) {
    loading.value = true;
    try {
      return await authStore.issueNewToken(payload);
    } finally {
      loading.value = false;
    }
  }

  async function verify() {
    return await authStore.verifyCurrentToken();
  }

  onMounted(() => {
    loadToken();
    loadStatus();
  });

  return {
    token: computed(() => authStore.currentToken),
    status: computed(() => authStore.authStatus),
    verification: computed(() => authStore.verificationResult),
    loading,
    loadToken,
    loadStatus,
    issueNew,
    verify
  };
}
