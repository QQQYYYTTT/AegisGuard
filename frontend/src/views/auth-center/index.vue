<script setup lang="ts">
import { onMounted } from "vue";
import TokenPanel from "@/components/security/TokenPanel.vue";
import CryptoStatus from "@/components/security/CryptoStatus.vue";
import VerificationFlow from "@/components/security/VerificationFlow.vue";
import RiskBadge from "@/components/common/RiskBadge.vue";
import { useAuth } from "@/hooks/useAuth";
import { useCryptoStatus } from "@/hooks/useCryptoStatus";

defineOptions({ name: "AuthCenterIndex" });

const {
  token,
  verification,
  verify,
  loading: authLoading
} = useAuth();
const {
  sm2Status,
  sm3Status,
  sm4Status,
  keyExpiry,
  activeTokens,
  revokedTokens
} = useCryptoStatus();

onMounted(() => {
  verify();
});
</script>

<template>
  <div class="auth-center p-4">
    <h1 class="text-2xl font-bold mb-4">SM Authorization Center</h1>

    <el-row :gutter="16" class="mb-4">
      <el-col :span="12">
        <CryptoStatus
          :sm2-active="sm2Status"
          :sm3-active="sm3Status"
          :sm4-active="sm4Status"
          :key-expiry="keyExpiry"
          :active-tokens="activeTokens"
          :revoked-tokens="revokedTokens"
        />
      </el-col>
      <el-col :span="12">
        <el-card shadow="hover">
          <template #header>
            <span class="font-semibold">Current Task & Action</span>
          </template>
          <div v-if="token" class="space-y-3">
            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="Task ID">
                <code class="text-xs">{{ token.task_id }}</code>
              </el-descriptions-item>
              <el-descriptions-item label="Tool">
                {{ token.tool_name }}
              </el-descriptions-item>
              <el-descriptions-item label="Agent">
                {{ token.agent_id }}
              </el-descriptions-item>
              <el-descriptions-item label="Risk Level">
                <RiskBadge :level="(token.risk_level as any)" />
              </el-descriptions-item>
              <el-descriptions-item label="Scope">
                <el-tag size="small">{{ token.scope }}</el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="Call Budget">
                {{ token.call_count }} / {{ token.max_calls }}
              </el-descriptions-item>
            </el-descriptions>
          </div>
          <el-empty v-else description="No active task" :image-size="60" />
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16">
      <el-col :span="12">
        <TokenPanel :token="token" />
      </el-col>
      <el-col :span="12">
        <VerificationFlow
          v-if="verification?.checks"
          :checks="verification.checks"
        />
        <el-card v-else shadow="hover">
          <template #header>
            <span class="font-semibold">Verification Flow</span>
          </template>
          <el-button :loading="authLoading" @click="verify()">
            Run Verification
          </el-button>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>
