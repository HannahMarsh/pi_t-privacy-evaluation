package main

import (
	"bufio"
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
	"gonum.org/v1/plot/vg/draw"
	"image/color"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"
)

var expectedValues view.ExpectedValues

var firstColor = color.RGBA{R: 217, G: 156, B: 201, A: 255}
var secondColor = color.RGBA{R: 173, G: 202, B: 237, A: 255}
var overlap = color.RGBA{R: 143, G: 106, B: 176, A: 255}

var cache map[adversary.P]*adversary.V = make(map[adversary.P]*adversary.V)

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
		return
	}

	//// Read the existing JSON file
	//filePath = "static/data.json"
	//fileContent, err = ioutil.ReadFile(filePath)
	//if err != nil {
	//	fmt.Printf("Error reading file: %v\n", err)
	//	return
	//}
	//
	//// Unmarshal the JSON content into a struct
	//if err := json.Unmarshal(fileContent, &allData); err != nil {
	//	fmt.Printf("Error unmarshaling JSON: %v\n", err)
	//	return
	//}
	//
	//slog.Info("Getting values of N...")
	//expectedValues.N = utils.RemoveDuplicates(utils.Map(allData.Data, func(d view.Data) int {
	//	return d.Params.N
	//}))
	//utils.SortOrdered(expectedValues.N)
	//defaults.N = expectedValues.N[0]
	//slog.Info("Got values of N", "N", expectedValues.N)
	//expectedValues.R = utils.RemoveDuplicates(utils.Map(allData.Data, func(d view.Data) int {
	//	return d.Params.R
	//}))
	//utils.SortOrdered(expectedValues.R)
	//defaults.R = expectedValues.R[0]
	//slog.Info("Geot values of R", "R", expectedValues.R)
	//expectedValues.ServerLoad = utils.RemoveDuplicates(utils.Map(allData.Data, func(d view.Data) int {
	//	return int(d.Params.ServerLoad)
	//}))
	//utils.SortOrdered(expectedValues.ServerLoad)
	//defaults.ServerLoad = expectedValues.ServerLoad[0]
	//slog.Info("Got values of ServerLoad", "ServerLoad", expectedValues.ServerLoad)
	//expectedValues.L = utils.RemoveDuplicates(utils.Map(allData.Data, func(d view.Data) int {
	//	return d.Params.L
	//}))
	//utils.SortOrdered(expectedValues.L)
	//defaults.L = expectedValues.L[0]
	//slog.Info("Got values of L", "L", expectedValues.L)
	//
	//slog.Info("Got values of Scenario", "Scenario", expectedValues.Scenario)
	//expectedValues.NumRuns = utils.Filter(utils.NewIntArray(1, utils.MaxOver(utils.Map(allData.Data, func(d view.Data) int {
	//	return len(d.Views)
	//}))+1), func(i int) bool {
	//	return i%(10) == 0
	//})
	//defaults.Scenario = expectedValues.Scenario[0]
	//utils.SortOrdered(expectedValues.NumRuns)
	//slog.Info("Got values of NumRuns", "NumRuns", expectedValues.NumRuns)
	//
	//expectedValues.NumBuckets = utils.RemoveDuplicates(expectedValues.NumBuckets)
	//utils.SortOrdered(expectedValues.NumBuckets)
	//
	//slog.Info("Got values of NumBuckets", "NumBuckets", expectedValues.NumBuckets)
	//
	//slog.Info("All data collected")

	// Start HTTP server
	// Serve static files from the "static" directory
	http.Handle("/", withHeaders(http.FileServer(http.Dir("static"))))
	http.Handle("/plots/", withHeaders(http.StripPrefix("/plots/", http.FileServer(http.Dir("static/plots")))))
	http.Handle("/query", withHeaders(http.HandlerFunc(queryHandler)))
	http.Handle("/expected", withHeaders(http.HandlerFunc(handleExpectedValues)))

	slog.Info("Starting server on :8200")
	if err := http.ListenAndServe(":8200", nil); err != nil {
		slog.Error("failed to start server", err)
	}
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

	var v *adversary.V

	if d, present := cache[p]; present && d != nil {
		v = d
	} else {
		// Convert parameters to strings
		CStr := strconv.Itoa(p.C)
		RStr := strconv.Itoa(p.R)
		serverLoadStr := strconv.FormatFloat(p.ServerLoad, 'f', 1, 64)
		XStr := strconv.FormatFloat(p.X, 'f', 1, 64)
		LStr := strconv.Itoa(p.L)
		numRunsStr := strconv.Itoa(numRuns)

		// Create the command
		//cmd := exec.Command("go", "run", "cmd/adversary/main.go", "-C", CStr, "-R", RStr, "-serverLoad", xStr, "-X", XStr, "-L", LStr, "-numRuns", numRunsStr)

		cmd := exec.Command("go", "run", "cmd/adversary/main.go",
			"-C", CStr,
			"-R", RStr,
			"-serverLoad", serverLoadStr,
			"-X", XStr,
			"-L", LStr,
			"-numRuns", numRunsStr,
		)

		// Debug: Print command
		fmt.Printf("Executing: go run cmd/adversary/main.go -C %s -R %s -serverLoad %s -X %s -L %s -numRuns %s\n", CStr, RStr, serverLoadStr, XStr, LStr, numRunsStr)

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
		} else {
			slog.Info("Success")
		}

		var vv adversary.V

		err = json.Unmarshal(outputBuf, &vv)
		if err != nil {
			slog.Error("Failed to unmarshall", err)
		} else {
			v = &vv
			//slog.Info("", "", v.P)
		}

		cache[p] = v
	}

	//
	//v := find(p)
	//v0 := utils.FlatMap(utils.Filter(v, func(data view.Data) bool {
	//	return data.Params.Scenario == 0
	//}), func(data view.Data) []view.View {
	//	return data.Views
	//})
	//v1 := utils.FlatMap(utils.Filter(v, func(data view.Data) bool {
	//	return data.Params.Scenario == 1
	//}), func(data view.Data) []view.View {
	//	return data.Views
	//})
	//
	//if len(v0) > numRuns {
	//	v0 = v0[:utils.Max(0, utils.Min(len(v0), numRuns))]
	//}
	//
	//if len(v1) > numRuns {
	//	v1 = v1[:utils.Max(0, utils.Min(len(v0), numRuns))]
	//}

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

