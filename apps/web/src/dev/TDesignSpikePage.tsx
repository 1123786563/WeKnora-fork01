// apps/web/src/dev/TDesignSpikePage.tsx
import { useState } from 'react';
// React 19 必需：tdesign-react 命令式 API（Message/Notification/DialogPlugin）
// 依赖 ReactDOM.render 兼容层，React 19 下需 side-effect 引入官方 adapter
// （spike 关键验证点；正式迁移时该 import 应移入应用入口）。
import 'tdesign-react/es/_util/react-19-adapter';
import {
  Button, Input, Select, Table, Dialog, Tabs, Switch, Tooltip,
  MessagePlugin, NotificationPlugin,
} from 'tdesign-react';
import {
  AddIcon, SearchIcon, DeleteIcon, EditIcon, CloseIcon, CheckIcon,
  UserIcon, SettingIcon, DownloadIcon, UploadIcon, RefreshIcon,
  ChevronLeftIcon, ChevronRightIcon, ChevronDownIcon, InfoCircleIcon,
  ErrorCircleIcon, CheckCircleIcon, TimeIcon, CalendarIcon, FolderIcon,
} from 'tdesign-icons-react';

const ICONS = [
  AddIcon, SearchIcon, DeleteIcon, EditIcon, CloseIcon, CheckIcon,
  UserIcon, SettingIcon, DownloadIcon, UploadIcon, RefreshIcon,
  ChevronLeftIcon, ChevronRightIcon, ChevronDownIcon, InfoCircleIcon,
  ErrorCircleIcon, CheckCircleIcon, TimeIcon, CalendarIcon, FolderIcon,
];

const ROWS = [
  { id: 1, name: 'alpha', status: 'active' },
  { id: 2, name: 'beta', status: 'inactive' },
];

export default function TDesignSpikePage() {
  const [dialogVisible, setDialogVisible] = useState(false);
  return (
    <div style={{ padding: 24 }}>
      <div>
        <Button theme="primary">主按钮</Button>
        <Button theme="default" variant="outline">描边按钮</Button>
        <Button theme="danger">危险</Button>
      </div>
      <div style={{ marginTop: 16 }}>
        <Input placeholder="请输入" defaultValue="spike-input" />
        <Select defaultValue="a" options={[{ label: '选项一', value: 'a' }, { label: '选项二', value: 'b' }]} />
      </div>
      <div style={{ marginTop: 16 }}>
        <Tabs value="t1" list={[{ label: '标签一', value: 't1' }, { label: '标签二', value: 't2' }]} />
        <Switch defaultValue />
        <Tooltip content="提示文本"><Button>Tooltip 宿主</Button></Tooltip>
      </div>
      <div style={{ marginTop: 16 }}>
        <Table
          rowKey="id"
          data={ROWS}
          columns={[
            { colKey: 'id', title: 'ID' },
            { colKey: 'name', title: '名称' },
            { colKey: 'status', title: '状态' },
          ]}
        />
      </div>
      <div style={{ marginTop: 16 }}>
        <Button onClick={() => setDialogVisible(true)}>打开 Dialog</Button>
        <Dialog visible={dialogVisible} header="Spike 弹窗" onConfirm={() => setDialogVisible(false)} onClose={() => setDialogVisible(false)}>
          <p>dialog-content-spike</p>
        </Dialog>
        <Button onClick={() => MessagePlugin.success('message-spike')}>触发 Message</Button>
        <Button onClick={() => NotificationPlugin.success({ title: 'notification-spike' })}>触发 Notification</Button>
      </div>
      <div style={{ marginTop: 16, fontSize: 20 }} data-spike-icons>
        {ICONS.map((I, i) => <I key={i} size="20px" />)}
      </div>
    </div>
  );
}
