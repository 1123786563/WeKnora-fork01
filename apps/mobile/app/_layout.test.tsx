import React from 'react';
import { create } from 'react-test-renderer';
import { afterEach, describe, expect, it, vi } from 'vitest';

const runtimeMock = vi.hoisted(() => ({
  calls: [] as Array<{ personalNode?: unknown; children?: React.ReactNode }>,
}));

vi.mock('expo-router', () => ({ Slot: () => null }));
vi.mock('expo-status-bar', () => ({ StatusBar: () => null }));
vi.mock('../src/runtime.tsx', () => ({
  MobileRuntimeProvider: (props: { personalNode?: unknown; children?: React.ReactNode }) => {
    runtimeMock.calls.push(props);
    return props.children;
  },
}));

import RootLayout, { createRootPersonalNode } from './_layout';

describe('mobile production root bootstrap', () => {
  const original = { ...process.env };
  afterEach(() => {
    runtimeMock.calls.length = 0;
    for (const key of Object.keys(process.env)) delete process.env[key];
    Object.assign(process.env, original);
  });

  it('composes the personal node from deployment configuration', () => {
    process.env.EXPO_PUBLIC_API_BASE_URL = 'https://api.example.test/api/v1';
    process.env.EXPO_PUBLIC_PASEO_URL = 'https://paseo.example.test';
    process.env.EXPO_PUBLIC_PASEO_ALLOWED_ORIGINS = 'https://paseo.example.test';
    expect(createRootPersonalNode()).toBeDefined();
    create(<RootLayout />).unmount();
    expect(runtimeMock.calls).toHaveLength(1);
    expect(runtimeMock.calls[0]?.personalNode).toBeDefined();
  });

  it('fails closed when the production configuration is incomplete', () => {
    delete process.env.EXPO_PUBLIC_API_BASE_URL;
    delete process.env.EXPO_PUBLIC_PASEO_URL;
    delete process.env.EXPO_PUBLIC_PASEO_ALLOWED_ORIGINS;
    expect(createRootPersonalNode()).toBeUndefined();
    create(<RootLayout />).unmount();
    expect(runtimeMock.calls).toHaveLength(1);
    expect(runtimeMock.calls[0]?.personalNode).toBeUndefined();
  });
});
