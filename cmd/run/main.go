package main

import (
	"encoding/json"
	"flag"
	"fmt"
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/cmd/view"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/interfaces"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/model/client"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/model/node"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/system"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"go.uber.org/automaxprocs/maxprocs"
	"golang.org/x/exp/slog"
	"os"
)

//var multiRuns map[interfaces.Params][]MultiView

func main() {
	// Define command-line flags
	//logLevel := flag.String("log-level", "debug", "Log level")
	//N := flag.Int("N", 10, "Number of nodes")
	//R := flag.Int("R", 10, "Number of clients")
	//ServerLoad := flag.Int("ServerLoad", 2, "Serverload")
	//L := flag.Int("L", 5, "Number of layers")
	//X := flag.Float64("X", 1.0, "Fraction of corrupted nodes")
	//Scenario := flag.Int("Scenario", 0, "Scenario")
	//numRuns := flag.Int("numRuns", 3, "Number of runs")
	logLevel := flag.String("log-level", "debug", "Log level")
	N := flag.Int("N", 1000, "Number of nodes")
	R := flag.Int("R", 2000, "Number of clients")
	ServerLoad := flag.Int("ServerLoad", 150, "Serverload")
	L := flag.Int("L", 100, "Number of layers")
	X := flag.Float64("X", 1.0, "Fraction of corrupted nodes")
	Scenario := flag.Int("Scenario", 0, "Scenario")
	numRuns := flag.Int("numRuns", 100, "Number of runs")
	flag.Usage = flag.PrintDefaults
	flag.Parse()

	pl.SetUpLogrusAndSlog(*logLevel)

	//slog.Info("started run", "N", *N, "R", *R, "ServerLoad", *ServerLoad, "L", *L, "X", *X, "Scenario", *Scenario, "NumRums", *numRuns)

	// set GOMAXPROCS
	if _, err := maxprocs.Set(); err != nil {
		slog.Error("failed set max procs", err)
		os.Exit(1)
	}

	serverLoad := float64(*N) / float64(*L)
	serverLoad = serverLoad / float64(3)
	serverLoad = serverLoad * float64(*ServerLoad)

	p := interfaces.Params{
		N:          *N,
		R:          *R,
		L:          *L,
		ServerLoad: *ServerLoad,
		X:          *X,
		Scenario:   *Scenario,
	}
	runs := make([]view.View, *numRuns)

	cn := utils.RandomSubset(utils.NewIntArray(p.R+1, p.R+p.N+1), int(p.X*float64(p.N)))
	isNodeCorrupted := utils.Map(utils.NewIntArray(p.R+1, p.R+p.N+1), func(id int) bool {
		return utils.ContainsElement(cn, id)
	})

	newSystem := system.NewSystem(p)

	nodes := make(map[int]interfaces.Node)

	for id := 1; id <= p.R; id++ {
		c, err := client.NewClient(id, newSystem)
		if err != nil {
			slog.Error("failed to create client", err)
			os.Exit(1)
		}
		nodes[id] = c
	}

	for id := p.R + 1; id <= p.R+p.N; id++ {
		c, err := node.NewNode(id, isNodeCorrupted[id-p.R-1], newSystem)
		if err != nil {
			slog.Error("failed to create client", err)
			os.Exit(1)
		}
		nodes[id] = c
	}

	for index := 0; index < *numRuns; index++ {
		i := index
		newSystem.StartRun()

		probabilities := newSystem.GetProbabilities(2)

		runs[i] = view.View{
			ProbR:   probabilities[p.R],
			ProbR_1: probabilities[p.R-1],
		}
	}

	data := view.Data{
		Params: p,
		Views:  runs,
	}

	outputJson, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		slog.Error("failed to marshal JSON", err)
		os.Exit(1)
	}

	fmt.Println(string(outputJson))
}
