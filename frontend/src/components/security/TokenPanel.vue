<script setup lang="ts">
defineOptions({ name: "TokenPanel" });

defineProps<{
  token: {
    token_id: string;
    tool_name: string;
    scope: string;
    agent_id: string;
    session_id: string;
    expires_at: string;
    nonce: string;
    risk_level: string;
    signature: string;
    signed: boolean;
    verified: boolean;
    max_calls: number;
    call_count: number;
  } | null;
}>();
</script>

<template>
  <el-card shadow="hover">
    <template #header>
      <div class="flex items-center justify-between">
        <span class="font-semibold">SM2 令牌 / SM2 Token</span>
        <div class="flex gap-2">
          <el-tag v-if="token?.signed" type="success" size="small" effect="dark">
            已签名
          </el-tag>
          <el-tag v-if="token?.verified" type="success" size="small">
            已校验
          </el-tag>
        </div>
      </div>
    </template>
    <div v-if="token" class="space-y-3">
      <el-descriptions :column="2" border size="small">
        <el-descriptions-item label="令牌 ID / Token ID">
          <code class="text-xs">{{ token.token_id }}</code>
        </el-descriptions-item>
        <el-descriptions-item label="工具 / Tool">
          {{ token.tool_name }}
        </el-descriptions-item>
        <el-descriptions-item label="范围 / Scope">
          <el-tag size="small">{{ token.scope }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="Agent">
          {{ token.agent_id }}
        </el-descriptions-item>
        <el-descriptions-item label="会话 / Session">
          <code class="text-xs">{{ token.session_id }}</code>
        </el-descriptions-item>
        <el-descriptions-item label="过期时间 / Expires">
          {{ token.expires_at }}
        </el-descriptions-item>
        <el-descriptions-item label="随机数 / Nonce">
          <code class="text-xs">{{ token.nonce }}</code>
        </el-descriptions-item>
        <el-descriptions-item label="风险等级 / Risk Level">
          <span
            :class="{
              'text-red-500': token.risk_level === 'critical',
              'text-orange-500': token.risk_level === 'high',
              'text-yellow-500': token.risk_level === 'medium',
              'text-green-500': token.risk_level === 'low'
            }"
          >
            {{ token.risk_level.toUpperCase() }}
          </span>
        </el-descriptions-item>
        <el-descriptions-item label="调用额度 / Call Budget">
          {{ token.call_count }} / {{ token.max_calls }}
        </el-descriptions-item>
      </el-descriptions>
      <div>
        <div class="text-xs text-gray-500 mb-1">签名值 / Signature (SM2)</div>
        <el-input
          :model-value="token.signature"
          type="textarea"
          :rows="2"
          readonly
          class="font-mono text-xs"
        />
      </div>
    </div>
    <el-empty v-else description="当前尚未签发令牌" :image-size="60" />
  </el-card>
</template>
