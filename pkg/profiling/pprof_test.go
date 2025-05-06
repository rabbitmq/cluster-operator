package profiling_test

import (
	"context"
	"fmt"
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rabbitmq/cluster-operator/v2/pkg/profiling"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var _ = Describe("Pprof", func() {

	var (
		opts            ctrl.Options
		mgr             ctrl.Manager
		err             error
		metricsEndpoint string
	)

	BeforeEach(func() {
		metricsEndpoint, err = getFreePort()
		opts = ctrl.Options{
			Metrics: server.Options{BindAddress: metricsEndpoint},
		}
		var o *ctrl.Options
		o, err = profiling.AddDebugPprofEndpoints(&opts)
		mgr, err = ctrl.NewManager(cfg, opts)
		opts = *o
		Expect(err).NotTo(HaveOccurred())

	})

	It("should serve extra endpoints", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(ctx)).NotTo(HaveOccurred())
		}()
		<-mgr.Elected()
		endpoint := fmt.Sprintf("http://%s/debug/pprof", metricsEndpoint)
		resp, err := http.Get(endpoint)
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			_ = resp.Body.Close()
		}()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		body, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).NotTo(BeEmpty())
	})
})
