const createNextIntlPlugin = require('next-intl/plugin');

const withNextIntl = createNextIntlPlugin('./i18n.ts');

/** @type {import('next').NextConfig} */
const nextConfig = {
  // Enable React strict mode for better development experience
  reactStrictMode: true,

  // Disable standalone output due to Next.js 15.5.9 bug with route groups
  // See: https://github.com/vercel/next.js/issues/...
  // output: 'standalone',

  // Configure supported locales
  experimental: {
    // Enable experimental features if needed
  }
};

module.exports = withNextIntl(nextConfig);
