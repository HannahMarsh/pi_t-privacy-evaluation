package main

import (
	"bufio"
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
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
)

var expectedValues view.ExpectedValues

var allData view.AllData
var mu sync.RWMutex

func main() {
	// Define command-line flags
	logLevel := flag.String("log-level", "debug", "Log level")
	from_ := flag.Int("from", 40, "from")
	to_ := flag.Int("to", 50, "to")

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

	from := *from_
	to := *to_
	index := -1

	slog.Info("", "num runs:", numTImes, "from", from, "to", to)

	wp := executor.NewWorkerPoolWithMax(8)

	err, allData = getData("static/data.json")
	if err != nil {
		slog.Error("failed to get data", err)
	}

	var count int64

	cmd := exec.Command("go", "build", "-o", "bin/run", "cmd/run/main.go")
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("failed to build: "+string(output), err)
		return
	}

	futs := make([]*executor.Future[view.Data], 0)

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
									slog.Error(fmt.Sprintf("%s -> (%d) %d:\tN=%d, R=%d, D=%d, L=%d, X=%f, s=%d", pl.GetFuncName(), count, i, n, r, d, l, x, scenario), err)
								})

								fut.ThenDo(func() {
									slog.Info(fmt.Sprintf("(%d) %d:\tN=%d, R=%d, D=%d, L=%d, X=%f, s=%d", atomic.AddInt64(&count, 1), i, n, r, d, l, x, scenario))
								})

								futs = append(futs, fut)
							}
						}
					}
				}
			}
		}
	}

	for _, fut := range futs {
		fut.Map(func(data view.Data) (view.Data, error) {
			saveData(data)
			return data, nil
		})
	}

	// Create a channel to receive OS signals
	sigChan := make(chan os.Signal, 1)
	// Relay incoming signals to sigChan
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start a goroutine to handle signals
	go func() {
		sig := <-sigChan
		fmt.Printf("Received signal: %s\n", sig)
		packageData()
		wp.Stop()
	}()

	wp.Wait()
	packageData()

	wp.Stop()
}

func packageData() {
	// Marshal the updated struct back into JSON
	mu.Lock()
	defer mu.Unlock()

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

	// Create the command
	cmd := exec.Command("./bin/run", "-N", NStr, "-R", RStr, "-ServerLoad", DStr, "-L", LStr, "-X", XStr, "-Scenario", ScenarioStr, "-numRuns", numRunsStr)

	// Set up pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		slog.Error("failed to get stdout pipe", err)
		return data, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		slog.Error("failed to get stderr pipe", err)
		return data, err
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		slog.Error("failed to start command", err)
		return data, err
	}

	var outputBuf, errorBuf []byte
	done := make(chan error)

	// Read stdout in a separate goroutine
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			outputBuf = append(outputBuf, line...)
			//slog.Info("stdout", "line", line)
		}
		if err := scanner.Err(); err != nil {
			slog.Error("error reading stdout", err)
		}
	}()

	// Read stderr in a separate goroutine
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			errorBuf = append(errorBuf, line...)
			pl.LogNewError(line)
		}
		if err := scanner.Err(); err != nil {
			slog.Error("error reading stderr", err)
		}
	}()

	// Wait for the command to finish
	go func() {
		done <- cmd.Wait()
	}()

	err = <-done
	if err != nil {
		slog.Error("Command execution failed", err)
		return data, err
	} else {
		slog.Info("Success")
	}

	//// Log the command and its arguments
	//slog.Info("Executing command",
	//	"command", cmd.Path,
	//	"args", cmd.Args,
	//)
	//
	//// Run the command and capture its output
	//output, err := cmd.CombinedOutput()
	//if err != nil {
	//	return data, pl.WrapError(fmt.Errorf(string(output)), "-> failed to run command: %s", err.Error())
	//} else {
	//	slog.Info("Command executed successfully",
	//		"command", cmd.Path,
	//		"args", cmd.Args,
	//		"output", string(output),
	//	)
	//}

	var newData view.Data

	err = json.Unmarshal(outputBuf, &newData)
	if err != nil {
		return data, pl.WrapError(err, "failed to unmarshal JSON")
	}

	return newData, nil
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
