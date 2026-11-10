import type { NextConfig } from "next";

const securityHeaders = [
  { key: "Referrer-Policy", value: "no-referrer" },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "Permissions-Policy", value: "camera=(), geolocation=(), microphone=()" },
];

const testDistDir = process.env.BIDORA_NEXT_TEST_DIST_DIR;
if (testDistDir && !/^\.next-auth-test-[1-9][0-9]*$/.test(testDistDir)) {
  throw new Error("BIDORA_NEXT_TEST_DIST_DIR must be a process-scoped authentication test directory");
}

const nextConfig: NextConfig = {
  output: "standalone",
  distDir: testDistDir || ".next",
  // An inherited config keeps Next.js from appending exact PID-specific type paths.
  typescript: { tsconfigPath: testDistDir ? "tsconfig.auth-test.json" : "tsconfig.json" },
  poweredByHeader: false,
  async headers() {
    return [
      { source: "/:path*", headers: securityHeaders },
      // The public email font may be loaded by mail clients on other origins.
      { source: "/fonts/manrope-latin.woff2", headers: [{ key: "Access-Control-Allow-Origin", value: "*" }] },
    ];
  },
  cacheComponents: true,
  partialPrefetching: true,
  turbopack: {
    root: process.cwd(),
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
};

export default nextConfig;
