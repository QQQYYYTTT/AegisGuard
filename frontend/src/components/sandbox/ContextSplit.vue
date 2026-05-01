<script setup lang="ts">
defineOptions({ name: "ContextSplit" });

defineProps<{
  trustedFields: Array<{ label: string; value: string }>;
  untrustedFields: Array<{
    label: string;
    value: string;
    dangerous?: boolean;
  }>;
}>();
</script>

<template>
  <div class="grid grid-cols-2 gap-4">
    <el-card shadow="hover" class="border-green-200 dark:border-green-800">
      <template #header>
        <div class="flex items-center gap-2">
          <el-icon class="text-green-500"><svg viewBox="0 0 1024 1024" fill="currentColor"><path d="M512 64a448 448 0 1 1 0 896 448 448 0 0 1 0-896zm-55.808 536.384-99.52-99.584a38.4 38.4 0 1 0-54.336 54.336l126.72 126.72a38.272 38.272 0 0 0 54.336 0l262.4-262.464a38.4 38.4 0 1 0-54.272-54.336L456.192 600.384z" /></svg></el-icon>
          <span class="font-semibold text-green-700 dark:text-green-400">
            Trusted Context
          </span>
        </div>
      </template>
      <div class="space-y-3">
        <div v-for="field in trustedFields" :key="field.label">
          <div class="text-xs text-gray-500 mb-1">{{ field.label }}</div>
          <el-input
            :model-value="field.value"
            type="textarea"
            :rows="2"
            readonly
            class="font-mono text-xs"
          />
        </div>
      </div>
    </el-card>

    <el-card shadow="hover" class="border-red-200 dark:border-red-800">
      <template #header>
        <div class="flex items-center gap-2">
          <el-icon class="text-red-500"><svg viewBox="0 0 1024 1024" fill="currentColor"><path d="M512 64a448 448 0 1 1 0 896 448 448 0 0 1 0-896zm0 128a38.4 38.4 0 0 0-38.4 38.4v192a38.4 38.4 0 0 0 76.8 0v-192A38.4 38.4 0 0 0 512 192zm0 512a38.4 38.4 0 1 0 0 76.8 38.4 38.4 0 0 0 0-76.8z" /></svg></el-icon>
          <span class="font-semibold text-red-700 dark:text-red-400">
            Untrusted / Sandbox
          </span>
        </div>
      </template>
      <div class="space-y-3">
        <div v-for="field in untrustedFields" :key="field.label">
          <div class="flex items-center gap-2 mb-1">
            <span class="text-xs text-gray-500">{{ field.label }}</span>
            <el-tag
              v-if="field.dangerous"
              type="danger"
              size="small"
              effect="dark"
            >
              DANGEROUS
            </el-tag>
          </div>
          <el-input
            :model-value="field.value"
            type="textarea"
            :rows="2"
            readonly
            class="font-mono text-xs"
            :class="{ 'border-red-400': field.dangerous }"
          />
        </div>
      </div>
    </el-card>
  </div>
</template>
