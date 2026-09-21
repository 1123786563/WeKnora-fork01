// Pass A compatibility alias for internal/modules/insights/metric — zero logic. Deleted by Pass B task B-insights.
//
// The metric package moved to internal/modules/insights/metric in Pass A (task A14).
// Its only importer (internal/application/service/metric_hook.go) was repaired in
// place and no forbidden shared file consumes the old path, so this alias has an
// empty forwarding surface (moauth precedent): it exists only to keep the old
// import path addressable until Pass B deletes it.
package metric
