import { ref } from "vue";

export const identity = ref(null);
export const token = ref(sessionStorage.getItem("axleo_admin") || "");

export async function api(path, opts = {}) {
  const headers = { ...opts.headers, Authorization: `Bearer ${token.value}` };
  if (opts.body && !(opts.body instanceof FormData))
    headers["Content-Type"] = "application/json";
  const response = await fetch(`/api/v1/${path}`, { ...opts, headers });
  const data = await response.json();
  if (response.status === 401) {
    token.value = "";
    identity.value = null;
    sessionStorage.removeItem("axleo_admin");
    location.hash = "#/login";
  }
  if (!response.ok) throw Error(data.error || `HTTP ${response.status}`);
  return data;
}

export async function logout() {
  try {
    await api("auth/logout", { method: "POST" });
  } finally {
    token.value = "";
    identity.value = null;
    sessionStorage.removeItem("axleo_admin");
    location.hash = "#/login";
  }
}

export async function restore() {
  if (!token.value) return false;
  try {
    const user = await api("me");
    if (user.role !== "admin" && user.role !== "leader") {
      await logout();
      return false;
    }
    identity.value = user;
    return true;
  } catch {
    return false;
  }
}
