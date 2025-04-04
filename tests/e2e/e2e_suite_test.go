package e2e

import (
	"fmt"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/giantswarm/konfigure-operator/tests/utils"
)

var (
	seed = utils.GetEnv("KO_E2E_SEED", utils.RandomString(6))
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)

	_, _ = fmt.Fprintf(GinkgoWriter, "Running with KO E2E seed: %s\n", seed)

	RunSpecs(t, "e2e")
}

var _ = BeforeSuite(func() {
	By("generating files")
	cmd := exec.Command("make", "-C", "../..", "api")
	_, _ = fmt.Fprintf(GinkgoWriter, "Running make api command\n")
	_, err := utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to run make api")
})

var _ = AfterSuite(func() {
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting konfigure-operator E2E test suite\n")
})
