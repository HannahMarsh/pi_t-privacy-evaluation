package main

import (
	"encoding/json"
	"flag"
	"fmt"
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"go.uber.org/automaxprocs/maxprocs"
	"golang.org/x/exp/slog"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
)

// Define your data structure
type ExpectedValues struct {
	N        []int     `json:"N"`
	R        []int     `json:"R"`
	D        []int     `json:"D"`
	L        []int     `json:"L"`
	X        []float64 `json:"X"`
	StdDev   []float64 `json:"StdDev"`
	Scenario []int     `json:"Scenario"`
	NumRuns  []int     `json:"NumRuns"`
}

var expectedValues ExpectedValues

//var multiRuns map[interfaces.Params][]MultiView

func main() {
	// Define command-line flags
	logLevel := flag.String("log-level", "debug", "Log level")
	flag.Usage = flag.PrintDefaults
	flag.Parse()

	pl.SetUpLogrusAndSlog(*logLevel)

	// set GOMAXPROCS
	if _, err := maxprocs.Set(); err != nil {
		slog.Error("failed set max procs", err)
		os.Exit(1)
	}

	// Read the existing JSON file
	filePath := "static/expectedValues.json"
	fileContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		slog.Error("failed to read file", err)
		return
	}

	// Unmarshal the JSON content into a struct
	if err := json.Unmarshal(fileContent, &expectedValues); err != nil {
		slog.Error("failed to unmarshal JSON", err)
		return
	}

	//from := 0
	//to := 10000

	index := 0

	slog.Info("", "num runs:", len(expectedValues.N)*len(expectedValues.R)*len(expectedValues.D)*len(expectedValues.L)*len(expectedValues.X)*len(expectedValues.StdDev)*len(expectedValues.Scenario)*len(expectedValues.NumRuns))

	for _, N := range expectedValues.N {
		for _, R := range expectedValues.R {
			for _, D := range expectedValues.D {
				for _, L := range expectedValues.L {
					if L < N {
						for _, X := range expectedValues.X {
							for _, StdDev := range expectedValues.StdDev {
								for _, Scenario := range expectedValues.Scenario {

									//if index > to {
									//	return
									//}
									//if index < from {
									//	index++
									//	continue
									//}
									index++

									// Convert all the numeric parameters to strings
									NStr := strconv.Itoa(N)
									RStr := strconv.Itoa(R)
									DStr := strconv.Itoa(D)
									LStr := strconv.Itoa(L)
									XStr := fmt.Sprintf("%f", X)
									StdDevStr := fmt.Sprintf("%f", StdDev)
									ScenarioStr := strconv.Itoa(Scenario)
									numRunsStr := strconv.Itoa(utils.MaxOver(expectedValues.NumRuns))

									// Create the command
									cmd := exec.Command("go", "run", "cmd/run/main.go", "-N", NStr, "-R", RStr, "-D", DStr, "-L", LStr, "-X", XStr, "-StdDev", StdDevStr, "-Scenario", ScenarioStr, "-numRuns", numRunsStr)

									// Run the command and capture its output
									output, err := cmd.CombinedOutput()
									if err != nil {
										fmt.Printf("Error running command: %v\n", err)
										return
									}

									// Print the output
									fmt.Printf("%s%d, ", output, index)
									if index%40 == 0 {
										fmt.Println()
									}
								}
							}
						}
					}
				}
			}
		}
	}
	slog.Info("All data collected")

}
