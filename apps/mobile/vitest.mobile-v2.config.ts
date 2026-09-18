import { defineConfig } from 'vitest/config';
import path from 'node:path';

/**
 * MX-008 挂载验证层专用配置：只收集 *.mobilev2.spec.tsx，
 * react-native 别名到 node 测试基底 mock（rn-mock.tsx）。
 * 不影响既有默认 vitest 入口（test 脚本仍为全量默认配置）。
 */
export default defineConfig({
  resolve: {
    alias: {
      'react-native': path.resolve(__dirname, 'tests/mobile-v2/rn-mock.tsx'),
    },
  },
  test: {
    include: ['sources/**/*.mobilev2.spec.{ts,tsx}'],
    environment: 'node',
  },
});
