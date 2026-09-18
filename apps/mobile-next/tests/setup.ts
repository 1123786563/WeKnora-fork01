// jest 环境：为纯逻辑测试提供最小 RN/Expo mock。
// 运行真实 RN 组件渲染由 @testing-library/react-native 承担（jest-expo preset 已加载 RN transform）。
jest.mock("expo-secure-store", () => ({
  setItemAsync: jest.fn(async () => undefined),
  getItemAsync: jest.fn(async () => null),
  deleteItemAsync: jest.fn(async () => undefined),
}));

jest.mock("expo-linking", () => ({
  createURL: jest.fn((path: string) => `weknora://${path}`),
  parse: jest.fn((url: string) => ({ path: url.replace(/^weknora:\/\//, "") })),
}));
