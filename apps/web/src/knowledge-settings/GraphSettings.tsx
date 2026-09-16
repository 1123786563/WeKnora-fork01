import { useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Alert, Button, Input, Status } from '@weknora/ui';

// The Vue parity test vocabulary remains useful when reviewing this surface:
// Custom instructions, Relationship types, Sample text, Entities, Relations.

export interface GraphNode { name: string; attributes: string[] }
export interface GraphRelation { node1: string; node2: string; type: string }
export interface GraphExtractConfig {
  enabled: boolean;
  text: string;
  tags: string[];
  nodes: GraphNode[];
  relations: GraphRelation[];
  customInstructions: string;
}

type GraphAction = 'tags' | 'text' | 'relations';
type ActionState = { status: 'idle' | 'loading' | 'success' | 'error'; message?: string };

const exampleText = '"Romeo and Juliet" is a tragedy written by William Shakespeare early in his career, and is one of the most frequently performed plays in world literature. The play follows two young lovers from feuding families in Verona, Italy — the Montagues and the Capulets. Written around 1594-1596, it was first published in quarto in 1597. The full title is "The Most Excellent and Lamentable Tragedy of Romeo and Juliet."';

export function graphDatabaseEnabled(engine?: string): boolean { return Boolean(engine && engine !== 'Not Enabled'); }
export function graphActionPath(action: GraphAction): string {
  return action === 'tags' ? '/api/v1/initialization/extract/fabri-tag' : action === 'text' ? '/api/v1/initialization/extract/fabri-text' : '/api/v1/initialization/extract/text-relation';
}
export function shouldRenderGraphActions(canRunGraphExtract?: boolean): boolean { return canRunGraphExtract === true; }
export function clearDisabledGraphData(config: GraphExtractConfig): GraphExtractConfig { return { ...config, enabled: false, text: '', tags: [], nodes: [], relations: [] }; }
export function validateGraphSettings(config: GraphExtractConfig, modelId: string, action: GraphAction): string[] {
  if (!modelId.trim()) return ['completeModelConfig'];
  if (action === 'relations' && !config.text.trim()) return ['pleaseInputText'];
  return [];
}
export function graphExample(): GraphExtractConfig {
  return {
    enabled: true,
    text: exampleText,
    tags: ['Author', 'Alias'],
    nodes: [
      { name: 'Romeo and Juliet', attributes: ['One of the most frequently performed plays', 'Written around 1594-1596', 'A tragedy'] },
      { name: 'The Most Excellent and Lamentable Tragedy of Romeo and Juliet', attributes: ['Full title of Romeo and Juliet'] },
      { name: 'William Shakespeare', attributes: ['English playwright', 'Author of Romeo and Juliet'] },
      { name: 'Verona', attributes: ['City in Italy', 'Setting of the play'] },
    ],
    relations: [
      { node1: 'Romeo and Juliet', node2: 'The Most Excellent and Lamentable Tragedy of Romeo and Juliet', type: 'Alias' },
      { node1: 'Romeo and Juliet', node2: 'William Shakespeare', type: 'Author' },
      { node1: 'Romeo and Juliet', node2: 'Verona', type: 'Setting' },
    ],
    customInstructions: '',
  };
}

interface GraphSettingsProps {
  graphExtract: GraphExtractConfig;
  modelId: string;
  client?: WeKnoraClient;
  embedded?: boolean;
  canRunGraphExtract?: boolean;
  graphDatabaseEngine?: string;
  onChange: (value: GraphExtractConfig) => void;
  onOpenGraphGuide?: () => void;
}

const cloneConfig = (value: GraphExtractConfig): GraphExtractConfig => structuredClone(value);
const errorMessage = (error: unknown, fallback: string) => error instanceof Error ? error.message : fallback;

