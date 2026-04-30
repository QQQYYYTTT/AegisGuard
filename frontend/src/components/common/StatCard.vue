<script setup lang="ts">
defineOptions({ name: "StatCard" });

defineProps<{
  title: string;
  value: string | number;
  icon?: string;
  color?: string;
  suffix?: string;
  trend?: "up" | "down" | "flat";
  trendValue?: string;
}>();
</script>

<template>
  <el-card shadow="hover" class="stat-card">
    <div class="stat-card__header">
      <span class="stat-card__title">{{ title }}</span>
      <el-icon v-if="icon" :size="20" :style="{ color: color || 'var(--aegis-primary)' }">
        <component :is="icon" />
      </el-icon>
    </div>
    <div class="stat-card__value" :style="{ color: color || 'var(--aegis-primary)' }">
      {{ value }}
      <span v-if="suffix" class="stat-card__suffix">{{ suffix }}</span>
    </div>
    <div v-if="trend" class="stat-card__trend">
      <span
        :class="[
          'stat-card__trend-text',
          trend === 'up' ? 'text-green-500' : trend === 'down' ? 'text-red-500' : 'text-gray-400'
        ]"
      >
        {{ trend === "up" ? "↑" : trend === "down" ? "↓" : "→" }}
        {{ trendValue }}
      </span>
    </div>
  </el-card>
</template>

<style scoped>
.stat-card {
  cursor: default;
}

.stat-card__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.stat-card__title {
  font-size: 14px;
  color: var(--el-text-color-secondary);
}

.stat-card__value {
  font-size: 28px;
  font-weight: 700;
  line-height: 1.2;
}

.stat-card__suffix {
  font-size: 14px;
  font-weight: 400;
  margin-left: 4px;
  color: var(--el-text-color-secondary);
}

.stat-card__trend {
  margin-top: 4px;
  font-size: 12px;
}
</style>
