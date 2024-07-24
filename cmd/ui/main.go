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

var firstColor = color.RGBA{R: 173, G: 202, B: 237, A: 255}
var secondColor = color.RGBA{R: 217, G: 156, B: 201, A: 255}
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
	filePath = "static/data.json"
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
	expectedValues.X = utils.RemoveDuplicates(utils.Map(allData.Data, func(d view.Data) float64 {
		return d.Params.X
	}))
	utils.SortOrdered(expectedValues.X)
	defaults.X = expectedValues.X[0]
	slog.Info("Got values of X", "X", expectedValues.X)

	slog.Info("Got values of Scenario", "Scenario", expectedValues.Scenario)
	expectedValues.NumRuns = utils.NewIntArray(1, utils.MaxOver(utils.Map(allData.Data, func(d view.Data) int {
		return len(d.Views)
	}))+1)
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

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			return
		}
		next.ServeHTTP(w, r)
	})
}

//var init_ bool

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

	//if !init_ {
	//	init_ = true
	//	numBuckets = 10
	//}

	slog.Info("Querying data", "Params", p, "NumRuns", numRuns, "NumBuckets", numBuckets)

	if numBuckets <= 0 {
		numBuckets = 15
	}

	v0 := utils.Find(allData.Data, func(data view.Data) bool {
		a := data.Params
		b := p
		return a.N == b.N && a.R == b.R && a.ServerLoad == b.ServerLoad && a.L == b.L && a.X == b.X && a.Scenario == 0
	})

	if v0 == nil {
		v0 = utils.Find(allData.Data, func(data view.Data) bool {
			a := data.Params
			b := defaults
			return a.N == b.N && a.R == b.R && a.ServerLoad == b.ServerLoad && a.L == b.L && a.X == b.X && a.Scenario == 0
		})
	}

	v1 := utils.Find(allData.Data, func(data view.Data) bool {
		a := data.Params
		b := p
		return a.N == b.N && a.R == b.R && a.ServerLoad == b.ServerLoad && a.L == b.L && a.X == b.X && a.Scenario == 1
	})

	if v1 == nil {
		v1 = utils.Find(allData.Data, func(data view.Data) bool {
			a := data.Params
			b := defaults
			return a.N == b.N && a.R == b.R && a.ServerLoad == b.ServerLoad && a.L == b.L && a.X == b.X && a.Scenario == 1
		})
	}

	images, err := plotView(v0.Views[:utils.Max(1, utils.Min(len(v0.Views), numRuns))], v1.Views[:utils.Max(1, utils.Min(len(v0.Views), numRuns))], numBuckets)
	if err != nil {
		slog.Error("failed to plot view", err)
		http.Error(w, "Failed to plot view", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(images); err != nil {
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
	Probabilities string `json:"probabilities_img"`
	ReceivedR     string `json:"receivedR_1_img"`
	ReceivedR_1   string `json:"receivedR_img"`
	ProbScen0     string `json:"probScen0_img"`
	ProbScen1     string `json:"probScen1_img"`
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

	//// Remove the directory and its contents
	//if err := os.RemoveAll("static/plots"); err != nil {
	//	return Images{}, pl.WrapError(err, "failed to remove directory")
	//}

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

	//// Recreate the directory
	//if err := os.Mkdir("static/plots", 0755); err != nil {
	//	return Images{}, pl.WrapError(err, "failed to create directory")
	//}

	prImage, err := createPlot("Probabilities", probabilities0, probabilities1)
	if err != nil {
		return Images{}, pl.WrapError(err, "failed to create plot")
	}

	//cdfN, shiftN := computeCDF(utils.Map(v0, view.GetReceivedR), utils.Map(v1, view.GetReceivedR), numBuckets)
	//cdfN_1, shiftN_1 := computeCDF(utils.Map(v0, view.GetReceivedR_1), utils.Map(v1, view.GetReceivedR_1), numBuckets)

	//prReceivedR, err := createCDFPlot("ReceivedR", cdfN, float64(shiftN), "Client R", "Number of onions received", "CDF")
	//if err != nil {
	//	return Images{}, pl.WrapError(err, "failed to create CDF plot")
	//}
	//prReceivedR_1, err := createCDFPlot("ReceivedR_1", cdfN_1, float64(shiftN_1), "Client R-1", "Number of onions received", "CDF")
	//if err != nil {
	//	return Images{}, pl.WrapError(err, "failed to create CDF plot")
	//}

	cdfProb0, shift0 := computeFloatCDF(utils.Map(v0, view.GetProbScen0), utils.Map(v1, view.GetProbScen0), numBuckets)
	cdfProb1, shift1 := computeFloatCDF(utils.Map(v0, view.GetProbScen1), utils.Map(v1, view.GetProbScen1), numBuckets)

	prProb0, err := createFloatCDFPlot("Prob0", cdfProb0, shift0, "Probabilities of Being in Scenario 0", "Probability", "PDF")
	if err != nil {
		return Images{}, pl.WrapError(err, "failed to create CDF plot")
	}
	prProb1, err := createFloatCDFPlot("Prob1", cdfProb1, shift1, "Probabilities of Being in Scenario 1", "Probability", "PDF")
	if err != nil {
		return Images{}, pl.WrapError(err, "failed to create CDF plot")
	}

	return Images{
		Probabilities: prImage,
		//ReceivedR:     prReceivedR,
		//ReceivedR_1:   prReceivedR_1,
		ProbScen0: prProb0,
		ProbScen1: prProb1,
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

//func computeCDF(data0, data1 []int, numBuckets int) ([]float64, []float64, int) {
//	// Compute frequencies
//	//freq := make(map[float64]int)
//	xMin := utils.Min(utils.MinOver(data1), utils.MinOver(data0))
//	xMax := utils.Max(utils.MaxOver(data1), utils.MaxOver(data0))
//	freq := make([]int, xMax-xMin+1)
//	for i := range freq {
//		freq[i] = 0
//	}
//	for _, value := range data0 {
//		freq[value-xMin] = freq[value-xMin] + 1
//	}
//	cumulativeSum := len(freq) - utils.Count(freq, 0)
//
//	cdf := make([]float64, 0)
//
//	bucketSize := utils.Max(1, (xMax-xMin)/utils.Max(numBuckets, 5))
//	for i := 0; i < len(freq); i += bucketSize {
//		count := 0
//		for j := i; j < utils.Min(len(freq), i+bucketSize); j++ {
//			count += freq[j]
//		}
//		cdf = append(cdf, float64(count)/float64(cumulativeSum))
//	}
//	return cdf, xMin
//}

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
	value1   float64
	interval float64
}

func computeFloatCDF(data0, data1 []float64, numBuckets int) ([]pair, float64) {
	// Compute frequencies
	//freq := make(map[float64]int)
	numBuckets = utils.Max(numBuckets, 5)
	xMin := utils.Min(utils.MinOver(data1), utils.MinOver(data0))
	xMax := utils.Max(utils.MaxOver(data1), utils.MaxOver(data0))
	interval := (xMax - xMin) / float64(numBuckets)
	freq := make([]*range_, numBuckets)
	totalCount0 := 0
	totalCount1 := 0
	for i := range freq {
		freq[i] = newRange(xMin+(float64(i)*interval), xMin+(float64(i+1)*interval), i, interval)
	}
	for _, value := range data0 {
		if r := utils.FindPointer(freq, func(r *range_) bool {
			return r.contains(value)
		}); r != nil {
			r.add0()
			totalCount0++
		}
	}
	for _, value := range data1 {
		if r := utils.FindPointer(freq, func(r *range_) bool {
			return r.contains(value)
		}); r != nil {
			r.add1()
			totalCount1++
		}
	}
	//numValues := len(freq) - utils.CountAny(freq, func(r *range_) bool {
	//	return r.count == 0
	//})

	cdf := make([]pair, 0)

	for _, r := range freq {
		cdf = append(cdf, pair{
			key:      r.min,
			value0:   float64(r.count0) / float64(totalCount0),
			value1:   float64(r.count1) / float64(totalCount1),
			interval: r.width,
		})
	}

	return cdf, xMin
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

func createCDFPlot(file string, probabilities []float64, xMin float64, title, xLabel, yLabel string) (string, error) {
	newName := fmt.Sprintf("/plots/%s_%d.png", file, time.Now().UnixNano()/int64(time.Millisecond))
	// Create a new plot
	p := plot.New()
	p.Title.Text = title
	p.Y.Label.Text = yLabel
	p.X.Label.Text = xLabel

	xLabels := make([]string, len(probabilities))
	for i := range xLabels {
		xLabels[i] = fmt.Sprintf("%2.f", float64(i)+xMin)
	}

	// Calculate bar width based on the number of points
	plotWidth := 8 * vg.Inch
	barWidth := plotWidth / vg.Length(len(probabilities)*2)

	// Create a bar chart
	//w := vg.Points(20) // Width of the bars
	bars, err := plotter.NewBarChart(plotter.Values(probabilities), barWidth)
	if err != nil {
		return "", pl.WrapError(err, "failed to create bar chart")
	}
	bars.LineStyle.Width = vg.Length(0)                                  // No line around bars
	bars.Color = color.Color(color.RGBA{R: 145, G: 112, B: 222, A: 250}) // Set the color of the bars

	p.Add(bars)
	p.NominalX(xLabels...) // Set node IDs as labels on the X-axis

	// Save the plot to a PNG file
	if err := p.Save(8*vg.Inch, 4*vg.Inch, "static"+newName); err != nil {
		log.Panic(err)
	}
	return newName, nil
}

func createFloatCDFPlot(file string, probabilities []pair, xMin float64, title, xLabel, yLabel string) (string, error) {
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
	values1 := make([]float64, len(keys))
	overlapValues := make([]float64, len(keys))
	ratios := make([]float64, len(keys))

	totalArea0 := 0.0
	totalArea1 := 0.0
	totalOverlapArea := 0.0
	yMax := 0.0

	for i, label := range keys {
		xLabels[i] = fmt.Sprintf("%.2f", label)
		values := utils.Find(probabilities, func(p pair) bool {
			return p.key == label
		})
		values0[i] = values.value0
		values1[i] = values.value1
		overlapValues[i] = math.Min(values.value0, values.value1) // Calculate overlap

		area0 := values0[i] * values.interval
		area1 := values1[i] * values.interval

		ratios[i] = math.Min(area0, area1) / math.Max(area0, area1)

		if math.Max(area0, area1) == 0 {
			ratios[i] = 1
		}

		totalOverlapArea += overlapValues[i] * values.interval
		totalArea0 += area0
		totalArea1 += area1

		yMax = utils.Max(yMax, utils.Max(values0[i], values1[i]))
	}

	averageRatio := utils.Sum(ratios) / float64(len(ratios))

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

	bars1, err := plotter.NewBarChart(plotter.Values(values1), barWidth)
	if err != nil {
		return "", pl.WrapError(err, "failed to create bar chart")
	}
	bars1.LineStyle.Width = vg.Length(0)   // No line around bars
	bars1.Color = color.Color(secondColor) // Set the color of the bars

	p.Add(bars1)

	// Create a bar chart for overlap
	overlapBars, err := plotter.NewBarChart(plotter.Values(overlapValues), barWidth)
	if err != nil {
		return "", pl.WrapError(err, "failed to create overlap bar chart")
	}
	overlapBars.LineStyle.Width = vg.Length(0) // No line around bars
	overlapBars.Color = color.Color(overlap)   // Blue overlap

	p.Add(overlapBars)

	// Create a legend
	p.Legend.Add(fmt.Sprintf("Scenario 0 (total area = %.6f)", totalArea0), bars0)
	p.Legend.Add(fmt.Sprintf("Scenario 1 (total area = %.6f)", totalArea1), bars1)
	p.Legend.Add(fmt.Sprintf("Overlap (total area = %.6f)", totalOverlapArea), overlapBars)
	p.Legend.Top = true // Position the legend at the top

	// Add text annotation
	notes, _ := plotter.NewLabels(plotter.XYLabels{
		XYs:    []plotter.XY{{X: 1, Y: yMax * 1.1}}, // Position of the note
		Labels: []string{fmt.Sprintf("Ratio: %.6f", averageRatio)},
	})
	p.Add(notes)

	p.NominalX(xLabels...) // Set node IDs as labels on the X-axis

	// Save the plot to a PNG file
	if err := p.Save(8*vg.Inch, 4*vg.Inch, "static"+newName); err != nil {
		log.Panic(err)
	}
	return newName, nil
}