//
//func find(p interfaces.Params) []view.Data {
//
//	v := allData.Data
//
//	if v1 := utils.Filter(v, func(data view.Data) bool {
//		return data.Params.N == p.N
//	}); len(v1) > 0 {
//		v = v1
//	} else if v1 = utils.Filter(v, func(data view.Data) bool {
//		return data.Params.N == defaults.N
//	}); len(v1) > 0 {
//		v = v1
//	}
//
//	if v1 := utils.Filter(v, func(data view.Data) bool {
//		return data.Params.R == p.R
//	}); len(v1) > 0 {
//		v = v1
//	} else if v1 = utils.Filter(v, func(data view.Data) bool {
//		return data.Params.R == defaults.R
//	}); len(v1) > 0 {
//		v = v1
//	}
//
//	if v1 := utils.Filter(v, func(data view.Data) bool {
//		return data.Params.L == p.L
//	}); len(v1) > 0 {
//		v = v1
//	} else if v1 = utils.Filter(v, func(data view.Data) bool {
//		return data.Params.L == defaults.L
//	}); len(v1) > 0 {
//		v = v1
//	}
//
//	if v1 := utils.Filter(v, func(data view.Data) bool {
//		return data.Params.ServerLoad == p.ServerLoad
//	}); len(v1) > 0 {
//		v = v1
//	} else if v1 = utils.Filter(v, func(data view.Data) bool {
//		return data.Params.ServerLoad == defaults.ServerLoad
//	}); len(v1) > 0 {
//		v = v1
//	}
//
//	if v1 := utils.Filter(v, func(data view.Data) bool {
//		return data.Params.X == p.X
//	}); len(v1) > 0 {
//		v = v1
//	} else if v1 = utils.Filter(v, func(data view.Data) bool {
//		return data.Params.X == defaults.X
//	}); len(v1) > 0 {
//		v = v1
//	}
//
//	return v
//}

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

