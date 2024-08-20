package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	pl "github.com/HannahMarsh/PrettyLogger"
	data2 "github.com/HannahMarsh/pi_t-privacy-evaluation/internal/data"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/display"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"go.uber.org/automaxprocs/maxprocs"
	"golang.org/x/exp/slog"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
)

var dataFile = "static/data.json"

var expectedValues ExpectedValues

type ExpectedValues struct {
	N          []int     `json:"N"`
	R          []int     `json:"R"`
	ServerLoad []int     `json:"ServerLoad"`
	L          []int     `json:"L"`
	Scenario   []int     `json:"Scenario"`
	NumRuns    []int     `json:"NumRuns"`
	NumBuckets []int     `json:"NumBuckets"`
	X          []float64 `json:"X"`
}

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
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	// Unmarshal the JSON content into a struct
	if err := json.Unmarshal(fileContent, &expectedValues); err != nil {
		fmt.Printf("Error unmarshaling JSON: %v\n", err)
	}

	// Read the existing JSON file
	filePath = dataFile
	fileContent, err = ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	// Unmarshal the JSON content into a struct
	if err := json.Unmarshal(fileContent, &cache); err != nil {
		fmt.Printf("Error unmarshaling JSON: %v\n", err)
	}

	// Start HTTP server
	// Create a new HTTP server with specific configurations
	server := &http.Server{
		Addr: ":8200",
	}

	// Serve static files from the "static" directory
	http.Handle("/", withHeaders(http.FileServer(http.Dir("static"))))
	http.Handle("/plots/", withHeaders(http.StripPrefix("/plots/", http.FileServer(http.Dir("static/plots")))))
	http.Handle("/query", withHeaders(http.HandlerFunc(queryHandler)))
	http.Handle("/expected", withHeaders(http.HandlerFunc(handleExpectedValues)))

	ctx, cancel := context.WithCancel(context.Background())

	// Create a channel to receive OS signals
	sigChan := make(chan os.Signal, 1)
	// Relay incoming signals to sigChan
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start a goroutine to handle signals
	go func() {
		sig := <-sigChan
		fmt.Printf("Received signal: %s\n", sig)
		packageData()
		// Create a context with a timeout to gracefully shut down the server

		defer cancel()

		// Shutdown the server gracefully
		if err := server.Shutdown(ctx); err != nil {
			fmt.Printf("Error shutting down server: %v\n", err)
		} else {
			fmt.Println("Server gracefully stopped")
		}
		os.Exit(0)
	}()

	go collectData(expectedValues, ctx)

	slog.Info("Starting server on :8200")
	if err = server.ListenAndServe(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("failed to start server", err)
		}
	}
}

var cache = make(map[string]data2.Result)
var mu sync.RWMutex

func getOrCalcData(p data2.Parameters, numRuns int) (v data2.Result) {
	var present bool
	if v, present = getData(p); !present {
		//v = calcData(p, numRuns)
		return v
	}
	if len(v.Ratios) > numRuns {
		return data2.Result{
			P:      p,
			Pr0:    v.Pr0[:numRuns],
			Pr1:    v.Pr1[:numRuns],
			Ratios: v.Ratios[:numRuns],
		}
	}
	return v
}

func calcData(p data2.Parameters, numRuns int) (v data2.Result) {

	// Convert parameters to strings
	CStr := strconv.Itoa(p.C)
	RStr := strconv.Itoa(p.R)
	serverLoadStr := strconv.FormatFloat(p.ServerLoad, 'f', 1, 64)
	XStr := strconv.FormatFloat(p.X, 'f', 1, 64)
	LStr := strconv.Itoa(p.L)
	numRunsStr := strconv.Itoa(numRuns)

	cmd := exec.Command("go", "run", "cmd/simulation/main.go",
		"-C", CStr,
		"-R", RStr,
		"-serverLoad", serverLoadStr,
		"-X", XStr,
		"-L", LStr,
		"-numRuns", numRunsStr,
	)

	// Debug: Print command
	//fmt.Printf("Executing: go run cmd/simulation/main.go -C %s -R %s -serverLoad %s -X %s -L %s -numRuns %s\n", CStr, RStr, serverLoadStr, XStr, LStr, numRunsStr)

	// Set up pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		slog.Error("failed to get stdout pipe", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		slog.Error("failed to get stderr pipe", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		slog.Error("failed to start command", err)
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
		return v
	} else {
		slog.Info(fmt.Sprintf("Done with go run cmd/simulation/main.go -C %s -R %s -serverLoad %s -X %s -L %s -numRuns %s", CStr, RStr, serverLoadStr, XStr, LStr, numRunsStr))
	}

	err = json.Unmarshal(outputBuf, &v)
	if err != nil {
		slog.Error("Failed to unmarshall", err)
	}

	return setData(p, v)
}

func getData(p data2.Parameters) (v data2.Result, present bool) {
	mu.RLock()
	defer mu.RUnlock()
	var d data2.Result
	if d, present = cache[p.Hash()]; present {
		return d, true
	}
	return v, false
}

