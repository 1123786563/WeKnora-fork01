import * as SecureStore from 'expo-secure-store';
import { createOriginStorage } from './origin-storage';
export const nativeOriginStorage = createOriginStorage({ get: (key) => SecureStore.getItemAsync(key), set: (key, value) => SecureStore.setItemAsync(key, value), remove: (key) => SecureStore.deleteItemAsync(key) });
