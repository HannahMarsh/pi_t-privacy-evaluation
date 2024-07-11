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
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

func main() {
	//isMixer := flag.Bool("mixer", false, "Included if this node is a mixer")
	logLevel := flag.String("log-level", "debug", "Log level")
	output := flag.String("output", "results", "Output directory")

	flag.Usage = func() {
		if _, err := fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0]); err != nil {
			slog.Error("Usage of %s:\n", err, os.Args[0])
		}
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

	cfg := config.GlobalConfig

	slog.Info("⚡ init metrics", "host", cfg.Metrics.Host, "port", cfg.Metrics.Port)

	slog.Info("🌏 start metrics...", "address", fmt.Sprintf(" %s:%d ", cfg.Metrics.Host, cfg.Metrics.Port))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	collect(*output)

}

func collect(output_dir string) {

	Clients := make(map[string]structs.ClientStatus)
	Nodes := make(map[string]structs.NodeStatus)

	for _, client := range config.GlobalConfig.Clients {
		addr := fmt.Sprintf("http://%s:%d/status", client.Host, client.Port)
		resp, err := http.Get(addr)
		if err != nil {
			slog.Error("failed to get client status", err)
		} else {
			defer resp.Body.Close()
			var status structs.ClientStatus
			if err = json.NewDecoder(resp.Body).Decode(&status); err != nil {
				slog.Error("failed to decode client status", err)
			} else {
				Clients[addr] = status
			}
		}
	}

	for _, node := range config.GlobalConfig.Nodes {
		addr := fmt.Sprintf("http://%s:%d/status", node.Host, node.Port)
		resp, err := http.Get(addr)
		if err != nil {
			slog.Error("failed to get client status", err)
		} else {
			defer resp.Body.Close()
			var status structs.NodeStatus
			if err = json.NewDecoder(resp.Body).Decode(&status); err != nil {
				slog.Error("failed to decode client status", err)
			} else {
				Nodes[addr] = status
			}
		}
	}

	views := adversary.CollectViews(Nodes, Clients)

	probabilities := views.GetProbabilities()
	toPlot := probabilities[len(probabilities)-1][1 : config.GlobalConfig.R+1] // get probabilities for last round
	probData := appendAndGetData(output_dir, "probabilities", toPlot)
	averages := computeAverages(probData)

	createPlot(averages, fmt.Sprintf("%s/%s.png", output_dir, "probabilities"))

	numsN := views.GetNumOnionsReceived(len(config.GlobalConfig.Clients))
	numsN_1 := views.GetNumOnionsReceived(len(config.GlobalConfig.Clients) - 1)
	receivedN := utils.GetLast(numsN)
	receivedN_1 := utils.GetLast(numsN_1)

	slog.Info("", "receivedN", receivedN, "receivedN_1", receivedN_1)
	//numOnionsData := appendAndGetData(output_dir, "numOnionsReceived", []float64{float64(receivedN), float64(receivedN_1)})
	//cdf := computeCDF(numOnionsData)
	//createPlot(cdf, fmt.Sprintf("%s/%s.png", output_dir, "numOnionsReceived"))

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

func createPlot(probabilities []float64, file string) {
	// Create a new plot
	p := plot.New()
	p.Title.Text = "Node Probabilities"
	p.Y.Label.Text = "Probability"

	nodeIDs := make([]string, len(probabilities))
	for i := range nodeIDs {
		nodeIDs[i] = fmt.Sprintf("%d", i)
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
