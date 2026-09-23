import { Button } from 'tdesign-react';
import { Card } from './ui.tsx';
import { modelUsageBindingLabel, type ModelUsageDetails } from './model-usage.ts';

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
      <div className="wk-settings-panel-heading flex items-start justify-between gap-4 border-b border-[#eef1f5] pb-4 mb-4 max-[720px]:flex-col">
        <div>
          <h2 className="my-1">Model is still in use</h2>
          <p className="wk-muted text-muted m-0">{modelName} cannot be deleted until its active bindings are removed.</p>
        </div>
        <Button type="button" theme="default" variant="outline" onClick={onClose}>Close</Button>
      </div>
      {knowledgeBaseTotal > 0 ? (
        <section>
          <h3>Knowledge bases ({knowledgeBaseTotal})</h3>
          <ul className="wk-list m-0 list-none p-0">
            {details.knowledge_bases.map((item) => (
              <li key={item.id} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]">
                <strong>{item.name || item.id}</strong>
                <small>{item.bindings.map(modelUsageBindingLabel).join(' · ')}</small>
              </li>
            ))}
          </ul>
          {knowledgeBaseTotal > details.knowledge_bases.length ? <p className="wk-muted text-muted">Showing {details.knowledge_bases.length} of {knowledgeBaseTotal} knowledge-base bindings.</p> : null}
        </section>
      ) : null}
      {agentTotal > 0 ? (
        <section>
          <h3>Agents ({agentTotal})</h3>
          <ul className="wk-list m-0 list-none p-0">
            {details.agents.map((item) => (
              <li key={item.id} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]">
                <strong>{item.name || item.id}</strong>
                <small>{item.bindings.map(modelUsageBindingLabel).join(' · ')}</small>
              </li>
            ))}
          </ul>
          {agentTotal > details.agents.length ? <p className="wk-muted text-muted">Showing {details.agents.length} of {agentTotal} agent bindings.</p> : null}
        </section>
      ) : null}
      {details.long_term_memory.bindings.length > 0 ? <section><h3>Long-term memory</h3><p>{details.long_term_memory.bindings.map(modelUsageBindingLabel).join(' · ')}</p></section> : null}
    </Card>
  );
}
