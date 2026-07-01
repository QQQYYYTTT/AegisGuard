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
  background:
    linear-gradient(135deg, rgb(0 212 255 / 9%), transparent 42%),
    rgb(5 18 38 / 90%);
  border: 1px solid rgb(78 192 255 / 18%);
  border-radius: 8px;
  box-shadow: inset 0 0 28px rgb(0 212 255 / 4%);
}

.stat-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.stat-card__title {
  font-size: 14px;
  color: #8fb6d8;
}

.stat-card__value {
  font-size: 28px;
  font-weight: 700;
  line-height: 1.2;
  text-shadow: 0 0 18px rgb(0 212 255 / 18%);
}

.stat-card__suffix {
  margin-left: 4px;
  font-size: 14px;
  font-weight: 400;
  color: #8fb6d8;
}

.stat-card__trend {
  margin-top: 4px;
  font-size: 12px;
}
</style>
