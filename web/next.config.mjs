/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'export',
  images: { unoptimized: true },
  // Dev-only: proxy /api to the Go server so the UI and API share an origin.
  // In production the Go server serves the built static bundle directly.
  async rewrites() {
    return [{ source: '/api/:path*', destination: 'http://localhost:8080/api/:path*' }];
  },
};

export default nextConfig;
