// Unistyles 必须先于任何路由模块的模块级 StyleSheet.create 完成 configure。
// expo-router 的 require-context 使 "(app)/..." 先于 "_layout.tsx" 求值（ASCII '('<'_'），
// 导致 artifacts/[id].tsx 等在 theme.css 链执行前崩溃（2026-09-18 Android 实测）。
import './sources/unistyles.ts';
import 'expo-router/entry';
