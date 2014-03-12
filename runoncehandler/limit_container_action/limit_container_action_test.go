package limit_container_action_test

import (
	"errors"
	"github.com/cloudfoundry-incubator/executor/action_runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry-incubator/runtime-schema/models"
	steno "github.com/cloudfoundry/gosteno"
	"github.com/vito/gordon/fake_gordon"

	. "github.com/cloudfoundry-incubator/executor/runoncehandler/limit_container_action"
)

var _ = Describe("LimitContainerAction", func() {
	var action action_runner.Action

	var runOnce models.RunOnce
	var gordon *fake_gordon.FakeGordon
	var handle string
	var containerInodeLimit int

	BeforeEach(func() {
		gordon = fake_gordon.New()
		handle = "some-container-handle"
		containerInodeLimit = 200000
		runOnce = models.RunOnce{
			ContainerHandle: handle,
			Guid:            "totally-unique",
			Stack:           "penguin",
			MemoryMB:        1024,
			DiskMB:          512,
			Actions: []models.ExecutorAction{
				{
					models.RunAction{
						Script: "sudo reboot",
					},
				},
			},

			ExecutorID: "some-executor-id",
		}

		action = New(
			&runOnce,
			steno.NewLogger("test-logger"),
			gordon,
			containerInodeLimit,
		)
	})

	Describe("Perform", func() {
		var err error
		disaster := errors.New("oh no!")

		It("should limit memory", func() {
			err := action.Perform()
			Ω(err).Should(BeNil())

			Ω(gordon.MemoryLimits()).Should(HaveLen(1))
			Ω(gordon.MemoryLimits()[0].Handle).Should(Equal(handle))
			Ω(gordon.MemoryLimits()[0].Limit).Should(BeNumerically("==", 1024*1024*1024))
		})

		Context("when limiting memory fails", func() {
			BeforeEach(func() {
				gordon.SetLimitMemoryError(disaster)
				err = action.Perform()
			})

			It("sends back the error", func() {
				Ω(err).Should(Equal(disaster))
			})
		})

		It("should limit disk", func() {
			err := action.Perform()
			Ω(err).Should(BeNil())

			Ω(gordon.DiskLimits()).Should(HaveLen(1))
			Ω(gordon.DiskLimits()[0].Handle).Should(Equal(handle))
			Ω(gordon.DiskLimits()[0].Limits.ByteLimit).Should(BeNumerically("==", 512*1024*1024))
			Ω(gordon.DiskLimits()[0].Limits.InodeLimit).Should(BeNumerically("==", containerInodeLimit))
		})

		Context("when limiting disk fails", func() {
			BeforeEach(func() {
				gordon.SetLimitDiskError(disaster)
				err = action.Perform()
			})

			It("sends back the error", func() {
				Ω(err).Should(Equal(disaster))
			})
		})
	})
})
