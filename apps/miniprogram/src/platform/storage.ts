import Taro from '@tarojs/taro';
import type { ValueStore } from '../core/intent.ts';
/** Ordinary device storage, not hardware-backed secure storage. No logs. */
export const storage:ValueStore={
  read(key){const value:unknown=Taro.getStorageSync(key);return value===''?undefined:value},
  write(key,value){Taro.setStorageSync(key,value)},
  remove(key){Taro.removeStorageSync(key)},
  keys(){return Taro.getStorageInfoSync().keys},
};
export function clearPrivateCache():void{
  for(const key of Taro.getStorageInfoSync().keys){if(key.startsWith('wk:')&&!key.startsWith('wk:auth:'))Taro.removeStorageSync(key)}
}
