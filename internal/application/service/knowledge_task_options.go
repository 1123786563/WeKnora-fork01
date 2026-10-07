package service

import (
	"time"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/hibiken/asynq"
)

func DocumentProcessTaskOptions(cfg *config.Config, extra ...asynq.Option) []asynq.Option {
	opts := []asynq.Option{
		asynq.Queue(types.QueueDefault),
		asynq.Timeout(config.DocumentProcessTimeout(cfg)),
		asynq.MaxRetry(3),
	}
	opts = append(opts, extra...)
	return opts
}

func KnowledgePostProcessTaskOptions() []asynq.Option {
	return []asynq.Option{
		asynq.Queue(types.QueuePostProcess),
		asynq.MaxRetry(3),
		asynq.Timeout(30 * time.Minute),
	}
}
