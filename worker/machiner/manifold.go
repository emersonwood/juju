// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package machiner

import (
	"github.com/juju/errors"
	"gopkg.in/juju/names.v2"
	worker "gopkg.in/juju/worker.v1"

	"github.com/juju/juju/agent"
	apiagent "github.com/juju/juju/api/agent"
	"github.com/juju/juju/api/base"
	apimachiner "github.com/juju/juju/api/machiner"
	"github.com/juju/juju/cmd/jujud/agent/engine"
	"github.com/juju/juju/worker/dependency"
)

// ManifoldConfig defines the names of the manifolds on which a
// Manifold will depend.
type ManifoldConfig engine.AgentAPIManifoldConfig

// Manifold returns a dependency manifold that runs a machiner worker, using
// the resource names defined in the supplied config.
func Manifold(config ManifoldConfig) dependency.Manifold {
	typedConfig := engine.AgentAPIManifoldConfig(config)
	return engine.AgentAPIManifold(typedConfig, newWorker)
}

// newWorker non-trivially wraps NewMachiner to specialise a engine.AgentAPIManifold.
//
// TODO(waigani) This function is currently covered by functional tests
// under the machine agent. Add unit tests once infrastructure to do so is
// in place.
func newWorker(a agent.Agent, apiCaller base.APICaller) (worker.Worker, error) {
	currentConfig := a.CurrentConfig()

	// TODO(fwereade): this functionality should be on the
	// machiner facade instead -- or, better yet, separate
	// the networking concerns from the lifecycle ones and
	// have completey separate workers.
	//
	// (With their own facades.)
	agentFacade := apiagent.NewState(apiCaller)
	modelConfig, err := agentFacade.ModelConfig()
	if err != nil {
		return nil, errors.Errorf("cannot read environment config: %v", err)
	}

	ignoreMachineAddresses, _ := modelConfig.IgnoreMachineAddresses()
	// Containers only have machine addresses, so we can't ignore them.
	tag := currentConfig.Tag()
	if names.IsContainerMachine(tag.Id()) {
		ignoreMachineAddresses = false
	}
	if ignoreMachineAddresses {
		logger.Infof("machine addresses not used, only addresses from provider")
	}
	accessor := APIMachineAccessor{apimachiner.NewState(apiCaller)}
	w, err := NewMachiner(Config{
		MachineAccessor: accessor,
		Tag:             tag.(names.MachineTag),
		ClearMachineAddressesOnStart: ignoreMachineAddresses,
		NotifyMachineDead: func() error {
			return agent.SetCanUninstall(a)
		},
	})
	if err != nil {
		return nil, errors.Annotate(err, "cannot start machiner worker")
	}
	return w, err
}
