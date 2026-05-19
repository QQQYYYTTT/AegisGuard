<script setup lang="ts">
import { ref } from "vue";
import { message } from "@/utils/message";
import { useNav } from "@/layout/hooks/useNav";
import { useUserStoreHook } from "@/store/modules/user";

import LogoutCircleRLine from "~icons/ri/logout-circle-r-line";
import User3Line from "~icons/ri/user-3-line";

const { logout, username, userAvatar } = useNav();

const userStore = useUserStoreHook();
const profileVisible = ref(false);
const profileLoading = ref(false);
const profile = ref({
  id: 0,
  username: "",
  nickname: "",
  created_at: ""
});

async function openProfile() {
  profileLoading.value = true;
  try {
    const res = await userStore.fetchProfile();
    if (res.success) {
      profile.value = res.data;
      profileVisible.value = true;
    } else {
      message(res.message || "Failed to load profile", { type: "error" });
    }
  } catch (error: any) {
    message(error?.response?.data?.message || "Failed to load profile", {
      type: "error"
    });
  } finally {
    profileLoading.value = false;
  }
}
</script>

<template>
  <div class="user-dropdown-wrap">
    <el-dropdown trigger="click">
      <span class="el-dropdown-link navbar-bg-hover select-none">
        <img :src="userAvatar" alt="avatar" class="user-avatar" />
        <p v-if="username" class="dark:text-white">{{ username }}</p>
      </span>
      <template #dropdown>
        <el-dropdown-menu class="logout">
          <el-dropdown-item :disabled="profileLoading" @click="openProfile">
            <IconifyIconOffline :icon="User3Line" style="margin: 5px" />
            Profile
          </el-dropdown-item>
          <el-dropdown-item @click="logout">
            <IconifyIconOffline
              :icon="LogoutCircleRLine"
              style="margin: 5px"
            />
            Logout
          </el-dropdown-item>
        </el-dropdown-menu>
      </template>
    </el-dropdown>

    <el-dialog v-model="profileVisible" title="Profile" width="420px">
      <el-descriptions :column="1" border>
        <el-descriptions-item label="User ID">
          {{ profile.id }}
        </el-descriptions-item>
        <el-descriptions-item label="Username">
          {{ profile.username }}
        </el-descriptions-item>
        <el-descriptions-item label="Nickname">
          {{ profile.nickname || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="Created At">
          {{ profile.created_at || "-" }}
        </el-descriptions-item>
      </el-descriptions>
    </el-dialog>
  </div>
</template>

<style lang="scss" scoped>
.user-dropdown-wrap {
  display: flex;
  align-items: center;
}

.el-dropdown-link {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 48px;
  padding: 0 10px;
  color: #000000d9;
  cursor: pointer;

  p {
    margin: 0;
    font-size: 14px;
    line-height: 1;
  }
}

.user-avatar {
  width: 28px;
  height: 28px;
  flex: 0 0 28px;
  display: block;
  border-radius: 9999px;
  object-fit: cover;
  overflow: hidden;
  border: 1px solid rgba(0, 0, 0, 0.06);
}

.logout {
  width: 140px;

  ::v-deep(.el-dropdown-menu__item) {
    display: inline-flex;
    flex-wrap: wrap;
    min-width: 100%;
  }
}
</style>
