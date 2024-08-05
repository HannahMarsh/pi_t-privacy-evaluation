package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/data"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/simulation"
	"golang.org/x/exp/slog"
)

func main() {

	C := flag.Int("C", 1000, "Number of clients")
	R := flag.Int("R", 1, "Number of relays")
	X := flag.Float64("X", 0.0, "Fraction of corrupted relays")
	serverLoad := flag.Float64("serverLoad", 100000.0, "Server load, i.e. the expected number of onions processed per relay per relay")
	L := flag.Int("L", 1, "Number of rounds")
	numRuns := flag.Int("numRuns", 1, "Number of runs")

	flag.Parse()

	//if int(*serverLoad) > int(float64(*L)*float64(*C)/float64(*R)) {
	//	pl.LogNewError(fmt.Sprintf("Server load %d too high. Reduce server load to %d.\n", int(*serverLoad), int(float64(*L)*float64(*C)/float64(*R))))
	//	os.Exit(1)
	//	return
	//}

	p := data.Parameters{
		C:          *C,
		R:          *R,
		X:          *X,
		ServerLoad: *serverLoad,
		L:          *L,
	}
	v := simulation.Run(p, *numRuns)

	str, err := json.Marshal(v)
	if err != nil {
		slog.Error("Couldn't marshall Result.", err)
	} else {
		fmt.Println(string(str))
	}
}
