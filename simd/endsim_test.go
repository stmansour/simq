package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEndSim(t *testing.T) {

	app.cfg.DispatcherURL = "http://localhost:8250/"
	s := Simulation{
		SID:       21,
		Directory: "/Users/stevemansour/Documents/src/go/src/simq/simd/simulations/21",
	}

	err := s.sendEndSimulationRequest()
	assert.NoError(t, err)

}
