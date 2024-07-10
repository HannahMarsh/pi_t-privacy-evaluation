package main

import (
	"encoding/json"
	"flag"
	"fmt"
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/config"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/api/structs"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/model/adversary"
	_ "github.com/lib/pq"
	"go.uber.org/automaxprocs/maxprocs"
	"golang.org/x/exp/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	//isMixer := flag.Bool("mixer", false, "Included if this node is a mixer")
	logLevel := flag.String("log-level", "debug", "Log level")

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

	go collect()

	select {
	case v := <-quit:
		config.GlobalCancel()
		slog.Info("signal.Notify", v)
	case done := <-config.GlobalCtx.Done():
		slog.Info("ctx.Done", done)
	}

}

func collect() {

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

}
