<script setup lang="ts">
defineOptions({ name: "RealTimeTag" });

defineProps<{
  label: string;
  active?: boolean;
}>();
</script>

<template>
  <el-tag :type="active ? 'success' : 'info'" effect="dark" size="small" round>
    <span class="inline-flex items-center gap-1">
      <span
        class="inline-block w-2 h-2 rounded-full"
        :class="active ? 'bg-green-400 animate-pulse' : 'bg-gray-400'"
      />
      {{ label }}
    </span>
  </el-tag>
</template>
