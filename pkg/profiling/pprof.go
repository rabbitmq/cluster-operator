package profiling

import (
	"net/http"
	"net/http/pprof"
	ctrl "sigs.k8s.io/controller-runtime"
)

func AddDebugPprofEndpoints(mgr ctrl.Manager) (ctrl.Manager, error) {
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
	for path, handler := range pprofEndpoints {
		err := mgr.AddMetricsExtraHandler(path, handler)
		if err != nil {
			return mgr, err
		}
	}
	return mgr, nil
}
