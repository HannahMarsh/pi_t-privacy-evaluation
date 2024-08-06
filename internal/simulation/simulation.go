package simulation

import (
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/data"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/simulation/rounds"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
)

func createGraph(p data.Parameters) *rounds.Rounds {
	clientIds := utils.NewIntArray(1, p.C+1)
	relayIds := utils.NewIntArray(p.C+1, p.C+1+p.R)

	system := rounds.SetUpSystem(clientIds, relayIds, p)

	//slog.Info("here")

	initial := make(map[int]float64)
	for _, clientId := range clientIds {
		initial[clientId] = 0.0
	}
	initial[clientIds[1]] = 1.0

	system.CalculateProbabilities(initial)

	return system
}

func Run(p data.Parameters, numRuns int) *data.Result {
	P0 := make([]float64, numRuns)
	P1 := make([]float64, numRuns)
	ratios := make([]float64, numRuns)

	for i := 0; i < numRuns; i++ {

		index := i

		system := createGraph(p)

		P0[index] = system.GetProb0()
		P1[index] = system.GetProb1()
		ratios[index] = system.GetRatio()

	}

	return &data.Result{
		P:      p,
		Pr0:    P0,
		Pr1:    P1,
		Ratios: ratios,
	}
}
