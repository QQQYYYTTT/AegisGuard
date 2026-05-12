import { ref, computed, onMounted } from "vue";
import { useSandboxStoreHook } from "@/store/modules/sandbox";

export function useSandbox() {
  const sandboxStore = useSandboxStoreHook();
  const loading = ref(false);

  async function loadContext() {
    loading.value = true;
    try {
      await sandboxStore.fetchContext();
    } finally {
      loading.value = false;
    }
  }

  async function loadTransfers(params?: object) {
    return await sandboxStore.fetchTransfers(params);
  }

  async function isolateNew(payload?: object) {
    return await sandboxStore.isolate(payload);
  }

  const trustedFields = computed(() => {
    if (!sandboxStore.context) return [];
    const t = sandboxStore.context.trusted;
    return [
      { label: "System Prompt", value: t.system_prompt },
      ...(t.tool_definitions || []).map((d, i) => ({
        label: `Tool Definition ${i + 1}`,
        value: d
      })),
      { label: "Memory", value: t.memory },
      { label: "Task State", value: t.task_state || "" }
    ].filter(field => field.value !== "");
  });

  const untrustedFields = computed(() => {
    if (!sandboxStore.context) return [];
    const u = sandboxStore.context.untrusted;
    return [
      { label: "User Input", value: u.user_input, dangerous: true },
      { label: "External Data", value: u.external_data, dangerous: false },
      { label: "Injected Content", value: u.injected_content, dangerous: true }
    ];
  });

  onMounted(() => {
    loadContext();
    loadTransfers();
  });

  return {
    context: computed(() => sandboxStore.context),
    transfers: computed(() => sandboxStore.transfers),
    trustedFields,
    untrustedFields,
    loading,
    loadContext,
    loadTransfers,
    isolateNew
  };
}
