<script setup lang="ts">
import Motion from "./utils/motion";
import { useRouter } from "vue-router";
import { message } from "@/utils/message";
import { loginRules } from "./utils/rule";
import { ref, reactive, toRaw } from "vue";
import { debounce } from "@pureadmin/utils";
import { useNav } from "@/layout/hooks/useNav";
import { useEventListener } from "@vueuse/core";
import type { FormInstance } from "element-plus";
import { useLayout } from "@/layout/hooks/useLayout";
import { useUserStoreHook } from "@/store/modules/user";
import { initRouter } from "@/router/utils";
import { bg, illustration } from "./utils/static";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import { useDataThemeChange } from "@/layout/hooks/useDataThemeChange";

import dayIcon from "@/assets/svg/day.svg?component";
import darkIcon from "@/assets/svg/dark.svg?component";
import Lock from "~icons/ri/lock-fill";
import User from "~icons/ri/user-3-fill";

defineOptions({
  name: "Login"
});

const router = useRouter();
const loading = ref(false);
const disabled = ref(false);
const ruleFormRef = ref<FormInstance>();
const formMode = ref<"login" | "register">("login");

const { initStorage } = useLayout();
initStorage();

const { dataTheme, overallStyle, dataThemeChange } = useDataThemeChange();
dataThemeChange(overallStyle.value);
const { title } = useNav();

const ruleForm = reactive({
  username: "admin",
  password: "admin123",
  nickname: ""
});

const onSubmit = async (formEl: FormInstance | undefined) => {
  if (!formEl) return;
  await formEl.validate(valid => {
    if (!valid) return;

    loading.value = true;
    const userStore = useUserStoreHook();
    const action =
      formMode.value === "login"
        ? userStore.loginByUsername({
            username: ruleForm.username,
            password: ruleForm.password
          })
        : userStore.registerByUsername({
            username: ruleForm.username,
            password: ruleForm.password,
            nickname: ruleForm.nickname
          });

    action
      .then(res => {
        if (!res.success) {
          message(
            res.message ||
              (formMode.value === "login" ? "登录失败" : "注册失败"),
            { type: "error" }
          );
          return;
        }

        return initRouter().then(() => {
          disabled.value = true;
          router
            .push("/screen")
            .then(() => {
              message(
                formMode.value === "login" ? "登录成功" : "注册并登录成功",
                { type: "success" }
              );
            })
            .finally(() => (disabled.value = false));
        });
      })
      .catch(error => {
        message(error?.response?.data?.message || "请求失败", {
          type: "error"
        });
      })
      .finally(() => (loading.value = false));
  });
};

const immediateDebounce: any = debounce(
  formRef => onSubmit(formRef),
  1000,
  true
);

useEventListener(document, "keydown", ({ code }) => {
  if (
    ["Enter", "NumpadEnter"].includes(code) &&
    !disabled.value &&
    !loading.value
  ) {
    immediateDebounce(ruleFormRef.value);
  }
});
</script>

<template>
  <div class="select-none">
    <img :src="bg" class="wave" />
    <div class="absolute left-5 top-3">
      <el-button plain @click="router.push('/screen')">返回态势大屏</el-button>
    </div>
    <div class="flex-c absolute right-5 top-3">
      <el-switch
        v-model="dataTheme"
        inline-prompt
        :active-icon="dayIcon"
        :inactive-icon="darkIcon"
        @change="dataThemeChange"
      />
    </div>
    <div class="login-container">
      <div class="img">
        <component :is="toRaw(illustration)" />
      </div>
      <div class="login-box">
        <div class="login-form">
          <div class="project-logo-wrap">
            <img src="/logo.svg?v=aegisguard-20260617" alt="AegisGuard logo" />
          </div>
          <Motion>
            <h2 class="outline-hidden">{{ title }}</h2>
          </Motion>

          <Motion :delay="50">
            <div class="mb-4 flex gap-2">
              <el-button
                class="flex-1"
                :type="formMode === 'login' ? 'primary' : 'default'"
                @click="formMode = 'login'"
              >
                登录
              </el-button>
              <el-button
                class="flex-1"
                :type="formMode === 'register' ? 'primary' : 'default'"
                @click="formMode = 'register'"
              >
                注册
              </el-button>
            </div>
          </Motion>

          <el-form
            ref="ruleFormRef"
            :model="ruleForm"
            :rules="loginRules"
            size="large"
          >
            <Motion :delay="100">
              <el-form-item
                :rules="[
                  {
                    required: true,
                    message: '请输入账号',
                    trigger: 'blur'
                  }
                ]"
                prop="username"
              >
                <el-input
                  v-model="ruleForm.username"
                  clearable
                  placeholder="账号"
                  :prefix-icon="useRenderIcon(User)"
                />
              </el-form-item>
            </Motion>

            <Motion :delay="150">
              <el-form-item v-if="formMode === 'register'" prop="nickname">
                <el-input
                  v-model="ruleForm.nickname"
                  clearable
                  placeholder="昵称（可选）"
                  :prefix-icon="useRenderIcon(User)"
                />
              </el-form-item>
            </Motion>

            <Motion :delay="200">
              <el-form-item prop="password">
                <el-input
                  v-model="ruleForm.password"
                  clearable
                  show-password
                  placeholder="密码"
                  :prefix-icon="useRenderIcon(Lock)"
                />
              </el-form-item>
            </Motion>

            <Motion :delay="250">
              <el-button
                class="w-full mt-4!"
                size="default"
                type="primary"
                :loading="loading"
                :disabled="disabled"
                @click="onSubmit(ruleFormRef)"
              >
                {{ formMode === "login" ? "登录" : "注册并登录" }}
              </el-button>
            </Motion>
          </el-form>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import url("@/style/login.css");
</style>

<style lang="scss" scoped>
:deep(.el-input-group__append, .el-input-group__prepend) {
  padding: 0;
}
</style>
