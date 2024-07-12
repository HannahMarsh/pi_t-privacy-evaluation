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
	N        []int     `json:"N"`
	R        []int     `json:"R"`
	D        []int     `json:"D"`
	L        []int     `json:"L"`
	X        []float64 `json:"X"`
	StdDev   []float64 `json:"StdDev"`
	Scenario []int     `json:"Scenario"`
	NumRuns  []int     `json:"NumRuns"`
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

	v := utils.Find(allData.Data, func(data Data) bool {
		return data.Params == p
	})

	if v == nil {
		v = &(allData.Data[0])
		return
	}

	plotView(v.Views[:numRuns])

	if err := json.NewEncoder(w).Encode([]string{"goood"}); err != nil {
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

func plotView(view []View) {
	if len(view) == 0 {
		return
	}
	probabilities := make([]float64, len(view[0].Probabilities))
	for i := 0; i < len(view[0].Probabilities); i++ {
		sum := 0.0
		for j := 0; j < len(view); j++ {
			sum += view[j].Probabilities[i]
		}
		probabilities[i] = sum / float64(len(view))
	}

	createPlot("static/plots/Probabilities.png", probabilities)

	cdfN, shiftN := computeCDF(utils.Map(view, getReceivedR))
	cdfN_1, shiftN_1 := computeCDF(utils.Map(view, getReceivedR_1))
	createCDFPlot("static/plots/ReceivedR.png", cdfN, shiftN, 2, "Client R", "Number of onions received", "CDF")
	createCDFPlot("static/plots/ReceivedR_1.png", cdfN_1, shiftN_1, 2, "Client R-1", "Number of onions received", "CDF")
}

func computeCDF(data []int) ([]float64, int) {
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

	for i := 0; i < len(freq); i += 2 {
		count := 0
		for j := i; j < utils.Min(len(freq), i+2); j++ {
			count += freq[j]
		}
		cdf = append(cdf, float64(count)/float64(cumulativeSum))
	}
	return cdf, xMin
}

func createPlot(file string, probabilities []float64) {
	// Create a new plot
	p := plot.New()
	p.Title.Text = "Node Probabilities"
	p.Y.Label.Text = "Probability"

	nodeIDs := make([]string, len(probabilities))
	for i := range nodeIDs {
		nodeIDs[i] = fmt.Sprintf("%d", i+1)
	}

	// Create a bar chart
	w := vg.Points(20) // Width of the bars
	bars, err := plotter.NewBarChart(plotter.Values(probabilities), w)
	if err != nil {
		log.Panic(err)
	}
	bars.LineStyle.Width = vg.Length(0)                  // No line around bars
	bars.Color = color.Color(color.RGBA{R: 255, A: 255}) // Set the color of the bars

	p.Add(bars)
	p.NominalX(nodeIDs...) // Set node IDs as labels on the X-axis

	// Save the plot to a PNG file
	if err := p.Save(8*vg.Inch, 4*vg.Inch, file); err != nil {
		log.Panic(err)
	}
}

func createCDFPlot(file string, probabilities []float64, xMin int, bucketSize int, title, xLabel, yLabel string) {
	// Create a new plot
	p := plot.New()
	p.Title.Text = title
	p.Y.Label.Text = yLabel
	p.X.Label.Text = xLabel

	xLabels := make([]string, len(probabilities))
	for i := range xLabels {
		xLabels[i] = fmt.Sprintf("%d", (i+xMin)+(i*bucketSize))
	}

	// Create a bar chart
	w := vg.Points(20) // Width of the bars
	bars, err := plotter.NewBarChart(plotter.Values(probabilities), w)
	if err != nil {
		log.Panic(err)
	}
	bars.LineStyle.Width = vg.Length(0)                  // No line around bars
	bars.Color = color.Color(color.RGBA{R: 255, A: 255}) // Set the color of the bars

	p.Add(bars)
	p.NominalX(xLabels...) // Set node IDs as labels on the X-axis

	// Save the plot to a PNG file
	if err := p.Save(8*vg.Inch, 4*vg.Inch, file); err != nil {
		log.Panic(err)
	}
}
