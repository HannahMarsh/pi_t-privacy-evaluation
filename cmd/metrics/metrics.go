package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/config"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/api/structs"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/model/adversary"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	_ "github.com/lib/pq"
	"go.uber.org/automaxprocs/maxprocs"
	"golang.org/x/exp/slog"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"image/color"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func main() {
	//isMixer := flag.Bool("mixer", false, "Included if this node is a mixer")
	logLevel := flag.String("log-level", "debug", "Log level")
	flag.Usage = func() {
		flag.PrintDefaults()
	}

	flag.Parse()

	pl.SetUpLogrusAndSlog(*logLevel)

	// set GOMAXPROCS
	if _, err := maxprocs.Set(); err != nil {
		slog.Error("failed set max procs", err)
		os.Exit(1)
	}

	if err := config.InitGlobal(); err != nil {
		slog.Error("failed to init config", err)
		os.Exit(1)
	}

	collect()

}

func collect() {

	output_dir := fmt.Sprintf("results/%d_%d_%d_%d/%d", config.GlobalConfig.N, config.GlobalConfig.R, config.GlobalConfig.D, config.GlobalConfig.L, config.GlobalConfig.Scenario)

	err := os.MkdirAll(output_dir, os.ModePerm)
	if err != nil {
		fmt.Println("Error creating directories:", err)
		return
	}

	Clients := make(map[int]*structs.ClientStatus)
	Nodes := make(map[int]*structs.NodeStatus)

	for _, client := range config.GlobalConfig.Clients {

		addr := fmt.Sprintf("http://%s:%d", client.Host, client.Port)
		resp, err := http.Get(addr + "/status")
		if err != nil {
			slog.Error("failed to get client status", err)
		} else {
			defer resp.Body.Close()
			var status = new(structs.ClientStatus)
			if err = json.NewDecoder(resp.Body).Decode(status); err != nil {
				slog.Error("failed to decode client status", err)
			} else {
				Clients[addressToId(addr)] = status
			}
		}
	}

	for _, node := range config.GlobalConfig.Nodes {
		addr := fmt.Sprintf("http://%s:%d", node.Host, node.Port)
		resp, err := http.Get(addr + "/status")
		if err != nil {
			slog.Error("failed to get client status", err)
		} else {
			defer resp.Body.Close()
			var status = new(structs.NodeStatus)
			if err = json.NewDecoder(resp.Body).Decode(status); err != nil {
				slog.Error("failed to decode client status", err)
			} else {
				Nodes[addressToId(addr)] = status
			}
		}
	}

	views := adversary.CollectViews(Nodes, Clients)

	probabilities := views.GetProbabilities(2)
	toPlot := probabilities[len(probabilities)-1][1 : config.GlobalConfig.R+1] // get probabilities for last round
	probData := appendAndGetData(output_dir, "probabilities", toPlot)
	averages := computeAverages(probData)

	createPlot(fmt.Sprintf("%s/%s.png", output_dir, "probabilities"), averages)

	receivedN := views.GetNumOnionsReceived(len(config.GlobalConfig.Clients), config.GlobalConfig.L+1)
	receivedN_1 := views.GetNumOnionsReceived(len(config.GlobalConfig.Clients)-1, config.GlobalConfig.L+1)

	slog.Info("", "receivedN", receivedN, "receivedN_1", receivedN_1)
	numOnionsData := appendAndGetData(output_dir, "numOnionsReceived", []float64{float64(receivedN), float64(receivedN_1)})

	NData := utils.Map(numOnionsData, func(x []float64) int { return int(utils.GetFirst(x)) })
	N_1Data := utils.Map(numOnionsData, func(x []float64) int { return int(utils.GetLast(x)) })

	cdfN, shiftN := computeCDF(NData)
	cdfN_1, shiftN_1 := computeCDF(N_1Data)

	createCDFPlot(fmt.Sprintf("%s/%s.png", output_dir, "numOnionsReceivedR"), cdfN, shiftN, config.GlobalConfig.Range, fmt.Sprintf("Client %d", config.GlobalConfig.R), "Number of onions received", "CDF")
	createCDFPlot(fmt.Sprintf("%s/%s.png", output_dir, "numOnionsReceivedR_1"), cdfN_1, shiftN_1, config.GlobalConfig.Range, fmt.Sprintf("Client %d", config.GlobalConfig.R-1), "Number of onions received", "CDF")
}

func appendAndGetData(output_dir string, name string, toPlot []float64) [][]float64 {

	appendData(fmt.Sprintf("%s/%s.csv", output_dir, name), toPlot)

	// Read and process the data
	d, err := readCSV(fmt.Sprintf("%s/%s.csv", output_dir, name))
	if err != nil {
		fmt.Println("Error reading CSV:", err)
		return nil
	}
	return d
}

func appendData(filePath string, newData []float64) {
	// Convert float slice to a comma-separated string
	line := make([]string, len(newData))
	for i, value := range newData {
		line[i] = fmt.Sprintf("%f", value)
	}

	// Open the file in append mode, create it if not existing
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Failed to open file: %v", err)
		return
	}
	defer file.Close()

	// Write the line to the file
	if _, err := file.WriteString(strings.Join(line, ", ") + "\n"); err != nil {
		fmt.Printf("Failed to write to file: %v", err)
	} else {
		fmt.Println("Data appended successfully")
	}
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

func readCSV(filePath string) ([][]float64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ',' // Set the delimiter

	var data [][]float64
	for {
		record, err := reader.Read()
		if err != nil {
			break
		}

		var row []float64
		for _, value := range record {
			if floatVal, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
				row = append(row, floatVal)
			} else {
				return nil, err
			}
		}
		data = append(data, row)
	}

	return data, nil
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

	for i := 0; i < len(freq); i += config.GlobalConfig.Range {
		count := 0
		for j := i; j < utils.Min(len(freq), i+config.GlobalConfig.Range); j++ {
			count += freq[j]
		}
		cdf = append(cdf, float64(count)/float64(cumulativeSum))
	}
	return cdf, xMin
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

func print2DArray(arr [][]int) {
	// Determine the maximum number of digits in the largest number for proper alignment
	maxWidth := 0
	for _, row := range arr {
		for _, value := range row {
			digits := len(fmt.Sprintf("%d", value))
			if digits > maxWidth {
				maxWidth = digits
			}
		}
	}

	// Print the array with each element aligned right according to the maxWidth
	for _, row := range arr {
		for _, value := range row {
			fmt.Printf("%*d ", maxWidth, value) // Right-align the number in a field of maxWidth
		}
		fmt.Println()
	}
}

func addressToId(addr string) int {
	if node := utils.Find(config.GlobalConfig.Nodes, func(node config.Node) bool {
		return strings.Contains(node.Address, addr) || strings.Contains(addr, node.Address)
	}); node != nil {
		return node.ID + len(config.GlobalConfig.Clients)
	} else if client := utils.Find(config.GlobalConfig.Clients, func(client config.Client) bool {
		return strings.Contains(client.Address, addr) || strings.Contains(addr, client.Address)
	}); client != nil {
		return client.ID
	} else {
		pl.LogNewError("addressToId(): address not found %s", addr)
		return -1
	}
}

func networkIdToAddress(networkId int) string {
	if networkId <= len(config.GlobalConfig.Clients) {
		return config.GlobalConfig.GetClientAddress(networkId)
	} else {
		return config.GlobalConfig.GetNodeAddress(networkId - len(config.GlobalConfig.Clients))
	}
}
