import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

export default defineConfig({
  plugins: [vue()],
  base: "/admin/",
  build: {
    outDir: "../server/web",
    emptyOutDir: true,
    chunkSizeWarningLimit: 3000,
    // go:embed excludes files whose basename starts with '_' or '.'.
    rollupOptions: {
      output: { chunkFileNames: "assets/chunk-[name]-[hash].js" },
    },
  },
  server: { host: true, port: 5180 },
});
