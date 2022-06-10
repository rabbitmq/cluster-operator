package profiling_test

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rabbitmq/cluster-operator/pkg/profiling"
	"io"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
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
			MetricsBindAddress: metricsEndpoint,
		}
		mgr, err = ctrl.NewManager(cfg, opts)
		Expect(err).NotTo(HaveOccurred())
		mgr, err = profiling.AddDebugPprofEndpoints(mgr)
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
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		body, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).NotTo(BeEmpty())
	})
})
