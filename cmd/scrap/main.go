package main

import (
	"fmt"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"math/rand"
)

// if we have a float array of size R called `relays` initialized to all 0 and we do this:
//
//	for i = 0; i < D; i++ {
//	   get random number r from 1 to R;
//	   relays[r - 1]++
//	}
//
// then we assign another array called probabilities = relays.map(relay -> relay / R)
//
// what should we expect the average
func main() {
	for R := 10; R < 100; R += 10 {

		relays := make([]float64, R)

		for i := 0; i < R; i++ {
			relays[rand.Intn(R)] += 1.0
		}

		probabilities := utils.Map(relays, func(f float64) float64 {
			return f / float64(R)
		})

		mean := utils.Mean(probabilities)
		max := utils.MaxOver(probabilities)
		min := utils.MinOver(probabilities)

		fmt.Printf("R: %d \t -> %f\n", R, mean)
		fmt.Printf("\t\tmax -> %f, min -> %f\n", max, min)
	}

}
