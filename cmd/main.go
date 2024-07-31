package main

import (
	"encoding/json"
	"flag"
	"fmt"
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/cmd/view"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils/executor"
	"go.uber.org/automaxprocs/maxprocs"
	"golang.org/x/exp/slog"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
)

var expectedValues view.ExpectedValues

var allData view.AllData
var mu sync.RWMutex

var numForks int64

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
	index := -1

	wp := executor.NewWorkerPool()

	err, allData = getData("static/data.json")
	if err != nil {
		slog.Error("failed to get data", err)
	}

	var count int64

	for _, N := range expectedValues.N {
		for _, R := range expectedValues.R {
			for _, D := range expectedValues.ServerLoad {
				for _, L := range expectedValues.L {
					if N <= R && L < N && D*N <= R*L { // number of onions each client sends out has to be less than path length
						for _, X := range expectedValues.X {
							for _, Scenario := range expectedValues.Scenario {
								index++
								if index < from || (to != -1 && index > to) {
									continue
								}
								index++
								n := N
								r := R
								d := D
								l := L
								x := X
								i := index
								scenario := Scenario

								fut := executor.SubmitWithError(wp, view.Data{}, func() (view.Data, error) {
									return doRun(n, r, d, l, scenario, i, x)
								})

								fut.HandleError(func(err error) {
									slog.Error(fmt.Sprintf("%s -> (%d) %d:\tN=%d, R=%d, D=%d, L=%d, X=%f, s=%d", pl.GetFuncName(), atomic.AddInt64(&count, 1), i, n, r, d, l, x, scenario), err)
								})

								fut2 := fut.Map(func(data view.Data) (view.Data, error) {
									saveData(data)
									return data, nil
								})

								fut2.HandleError(func(err error) {
									slog.Error("failed to save data for index "+fmt.Sprintf("%d", i), err)
								})

								fut2.ThenDo(func() {
									slog.Info(fmt.Sprintf("(%d) %d:\tN=%d, R=%d, D=%d, L=%d, X=%f, s=%d", atomic.AddInt64(&count, 1), i, n, r, d, l, x, scenario))
								})
							}
						}
					}
				}
			}
		}
	}

	wp.Wait()

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

func doRun(n, r, d, l, s, i int, x float64) (data view.Data, err error) {

	// Convert all the numeric parameters to strings
	NStr := strconv.Itoa(n)
	RStr := strconv.Itoa(r)
	DStr := strconv.Itoa(d)
	LStr := strconv.Itoa(l)
	XStr := fmt.Sprintf("%f", x)
	ScenarioStr := strconv.Itoa(s)
	numRunsStr := strconv.Itoa(utils.MaxOver(expectedValues.NumRuns))

	dataFilePath := fmt.Sprintf("static/temp/%d.json", i)

	forks := atomic.AddInt64(&numForks, 1)

	// Create the command
	cmd := exec.Command("go", "run", "cmd/run/main.go", "-N", NStr, "-R", RStr, "-ServerLoad", DStr, "-L", LStr, "-X", XStr, "-Scenario", ScenarioStr, "-numRuns", numRunsStr, "-filePath", dataFilePath)

	atomic.AddInt64(&numForks, -1)

	// Run the command and capture its output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return data, pl.WrapError(err, "forks = %d -> failed to run command. Got output: %s", forks, string(output))
	}
	fmt.Printf("%s", output)

	err, newData := getData(dataFilePath)
	if err != nil {
		return data, pl.WrapError(err, "failed to get data")
	}

	if err := os.Remove(dataFilePath); err != nil {
		return data, pl.WrapError(err, "failed to delete file")
	}

	if newData.Data != nil && len(newData.Data) > 0 {
		return newData.Data[0], nil
	}

	return data, nil
}

func saveData(data view.Data) {
	mu.Lock()
	defer mu.Unlock()
	allData.Data = append(allData.Data, data)
}

func getData(dataFilePath string) (error, view.AllData) {
	// Read the file contents
	fc, err := ioutil.ReadFile(dataFilePath)
	if err != nil {
		return nil, view.AllData{}
	}

	var ad view.AllData

	// Unmarshal the JSON content into a struct
	if err = json.Unmarshal(fc, &ad); err != nil {
		ad.Data = make([]view.Data, 0)
	}
	return nil, ad
}
