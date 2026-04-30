import { ref, computed, onMounted } from "vue";
import { useAuthStoreHook } from "@/store/modules/auth";

export function useCryptoStatus() {
  const authStore = useAuthStoreHook();
  const loading = ref(false);

  async function refresh() {
    loading.value = true;
    try {
      await authStore.fetchAuthStatus();
    } finally {
      loading.value = false;
    }
  }

  const sm2Status = computed(() => authStore.authStatus?.sm2_active ?? false);
  const sm3Status = computed(() => authStore.authStatus?.sm3_active ?? false);
  const sm4Status = computed(() => authStore.authStatus?.sm4_active ?? false);
  const keyExpiry = computed(() => authStore.authStatus?.key_expires_at ?? "");
  const activeTokens = computed(() => authStore.authStatus?.active_tokens ?? 0);
  const revokedTokens = computed(
    () => authStore.authStatus?.revoked_tokens ?? 0
  );

  onMounted(() => {
    refresh();
  });

  return {
    sm2Status,
    sm3Status,
    sm4Status,
    keyExpiry,
    activeTokens,
    revokedTokens,
    loading,
    refresh
  };
}
