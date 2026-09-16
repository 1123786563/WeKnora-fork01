# r113 Android 知识库创建写路径验收

- `test36-small` 真实认证态打开知识库列表，点击“+ 新建知识库”。
- 空表单提交被阻止并显示“请输入知识库名称”校验提示，未发起写请求。
- 输入测试名称后提交成功，列表刷新并显示新建知识库卡片、描述回退“无描述”和“文档 · 0 项”。
- 为保持测试租户整洁，随后通过同一认证 API 删除该临时知识库；删除接口返回成功。
- 截图：`/tmp/android-kb-create-form.png`、`/tmp/android-kb-create-validation.png`、`/tmp/android-kb-created.png`。
