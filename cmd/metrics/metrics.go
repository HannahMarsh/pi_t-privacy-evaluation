package main

import (
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

	// Create a new file
	file, err := os.Create("results/results.json")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// Create a JSON encoder that writes to the file
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // Optional: Include indent for pretty-printing

	if err = encoder.Encode(views); err != nil {
		slog.Error("failed to marshal views", err)
	}

	slog.Info("done")

	probabilities := views.GetProbabilities()

	toPlot := probabilities[len(probabilities)-1][1 : config.GlobalConfig.R+1]

	createPlot(toPlot, output_dir+"/probabilities.png")

	// Open the file in append mode, create it if not existing, only write mode
	file, err = os.OpenFile(output_dir+"/data", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Failed to open file: %v", err)
		return
	}
	defer file.Close()

	// The line to append
	lineToAppend := strings.Join(utils.Map(toPlot, func(p float64) string {
		return fmt.Sprintf("%f", p)
	}), ", ") + "\n"

	// Write the line to the file
	if _, err := file.WriteString(lineToAppend); err != nil {
		fmt.Printf("Failed to write to file: %v", err)
	} else {
		fmt.Println("Line appended successfully")
	}

	//for pr := range probabilities {
	//	slog.Info("", "Round", pr)
	//	createPlot(probabilities[pr], fmt.Sprintf("results/probabilities_round_%d.png", pr))
	//}

}

func createPlot(probabilities []float64, file string) {
	// Create a new plot
	p := plot.New()
	p.Title.Text = "Node Probabilities"
	p.Y.Label.Text = "Probability"

	nodeIDs := make([]string, len(probabilities))
	for i := range nodeIDs {
		nodeIDs[i] = fmt.Sprintf("Node %d", i)
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
