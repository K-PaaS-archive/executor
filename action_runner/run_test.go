package action_runner_test

import (
	"runtime"
	"time"

	. "github.com/cloudfoundry-incubator/executor/action_runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Run", func() {
	var (
		actions          []Action
		performedActions []string
	)

	spyAction := func(name string) FakeAction {
		return FakeAction{
			perform: func(result chan<- error) {
				performedActions = append(performedActions, name)
				result <- nil
			},
		}
	}

	BeforeEach(func() {
		actions = []Action{
			spyAction("foo"),
			spyAction("bar"),
		}
	})

	It("runs the provided actions asynchronously", func() {
		Eventually(Run(actions...)).Should(Receive())

		Ω(performedActions).To(Equal([]string{"foo", "bar"}))
	})

	Context("when no one reads the result", func() {
		It("does not leak the goroutine that provides it", func() {
			before := runtime.NumGoroutine()

			Run(actions...)

			time.Sleep(100 * time.Millisecond) // give time for the actions to at least start

			after := runtime.NumGoroutine()
			Ω(after).Should(Equal(before))
		})
	})
})