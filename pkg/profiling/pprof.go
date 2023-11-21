package profiling

import (
	"net/http"
	"net/http/pprof"
	ctrl "sigs.k8s.io/controller-runtime"
)

func AddDebugPprofEndpoints(managerOpts *ctrl.Options) (*ctrl.Options, error) {
	pprofEndpoints := map[string]http.HandlerFunc{
		"/debug/pprof":              http.HandlerFunc(pprof.Index),
		"/debug/pprof/allocs":       http.HandlerFunc(pprof.Index),
		"/debug/pprof/block":        http.HandlerFunc(pprof.Index),
		"/debug/pprof/cmdline":      http.HandlerFunc(pprof.Cmdline),
		"/debug/pprof/goroutine":    http.HandlerFunc(pprof.Index),
		"/debug/pprof/heap":         http.HandlerFunc(pprof.Index),
		"/debug/pprof/mutex":        http.HandlerFunc(pprof.Index),
		"/debug/pprof/profile":      http.HandlerFunc(pprof.Profile),
		"/debug/pprof/symbol":       http.HandlerFunc(pprof.Symbol),
		"/debug/pprof/threadcreate": http.HandlerFunc(pprof.Index),
		"/debug/pprof/trace":        http.HandlerFunc(pprof.Trace),
	}
	if managerOpts.Metrics.ExtraHandlers == nil {
		managerOpts.Metrics.ExtraHandlers = make(map[string]http.Handler)
	}
	for path, handler := range pprofEndpoints {
		managerOpts.Metrics.ExtraHandlers[path] = handler
	}
	return managerOpts, nil
}