export function GraphSettings({ graphExtract, modelId, client, embedded = false, canRunGraphExtract = false, graphDatabaseEngine, onChange, onOpenGraphGuide }: GraphSettingsProps) {
  const [local, setLocal] = useState<GraphExtractConfig>(() => cloneConfig(graphExtract));
  const [system, setSystem] = useState<{ status: 'loading' | 'ready' | 'error'; engine?: string; message?: string }>({ status: graphDatabaseEngine ? 'ready' : 'loading', engine: graphDatabaseEngine });
  const [actions, setActions] = useState<Record<GraphAction, ActionState>>({ tags: { status: 'idle' }, text: { status: 'idle' }, relations: { status: 'idle' } });

  useEffect(() => setLocal(cloneConfig(graphExtract)), [graphExtract]);
  useEffect(() => {
    if (graphDatabaseEngine !== undefined) {
      setSystem({ status: 'ready', engine: graphDatabaseEngine });
      return;
    }
    if (!client) {
      setSystem({ status: 'error', message: '知识图谱数据库状态不可用' });
      return;
    }
    let active = true;
    void client.request({ method: 'GET', path: '/api/v1/system/info' }).then((value: any) => {
      if (active) setSystem({ status: 'ready', engine: value?.data?.graph_database_engine });
    }).catch((error: unknown) => {
      if (active) setSystem({ status: 'error', message: errorMessage(error, 'Unable to load graph database status') });
    });
    return () => { active = false; };
  }, [client, graphDatabaseEngine]);

  const update = (next: GraphExtractConfig) => { setLocal(next); onChange(next); };
  const updateField = <K extends keyof GraphExtractConfig>(field: K, value: GraphExtractConfig[K]) => update({ ...local, [field]: value });
  const action = async (kind: GraphAction) => {
    const issues = validateGraphSettings(local, modelId, kind);
    if (issues.length > 0) { setActions((state) => ({ ...state, [kind]: { status: 'error', message: issues[0] === 'completeModelConfig' ? 'Complete model configuration first.' : 'Enter sample text first.' } })); return; }
    if (!client) { setActions((state) => ({ ...state, [kind]: { status: 'error', message: 'Graph action is unavailable.' } })); return; }
    setActions((state) => ({ ...state, [kind]: { status: 'loading' } }));
    try {
      const value: any = await client.request({ method: 'POST', path: graphActionPath(kind), body: kind === 'tags' ? {} : { ...(kind === 'text' ? { tags: local.tags } : { text: local.text, tags: local.tags }), model_id: modelId } });
      if (kind === 'tags') updateField('tags', Array.isArray(value?.data?.tags) ? value.data.tags : []);
      if (kind === 'text') updateField('text', typeof value?.data?.text === 'string' ? value.data.text : '');
      if (kind === 'relations') {
        update({ ...local, nodes: Array.isArray(value?.data?.nodes) ? value.data.nodes : [], relations: Array.isArray(value?.data?.relations) ? value.data.relations : [] });
      }
      setActions((state) => ({ ...state, [kind]: { status: 'success', message: kind === 'tags' ? 'Tags generated.' : kind === 'text' ? 'Sample text generated.' : 'Entities and relations extracted.' } }));
    } catch (error: unknown) {
      setActions((state) => ({ ...state, [kind]: { status: 'error', message: errorMessage(error, 'Graph action failed.') } }));
    }
  };
  const enabled = graphDatabaseEnabled(system.engine);
  const setEnabled = (checked: boolean) => update(checked ? { ...local, enabled: true } : clearDisabledGraphData(local));
  const addNode = () => update({ ...local, nodes: [...local.nodes, { name: '', attributes: [] }] });
  const addRelation = () => update({ ...local, relations: [...local.relations, { node1: '', node2: '', type: '' }] });

  return <div className={embedded ? 'wk-graph-settings wk-graph-settings-embedded' : 'wk-graph-settings'}>
    {!embedded ? <header className="wk-graph-header"><h2>知识图谱配置</h2><p className="wk-muted">配置实体关系提取，从文本中自动构建知识图谱。</p></header> : null}
    {system.status === 'loading' ? <Status>Loading graph database status…</Status> : null}
    {system.status === 'error' ? <Status tone="error">{system.message}</Status> : null}
    {system.status === 'ready' && !enabled ? <Alert tone="warning"><div>知识图谱数据库未启用，实体关系提取功能将无法使用</div>{!embedded && onOpenGraphGuide ? <Button type="button" variant="text" onClick={onOpenGraphGuide}>如何启用知识图谱？</Button> : null}</Alert> : null}
    {enabled ? <div className="wk-graph-form">
      <div className="wk-graph-row"><div><strong>启用实体关系提取</strong><p className="wk-muted">从上传的文本中提取实体和关系。</p></div><label><input type="checkbox" checked={local.enabled} onChange={(event) => setEnabled(event.target.checked)} /> 启用</label></div>
      {local.enabled ? <>
        <label className="wk-graph-field"><strong>自定义指令</strong><span className="wk-muted">实体和关系提取的可选指令。</span><textarea maxLength={4000} rows={4} value={local.customInstructions} onChange={(event) => updateField('customInstructions', event.target.value)} /></label>
        <div className="wk-graph-field"><strong>关系类型</strong><span className="wk-muted">添加或生成提取使用的关系类型。</span><div className="wk-graph-inline"><Input aria-label="关系类型" value={local.tags.join(', ')} onChange={(event) => updateField('tags', event.target.value.split(',').map((tag) => tag.trim()).filter(Boolean))} />{shouldRenderGraphActions(canRunGraphExtract) ? <Button type="button" loading={actions.tags.status === 'loading'} disabled={!modelId} onClick={() => void action('tags')}>生成随机标签</Button> : null}</div><ActionStatus state={actions.tags} /></div>
        <div className="wk-graph-field"><strong>示例文本</strong><span className="wk-muted">使用示例文本预览提取的图谱结构。</span><div className="wk-graph-inline wk-graph-stack">{shouldRenderGraphActions(canRunGraphExtract) ? <Button type="button" loading={actions.text.status === 'loading'} disabled={!modelId} onClick={() => void action('text')}>生成随机文本</Button> : null}<textarea rows={7} maxLength={5000} value={local.text} onChange={(event) => updateField('text', event.target.value)} /></div><ActionStatus state={actions.text} /></div>
        {local.nodes.length > 0 ? <div className="wk-graph-field"><strong>实体列表</strong><span className="wk-muted">管理实体名称和属性。</span>{local.nodes.map((node, index) => <div className="wk-graph-node" key={index}><div className="wk-graph-inline"><Input aria-label={`Entity ${index + 1}`} placeholder="Entity name" value={node.name} onChange={(event) => update({ ...local, nodes: local.nodes.map((item, i) => i === index ? { ...item, name: event.target.value } : item) })} /><Button type="button" variant="text" onClick={() => update({ ...local, nodes: local.nodes.filter((_, i) => i !== index) })}>删除</Button></div>{node.attributes.map((attribute, attributeIndex) => <div className="wk-graph-inline" key={attributeIndex}><Input aria-label={`Entity ${index + 1} attribute ${attributeIndex + 1}`} value={attribute} onChange={(event) => update({ ...local, nodes: local.nodes.map((item, i) => i === index ? { ...item, attributes: item.attributes.map((value, ai) => ai === attributeIndex ? event.target.value : value) } : item) })} /><Button type="button" variant="text" onClick={() => update({ ...local, nodes: local.nodes.map((item, i) => i === index ? { ...item, attributes: item.attributes.filter((_, ai) => ai !== attributeIndex) } : item) })}>×</Button></div>)}<Button type="button" variant="text" onClick={() => update({ ...local, nodes: local.nodes.map((item, i) => i === index ? { ...item, attributes: [...item.attributes, ''] } : item) })}>添加属性</Button></div>)}<Button type="button" variant="primary" onClick={addNode}>添加实体</Button></div> : null}
        {local.relations.length > 0 ? <div className="wk-graph-field"><strong>关系列表</strong><span className="wk-muted">使用关系类型连接实体。</span>{local.relations.map((relation, index) => <div className="wk-graph-inline" key={index}><select aria-label={`Relation ${index + 1} source`} value={relation.node1} onChange={(event) => update({ ...local, relations: local.relations.map((item, i) => i === index ? { ...item, node1: event.target.value } : item) })}><option value="">Select entity</option>{local.nodes.map((node) => <option key={node.name} value={node.name}>{node.name}</option>)}</select><span aria-hidden="true">→</span><Input aria-label={`Relation ${index + 1} type`} value={relation.type} placeholder="Relation type" onChange={(event) => update({ ...local, relations: local.relations.map((item, i) => i === index ? { ...item, type: event.target.value } : item) })} /><span aria-hidden="true">→</span><select aria-label={`Relation ${index + 1} target`} value={relation.node2} onChange={(event) => update({ ...local, relations: local.relations.map((item, i) => i === index ? { ...item, node2: event.target.value } : item) })}><option value="">Select entity</option>{local.nodes.map((node) => <option key={node.name} value={node.name}>{node.name}</option>)}</select><Button type="button" variant="text" onClick={() => update({ ...local, relations: local.relations.filter((_, i) => i !== index) })}>删除</Button></div>)}<Button type="button" variant="primary" onClick={addRelation}>添加关系</Button></div> : null}
        <div className="wk-graph-field"><strong>提取操作</strong><span className="wk-muted">保存配置前，可使用示例文本运行提取。</span><div className="wk-graph-inline">{shouldRenderGraphActions(canRunGraphExtract) ? <Button type="button" variant="primary" loading={actions.relations.status === 'loading'} disabled={!modelId || !local.text.trim()} onClick={() => void action('relations')}>开始提取</Button> : null}<Button type="button" onClick={() => update(graphExample())}>默认示例</Button><Button type="button" onClick={() => update({ ...local, text: '', tags: [], nodes: [], relations: [] })}>清空示例</Button></div><ActionStatus state={actions.relations} /></div>
      </> : null}
    </div> : null}
  </div>;
}

function ActionStatus({ state }: { state: ActionState }) { return state.status === 'success' ? <Status tone="success">{state.message}</Status> : state.status === 'error' ? <Status tone="error">{state.message}</Status> : null; }