func setData(p data2.Parameters, v data2.Result) data2.Result {
	mu.Lock()
	defer mu.Unlock()
	if d, present := cache[p.Hash()]; present && d.Ratios != nil && len(d.Ratios) > 0 {
		r := data2.Result{
			P:      p,
			Pr0:    append(d.Pr0, v.Pr0...),
			Pr1:    append(d.Pr1, v.Pr1...),
			Ratios: append(d.Ratios, v.Ratios...),
		}
		cache[p.Hash()] = r
		return r
	} else {
		cache[p.Hash()] = v
		return v
	}
}

func collectData(values ExpectedValues, ctx context.Context) {
	nValues := values.N
	rValues := values.R
	serverLoadValues := values.ServerLoad
	lValues := values.L
	xValues := values.X

	numRunsPerCall := 1

	index := 0

	ps := make([]data2.Parameters, 0)

	runs := 100

	for _, r := range nValues {
		for _, c := range rValues {
			for _, serverLoad := range serverLoadValues {
				for _, l := range lValues {
					for _, x := range xValues {
						if !((r == 1 && (x != 0.0 || l != 1)) || l > r || c <= r) {
							if err := ctx.Err(); err != nil {
								fmt.Printf("Stopping data collection: %v\n", err)
								return
							}
							p := data2.Parameters{
								C:          c,
								R:          r,
								ServerLoad: float64(serverLoad),
								L:          l,
								X:          x,
							}
							d, present := getData(p)
							if !present || len(d.Ratios) < runs {
								num := runs
								if d.Ratios != nil {
									num = runs - len(d.Ratios)
								}
								for i := 0; i < num; i += numRunsPerCall {
									index++
									ps = append(ps, p)
								}
							}
						}
					}
				}
			}
		}
	}

	utils.Shuffle(ps)

	slog.Info("", "Numtimes", index)

	var wg sync.WaitGroup

	total := float64(index) / float64(runs)
	index = 0

	for _, p := range ps {
		if err := ctx.Err(); err != nil {
			fmt.Printf("Stopping data collection: %v\n", err)
			wg.Wait()
			return
		}
		index++
		if index%3 == 0 {
			wg.Wait()
		}
		wg.Add(1)
		go func(pp data2.Parameters, i int) {
			defer wg.Done()
			calcData(pp, numRunsPerCall)
			slog.Info(fmt.Sprintf("Done with  %f%%", float64(i)/total))
		}(p, index)
	}
	wg.Wait()
	slog.Info("All data collected")
}

func packageData() {
	// Marshal the updated struct back into JSON

	slog.Info("Packaging data")

	mu.RLock()
	defer mu.RUnlock()

	updatedJSON, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		slog.Error("failed to marshal JSON", err)
		return
	}

	// Write the updated JSON back to the file
	if err = ioutil.WriteFile(dataFile, updatedJSON, 0644); err != nil {
		slog.Error("failed to write file", err)
		return
	}

	slog.Info("All data collected")
}

func withHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ngrok-skip-browser-warning", "true")
		h.ServeHTTP(w, r)
	})
}

func queryHandler(w http.ResponseWriter, r *http.Request) {
	p := data2.Parameters{
		C:          getIntQueryParam(r, "R"),
		R:          getIntQueryParam(r, "N"),
		ServerLoad: float64(getIntQueryParam(r, "ServerLoad")),
		L:          getIntQueryParam(r, "L"),
		X:          getFloatQueryParam(r, "X"),
	}
	numRuns := getIntQueryParam(r, "NumRuns")
	numBuckets := getIntQueryParam(r, "NumBuckets")

	slog.Info("Querying data", "Params", p, "NumRuns", numRuns, "NumBuckets", numBuckets)

	if numBuckets <= 0 {
		numBuckets = 15
	}

	v := getOrCalcData(p, numRuns)

	if v.Ratios == nil || len(v.Ratios) == 0 {
		http.Error(w, "No data available for the given parameters", http.StatusNotFound)
		return
	}

	images, err := display.PlotView(v, numBuckets)
	if err != nil {
		slog.Error("failed to plot view", err)
		http.Error(w, "Failed to plot view", http.StatusInternalServerError)
		return
	}

	if err = json.NewEncoder(w).Encode(images); err != nil {
		slog.Error("failed to encode response", err)
		http.Error(w, "Failed to encode data to JSON", http.StatusInternalServerError)
	}
}

func getIntQueryParam(r *http.Request, name string) int {
	value, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil {
		return 0
	}
	return value
}

func getFloatQueryParam(r *http.Request, name string) float64 {
	value, err := strconv.ParseFloat(r.URL.Query().Get(name), 64)
	if err != nil {
		return 0.0
	}
	return value
}

func handleExpectedValues(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(expectedValues); err != nil {
		http.Error(w, "Failed to encode expected values to JSON", http.StatusInternalServerError)
	}
}
