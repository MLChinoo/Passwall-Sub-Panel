import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'node:path';
export default defineConfig({
    plugins: [react()],
    resolve: {
        alias: {
            '@': path.resolve(__dirname, 'src'),
        },
    },
    server: {
        port: 5174,
        host: true,
        proxy: {
            '/api': { target: 'http://localhost:8788', changeOrigin: true },
            '/sub': { target: 'http://localhost:8788', changeOrigin: true },
        },
        fs: {
            // Allow Vite to serve files from the parent repo (npm hoisted some
            // packages — including @fontsource/noto-sans-sc — into the repo-root
            // node_modules instead of web-react/node_modules).
            allow: ['..'],
        },
    },
    build: {
        // Build directly into the Go embed location so `go build` picks up
        // the latest assets without a copy step.
        outDir: '../internal/web/dist',
        emptyOutDir: true,
    },
});
