import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    return [
      {
        source: "/healthz",
        destination: "/api/health",
      },
    ];
  },
};

export default nextConfig;
