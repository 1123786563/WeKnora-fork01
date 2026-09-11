export interface OriginStore { get(key: string): Promise<string | null>; set(key: string, value: string): Promise<void>; remove(key: string): Promise<void>; }
export const SELECTED_ORIGIN_KEY = 'weknora:selected-origin';
export function createOriginStorage(store: OriginStore, key = SELECTED_ORIGIN_KEY) {
  return { read: () => store.get(key), write: (origin: string) => store.set(key, origin), clear: () => store.remove(key) };
}
