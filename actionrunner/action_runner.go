package actionrunner

import (
	"fmt"
	"time"

	"github.com/cloudfoundry-incubator/runtime-schema/models"
	"github.com/vito/gordon"
)

type ActionRunnerInterface interface {
	Run(containerHandle string, actions []models.ExecutorAction) error
}

type BackendPlugin interface {
	BuildRunScript(models.RunAction) string
}

type ActionRunner struct {
	wardenClient  gordon.Client
	backendPlugin BackendPlugin
}

type RunActionTimeoutError struct {
	Action models.RunAction
}

func (e RunActionTimeoutError) Error() string {
	return fmt.Sprintf("action timed out after %s", e.Action.Timeout)
}

func New(wardenClient gordon.Client, backendPlugin BackendPlugin) *ActionRunner {
	return &ActionRunner{
		wardenClient:  wardenClient,
		backendPlugin: backendPlugin,
	}
}

func (runner *ActionRunner) Run(containerHandle string, actions []models.ExecutorAction) error {
	for _, action := range actions {
		var err error
		switch a := action.Action.(type) {
		case models.RunAction:
			err = runner.performRunAction(containerHandle, a)
		case models.CopyAction:
			// Copy
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (runner *ActionRunner) performRunAction(containerHandle string, action models.RunAction) error {
	exitStatusChan := make(chan uint32, 1)
	errChan := make(chan error, 1)

	var timeoutChan <-chan time.Time

	if action.Timeout != 0 {
		timeoutChan = time.After(action.Timeout)
	}

	go func() {
		_, stream, err := runner.wardenClient.Run(
			containerHandle,
			runner.backendPlugin.BuildRunScript(action),
		)

		if err != nil {
			errChan <- err
			return
		}

		for payload := range stream {
			if payload.ExitStatus != nil {
				exitStatusChan <- payload.GetExitStatus()
			}
		}
	}()

	select {
	case exitStatus := <-exitStatusChan:
		if exitStatus != 0 {
			return fmt.Errorf("Process returned with exit value: %d", exitStatus)
		}

		return nil

	case err := <-errChan:
		return err

	case <-timeoutChan:
		return RunActionTimeoutError{Action: action}
	}

	panic("unreachable")
}
