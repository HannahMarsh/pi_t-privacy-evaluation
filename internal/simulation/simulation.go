package simulation

import (
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/data"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/simulation/rounds"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"math/rand"
	"sync"
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

	// Create a WaitGroup to synchronize goroutines
	var wg sync.WaitGroup

	// Use a channel to collect results from goroutines
	resultCh := make(chan struct {
		index int
		prob0 float64
		prob1 float64
		ratio float64
	}, numRuns)

	for i := 0; i < numRuns; i++ {
		wg.Add(1)

		// Start a goroutine for each simulation run
		go func(index int) {
			defer wg.Done()

			// Seed random number generator for this goroutine
			rand.Seed(int64(index))

			system := createGraph(p)
			prob0 := system.GetProb0()
			prob1 := system.GetProb1()
			ratio := system.GetRatio()

			// Send the results back via the channel
			resultCh <- struct {
				index int
				prob0 float64
				prob1 float64
				ratio float64
			}{
				index: index,
				prob0: prob0,
				prob1: prob1,
				ratio: ratio,
			}
		}(i)
	}

	// Wait for all goroutines to finish
	wg.Wait()
	close(resultCh)

	// Collect results from the channel
	for result := range resultCh {
		P0[result.index] = result.prob0
		P1[result.index] = result.prob1
		ratios[result.index] = result.ratio
	}

	return &data.Result{
		P:      p,
		Pr0:    P0,
		Pr1:    P1,
		Ratios: ratios,
	}
}
