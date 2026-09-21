// Package workbench is a Pass A forwarding alias placeholder for the moved
// workbench contract package.
//
// Deleted by Pass B task B-workbench.
//
// At Pass A time no forbidden shared file or frozen module consumed the old
// import path: every reference (internal/application/repository,
// internal/handler/session, internal/modules/workbench/service/workbench) was
// repaired to internal/modules/workbench in the same pass. This stub exists
// only to satisfy the manifest alias_obligations 1:1 rule
// (docs/architecture/moves/workbench.yaml); it forwards zero symbols and the
// IA4 integrator may delete it without any import switch.
package workbench
