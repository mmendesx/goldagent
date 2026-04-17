import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [tailwindcss(), react()],
  server: {
    port: 3000,
    host: true,
  },
  test: {
    environment: 'node',
    exclude: ["**/node_modules/**", "**/dist/**", "tests/a11y.spec.ts", "tests/keyboard.spec.ts"],
  },
})
