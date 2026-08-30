import type { NextConfig } from "next";

// Panel, control-api ve gateway'e Next.js sunucusu üzerinden vekillenir:
// tarayıcı hep aynı origin'e konuşur, CORS derdi olmaz.
const controlUrl = process.env.CONTROL_API_URL ?? "http://localhost:8090";
const gatewayUrl = process.env.GATEWAY_URL ?? "http://localhost:8080";

const nextConfig: NextConfig = {
  async rewrites() {
    return [
      { source: "/control/:path*", destination: `${controlUrl}/:path*` },
      { source: "/gw/:path*", destination: `${gatewayUrl}/:path*` },
    ];
  },
};

export default nextConfig;
