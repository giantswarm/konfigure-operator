//go:build functional
// +build functional

package e2e

import (
	"fmt"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo/v2" //nolint:all
	. "github.com/onsi/gomega"    //nolint:all

	"github.com/giantswarm/konfigure-operator/tests/ats/utils"
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
	cmd := exec.Command("make", "-C", "../../..", "api")
	_, _ = fmt.Fprintf(GinkgoWriter, "Running make api command\n")
	_, err := utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to run make api")

	//By("Install kind if not found")
	//cmd = exec.Command("kind", "version")
	//out, err := utils.Run(cmd)
	//if err != nil {
	//	_, _ = fmt.Fprintf(GinkgoWriter, "Kind not found, installing via go\n")
	//	cmd = exec.Command("go", "install", "sigs.k8s.io/kind@v0.27.0")
	//	_, err = utils.Run(cmd)
	//	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed find and install kind")
	//} else {
	//	_, _ = fmt.Fprintf(GinkgoWriter, "Kind found of version:\n%s\n", out)
	//}
	//
	//By("Create kind cluster")
	//cmd = exec.Command("kind", "create", "cluster", "--name", "ko-e2e-"+seed) //nolint:all
	//_, err = utils.Run(cmd)
	//ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to create kind cluster")
})

var _ = AfterSuite(func() {
	By("Clean up kind cluster")
	cmd := exec.Command("kind", "delete", "cluster", "--name", "ko-e2e-"+seed) //nolint:all
	_, err := utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to clean up kind cluster")
})
