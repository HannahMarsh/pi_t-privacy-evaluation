package node

import (
	"bytes"
	"encoding/json"
	"fmt"
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/config"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/api/api_functions"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/api/structs"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"golang.org/x/exp/slog"
	"io"
	"net/http"
	"sync"
)

var adversary config.Adversary
var alwaysDropFrom []string

// Node represents a node in the onion routing network
type Node struct {
	ID               int
	Host             string
	Port             int
	Address          string
	mu               sync.RWMutex
	BulletinBoardUrl string
	status           *structs.NodeStatus
	isAdversary      bool
}

// NewNode creates a new node
func NewNode(id int, host string, port int, bulletinBoardUrl string, isAdversary bool) (*Node, error) {
	n := &Node{
		ID:               id,
		Host:             host,
		Port:             port,
		Address:          fmt.Sprintf("http://%s:%d", host, port),
		BulletinBoardUrl: bulletinBoardUrl,
		status:           structs.NewNodeStatus(id, fmt.Sprintf("http://%s:%d", host, port)),
		isAdversary:      isAdversary,
	}
	if err2 := n.RegisterWithBulletinBoard(); err2 != nil {
		return nil, pl.WrapError(err2, "node.NewNode(): failed to register with bulletin board")
	}
	adversary = config.GlobalConfig.Adversary
	alwaysDropFrom = utils.Map(adversary.AlwaysDropFrom, config.GlobalConfig.GetClientAddress)
	return n, nil
}

func (n *Node) GetStatus() string {
	return n.status.GetStatus()
}

func (n *Node) getPublicNodeInfo() structs.PublicNodeApi {
	return structs.PublicNodeApi{
		ID:      n.ID,
		Address: fmt.Sprintf("http://%s:%d", n.Host, n.Port),
	}
}

func (n *Node) startRun(start structs.StartRunAPI) {
	slog.Info("Starting run", "num clients", len(start.Clients), "num nodes", len(start.Nodes))
}

func (n *Node) Receive(o structs.Onion) error {
	if o.To != fmt.Sprintf("http://%s:%d", n.Host, n.Port) {
		return pl.NewError("node.Receive(): onion not meant for this node")
	}

	slog.Info("Received onion", "from", config.AddressToName(o.From), "layer", o.Layer)
	peeled, _, err := o.Peel()
	if err != nil {
		return pl.WrapError(err, "node.Receive(): failed to peel onion")
	}

	if n.isAdversary && utils.ContainsElement(alwaysDropFrom, o.From) || utils.ContainsElement(alwaysDropFrom, peeled.To) {
		slog.Info("Dropping onion", "from", config.AddressToName(o.From), "to", config.AddressToName(o.To))
		go n.status.AddOnion(o.From, n.Address, peeled.To, o.Layer, false, true)
		return nil
	}

	go n.status.AddOnion(o.From, n.Address, peeled.To, o.Layer, false, false)

	if err = api_functions.SendOnion(peeled); err != nil {
		return pl.WrapError(err, "node.Receive(): failed to send to next node")
	}
	return nil
}

func (n *Node) RegisterWithBulletinBoard() error {
	slog.Info("Sending node registration request.", "id", n.ID)
	err := n.updateBulletinBoard("/registerNode", http.StatusCreated)
	if err != nil {
		return pl.WrapError(err, "node.RegisterWithBulletinBoard(): failed to register node")
	} else {
		slog.Info("Node successfully registered with bulletin board", "id", n.ID)
	}
	return nil
}

func (n *Node) GetActiveNodes() ([]structs.PublicNodeApi, error) {
	url := fmt.Sprintf("%s/nodes", n.BulletinBoardUrl)
	resp, err := http.Get(url)
	if err != nil {
		return nil, pl.WrapError(err, fmt.Sprintf("error making GET request to %s", url))
	}
	defer func(Body io.ReadCloser) {
		if err2 := Body.Close(); err2 != nil {
			fmt.Printf("error closing response body: %v\n", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, pl.NewError("unexpected status code: %d", resp.StatusCode)
	}

	var activeNodes []structs.PublicNodeApi
	if err = json.NewDecoder(resp.Body).Decode(&activeNodes); err != nil {
		return nil, pl.WrapError(err, "error decoding response body")
	}

	return activeNodes, nil
}

func (n *Node) updateBulletinBoard(endpoint string, expectedStatusCode int) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if data, err := json.Marshal(structs.PublicNodeApi{
		ID:      n.ID,
		Address: fmt.Sprintf("http://%s:%d", n.Host, n.Port),
	}); err != nil {
		return pl.WrapError(err, "node.UpdateBulletinBoard(): failed to marshal node info")
	} else {
		url := n.BulletinBoardUrl + endpoint
		//slog.Info("Sending node registration request.", "url", url, "id", n.ID)
		if resp, err2 := http.Post(url, "application/json", bytes.NewBuffer(data)); err2 != nil {
			return pl.WrapError(err2, "node.UpdateBulletinBoard(): failed to send POST request to bulletin board")
		} else {
			defer func(Body io.ReadCloser) {
				if err3 := Body.Close(); err3 != nil {
					fmt.Printf("node.UpdateBulletinBoard(): error closing response body: %v\n", err2)
				}
			}(resp.Body)
			if resp.StatusCode != expectedStatusCode {
				return pl.NewError("failed to %s node, status code: %d, %s", endpoint, resp.StatusCode, resp.Status)
			}
			return nil
		}
	}
}
