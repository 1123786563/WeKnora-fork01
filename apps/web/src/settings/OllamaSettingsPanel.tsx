// Ollama settings — TDesign 同构迁移（T12b）平移自
// frontend/src/views/settings/OllamaSettings.vue。DOM/类名/文案逐节点对照
// Vue SFC（t-tag 状态徽标 / t-button 重新检测 / t-input 禁用地址 /
// t-alert 失败告警 / 下载与已装模型分区）；样式在 settings.td.css §10。
import { useEffect, useRef, useState } from 'react';
import type { OllamaModel, OllamaStatus, SettingsPayload, WeKnoraClient } from '@weknora/api-client';
import { Icon as TIcon } from 'tdesign-icons-react';
import { Alert as TAlert, Button as TButton, Input as TInput, Loading as TLoading, Progress as TProgress, Tag as TTag } from 'tdesign-react';
import { ollamaModelInput } from './surface.ts';
import { readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';
import { pushSettingsToast } from './settings-toast.tsx';

type OllamaPayload = { status?: OllamaStatus; models?: OllamaModel[] };
function object(value: unknown): Record<string, unknown> { return value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {}; }
function payload(value: unknown): OllamaPayload { const row = object(value); return { status: object(row.status) as OllamaStatus, models: Array.isArray(row.models) ? row.models.filter((item): item is OllamaModel => item !== null && typeof item === 'object' && typeof (item as Record<string, unknown>).name === 'string') : [] }; }
function taskId(value: SettingsPayload): string { const id = value.task_id ?? value.taskId ?? value.id; return typeof id === 'string' || typeof id === 'number' ? String(id) : ''; }

// Vue OllamaSettings formatSize: tiered fixed-2 units.
function formatSize(bytes: number | undefined): string {
  const value = Number(bytes);
  if (!value || value === 0 || Number.isNaN(value)) return '0 B';
  if (value < 1024) return value + ' B';
  if (value < 1024 * 1024) return (value / 1024).toFixed(2) + ' KB';
  if (value < 1024 * 1024 * 1024) return (value / (1024 * 1024)).toFixed(2) + ' MB';
  return (value / (1024 * 1024 * 1024)).toFixed(2) + ' GB';
}

// Vue OllamaSettings formatDate: 今天/昨天/N 天前, then the locale date.
function formatDate(dateStr: string | undefined, t: (key: string, values?: Record<string, string | number>) => string): string {
  if (!dateStr) return t('ollama.unknown');
  const date = new Date(dateStr);
  if (Number.isNaN(date.getTime())) return t('ollama.unknown');
  const days = Math.floor((Date.now() - date.getTime()) / (1000 * 60 * 60 * 24));
  if (days === 0) return t('ollama.today');
  if (days === 1) return t('ollama.yesterday');
  if (days < 7) return t('ollama.daysAgo', { days });
  return date.toLocaleDateString();
}

export function OllamaSettingsPanel({ client, initialValue }: { client: WeKnoraClient; initialValue: unknown }) {
  const initial = payload(initialValue);
  const t = settingsT(readInitialLocale());
  // Vue localBaseUrl：store 配置 → 探测回填 → 默认 http://localhost:11434。
  const [localBaseUrl, setLocalBaseUrl] = useState(typeof initial.status?.baseUrl === 'string' && initial.status.baseUrl ? initial.status.baseUrl : 'http://localhost:11434');
  const [status, setStatus] = useState<OllamaStatus | null>(initial.status ?? null);
  const [models, setModels] = useState<OllamaModel[]>(initial.models ?? []);
  const [loadingModels, setLoadingModels] = useState(false);
  const [testing, setTesting] = useState(false);
  const [downloading, setDownloading] = useState(false);
  const [downloadModelName, setDownloadModelName] = useState('');
  const [downloadProgress, setDownloadProgress] = useState(0);
  const progressTimerRef = useRef<number | null>(null);

  const connectionStatus: boolean | null = testing ? null : status ? status.available === true : null;

  useEffect(() => {
    const next = payload(initialValue);
    setStatus(next.status ?? { available: false });
    setModels(next.models ?? []);
    if (typeof next.status?.baseUrl === 'string' && next.status.baseUrl) setLocalBaseUrl(next.status.baseUrl);
  }, [initialValue]);

  useEffect(() => () => { if (progressTimerRef.current !== null) window.clearInterval(progressTimerRef.current); }, []);

  // Vue refreshModels（listOllamaModels）。
  async function refreshModels() {
    setLoadingModels(true);
    try {
      const nextModels = await client.settings.ollama.models();
      setModels(nextModels);
    } catch (reason) {
      pushSettingsToast(reason instanceof Error ? reason.message : t('ollamaSettings.toasts.listFailed'));
    } finally { setLoadingModels(false); }
  }

  // Vue testConnection（checkOllamaStatus）。
  async function testConnection() {
    setTesting(true);
    setStatus(null);
    try {
      const nextStatus = await client.settings.ollama.status();
      if (typeof nextStatus.baseUrl === 'string' && nextStatus.baseUrl && nextStatus.baseUrl !== localBaseUrl) setLocalBaseUrl(nextStatus.baseUrl);
      setStatus(nextStatus);
      if (nextStatus.available === true) {
        pushSettingsToast(t('ollamaSettings.toasts.connected'), 'success');
        void refreshModels();
      } else {
        pushSettingsToast((nextStatus as { error?: string }).error || t('ollamaSettings.toasts.connectFailed'));
      }
    } catch (reason) {
      setStatus({ available: false });
      pushSettingsToast(reason instanceof Error ? reason.message : t('ollamaSettings.toasts.connectFailed'));
    } finally { setTesting(false); }
  }

  // Vue downloadModel + 1s 进度轮询（getDownloadProgress）。
  async function downloadModel() {
    if (!downloadModelName.trim()) return;
    setDownloading(true);
    setDownloadProgress(0);
    try {
      const result = await client.settings.ollama.download(ollamaModelInput(downloadModelName));
      const id = taskId(result);
      if ((result as { status?: string }).status === 'failed' || !id) {
        pushSettingsToast(t('ollamaSettings.toasts.downloadFailed'));
        setDownloading(false);
        setDownloadProgress(0);
        return;
      }
      pushSettingsToast(t('ollamaSettings.toasts.downloadStarted', { name: downloadModelName }), 'success');
      if (progressTimerRef.current !== null) window.clearInterval(progressTimerRef.current);
      progressTimerRef.current = window.setInterval(async () => {
        try {
          const task = await client.settings.ollama.progress(id);
          const value = typeof task.progress === 'number' ? task.progress : 0;
          setDownloadProgress(value);
          if (task.status === 'completed') {
            window.clearInterval(progressTimerRef.current!);
            progressTimerRef.current = null;
            pushSettingsToast(t('ollamaSettings.toasts.downloadCompleted', { name: downloadModelName }), 'success');
            setDownloadModelName('');
            setDownloadProgress(0);
            setDownloading(false);
            void refreshModels();
          } else if (task.status === 'failed') {
            window.clearInterval(progressTimerRef.current!);
            progressTimerRef.current = null;
            pushSettingsToast((task as { message?: string }).message || t('ollamaSettings.toasts.downloadFailed'));
            setDownloading(false);
            setDownloadProgress(0);
          }
        } catch {
          window.clearInterval(progressTimerRef.current!);
          progressTimerRef.current = null;
          pushSettingsToast(t('ollamaSettings.toasts.progressFailed'));
          setDownloading(false);
          setDownloadProgress(0);
        }
      }, 1000);
    } catch (reason) {
      pushSettingsToast(reason instanceof Error ? reason.message : t('ollamaSettings.toasts.downloadFailed'));
      setDownloading(false);
      setDownloadProgress(0);
    }
  }

  return (
    <div className="ollama-settings">
      <div className="section-header">
        <h2>{t('ollamaSettings.title')}</h2>
        <p className="section-description">{t('ollamaSettings.description')}</p>
      </div>

      <div className="settings-group">
        {/* Ollama 服务状态 */}
        <div className="setting-row">
          <div className="setting-info">
            <label>{t('ollamaSettings.status.label')}</label>
            <p className="desc">{t('ollamaSettings.status.desc')}</p>
          </div>
          <div className="setting-control">
            <div className="status-display">
              {/* Vue 模板 t-tag 内 icon 与 {{ }} 插值间的换行缩进编译为一个
                  前导空格文本（「 不可用」）；单文本节点复刻（台账 #11/#13
                  先例：模板字符串拼空格，勿拆两个文本节点）。 */}
              {testing ? <TTag theme="default" variant="light">
                <TIcon name="loading" className="status-icon spinning" />
                {` ${t('ollamaSettings.status.testing')}`}
              </TTag>
                : connectionStatus === true ? <TTag theme="success" variant="light">
                <TIcon name="check-circle-filled" />
                {` ${t('ollamaSettings.status.available')}`}
              </TTag>
                  : connectionStatus === false ? <TTag theme="danger" variant="light">
                <TIcon name="close-circle-filled" />
                {` ${t('ollamaSettings.status.unavailable')}`}
              </TTag>
                    : <TTag theme="default" variant="light">
                <TIcon name="help-circle" />
                {` ${t('ollamaSettings.status.untested')}`}
              </TTag>}
              {/* Vue #icon slot 使图标渲染在 .t-button__text 之外（icon prop
                  直译，playbook §3.3）；label 同样带前导空格。 */}
              <TButton size="small" variant="text" loading={testing} icon={<TIcon name="refresh" />} onClick={() => void testConnection()}>
                {` ${t('ollamaSettings.status.retest')}`}
              </TButton>
            </div>
          </div>
        </div>

        {/* Ollama 服务地址 */}
        <div className="setting-row">
          <div className="setting-info">
            <label>{t('ollamaSettings.address.label')}</label>
            <p className="desc">{t('ollamaSettings.address.desc')}</p>
          </div>
          <div className="setting-control">
            <div className="url-control-group">
              <TInput
                value={localBaseUrl}
                placeholder={t('ollamaSettings.address.placeholder')}
                disabled
                style={{ flex: 1 }}
              />
            </div>
            {connectionStatus === false ? <TAlert
              theme="warning"
              message={t('ollamaSettings.address.failed')}
              style={{ marginTop: '8px' }}
            /> : null}
          </div>
        </div>
      </div>

      {/* 下载新模型 */}
      {connectionStatus === true ? <div className="model-category-section">
        <div className="category-header">
          <div className="header-info">
            <h3>{t('ollamaSettings.download.title')}</h3>
            <p>
              {t('ollamaSettings.download.descPrefix')}
              {/* Vue 文本插值与 t-icon 间的换行缩进＝尾部空格文本节点
                  （「浏览模型库 」），单文本节点复刻。 */}
              <a href="https://ollama.com/search" target="_blank" rel="noopener noreferrer" className="doc-link">
                {`${t('ollamaSettings.download.browse')} `}
                <TIcon name="link" className="link-icon" />
              </a>
            </p>
          </div>
        </div>

        <div className="download-content">
          <div className="input-group">
            <TInput
              value={downloadModelName}
              placeholder={t('ollamaSettings.download.placeholder')}
              style={{ flex: 1 }}
              onChange={(value) => setDownloadModelName(String(value ?? ''))}
            />
            <TButton
              variant="base"
              theme="default"
              size="small"
              className="download-btn"
              loading={downloading}
              disabled={!downloadModelName.trim()}
              icon={<TIcon name="download" />}
              onClick={() => void downloadModel()}
            >
              {` ${t('ollamaSettings.download.download')}`}
            </TButton>
          </div>

          {downloadProgress > 0 ? <div className="download-progress">
            <div className="progress-info">
              <span>{t('ollamaSettings.download.downloading', { name: downloadModelName })}</span>
              <span>{downloadProgress.toFixed(2)}%</span>
            </div>
            <TProgress percentage={downloadProgress} size="small" />
          </div> : null}
        </div>
      </div> : null}

      {/* 已下载的模型 */}
      {connectionStatus === true ? <div className="model-category-section">
        <div className="category-header">
          <div className="header-info">
            <h3>{t('ollamaSettings.installed.title')}</h3>
            <p>{t('ollamaSettings.installed.desc')}</p>
          </div>
          <TButton size="small" variant="text" loading={loadingModels} icon={<TIcon name="refresh" />} onClick={() => void refreshModels()}>
            {` ${t('common.refresh')}`}
          </TButton>
        </div>

        {loadingModels ? <div className="loading-state">
          <TLoading size="small" />
          <span>{t('common.loading')}</span>
        </div>
          : models.length > 0 ? <div className="model-list-container">
            {models.map((model) => (
              <div key={model.name} className="model-card">
                <div className="model-info">
                  <div className="model-name">{model.name}</div>
                  <div className="model-meta">
                    <span className="model-size">{formatSize(Number(model.size))}</span>
                    <span className="model-modified">{formatDate(model.modified_at, t)}</span>
                  </div>
                </div>
              </div>
            ))}
          </div>
            : <div className="empty-state">
              <p className="empty-text">{t('ollamaSettings.installed.empty')}</p>
            </div>}
      </div> : null}
    </div>
  );
}
