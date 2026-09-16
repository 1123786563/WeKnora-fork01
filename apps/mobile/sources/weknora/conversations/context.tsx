import * as React from 'react';
import type { ConversationViewModel } from './view-model';

export const ConversationViewModelContext = React.createContext<ConversationViewModel | null>(null);

export function useConversationViewModel(): ConversationViewModel | null {
  return React.useContext(ConversationViewModelContext);
}
