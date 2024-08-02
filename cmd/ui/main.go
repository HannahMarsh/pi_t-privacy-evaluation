package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/cmd/adversary/adversary"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/cmd/view"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"go.uber.org/automaxprocs/maxprocs"
	"golang.org/x/exp/slog"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
	"image/color"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

var expectedValues view.ExpectedValues

var firstColor = color.RGBA{R: 217, G: 156, B: 201, A: 255}
var secondColor = color.RGBA{R: 173, G: 202, B: 237, A: 255}
var overlap = color.RGBA{R: 143, G: 106, B: 176, A: 255}

var cache map[string]data = make(map[string]data)
var mu sync.RWMutex

func getOrCalcData(p adversary.P, numRuns int) adversary.V {
	if v, present := getData(p); !present {
		// Convert parameters to strings
		CStr := strconv.Itoa(p.C)
		RStr := strconv.Itoa(p.R)
		serverLoadStr := strconv.FormatFloat(p.ServerLoad, 'f', 1, 64)
		XStr := strconv.FormatFloat(p.X, 'f', 1, 64)
		LStr := strconv.Itoa(p.L)
		numRunsStr := strconv.Itoa(numRuns)

		cmd := exec.Command("go", "run", "cmd/adversary/main.go",
			"-C", CStr,
			"-R", RStr,
			"-serverLoad", serverLoadStr,
			"-X", XStr,
			"-L", LStr,
			"-numRuns", numRunsStr,
		)

		// Debug: Print command
		//fmt.Printf("Executing: go run cmd/adversary/main.go -C %s -R %s -serverLoad %s -X %s -L %s -numRuns %s\n", CStr, RStr, serverLoadStr, XStr, LStr, numRunsStr)

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
			slog.Info(fmt.Sprintf("Done with go run cmd/adversary/main.go -C %s -R %s -serverLoad %s -X %s -L %s -numRuns %s", CStr, RStr, serverLoadStr, XStr, LStr, numRunsStr))
		}

		var vv adversary.V

		err = json.Unmarshal(outputBuf, &vv)
		if err != nil {
			slog.Error("Failed to unmarshall", err)
		} else {
			v = vv
		}

		setData(p, v)
	}

	v, _ := getData(p)
	return v
}

func getData(p adversary.P) (v adversary.V, present bool) {
	mu.RLock()
	defer mu.RUnlock()
	var d data
	if d, present = cache[p.Hash()]; present {
		return d.V, true
	}
	return v, false
}

func hasData(p adversary.P) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, present := cache[p.Hash()]
	return present
}
func setData(p adversary.P, v adversary.V) {
	mu.Lock()
	defer mu.Unlock()
	cache[p.Hash()] = data{
		P: p,
		V: v,
	}
}

type data struct {
	P adversary.P `json:"P"`
	V adversary.V `json:"V"`
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
	filePath = "static/data3.json"
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

	//go collectData(expectedValues, ctx)

	slog.Info("Starting server on :8200")
	if err := server.ListenAndServe(); err != nil {
		if err != http.ErrServerClosed {
			slog.Error("failed to start server", err)
		}
	}
}

func collectData(values view.ExpectedValues, ctx context.Context) {
	nValues := values.N
	rValues := values.R
	serverLoadValues := values.ServerLoad
	lValues := values.L
	xValues := values.X

	index := 0

	ps := make([]adversary.P, 0)

	for _, r := range nValues {
		for _, c := range rValues {
			for _, serverLoad := range serverLoadValues {
				for _, l := range lValues {
					for _, x := range xValues {
						if err := ctx.Err(); err != nil {
							fmt.Printf("Stopping data collection: %v\n", err)
							return
						}
						p := adversary.P{
							C:          c,
							R:          r,
							ServerLoad: float64(serverLoad),
							L:          l,
							X:          x,
						}
						if !hasData(p) {
							index++
							ps = append(ps, p)
						}
					}
				}
			}
		}
	}

	utils.Shuffle(ps)

	slog.Info("", "Numtimes", index)

	var wg sync.WaitGroup

	total := float64(index) / 100.0
	index = 0

	for _, p := range ps {
		if err := ctx.Err(); err != nil {
			fmt.Printf("Stopping data collection: %v\n", err)
			return
		}
		if !hasData(p) {
			index++
			if index%5 == 0 {
				wg.Wait()
			}
			wg.Add(1)
			go func(pp adversary.P, i int) {
				defer wg.Done()
				getOrCalcData(pp, 100)
				slog.Info(fmt.Sprintf("Done with %f%%", float64(i)/total))
			}(p, index)
		}
	}
	slog.Info("All data collected")
}

