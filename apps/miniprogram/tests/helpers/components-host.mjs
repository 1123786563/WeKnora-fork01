// '@tarojs/components' 契约替身：把每个 Taro 组件映射为携带 host 标记的普通
// React 元素，由 tests/helpers/host-render.mjs 的自定义 reconciler 落成纯对象树。
// 只承载渲染测试关心的部分：标签名、className、事件回调、表单值；样式属性（如
// Image 的 mode）原样保留在 props 上，便于断言。
import { createElement } from 'react';

/** host 元素类型标记：所有替身组件都渲染为 HOST 元素，真实标签放在 props.host。 */
export const HOST = 'wk-test-host';

function hostComponent(tag) {
  function HostComponent(props) {
    // key 由 createElement 从 config 提取；children 经 props 传入后同样交还 HOST 元素。
    return createElement(HOST, { ...props, host: tag }, props.children);
  }
  HostComponent.displayName = `TaroStub.${tag}`;
  return HostComponent;
}

export const View = hostComponent('view');
export const Text = hostComponent('text');
export const Button = hostComponent('button');
export const Input = hostComponent('input');
export const Textarea = hostComponent('textarea');
export const Image = hostComponent('image');
export const Checkbox = hostComponent('checkbox');
export const CheckboxGroup = hostComponent('checkbox-group');
export const ScrollView = hostComponent('scroll-view');
export const Swiper = hostComponent('swiper');
export const SwiperItem = hostComponent('swiper-item');
export const Label = hostComponent('label');
export const Progress = hostComponent('progress');
export const RichText = hostComponent('rich-text');
export const Slider = hostComponent('slider');
export const Switch = hostComponent('switch');
export const Picker = hostComponent('picker');
