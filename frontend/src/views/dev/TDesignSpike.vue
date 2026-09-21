<template>
  <div style="padding: 24px">
    <div>
      <t-button theme="primary">主按钮</t-button>
      <t-button theme="default" variant="outline">描边按钮</t-button>
      <t-button theme="danger">危险</t-button>
    </div>
    <div style="margin-top: 16px">
      <t-input placeholder="请输入" default-value="spike-input" />
      <t-select default-value="a" :options="selectOptions" />
    </div>
    <div style="margin-top: 16px">
      <t-tabs value="t1" :list="tabList" />
      <t-switch :default-value="true" />
      <t-tooltip content="提示文本"><t-button>Tooltip 宿主</t-button></t-tooltip>
    </div>
    <div style="margin-top: 16px">
      <t-table row-key="id" :data="rows" :columns="columns" />
    </div>
    <div style="margin-top: 16px">
      <t-button @click="dialogVisible = true">打开 Dialog</t-button>
      <t-dialog
        :visible="dialogVisible"
        header="Spike 弹窗"
        @confirm="dialogVisible = false"
        @close="dialogVisible = false"
      >
        <p>dialog-content-spike</p>
      </t-dialog>
      <t-button @click="triggerMessage">触发 Message</t-button>
      <t-button @click="triggerNotification">触发 Notification</t-button>
    </div>
    <div style="margin-top: 16px; font-size: 20px" data-spike-icons>
      <AddIcon size="20px" />
      <SearchIcon size="20px" />
      <DeleteIcon size="20px" />
      <EditIcon size="20px" />
      <CloseIcon size="20px" />
      <CheckIcon size="20px" />
      <UserIcon size="20px" />
      <SettingIcon size="20px" />
      <DownloadIcon size="20px" />
      <UploadIcon size="20px" />
      <RefreshIcon size="20px" />
      <ChevronLeftIcon size="20px" />
      <ChevronRightIcon size="20px" />
      <ChevronDownIcon size="20px" />
      <InfoCircleIcon size="20px" />
      <ErrorCircleIcon size="20px" />
      <CheckCircleIcon size="20px" />
      <TimeIcon size="20px" />
      <CalendarIcon size="20px" />
      <FolderIcon size="20px" />
    </div>
  </div>
</template>

<script setup lang="ts">
// Phase 0 throwaway spike 页：与 React 端 apps/web/src/dev/TDesignSpikePage.tsx
// 逐节点 1:1 对照（相同组件顺序 / 文本 / inline style / data-spike-icons），
// 供像素对比验证。Task 3 验证完成后整体删除。
// props 差异按 tdesign-vue-next 各自正确 API 写：
// - React `defaultValue` → Vue `default-value`；
// - Switch 的 defaultValue 类型为 [String, Number, Boolean]（String 在前），
//   Vue 布尔断言不生效，必须显式绑定 :default-value="true"；
// - Select/ Tabs 传 options / list 数组与 React 一致，用 :options / :list 绑定；
// - 事件用 @confirm / @close / @click 而非 onConfirm 等属性式回调；
// - 命令式 Notification 在 tdesign-vue-next 里导出名为 NotifyPlugin
//   （tdesign-react 为 NotificationPlugin），调用签名一致。
import { ref } from 'vue'
import { MessagePlugin, NotifyPlugin } from 'tdesign-vue-next'
import {
  AddIcon, SearchIcon, DeleteIcon, EditIcon, CloseIcon, CheckIcon,
  UserIcon, SettingIcon, DownloadIcon, UploadIcon, RefreshIcon,
  ChevronLeftIcon, ChevronRightIcon, ChevronDownIcon, InfoCircleIcon,
  ErrorCircleIcon, CheckCircleIcon, TimeIcon, CalendarIcon, FolderIcon,
} from 'tdesign-icons-vue-next'

const dialogVisible = ref(false)

const selectOptions = [
  { label: '选项一', value: 'a' },
  { label: '选项二', value: 'b' },
]

const tabList = [
  { label: '标签一', value: 't1' },
  { label: '标签二', value: 't2' },
]

const rows = [
  { id: 1, name: 'alpha', status: 'active' },
  { id: 2, name: 'beta', status: 'inactive' },
]

const columns = [
  { colKey: 'id', title: 'ID' },
  { colKey: 'name', title: '名称' },
  { colKey: 'status', title: '状态' },
]

function triggerMessage() {
  MessagePlugin.success('message-spike')
}

function triggerNotification() {
  NotifyPlugin.success({ title: 'notification-spike' })
}
</script>
