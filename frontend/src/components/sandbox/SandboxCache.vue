<script setup lang="ts">
defineOptions({ name: "SandboxCache" });

defineProps<{
  fingerprint: string;
  isolatedAt: string;
}>();
</script>

<template>
  <el-card shadow="hover">
    <template #header>
      <span class="font-semibold">Sandbox Cache</span>
    </template>
    <el-descriptions :column="1" border size="small">
      <el-descriptions-item label="SM3 Fingerprint">
        <code class="text-xs break-all">{{ fingerprint }}</code>
      </el-descriptions-item>
      <el-descriptions-item label="Isolated At">
        {{ isolatedAt }}
      </el-descriptions-item>
    </el-descriptions>
  </el-card>
</template>
