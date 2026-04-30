<script setup lang="ts">
import RealTimeTag from "@/components/common/RealTimeTag.vue";

defineOptions({ name: "CryptoStatus" });

defineProps<{
  sm2Active: boolean;
  sm3Active: boolean;
  sm4Active: boolean;
  keyExpiry: string;
  activeTokens: number;
  revokedTokens: number;
}>();
</script>

<template>
  <el-card shadow="hover">
    <template #header>
      <span class="font-semibold">国密算法状态</span>
    </template>
    <div class="space-y-4">
      <div class="flex gap-3">
        <RealTimeTag label="SM2 签名" :active="sm2Active" />
        <RealTimeTag label="SM3 摘要" :active="sm3Active" />
        <RealTimeTag label="SM4 加密" :active="sm4Active" />
      </div>
      <el-descriptions :column="2" border size="small">
        <el-descriptions-item label="密钥有效期">
          {{ keyExpiry }}
        </el-descriptions-item>
        <el-descriptions-item label="活跃 Token">
          <el-tag type="success" size="small">{{ activeTokens }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="已撤销 Token">
          <el-tag type="danger" size="small">{{ revokedTokens }}</el-tag>
        </el-descriptions-item>
      </el-descriptions>
    </div>
  </el-card>
</template>
