package main

import (
	"encoding/json"
	"flag"
	"fmt"
	pl "github.com/HannahMarsh/PrettyLogger"
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
	"net/http"
	"os"
	"strconv"
	"time"
)

type AllData struct {
	Data []Data `json:"Data"`
}

type Data struct {
	Params interfaces.Params `json:"Params"`
	Views  []View            `json:"Views"`
}

type View struct {
	Probabilities []float64 `json:"Probabilities"`
	ReceivedR     int       `json:"ReceivedR"`
	ReceivedR_1   int       `json:"ReceivedR_1"`
}

func getReceivedR(v View) int {
	return v.ReceivedR
}
func getReceivedR_1(v View) int {
	return v.ReceivedR_1
}

var allData AllData
var expectedValues ExpectedValues

// Define your data structure
type ExpectedValues struct {
	N          []int     `json:"N"`
	R          []int     `json:"R"`
	D          []int     `json:"D"`
	L          []int     `json:"L"`
	X          []float64 `json:"X"`
	StdDev     []float64 `json:"StdDev"`
	Scenario   []int     `json:"Scenario"`
	NumRuns    []int     `json:"NumRuns"`
	NumBuckets []int     `json:"NumBuckets"`
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
	expectedValues.N = utils.RemoveDuplicates(utils.Map(allData.Data, func(d Data) int {
		return d.Params.N
	}))
	utils.SortOrdered(expectedValues.N)
	slog.Info("Got values of N", "N", expectedValues.N)
	expectedValues.R = utils.RemoveDuplicates(utils.Map(allData.Data, func(d Data) int {
		return d.Params.R
	}))
	utils.SortOrdered(expectedValues.R)
	slog.Info("Geot values of R", "R", expectedValues.R)
	expectedValues.D = utils.RemoveDuplicates(utils.Map(allData.Data, func(d Data) int {
		return d.Params.D
	}))
	utils.SortOrdered(expectedValues.D)
	slog.Info("Got values of D", "D", expectedValues.D)
	expectedValues.L = utils.RemoveDuplicates(utils.Map(allData.Data, func(d Data) int {
		return d.Params.L
	}))
	utils.SortOrdered(expectedValues.L)
	slog.Info("Got values of L", "L", expectedValues.L)
	expectedValues.X = utils.RemoveDuplicates(utils.Map(allData.Data, func(d Data) float64 {
		return d.Params.X
	}))
	utils.SortOrdered(expectedValues.X)
	slog.Info("Got values of X", "X", expectedValues.X)
	expectedValues.StdDev = utils.RemoveDuplicates(utils.Map(allData.Data, func(d Data) float64 {
		return d.Params.StdDev
	}))
	utils.SortOrdered(expectedValues.StdDev)
	slog.Info("Got values of StdDev", "StdDev", expectedValues.StdDev)
	slog.Info("Got values of Scenario", "Scenario", expectedValues.Scenario)
	expectedValues.NumRuns = utils.NewIntArray(1, utils.MaxOver(utils.Map(allData.Data, func(d Data) int {
		return len(d.Views)
	}))+1)
	utils.SortOrdered(expectedValues.NumRuns)
	slog.Info("Got values of NumRuns", "NumRuns", expectedValues.NumRuns)
	maxBuckets := utils.MaxOver(utils.Map(allData.Data, func(d Data) int {
		r := utils.Map(d.Views, getReceivedR)
		return utils.MaxOver(r) - utils.MinOver(r) + 1
	}))
	expectedValues.NumBuckets = utils.Filter(utils.Map(utils.NewIntArray(1, 5), func(i int) int {
		return maxBuckets / (5 - i)
	}), func(i int) bool {
		return i > 5
	})
	utils.SortOrdered(expectedValues.NumBuckets)
	slog.Info("Got values of NumBuckets", "NumBuckets", expectedValues.NumBuckets)

	slog.Info("All data collected")

	// Start HTTP server
	// Serve static files from the "static" directory
	http.Handle("/", http.FileServer(http.Dir("static")))
	http.Handle("/plots/", http.StripPrefix("/plots/", http.FileServer(http.Dir("static/plots"))))
	http.HandleFunc("/query", queryHandler)
	http.HandleFunc("/expected", handleExpectedValues)

	slog.Info("Starting server on :8200")
	if err := http.ListenAndServe(":8200", nil); err != nil {
		slog.Error("failed to start server", err)
	}
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

func queryHandler(w http.ResponseWriter, r *http.Request) {
	p := interfaces.Params{
		N:        getIntQueryParam(r, "N"),
		R:        getIntQueryParam(r, "R"),
		D:        getIntQueryParam(r, "D"),
		L:        getIntQueryParam(r, "L"),
		X:        getFloatQueryParam(r, "X"),
		StdDev:   getFloatQueryParam(r, "StdDev"),
		Scenario: getIntQueryParam(r, "Scenario"),
	}
	numRuns := getIntQueryParam(r, "NumRuns")
	numBuckets := getIntQueryParam(r, "NumBuckets")

	slog.Info("Querying data", "Params", p, "NumRuns", numRuns, "NumBuckets", numBuckets)

	if numBuckets <= 0 {
		numBuckets = 20
	}

	v := utils.Find(allData.Data, func(data Data) bool {
		return data.Params == p
	})

	if v == nil {
		v = &(allData.Data[0])
	}

	images, err := plotView(v.Views[:utils.Max(1, utils.Min(len(v.Views), numRuns))], numBuckets)
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
	Top10         string `json:"top10"`
}

func plotView(view []View, numBuckets int) (Images, error) {
	if len(view) == 0 {
		return Images{}, pl.NewError("no views to plot")
	}
	probabilities := computeAverages(utils.Map(view, func(v View) []float64 {
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

	prImage, top10, err := createPlot("Probabilities", probabilities)
	if err != nil {
		return Images{}, pl.WrapError(err, "failed to create plot")
	}

	cdfN, shiftN := computeCDF(utils.Map(view, getReceivedR), numBuckets)
	cdfN_1, shiftN_1 := computeCDF(utils.Map(view, getReceivedR_1), numBuckets)
	prReceivedR, err := createCDFPlot("ReceivedR", cdfN, shiftN, "Client R", "Number of onions received", "CDF")
	if err != nil {
		return Images{}, pl.WrapError(err, "failed to create CDF plot")
	}
	prReceivedR_1, err := createCDFPlot("ReceivedR_1", cdfN_1, shiftN_1, "Client R-1", "Number of onions received", "CDF")
	if err != nil {
		return Images{}, pl.WrapError(err, "failed to create CDF plot")
	}

	return Images{
		Probabilities: prImage,
		ReceivedR:     prReceivedR,
		ReceivedR_1:   prReceivedR_1,
		Top10:         top10,
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

func computeCDF(data []int, numBuckets int) ([]float64, int) {
	// Compute frequencies
	//freq := make(map[float64]int)
	xMin := utils.MinOver(data)
	xMax := utils.MaxOver(data)
	freq := make([]int, xMax-xMin+1)
	for i := range freq {
		freq[i] = 0
	}
	for _, value := range data {
		freq[value-xMin] = freq[value-xMin] + 1
	}
	cumulativeSum := len(freq) - utils.Count(freq, 0)

	cdf := make([]float64, 0)

	bucketSize := utils.Max(1, (xMax-xMin)/utils.Max(numBuckets, 5))
	for i := 0; i < len(freq); i += bucketSize {
		count := 0
		for j := i; j < utils.Min(len(freq), i+bucketSize); j++ {
			count += freq[j]
		}
		cdf = append(cdf, float64(count)/float64(cumulativeSum))
	}
	return cdf, xMin
}

func createPlot(file string, probabilities []float64) (string, string, error) {

	newName := fmt.Sprintf("/plots/%s_%d.png", file, time.Now().UnixNano()/int64(time.Millisecond))

	type temp struct {
		value float64
		label string
	}

	t := make([]temp, len(probabilities))
	for i := range probabilities {
		t[i] = temp{value: probabilities[i], label: fmt.Sprintf("%d", i+1)}
	}

	utils.Sort(t, func(i, j temp) bool {
		return i.value > j.value
	})

	probabilities = utils.Map(t, func(i temp) float64 {
		return i.value
	})

	// Create a new plot
	p := plot.New()
	p.Title.Text = "Node Probabilities"
	p.Y.Label.Text = "Probability"

	nodeIDs := utils.Map(t, func(i temp) string {
		return i.label
	})

	// Create a bar chart
	w := vg.Points(20) // Width of the bars
	bars, err := plotter.NewBarChart(plotter.Values(probabilities), w)
	if err != nil {
		return "", "", pl.WrapError(err, "failed to create bar chart")
	}
	bars.LineStyle.Width = vg.Length(0)                                  // No line around bars
	bars.Color = color.Color(color.RGBA{R: 145, G: 112, B: 222, A: 250}) // Set the color of the bars

	p.Add(bars)
	p.NominalX(nodeIDs...) // Set node IDs as labels on the X-axis

	// Display top 10 most likely elements
	top10 := "Top 10 Most Likely Receivers:\n"
	for i := 0; i < 10 && i < len(t); i++ {
		top10 += fmt.Sprintf(", %s", t[i].label)
	}

	// Save the plot to a PNG file
	if err := p.Save(16*vg.Inch, 4*vg.Inch, "static"+newName); err != nil {
		log.Panic(err)
	}

	return newName, top10, nil
}

func createCDFPlot(file string, probabilities []float64, xMin int, title, xLabel, yLabel string) (string, error) {
	newName := fmt.Sprintf("/plots/%s_%d.png", file, time.Now().UnixNano()/int64(time.Millisecond))
	// Create a new plot
	p := plot.New()
	p.Title.Text = title
	p.Y.Label.Text = yLabel
	p.X.Label.Text = xLabel

	xLabels := make([]string, len(probabilities))
	for i := range xLabels {
		xLabels[i] = fmt.Sprintf("%d", (i + xMin))
	}

	// Create a bar chart
	w := vg.Points(20) // Width of the bars
	bars, err := plotter.NewBarChart(plotter.Values(probabilities), w)
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
