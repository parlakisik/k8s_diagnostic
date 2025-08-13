/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  swcMinify: true,
  
  // API route configuration
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: '/api/:path*',
      },
    ]
  },

  // Environment variables
  env: {
    CUSTOM_KEY: 'k8s_diagnostic_ui',
  },

  // Webpack configuration for handling imports
  webpack: (config, { buildId, dev, isServer, defaultLoaders, webpack }) => {
    // Important: return the modified config
    return config;
  },

  // Experimental features (if needed)
  experimental: {
    // Enable app directory if using Next.js 13+
    // appDir: true,
  },

  // Output configuration
  output: 'standalone',
  
  // Disable eslint during build (optional, for faster builds)
  eslint: {
    ignoreDuringBuilds: false,
  },

  // Custom headers for better security
  async headers() {
    return [
      {
        source: '/(.*)',
        headers: [
          {
            key: 'X-Frame-Options',
            value: 'DENY',
          },
          {
            key: 'X-Content-Type-Options',
            value: 'nosniff',
          },
          {
            key: 'Referrer-Policy',
            value: 'origin-when-cross-origin',
          },
        ],
      },
      {
        source: '/api/:path*',
        headers: [
          {
            key: 'Cache-Control',
            value: 'no-cache, no-store, must-revalidate',
          },
        ],
      },
    ];
  },
}

module.exports = nextConfig
