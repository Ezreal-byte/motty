import { createMDX } from 'fumadocs-mdx/next';

const withMDX = createMDX();

/** @type {import('next').NextConfig} */
const config = {
  output: 'export',
  reactStrictMode: true,
  images: { unoptimized: true },
  agentRules: false,
  // next dev is reached through 127.0.0.1 as well as localhost. Without this the
  // dev server answers its HMR and dev-font requests with a cross-origin block,
  // so the page loads without hot reload and with fallback fonts.
  allowedDevOrigins: ['127.0.0.1'],
};

export default withMDX(config);
