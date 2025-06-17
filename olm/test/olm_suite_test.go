package olm_test

import (
	"fmt"
	ge "github.com/onsi/gomega/gexec"
	"os"
	"os/exec"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestClusterOperator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OLM Suite", Label("E2E", "system"))
}

var _ = BeforeSuite(func() {
	const (
		mkPathEnvKey     = "MK_PATH"
		bundleImageKey   = "BUNDLE_IMAGE"
		bundleVersionKey = "BUNDLE_VERSION"
		catalogImageKey  = "CATALOG_IMAGE"
		registryKey      = "REGISTRY"
	)

	mkPath := "olm.mk"
	if s, isSet := os.LookupEnv(mkPathEnvKey); isSet {
		mkPath = s
	}

	cmdEnv := os.Environ()
	if s, isSet := os.LookupEnv(bundleImageKey); isSet {
		cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", bundleImageKey, s))
	}
	if s, isSet := os.LookupEnv(bundleVersionKey); isSet {
		cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", bundleVersionKey, s))
	}
	if s, isSet := os.LookupEnv(catalogImageKey); isSet {
		cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", catalogImageKey, s))
	}
	if s, isSet := os.LookupEnv(registryKey); isSet {
		cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", registryKey, s))
	}

	mkCmd := exec.Command("make", "-f", mkPath, "catalog-all")
	mkCmd.Env = cmdEnv

	session, err := ge.Start(mkCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())
	Eventually(session).WithTimeout(time.Minute).Should(ge.Exit(0))
})
