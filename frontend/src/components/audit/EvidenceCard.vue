<script setup lang="ts">
defineOptions({ name: "EvidenceCard" });

defineProps<{
  evidence: {
    request_id: string;
    body_hash: string;
    method: string;
    path: string;
    status: number;
    duration_ms: number;
    agent_id: string;
    session_id: string;
  };
}>();
</script>

<template>
  <el-card shadow="hover" class="evidence-card">
    <template #header>
      <span class="font-semibold text-sm">Evidence Summary</span>
    </template>
    <el-descriptions :column="2" border size="small">
      <el-descriptions-item label="Request ID">
        <code class="text-xs">{{ evidence.request_id }}</code>
      </el-descriptions-item>
      <el-descriptions-item label="Status">
        <el-tag
          :type="evidence.status < 400 ? 'success' : 'danger'"
          size="small"
        >
          {{ evidence.status }}
        </el-tag>
      </el-descriptions-item>
      <el-descriptions-item label="Method">
        {{ evidence.method }}
      </el-descriptions-item>
      <el-descriptions-item label="Path">
        <code class="text-xs">{{ evidence.path }}</code>
      </el-descriptions-item>
      <el-descriptions-item label="Duration">
        {{ evidence.duration_ms }}ms
      </el-descriptions-item>
      <el-descriptions-item label="Agent">
        {{ evidence.agent_id }}
      </el-descriptions-item>
      <el-descriptions-item label="Body Hash (SM3)" :span="2">
        <code class="text-xs break-all">{{ evidence.body_hash }}</code>
      </el-descriptions-item>
    </el-descriptions>
  </el-card>
</template>
