<script setup lang="ts">
defineOptions({ name: "VerificationFlow" });

defineProps<{
  checks: Record<string, boolean>;
}>();

const checkLabels: Record<string, string> = {
  signature_valid: "SM2 签名校验 / Signature",
  expiry_valid: "Token 有效期 / Expiry",
  nonce_valid: "Nonce 防重放 / Nonce",
  call_budget_ok: "调用预算 / Call Budget",
  schema_hash_match: "Schema 哈希匹配 / Schema Hash",
  scope_match: "权限范围匹配 / Scope",
  risk_level_ok: "风险等级检查 / Risk Level"
};
</script>

<template>
  <el-card shadow="hover">
    <template #header>
      <span class="font-semibold">校验流程 / Verification Flow</span>
    </template>
    <div class="space-y-2">
      <div
        v-for="(label, key) in checkLabels"
        :key="key"
        class="flex items-center gap-3 p-2 rounded"
        :class="checks[key] ? 'bg-green-50 dark:bg-green-900/20' : 'bg-red-50 dark:bg-red-900/20'"
      >
        <el-icon
          :size="18"
          :class="checks[key] ? 'text-green-500' : 'text-red-500'"
        >
          <template v-if="checks[key]">
            <svg viewBox="0 0 1024 1024" fill="currentColor">
              <path d="M512 64a448 448 0 1 1 0 896 448 448 0 0 1 0-896zm-55.808 536.384-99.52-99.584a38.4 38.4 0 1 0-54.336 54.336l126.72 126.72a38.272 38.272 0 0 0 54.336 0l262.4-262.464a38.4 38.4 0 1 0-54.272-54.336L456.192 600.384z" />
            </svg>
          </template>
          <template v-else>
            <svg viewBox="0 0 1024 1024" fill="currentColor">
              <path d="M512 64a448 448 0 1 1 0 896 448 448 0 0 1 0-896zm0 128a38.4 38.4 0 0 0-38.4 38.4v192a38.4 38.4 0 0 0 76.8 0v-192A38.4 38.4 0 0 0 512 192zm0 512a38.4 38.4 0 1 0 0 76.8 38.4 38.4 0 0 0 0-76.8z" />
            </svg>
          </template>
        </el-icon>
        <span class="text-sm">{{ label }}</span>
        <el-tag
          :type="checks[key] ? 'success' : 'danger'"
          size="small"
          class="ml-auto"
        >
          {{ checks[key] ? "PASS" : "FAIL" }}
        </el-tag>
      </div>
    </div>
  </el-card>
</template>