func plotView(v *adversary.V, numBuckets int) (Images, error) {

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

	prConfidence, err := createFloatCDFPlot("ratio", ratiosPDF, "Ratio of ProbR_1 Over Prob R", "Ratio", "Frequency (# of trials)")
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

func computeAverages(data [][]float64) []float64 {
	if len(data) == 0 {
		return nil
	}

	averages := make([]float64, len(data[0]))
	for i := 0; i < len(data[0]); i++ {
		sum := 0.0
		for j := 0; j < len(data); j++ {
			sum += data[j][i]
		}
		averages[i] = sum / float64(len(data))
	}

	return averages
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

func createPlot(file string, probabilities0, probabilities1 []float64) (string, error) {

	newName := fmt.Sprintf("/plots/%s_%d.png", file, time.Now().UnixNano()/int64(time.Millisecond))

	type temp struct {
		value0  float64
		value1  float64
		overlap float64
		label   string
	}

	t := make([]temp, len(probabilities0))

	for i := range probabilities0 {
		t[i] = temp{value0: probabilities0[i], value1: probabilities1[i], overlap: math.Min(probabilities0[i], probabilities1[i]), label: fmt.Sprintf("%d", i+1)}
	}

	//utils.Sort(t, func(i, j temp) bool {
	//	return i.value0 > j.value0
	//})

	probabilities0 = utils.Map(t, func(i temp) float64 {
		return i.value0
	})

	probabilities1 = utils.Map(t, func(i temp) float64 {
		return i.value1
	})

	overlap_ := utils.Map(t, func(i temp) float64 {
		return i.overlap
	})

	// Create a new plot
	p := plot.New()
	p.Title.Text = "Node Probabilities"
	p.Y.Label.Text = "Probability"

	nodeIDs := utils.Map(t, func(i temp) string {
		return i.label
	})

	// Calculate bar width based on the number of points
	plotWidth := 16 * vg.Inch
	barWidth := plotWidth / vg.Length(int(float64(len(probabilities0))*float64(1.2)))

	// Create a bar chart
	//w := vg.Points(20) // Width of the bars
	bars0, err := plotter.NewBarChart(plotter.Values(probabilities0), barWidth)
	if err != nil {
		return "", pl.WrapError(err, "failed to create bar chart")
	}
	bars0.LineStyle.Width = vg.Length(0)  // No line around bars
	bars0.Color = color.Color(firstColor) // Set the color of the bars

	p.Add(bars0)

	bars1, err := plotter.NewBarChart(plotter.Values(probabilities1), barWidth)
	if err != nil {
		return "", pl.WrapError(err, "failed to create bar chart")
	}
	bars1.LineStyle.Width = vg.Length(0)   // No line around bars
	bars1.Color = color.Color(secondColor) // Set the color of the bars

	p.Add(bars1)

	// Create a bar chart for overlap
	overlapBars, err := plotter.NewBarChart(plotter.Values(overlap_), barWidth)
	if err != nil {
		return "", pl.WrapError(err, "failed to create overlap bar chart")
	}
	overlapBars.LineStyle.Width = vg.Length(0) // No line around bars
	overlapBars.Color = color.Color(overlap)   // Blue overlap

	p.Add(overlapBars)

	// Create a legend
	p.Legend.Add("Scenario 0", bars0)
	p.Legend.Add("Scenario 1", bars1)
	p.Legend.Top = true // Position the legend at the top

	p.NominalX(nodeIDs...) // Set node IDs as labels on the X-axis

	// Save the plot to a PNG file
	if err := p.Save(16*vg.Inch, 4*vg.Inch, "static"+newName); err != nil {
		log.Panic(err)
	}

	return newName, nil
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
			if ratio < bound {
				return 1.0
			}
			return 0.0
		}))
	})

	return guess(epsilonValues, deltaValues, "Epsilon", "Delta", "Epsilon-Delta Plot", "Epsilon-Delta", "epsilon_delta")
}

func createRatiosPlot(probR, probR_1 []float64) (string, error) {

	return createDotPlot(probR_1, probR, "Probability of Being in Scenario 0", "Probability of Being in Scenario 1", "Epsilon-Delta Plot", "Trials", "epsilon_delta")
}

func createLinePlot(x []float64, y []float64, xAxis, yAxis, title, lineLabel, file string) (string, error) {
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

	line, points, err := plotter.NewLinePoints(pts)
	if err != nil {
		return "", err
	}

	// Customize the line and points
	line.LineStyle.Width = vg.Points(5) // Thicker line
	line.LineStyle.Color = firstColor
	points.Shape = draw.CircleGlyph{}
	points.Color = overlap
	points.Radius = vg.Points(3)

	p.Add(line, points)
	p.Legend.Add(lineLabel, line, points)

	//err := plotutil.AddLinePoints(p, lineLabel, pts)
	//if err != nil {
	//	return "", pl.WrapError(err, "failed to add line points")
	//}

	if err := p.Save(8*vg.Inch, 6*vg.Inch, "static"+newName); err != nil {
		return "", pl.WrapError(err, "failed to save plot")
	}

	return newName, nil
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
