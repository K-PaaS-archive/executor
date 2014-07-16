package server

import (
	"os"
	"sync"

	"github.com/cloudfoundry-incubator/executor/api"
	"github.com/cloudfoundry-incubator/executor/registry"
	"github.com/cloudfoundry-incubator/executor/server/allocate_container"
	"github.com/cloudfoundry-incubator/executor/server/delete_container"
	"github.com/cloudfoundry-incubator/executor/server/get_container"
	"github.com/cloudfoundry-incubator/executor/server/initialize_container"
	"github.com/cloudfoundry-incubator/executor/server/list_containers"
	"github.com/cloudfoundry-incubator/executor/server/ping"
	"github.com/cloudfoundry-incubator/executor/server/remaining_resources"
	"github.com/cloudfoundry-incubator/executor/server/run_actions"
	"github.com/cloudfoundry-incubator/executor/server/total_resources"
	"github.com/cloudfoundry-incubator/executor/transformer"
	"github.com/cloudfoundry-incubator/garden/warden"
	"github.com/cloudfoundry/gosteno"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/rata"
)

type Server struct {
	Address               string
	Registry              registry.Registry
	WardenClient          warden.Client
	ContainerOwnerName    string
	ContainerMaxCPUShares uint64
	Transformer           *transformer.Transformer
	Logger                *gosteno.Logger
	RunWaitGroup          *sync.WaitGroup
	RunCanceller          chan struct{}
}

func (s *Server) Run(sigChan <-chan os.Signal, readyChan chan<- struct{}) error {

	handlers := s.NewHandlers(s.RunWaitGroup, s.RunCanceller)
	for key, handler := range handlers {
		handlers[key] = LogWrap(handler, s.Logger)
	}

	router, err := rata.NewRouter(api.Routes, handlers)
	if err != nil {
		return err
	}

	server := ifrit.Envoke(http_server.New(s.Address, router))

	close(readyChan)

	for {
		select {
		case sig := <-sigChan:
			server.Signal(sig)
			s.Logger.Info("executor.server.signaled-to-stop")
		case err := <-server.Wait():
			if err != nil {
				s.Logger.Errord(map[string]interface{}{
					"error": err.Error(),
				}, "executor.server.failed")
			}
			s.Logger.Info("executor.server.stopped")
			return err
		}
	}
}

func (s *Server) NewHandlers(runWaitGroup *sync.WaitGroup, runCanceller chan struct{}) rata.Handlers {
	return rata.Handlers{
		api.AllocateContainer:     allocate_container.New(s.Registry, s.Logger),
		api.GetContainer:          get_container.New(s.Registry, s.Logger),
		api.ListContainers:        list_containers.New(s.Registry, s.Logger),
		api.DeleteContainer:       delete_container.New(s.WardenClient, s.Registry, s.Logger),
		api.GetRemainingResources: remaining_resources.New(s.Registry, s.Logger),
		api.GetTotalResources:     total_resources.New(s.Registry, s.Logger),
		api.Ping:                  ping.New(s.WardenClient),
		api.InitializeContainer: initialize_container.New(
			s.ContainerOwnerName,
			s.ContainerMaxCPUShares,
			s.WardenClient,
			s.Registry,
			s.Logger,
		),
		api.RunActions: run_actions.New(
			s.WardenClient,
			s.Registry,
			s.Transformer,
			runWaitGroup,
			runCanceller,
			s.Logger,
		),
	}
}
