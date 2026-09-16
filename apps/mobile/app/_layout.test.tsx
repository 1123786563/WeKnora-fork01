import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('expo-router', () => ({ Slot: () => null }));
vi.mock('expo-status-bar', () => ({ StatusBar: () => null }));
vi.mock('../src/runtime.tsx', () => ({ MobileRuntimeProvider: ({ children }: { children: unknown }) => children }));

import { createRootPersonalNode } from './_layout';

describe('mobile production root bootstrap', () => {
  const original = { ...process.env };
  afterEach(() => {
    for (const key of Object.keys(process.env)) delete process.env[key];
    Object.assign(process.env, original);
  });

  it('composes the personal node from deployment configuration', () => {
    process.env.EXPO_PUBLIC_API_BASE_URL = 'https://api.example.test/api/v1';
    process.env.EXPO_PUBLIC_PASEO_URL = 'https://paseo.example.test';
    process.env.EXPO_PUBLIC_PASEO_ALLOWED_ORIGINS = 'https://paseo.example.test';
    expect(createRootPersonalNode()).toBeDefined();
  });

  it('fails closed when the production configuration is incomplete', () => {
    delete process.env.EXPO_PUBLIC_API_BASE_URL;
    delete process.env.EXPO_PUBLIC_PASEO_URL;
    delete process.env.EXPO_PUBLIC_PASEO_ALLOWED_ORIGINS;
    expect(createRootPersonalNode()).toBeUndefined();
  });
});
