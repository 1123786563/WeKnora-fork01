export const knowledgeHeaderLayout = {
  chatRoute: '/chat' as const,
  container: {
    flexDirection: 'column' as const,
    marginBottom: 12,
  },
  title: {
    fontSize: 24,
    fontWeight: '700' as const,
  },
  actions: {
    flexDirection: 'row' as const,
    flexWrap: 'wrap' as const,
    gap: 12,
    width: '100%' as const,
  },
  actionText: {
    color: '#2864dc',
  },
} as const;
