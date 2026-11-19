<template>
  <div class="login">
    <el-card class="login-card">
      <div class="login-brand">
        <div class="logo">
          <svg viewBox="0 0 32 32">
            <path d="M6 16l10-9 10 9v10h-7v-7h-6v7H6z" fill="#c9a227" />
          </svg>
        </div>
        <div>
          <div class="serif brand-name">Place Sunnybank</div>
          <div class="small muted">Rex Capture · Web 工作台</div>
        </div>
      </div>
      <h2>{{ t("管理后台登录") }}</h2>
      <el-form @submit.prevent="submit">
        <el-form-item :label="t('邮箱')">
          <el-input
            v-model="email"
            type="email"
            autocomplete="username"
            required
          />
        </el-form-item>
        <el-form-item :label="t('密码')">
          <el-input
            v-model="password"
            type="password"
            autocomplete="current-password"
            required
            show-password
          />
        </el-form-item>
        <el-alert
          v-if="error"
          :title="error"
          type="error"
          show-icon
          class="error"
        />
        <el-button native-type="submit" type="primary" class="submit">{{
          t("登录")
        }}</el-button>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { ref } from "vue";
import { useRouter } from "vue-router";
import { token, identity } from "../api";
import { t } from "../i18n";

const router = useRouter();
const email = ref("");
const password = ref("");
const error = ref("");

async function submit() {
  error.value = "";
  try {
    const response = await fetch("/api/v1/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        email: email.value,
        password: password.value,
        device: "Web administration",
      }),
    });
    const data = await response.json();
    if (!response.ok) throw Error(data.error);
    if (data.user.role !== "admin" && data.user.role !== "leader") {
      await fetch("/api/v1/auth/logout", {
        method: "POST",
        headers: { Authorization: `Bearer ${data.token}` },
      });
      throw Error(t("仅管理员可访问"));
    }
    token.value = data.token;
    identity.value = data.user;
    sessionStorage.setItem("axleo_admin", data.token);
    router.push("/dashboard");
  } catch (cause) {
    error.value = cause.message;
  }
}
</script>

<style scoped>
.login {
  min-height: 100vh;
  display: grid;
  place-items: center;
  background: var(--bg);
}
.login-card {
  width: 390px;
  padding: 14px;
}
.login-brand {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 30px;
}
.logo {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  background: var(--navy);
  display: grid;
  place-items: center;
}
.logo svg {
  width: 25px;
}
.brand-name {
  font-size: 19px;
  font-weight: 700;
}
h2 {
  font-size: 21px;
  margin-bottom: 24px;
}
.error {
  margin-bottom: 16px;
}
.submit {
  width: 100%;
}
</style>
