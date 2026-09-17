import './platform/polyfills.ts';
import type { PropsWithChildren } from 'react';
import { useLaunch, useDidHide } from '@tarojs/taro';
import { auth, stopSubscriptions } from './services/runtime.ts';
import './app.scss';
export default function App({children}:PropsWithChildren){
  useLaunch(()=>{void auth.bootstrap()});
  useDidHide(stopSubscriptions);
  return children;
}
