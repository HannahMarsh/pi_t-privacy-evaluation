package main

import (
	"encoding/json"
	"flag"
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/cmd/view"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/interfaces"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/model/client"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/model/node"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/system"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"go.uber.org/automaxprocs/maxprocs"
	"golang.org/x/exp/slog"
	"io/ioutil"
	"log"
	"os"
)

//var multiRuns map[interfaces.Params][]MultiView

func main() {
	// Define command-line flags
	logLevel := flag.String("log-level", "debug", "Log level")
	N := flag.Int("N", 10, "Number of nodes")
	R := flag.Int("R", 10, "Number of clients")
	ServerLoad := flag.Int("ServerLoad", 2, "Serverload")
	L := flag.Int("L", 5, "Number of layers")
	X := flag.Float64("X", 1.0, "Fraction of corrupted nodes")
	Scenario := flag.Int("Scenario", 0, "Scenario")
	numRuns := flag.Int("numRuns", 3, "Number of runs")
	filePath := flag.String("filePath", "static/data.json", "File path")
	flag.Usage = flag.PrintDefaults
	flag.Parse()

	//for i := 0; i < 20; i++ {
	//	kk := utils.Max(1, int((rand.NormFloat64()*(*StdDev))+float64(*ServerLoad)))
	//	fmt.Println(kk)
	//}

	pl.SetUpLogrusAndSlog(*logLevel)

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
	//slog.Info("Starting runs", "N", N, "R", R, "ServerLoad", ServerLoad, "L", L, "X", X, "StdDev", StdDev, "Scenario", Scenario)

	for i := 0; i < *numRuns; i++ {
		if err := newSystem.StartRun(); err != nil {
			slog.Error("failed to start run", err)
			os.Exit(1)
		}

		probabilities := newSystem.GetProbabilities(2)
		receivedR := newSystem.GetNumOnionsReceived(p.R)
		receivedR_1 := newSystem.GetNumOnionsReceived(p.R - 1)
		//probScen0 := probabilities[p.R-1]
		//probScen1 := probabilities[p.R]
		//
		//if probScen0+probScen1 == 0 {
		//	probScen0 = 0.5
		//	probScen1 = 0.5
		//}

		runs[i] = view.View{
			Probabilities: probabilities[:p.R+1],
			ReceivedR:     receivedR,
			ReceivedR_1:   receivedR_1,
			//ProbScen0:     probScen0 / (probScen1 + probScen0), // normalise
			//ProbScen1:     probScen1 / (probScen1 + probScen0), // normalise
		}
	}

	// Open the file with the appropriate flags
	file, err := os.OpenFile(*filePath, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		log.Fatalf("Failed to open file: %s", err)
	}
	defer file.Close()

	// Read the file contents
	fileContent, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatalf("Failed to read file: %s", err)
	}

	var allData view.AllData

	// Unmarshal the JSON content into a struct
	if err = json.Unmarshal(fileContent, &allData); err != nil {
		//slog.Error("failed to unmarshal JSON", err)
		allData.Data = make([]view.Data, 0)
	}

	//	slog.Info("All data collected")

	didAppend := false
	for i := range allData.Data {
		if allData.Data[i].Params == p {
			allData.Data[i].Views = append(allData.Data[i].Views, runs...)
			didAppend = true
		}
	}

	if !didAppend {

		allData.Data = append(allData.Data, view.Data{
			Params: p,
			Views:  runs,
		})
	}

	// Marshal the updated struct back into JSON
	updatedJSON, err := json.MarshalIndent(allData, "", "  ")
	if err != nil {
		slog.Error("failed to marshal JSON", err)
		return
	}

	// Write the updated JSON back to the file
	if err := ioutil.WriteFile(*filePath, updatedJSON, 0666); err != nil {
		slog.Error("failed to write file", err)
		return
	}

	//fmt.Println("File updated successfully.")
}
