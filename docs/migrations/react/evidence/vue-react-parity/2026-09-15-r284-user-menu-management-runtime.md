# R284 — User menu management shortcuts runtime evidence

- Environment: authenticated React Web page `http://localhost:5181/platform/knowledge-bases`, existing paritytester session.
- Action: opened the user menu through the accessible popup button.
- Observed AX menu entries: `新手引导`, `个人设置`, `空间信息`, `成员管理`, `模型管理`, `技能管理`, `退出`.
- This confirms the new management shortcuts are present in the real authenticated shell. Tenant switching and protected CRUD remain outside this slice.
