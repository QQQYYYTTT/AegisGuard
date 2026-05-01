<script setup lang="ts">
import ContextSplit from "@/components/sandbox/ContextSplit.vue";
import SandboxCache from "@/components/sandbox/SandboxCache.vue";
import StructuredSummary from "@/components/sandbox/StructuredSummary.vue";
import { useSandbox } from "@/hooks/useSandbox";

defineOptions({ name: "SandboxIndex" });

const {
  context,
  transfers,
  trustedFields,
  untrustedFields,
  loading
} = useSandbox();
</script>

<template>
  <div class="sandbox-page p-4">
    <h1 class="text-2xl font-bold mb-4">Memory Sandbox</h1>
    <p class="text-gray-500 mb-6">
      Context isolation: trusted core memory vs untrusted external data
    </p>

    <div v-loading="loading">
      <ContextSplit
        :trusted-fields="trustedFields"
        :untrusted-fields="untrustedFields"
      />

      <el-row :gutter="16" class="mt-4">
        <el-col :span="8">
          <SandboxCache
            v-if="context"
            :fingerprint="context.sm3_fingerprint"
            :isolated-at="context.isolated_at"
          />
        </el-col>
        <el-col :span="16">
          <StructuredSummary :transfers="transfers" />
        </el-col>
      </el-row>
    </div>
  </div>
</template>
