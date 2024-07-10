package bulletin_board

import (
	"bytes"
	"encoding/json"
	"fmt"
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/config"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/api/structs"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"golang.org/x/exp/slog"
	"math/rand"
	"net/http"
	"sync"
)

// BulletinBoard represents the bulletin board that keeps track of active nodes and coordinates the start signal
type BulletinBoard struct {
	Network map[int]structs.PublicNodeApi // Maps node IDs
	Clients map[int]structs.PublicNodeApi // Maps client IDs
	started bool
	mu      sync.RWMutex
}

// NewBulletinBoard creates a new bulletin board
func NewBulletinBoard() *BulletinBoard {
	return &BulletinBoard{
		Network: make(map[int]structs.PublicNodeApi),
		Clients: make(map[int]structs.PublicNodeApi),
		started: false,
	}
}

// RegisterNode adds a node to the active nodes list
func (bb *BulletinBoard) RegisterNode(node structs.PublicNodeApi) {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	if !bb.started {
		if _, present := bb.Network[node.ID]; !present {
			bb.Network[node.ID] = node
			go bb.checkIfReadyToStart()
		}
	}
}

func (bb *BulletinBoard) RegisterClient(client structs.PublicNodeApi) {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	if !bb.started {
		if _, present := bb.Clients[client.ID]; !present {
			bb.Clients[client.ID] = client
			go bb.checkIfReadyToStart()
		}
	}
}

func (bb *BulletinBoard) checkIfReadyToStart() {
	doStart := false
	bb.mu.Lock()
	if !bb.started {
		if len(bb.Network) >= config.GlobalConfig.N && len(bb.Clients) >= config.GlobalConfig.R {
			bb.started = true
			doStart = true
			slog.Info("Ready to start", "num_nodes", len(bb.Network), "num_clients", len(bb.Clients))
		}
	}
	bb.mu.Unlock()
	if doStart {
		if err := bb.signalStart(); err != nil {
			slog.Error("Error signaling start", err)
		}
	}
}

func (bb *BulletinBoard) GetStartRunAPI() structs.StartRunAPI {
	bb.mu.RLock()
	defer bb.mu.RUnlock()
	activeNodes := utils.Filter(utils.MapEntries(bb.Network, func(_ int, node structs.PublicNodeApi) structs.PublicNodeApi {
		return node
	}), func(n structs.PublicNodeApi) bool {
		return n.Address != ""
	})
	participatingClients := utils.Filter(utils.MapEntries(bb.Clients, func(_ int, client structs.PublicNodeApi) structs.PublicNodeApi {
		return client
	}), func(n structs.PublicNodeApi) bool {
		return n.Address != ""
	})

	scenario := rand.Intn(len(config.GlobalConfig.Scenarios))

	slog.Info("Selected scenario", "scenario", scenario)

	return structs.StartRunAPI{
		Clients:  participatingClients,
		Nodes:    activeNodes,
		Scenario: scenario,
	}
}

func (bb *BulletinBoard) signalStart() error {
	slog.Info("Signaling nodes to start")

	vs := bb.GetStartRunAPI()

	all := append(utils.Copy(vs.Nodes), vs.Clients...)

	if data, err := json.Marshal(vs); err != nil {
		return pl.WrapError(err, "failed to marshal start signal")
	} else {
		var wg sync.WaitGroup
		for _, n := range all {
			wg.Add(1)
			go func(n structs.PublicNodeApi) {
				defer wg.Done()
				url := fmt.Sprintf("%s/start", n.Address)
				if resp, err2 := http.Post(url, "application/json", bytes.NewBuffer(data)); err2 != nil {
					slog.Error("Error signaling node to start \n"+url, err2)
				} else if err3 := resp.Body.Close(); err3 != nil {
					fmt.Printf("Error closing response body: %v\n", err3)
				}
			}(n)
		}
		wg.Wait()
		return nil
	}
}
