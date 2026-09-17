import Taro from '@tarojs/taro';
import { pageUrl, ROUTES, type PageId } from '../core/routes.ts';
export async function navigate(id:PageId,params:Record<string,string|number|undefined>={}):Promise<void>{
  const url=pageUrl(id,params);
  if(ROUTES[id].tab)await Taro.switchTab({url});else await Taro.navigateTo({url});
}
export async function back():Promise<void>{
  if(Taro.getCurrentPages().length>1)await Taro.navigateBack();else await navigate('home');
}
export function routeParam(name:string):string{return Taro.getCurrentInstance().router?.params[name]??''}
