import { Button } from 'tdesign-react';
import { Card } from './ui.tsx';
import { modelUsageBindingLabel, type ModelUsageDetails } from './model-usage.ts';
import './config-u.css';

export function ModelUsageNotice({
  modelName,
  details,
  onClose,
}: {
  modelName: string;
  details: ModelUsageDetails;
  onClose: () => void;
}) {
  const knowledgeBaseTotal = Math.max(details.knowledge_base_total, details.knowledge_bases.length);
  const agentTotal = Math.max(details.agent_total, details.agents.length);
  return (
    <Card className="wk-configuration-usage" role="alert">
      <div className="wk-settings-panel-heading wk-cfg-mun-1">
        <div>
          <h2 className="wk-cfg-mun-2">Model is still in use</h2>
          <p className="wk-muted wk-cfg-mun-3">{modelName} cannot be deleted until its active bindings are removed.</p>
        </div>
        <Button type="button" theme="default" variant="outline" onClick={onClose}>Close</Button>
      </div>
      {knowledgeBaseTotal > 0 ? (
        <section>
          <h3>Knowledge bases ({knowledgeBaseTotal})</h3>
          <ul className="wk-list wk-cfg-mun-4">
            {details.knowledge_bases.map((item) => (
              <li key={item.id} className="wk-cfg-mun-5">
                <strong>{item.name || item.id}</strong>
                <small>{item.bindings.map(modelUsageBindingLabel).join(' · ')}</small>
              </li>
            ))}
          </ul>
          {knowledgeBaseTotal > details.knowledge_bases.length ? <p className="wk-muted wk-cfg-mun-6">Showing {details.knowledge_bases.length} of {knowledgeBaseTotal} knowledge-base bindings.</p> : null}
        </section>
      ) : null}
      {agentTotal > 0 ? (
        <section>
          <h3>Agents ({agentTotal})</h3>
          <ul className="wk-list wk-cfg-mun-4">
            {details.agents.map((item) => (
              <li key={item.id} className="wk-cfg-mun-5">
                <strong>{item.name || item.id}</strong>
                <small>{item.bindings.map(modelUsageBindingLabel).join(' · ')}</small>
              </li>
            ))}
          </ul>
          {agentTotal > details.agents.length ? <p className="wk-muted wk-cfg-mun-6">Showing {details.agents.length} of {agentTotal} agent bindings.</p> : null}
        </section>
      ) : null}
      {details.long_term_memory.bindings.length > 0 ? <section><h3>Long-term memory</h3><p>{details.long_term_memory.bindings.map(modelUsageBindingLabel).join(' · ')}</p></section> : null}
    </Card>
  );
}
