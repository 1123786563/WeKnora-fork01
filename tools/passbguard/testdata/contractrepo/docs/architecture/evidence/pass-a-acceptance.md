# Pass A 验收台账（fixture 摘录，基线计数参数化样本）

| 指标 | F0 | 终态 | 判定 |
|---|---|---|---|
| 路由注册 | 7（5 literal + 2 apiKeyRoute） | 7（同分解） | ✅ 零漂移 |
| Redis/Lite worker | 4 任务类型 + 6 池 / 4 | 同 | ✅ 集合一致（guard 双侧计数） |
| container.Invoke hooks | 9 | 9 | ✅ |