func packageData() {
	// Marshal the updated struct back into JSON

	updatedJSON, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		slog.Error("failed to marshal JSON", err)
		return
	}

	// Write the updated JSON back to the file
	if err = ioutil.WriteFile("static/data3.json", updatedJSON, 0644); err != nil {
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
	p := adversary.P{
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

	images, err := plotView(v, numBuckets)
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

type Images struct {
	Ratios       string `json:"ratios_img"`
	EpsilonDelta string `json:"epsilon_delta_img"`
	RatiosPlot   string `json:"ratios_plot_img"`
}

func plotView(v adversary.V, numBuckets int) (Images, error) {

	// Read the contents of the directory
	contents, err := ioutil.ReadDir("static/plots")
	if err != nil {
		return Images{}, pl.WrapError(err, "failed to read directory")
	}

	// Remove each item in the directory
	for _, item := range contents {
		itemPath := "static/plots/" + item.Name()
		if err = os.RemoveAll(itemPath); err != nil {
			return Images{}, pl.WrapError(err, "failed to remove item")
		}
	}

	ratios := v.Ratios
	ratiosPDF := computeHistogram(ratios, numBuckets)

	meanRatio := utils.Mean(ratios)

	prConfidence, err := createFloatCDFPlot("ratio", ratiosPDF, "Ratio of Pr[0] Over Pr[1] "+fmt.Sprintf("(mean=%f)", meanRatio), "Ratio", "Frequency (# of trials)")
	if err != nil {
		return Images{}, pl.WrapError(err, "failed to create CDF plot")
	}

	epDelta, err := createEpsilonDeltaPlot(ratios)
	if err != nil {
		return Images{}, pl.WrapError(err, "failed to create CDF plot")
	}

	ratiosPlot, err := createRatiosPlot(v.Pr0, v.Pr1)
	if err != nil {
		return Images{}, pl.WrapError(err, "failed to create CDF plot")
	}

	return Images{
		Ratios:       prConfidence,
		EpsilonDelta: epDelta,
		RatiosPlot:   ratiosPlot,
	}, nil
}

type range_ struct {
	min, max float64
	count0   int
	count1   int
	interval int
	width    float64
}

func newRange(min, max float64, interval int, width float64) *range_ {
	return &range_{min: min, max: max, interval: interval, width: width}
}

func (r range_) contains(value float64) bool {
	return r.min <= value && value <= r.max
}

func (r *range_) add0() {
	r.count0++
}
func (r *range_) add1() {
	r.count1++
}

type pair struct {
	key      float64
	value0   float64
	interval float64
}

func computeHistogram(data []float64, numBuckets int) []pair {
	// Compute frequencies
	//freq := make(map[float64]int)
	numBuckets = utils.Max(numBuckets, 5)
	xMin := utils.MinOver(data)
	xMax := utils.MaxOver(data)
	interval := (xMax - xMin) / float64(numBuckets)
	freq := make([]*range_, numBuckets)
	for i := range freq {
		freq[i] = newRange(xMin+(float64(i)*interval), xMin+(float64(i+1)*interval), i, interval)
	}
	for _, value := range data {
		if r := utils.FindPointer(freq, func(r *range_) bool {
			return r.contains(value)
		}); r != nil {
			r.add0()
		}
	}

	pdf := make([]pair, 0)

	for _, r := range freq {
		pdf = append(pdf, pair{
			key:      r.min,
			value0:   float64(r.count0),
			interval: r.width,
		})
	}

	return pdf
}

func createEpsilonDeltaPlot(ratios []float64) (string, error) {

	//fmt.Printf("\nR=\\left[%s\\right]\n", strings.Join(utils.Map(ratios, func(ratio float64) string {
	//	return fmt.Sprintf("%.7f", ratio)
	//}), ","))

	epsilonValues := utils.Map(ratios, func(ratio float64) float64 {
		return math.Log(ratio)
	})

	deltaValues := utils.Map(epsilonValues, func(epsilon float64) float64 {
		bound := math.Pow(math.E, epsilon)
		return utils.Mean(utils.Map(ratios, func(ratio float64) float64 {
			if ratio >= bound {
				return 1.0
			}
			return 0.0
		}))
	})

	return guess(epsilonValues, deltaValues, "Epsilon", "Delta", "Values of ϵ and δ for which (ϵ,δ)-DP is Satisfied", "Epsilon-Delta", "epsilon_delta")
}

func createRatiosPlot(prob0, prob1 []float64) (string, error) {

	mean0 := utils.Mean(prob0)
	mean1 := utils.Mean(prob1)

	return createDotPlot(prob1, prob0, "Probability of Being in Scenario 0 "+fmt.Sprintf("(mean=%f", mean0), "Probability of Being in Scenario 1 "+fmt.Sprintf("(mean=%f", mean1), "Observed Data Pairs", "A single trial: (pr[0], Pr[1])", "epsilon_delta")
}

func createDotPlot(x []float64, y []float64, xAxis, yAxis, title, lineLabel, file string) (string, error) {
	newName := fmt.Sprintf("/plots/%s_%d.png", file, time.Now().UnixNano()/int64(time.Millisecond))

	// Create a new plot
	p := plot.New()
	p.Title.Text = title
	p.X.Label.Text = xAxis
	p.Y.Label.Text = yAxis

	pts := make(plotter.XYs, len(x))
	for i := range pts {
		pts[i].X = x[i]
		pts[i].Y = y[i]
	}

	utils.Sort(pts, func(i, j plotter.XY) bool {
		return i.X < j.X
	})

	err := plotutil.AddScatters(p, lineLabel, pts)
	if err != nil {
		return "", pl.WrapError(err, "failed to add line points")
	}

	if err := p.Save(8*vg.Inch, 6*vg.Inch, "static"+newName); err != nil {
		return "", pl.WrapError(err, "failed to save plot")
	}

	return newName, nil
}

func createFloatCDFPlot(file string, probabilities []pair, title, xLabel, yLabel string) (string, error) {
	newName := fmt.Sprintf("/plots/%s_%d.png", file, time.Now().UnixNano()/int64(time.Millisecond))
	// Create a new plot
	p := plot.New()
	p.Title.Text = title
	p.Y.Label.Text = yLabel
	p.X.Label.Text = xLabel

	keys := utils.Map(probabilities, func(p pair) float64 {
		return p.key
	})
	utils.SortOrdered(keys)

	xLabels := make([]string, len(keys))
	values0 := make([]float64, len(keys))

	totalArea0 := 0.0
	yMax := 0.0

	for i, label := range keys {
		xLabels[i] = fmt.Sprintf("%.2f", label)
		values := utils.Find(probabilities, func(p pair) bool {
			return p.key == label
		})
		values0[i] = values.value0

		area0 := values0[i] * values.interval

		totalArea0 += area0

		yMax = utils.Max(yMax, values0[i])
	}

	// Calculate bar width based on the number of points
	plotWidth := 8 * vg.Inch
	barWidth := plotWidth / vg.Length(int(float64(len(probabilities))*float64(1.2)))

	// Create a bar chart
	//w := vg.Points(20) // Width of the bars

	bars0, err := plotter.NewBarChart(plotter.Values(values0), barWidth)
	if err != nil {
		return "", pl.WrapError(err, "failed to create bar chart")
	}
	bars0.LineStyle.Width = vg.Length(0)  // No line around bars
	bars0.Color = color.Color(firstColor) // Set the color of the bars

	p.Add(bars0)

	// Create a legend
	p.Legend.Add(fmt.Sprintf("(total area = %.6f)", totalArea0), bars0)
	p.Legend.Top = true // Position the legend at the top

	// Add text annotation
	notes, _ := plotter.NewLabels(plotter.XYLabels{
		XYs:    []plotter.XY{{X: 1, Y: yMax * 1.1}}, // Position of the note
		Labels: []string{""},
	})
	p.Add(notes)

	p.NominalX(xLabels...) // Set node IDs as labels on the X-axis

	// Save the plot to a PNG file
	if err := p.Save(8*vg.Inch, 4*vg.Inch, "static"+newName); err != nil {
		log.Panic(err)
	}
	return newName, nil
}
