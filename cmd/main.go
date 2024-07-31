package main

import (
	"encoding/json"
	"flag"
	"fmt"
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/cmd/view"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"go.uber.org/automaxprocs/maxprocs"
	"golang.org/x/exp/slog"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"sync"
)

var expectedValues view.ExpectedValues

func main() {
	// Define command-line flags
	logLevel := flag.String("log-level", "debug", "Log level")
	flag.Usage = flag.PrintDefaults
	flag.Parse()

	pl.SetUpLogrusAndSlog(*logLevel)

	// set GOMAXPROCS
	if _, err := maxprocs.Set(); err != nil {
		slog.Error("failed to set max procs", err)
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

	numTImes := 0
	for _, N := range expectedValues.N {
		for _, R := range expectedValues.R {
			for _, D := range expectedValues.ServerLoad {
				for _, L := range expectedValues.L {
					for _ = range expectedValues.X {
						if N <= R && L < N && D*N <= R*L { // number of onions each client sends out has to be less than path length
							numTImes = numTImes + 2
						}
					}
				}
			}
		}
	}

	slog.Info("", "num runs:", numTImes)

	from := 0
	to := -1
	index := 0
	numWorkers := 12

	var wg sync.WaitGroup
	var wg2 sync.WaitGroup
	var mu sync.Mutex

	err2, allData := getData("static/data.json")
	if err2 != nil {
		slog.Error("failed to get data", err2)
	}

	count := 0

	for _, N := range expectedValues.N {
		for _, R := range expectedValues.R {
			for _, D := range expectedValues.ServerLoad {
				for _, L := range expectedValues.L {
					if N <= R && L < N && D*N <= R*L { // number of onions each client sends out has to be less than path length
						for _, X := range expectedValues.X {
							for _, Scenario := range expectedValues.Scenario {
								if index < from || (to != -1 && index > to) {
									index++
									fmt.Printf("%d, ", index)
									if index%40 == 0 {
										fmt.Println()
									}
									continue
								}
								index++

								mu.Lock()

								for count >= numWorkers {
									mu.Unlock()
									wg.Wait()
									mu.Lock()
								}
								count++
								if count == numWorkers {
									wg.Add(1)
								}
								mu.Unlock()

								wg2.Add(1)

								go func(n, r, d, l, s, i int) {

									defer wg2.Done()

									// Convert all the numeric parameters to strings
									NStr := strconv.Itoa(n)
									RStr := strconv.Itoa(r)
									DStr := strconv.Itoa(d)
									LStr := strconv.Itoa(l)
									XStr := fmt.Sprintf("%f", X)
									ScenarioStr := strconv.Itoa(s)
									numRunsStr := strconv.Itoa(utils.MaxOver(expectedValues.NumRuns))

									dataFilePath := fmt.Sprintf("static/temp/%d_%d_%d_%d_%d_%d.json", n, r, d, l, i, s)

									// Create the command
									cmd := exec.Command("go", "run", "cmd/run/main.go", "-N", NStr, "-R", RStr, "-ServerLoad", DStr, "-L", LStr, "-X", XStr, "-Scenario", ScenarioStr, "-numRuns", numRunsStr, "-filePath", dataFilePath)

									// Run the command and capture its output
									output, err := cmd.CombinedOutput()
									if err != nil {
										fmt.Printf("Error running command: %v\n", err)
										return
									}

									err2, newData := getData(dataFilePath)
									if err2 != nil {
										slog.Error("failed to get data", err2)
									}

									didAppend := false

									doDone := false

									mu.Lock()
									for i := range allData.Data {
										if allData.Data[i].Params == newData.Data[0].Params {
											allData.Data[i].Views = append(allData.Data[i].Views, newData.Data[0].Views...)
											didAppend = true
										}
									}

									if !didAppend {
										allData.Data = append(allData.Data, view.Data{
											Params: newData.Data[0].Params,
											Views:  newData.Data[0].Views,
										})
									}
									// Print the output
									fmt.Printf("%s%d, ", output, i)
									if i%40 == 0 {
										fmt.Println()
									}
									count--
									if count == numWorkers-1 {
										doDone = true
									}
									mu.Unlock()

									if doDone {
										wg.Done()
									}

									err = os.Remove(dataFilePath)
									if err != nil {
										slog.Error("Failed to delete file: %s", err)
									}
								}(N, R, D, L, Scenario, index)
							}
						}
					}
				}
			}
		}
	}

	wg2.Wait()

	// Marshal the updated struct back into JSON
	updatedJSON, err := json.MarshalIndent(allData, "", "  ")
	if err != nil {
		slog.Error("failed to marshal JSON", err)
		return
	}

	// Write the updated JSON back to the file
	if err = ioutil.WriteFile("static/data.json", updatedJSON, 0644); err != nil {
		slog.Error("failed to write file", err)
		return
	}

	slog.Info("All data collected")
}

func getData(dataFilePath string) (error, view.AllData) {
	// Read the file contents
	fc, err := ioutil.ReadFile(dataFilePath)
	if err != nil {
		return nil, view.AllData{}
	}

	var allData view.AllData

	// Unmarshal the JSON content into a struct
	if err = json.Unmarshal(fc, &allData); err != nil {
		allData.Data = make([]view.Data, 0)
	}
	return nil, allData
}
