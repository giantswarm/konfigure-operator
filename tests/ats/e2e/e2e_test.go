package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manager", Ordered, func() {
	Context("Manager", func() {
		It("should run successfully", func() {
			By("compare integers")
			verifyControllerUp := func(g Gomega) {
				g.Expect(1).To(Equal(1), "It's madness!")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should run successfully 2", func() {
			By("compare strings")
			verifyControllerUp := func(g Gomega) {
				g.Expect("asd").To(Equal("asd"), "It's madness!")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})
	})
})
