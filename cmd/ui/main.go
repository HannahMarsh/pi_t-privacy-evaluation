package main

import (
	"encoding/json"
	"flag"
	"fmt"
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/cmd/view"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/interfaces"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"go.uber.org/automaxprocs/maxprocs"
	"golang.org/x/exp/slog"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"image/color"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"
)

var allData view.AllData
var expectedValues view.ExpectedValues
var defaults interfaces.Params

var firstColor = color.RGBA{R: 217, G: 156, B: 201, A: 255}
var secondColor = color.RGBA{R: 173, G: 202, B: 237, A: 255}
var overlap = color.RGBA{R: 143, G: 106, B: 176, A: 255}

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

	// Read the existing JSON file
	filePath = "static/data_old1.json"
	fileContent, err = ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	// Unmarshal the JSON content into a struct
	if err := json.Unmarshal(fileContent, &allData); err != nil {
		fmt.Printf("Error unmarshaling JSON: %v\n", err)
		return
	}

	slog.Info("Getting values of N...")
	expectedValues.N = utils.RemoveDuplicates(utils.Map(allData.Data, func(d view.Data) int {
		return d.Params.N
	}))
	utils.SortOrdered(expectedValues.N)
	defaults.N = expectedValues.N[0]
	slog.Info("Got values of N", "N", expectedValues.N)
	expectedValues.R = utils.RemoveDuplicates(utils.Map(allData.Data, func(d view.Data) int {
		return d.Params.R
	}))
	utils.SortOrdered(expectedValues.R)
	defaults.R = expectedValues.R[0]
	slog.Info("Geot values of R", "R", expectedValues.R)
	expectedValues.ServerLoad = utils.RemoveDuplicates(utils.Map(allData.Data, func(d view.Data) int {
		return int(d.Params.ServerLoad)
	}))
	utils.SortOrdered(expectedValues.ServerLoad)
	defaults.ServerLoad = expectedValues.ServerLoad[0]
	slog.Info("Got values of ServerLoad", "ServerLoad", expectedValues.ServerLoad)
	expectedValues.L = utils.RemoveDuplicates(utils.Map(allData.Data, func(d view.Data) int {
		return d.Params.L
	}))
	utils.SortOrdered(expectedValues.L)
	defaults.L = expectedValues.L[0]
	slog.Info("Got values of L", "L", expectedValues.L)

	slog.Info("Got values of Scenario", "Scenario", expectedValues.Scenario)
	expectedValues.NumRuns = utils.Filter(utils.NewIntArray(1, utils.MaxOver(utils.Map(allData.Data, func(d view.Data) int {
		return len(d.Views)
	}))+1), func(i int) bool {
		return i%(10) == 0
	})
	defaults.Scenario = expectedValues.Scenario[0]
	utils.SortOrdered(expectedValues.NumRuns)
	slog.Info("Got values of NumRuns", "NumRuns", expectedValues.NumRuns)

	expectedValues.NumBuckets = utils.RemoveDuplicates(expectedValues.NumBuckets)
	utils.SortOrdered(expectedValues.NumBuckets)

	slog.Info("Got values of NumBuckets", "NumBuckets", expectedValues.NumBuckets)

	slog.Info("All data collected")

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
	p := interfaces.Params{
		N:          getIntQueryParam(r, "N"),
		R:          getIntQueryParam(r, "R"),
		ServerLoad: getIntQueryParam(r, "ServerLoad"),
		L:          getIntQueryParam(r, "L"),
		X:          getFloatQueryParam(r, "X"),
		Scenario:   getIntQueryParam(r, "Scenario"),
	}
	numRuns := getIntQueryParam(r, "NumRuns")
	numBuckets := getIntQueryParam(r, "NumBuckets")

	slog.Info("Querying data", "Params", p, "NumRuns", numRuns, "NumBuckets", numBuckets)

	if numBuckets <= 0 {
		numBuckets = 15
	}

	v := find(p)
	v0 := utils.FlatMap(utils.Filter(v, func(data view.Data) bool {
		return data.Params.Scenario == 0
	}), func(data view.Data) []view.View {
		return data.Views
	})
	v1 := utils.FlatMap(utils.Filter(v, func(data view.Data) bool {
		return data.Params.Scenario == 1
	}), func(data view.Data) []view.View {
		return data.Views
	})

	images, err := plotView(v0[:utils.Max(1, utils.Min(len(v0), numRuns))], v1[:utils.Max(1, utils.Min(len(v0), numRuns))], numBuckets)
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

func find(p interfaces.Params) []view.Data {

	v := allData.Data

	if v1 := utils.Filter(v, func(data view.Data) bool {
		return data.Params.N == p.N
	}); len(v1) > 0 {
		v = v1
	} else if v1 = utils.Filter(v, func(data view.Data) bool {
		return data.Params.N == defaults.N
	}); len(v1) > 0 {
		v = v1
	}

	if v1 := utils.Filter(v, func(data view.Data) bool {
		return data.Params.R == p.R
	}); len(v1) > 0 {
		v = v1
	} else if v1 = utils.Filter(v, func(data view.Data) bool {
		return data.Params.R == defaults.R
	}); len(v1) > 0 {
		v = v1
	}

	if v1 := utils.Filter(v, func(data view.Data) bool {
		return data.Params.L == p.L
	}); len(v1) > 0 {
		v = v1
	} else if v1 = utils.Filter(v, func(data view.Data) bool {
		return data.Params.L == defaults.L
	}); len(v1) > 0 {
		v = v1
	}

	if v1 := utils.Filter(v, func(data view.Data) bool {
		return data.Params.ServerLoad == p.ServerLoad
	}); len(v1) > 0 {
		v = v1
	} else if v1 = utils.Filter(v, func(data view.Data) bool {
		return data.Params.ServerLoad == defaults.ServerLoad
	}); len(v1) > 0 {
		v = v1
	}

	if v1 := utils.Filter(v, func(data view.Data) bool {
		return data.Params.X == p.X
	}); len(v1) > 0 {
		v = v1
	} else if v1 = utils.Filter(v, func(data view.Data) bool {
		return data.Params.X == defaults.X
	}); len(v1) > 0 {
		v = v1
	}

	return v
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
	Probabilities  string `json:"probabilities_img"`
	ProbConfidence string `json:"probConfidence_img"`
}

func plotView(v0, v1 []view.View, numBuckets int) (Images, error) {
	if len(v0) == 0 || len(v1) == 0 {
		return Images{}, pl.NewError("no views to plot")
	}
	probabilities0 := computeAverages(utils.Map(v0, func(v view.View) []float64 {
		return v.Probabilities
	}))
	probabilities1 := computeAverages(utils.Map(v1, func(v view.View) []float64 {
		return v.Probabilities
	}))

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

	prImage, err := createPlot("Probabilities", probabilities0, probabilities1)
	if err != nil {
		return Images{}, pl.WrapError(err, "failed to create plot")
	}

	confidence := append(utils.Map(v0, view.GetProbScen0), utils.Map(v1, view.GetProbScen1)...)
	confidencePDF := computeHistogram(confidence, numBuckets)

	prConfidence, err := createFloatCDFPlot("Confidence", confidencePDF, "Adversary's Confidence When Predicting the Correct Scenario", "Confidence Level (%)", "Frequency (# of trials)")
	if err != nil {
		return Images{}, pl.WrapError(err, "failed to create CDF plot")
	}

	return Images{
		Probabilities:  prImage,
		ProbConfidence: prConfidence,
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
