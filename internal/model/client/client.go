package client

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
	rng "math/rand"
	"net/http"
	"sync"
	"time"
)

type Client struct {
	ID               int
	Host             string
	Port             int
	Address          string
	mu               sync.RWMutex
	BulletinBoardUrl string
	status           *structs.ClientStatus
}

// NewNode creates a new client instance
func NewClient(id int, host string, port int, bulletinBoardUrl string) (*Client, error) {
	c := &Client{
		ID:               id,
		Host:             host,
		Port:             port,
		Address:          fmt.Sprintf("http://%s:%d", host, port),
		BulletinBoardUrl: bulletinBoardUrl,
		status:           structs.NewClientStatus(id, fmt.Sprintf("http://%s:%d", host, port)),
	}

	if err2 := c.RegisterWithBulletinBoard(); err2 != nil {
		return nil, pl.WrapError(err2, "%s: failed to register with bulletin board", pl.GetFuncName(id, host, port, bulletinBoardUrl))
	}

	return c, nil
}

// RegisterWithBulletinBoard registers the client with the bulletin board
func (c *Client) RegisterWithBulletinBoard() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if data, err := json.Marshal(structs.PublicNodeApi{
		ID:      c.ID,
		Address: c.Address,
	}); err != nil {
		return pl.WrapError(err, "%s: failed to marshal Client info", pl.GetFuncName())
	} else {
		url := c.BulletinBoardUrl + "/registerClient"
		if resp, err2 := http.Post(url, "application/json", bytes.NewBuffer(data)); err2 != nil {
			return pl.WrapError(err2, "%s: failed to send POST request to bulletin board", pl.GetFuncName())
		} else {
			defer func(Body io.ReadCloser) {
				if err3 := Body.Close(); err3 != nil {
					slog.Error(pl.GetFuncName()+": error closing response body", err2)
				}
			}(resp.Body)
			if resp.StatusCode != http.StatusCreated {
				return pl.NewError("%s: failed to register client, status code: %d, %s", pl.GetFuncName(), resp.StatusCode, resp.Status)
			} else {
				slog.Info("Client registered with bulletin board", "id", c.ID)
			}
			return nil
		}
	}
}

var rand = rng.New(rng.NewSource(time.Now().UnixNano()))

// DetermineRoutingPath determines a random routing path of a given length
func DetermineRoutingPath(l int, participants []structs.PublicNodeApi) []structs.PublicNodeApi {
	p := utils.Copy(participants)[:l]
	utils.Shuffle(p)
	return p
}

// DetermineCheckpointRoutingPath determines a routing path with a checkpoint
func DetermineCheckpointRoutingPath(l int, receiver structs.PublicNodeApi, layer int, nodes, clients []structs.PublicNodeApi) []structs.PublicNodeApi {
	removed := utils.Filter(nodes, func(api structs.PublicNodeApi) bool {
		return api.Address != receiver.Address || api.ID != receiver.ID
	})
	path := DetermineRoutingPath(l-1, removed)
	path2 := utils.InsertAtIndex(path, layer, receiver)
	hasUnique := utils.HasUniqueElements(path2)
	path3 := utils.InsertAtIndex(path, layer, receiver)

	slog.Info("Client determining checkpoint routing path", "path", path2, "has_unique", hasUnique, "path3", path3)
	return append(path2, utils.RandomElement(clients))
}

// formOnions forms the onions for the messages to be sent
func (c *Client) formOnions(start structs.StartRunAPI) (onions []structs.Onion, err error) {
	nodes := utils.Filter(utils.RemoveDuplicates(start.Nodes), func(node structs.PublicNodeApi) bool {
		return node.Address != ""
	})
	clients := utils.Filter(utils.RemoveDuplicates(start.Clients), func(node structs.PublicNodeApi) bool {
		return node.Address != c.Address && node.Address != ""
	})

	msgs := utils.Map(utils.Filter(config.GlobalConfig.Scenarios[start.Scenario].Messages, func(message config.Message) bool {
		return message.From == c.ID
	}), func(message config.Message) structs.Message {
		from := config.GlobalConfig.GetClientAddress(message.From)
		to := config.GlobalConfig.GetClientAddress(message.To)
		return structs.NewMessage(from, to, message.Content)
	})

	messages := utils.Filter(msgs, func(message structs.Message) bool {
		return utils.Contains(clients, func(client structs.PublicNodeApi) bool {
			return client.Address == message.To
		})
	})

	if len(messages) == 0 {
		sendTo := config.GlobalConfig.GetClientAddress((len(clients) - c.ID) + 2)
		messages = []structs.Message{structs.NewMessage(c.Address, sendTo, "empty message")}
	}

	if onions, err = utils.FlatMapParallel(messages, func(msg structs.Message) ([]structs.Onion, error) {
		if onions, err = c.processMessage(msg, nodes, clients); err != nil {
			return nil, pl.WrapError(err, "failed to process message")
		} else {
			return onions, nil
		}
	}); err != nil {
		return nil, pl.WrapError(err, "failed to form onions")
	} else {
		return onions, nil
	}
}

// processMessage processes a single message to form its onion
func (c *Client) processMessage(msg structs.Message, nodes, clients []structs.PublicNodeApi) (onions []structs.Onion, err error) {
	onions = make([]structs.Onion, 0)

	path := utils.Map(DetermineRoutingPath(config.GlobalConfig.L, nodes), func(node structs.PublicNodeApi) string {
		return node.Address
	})
	path = append(path, msg.To)
	o, err := c.formOnion(msg, path)
	onions = append(onions, o)

	numCheckPointOnionsToSend := utils.Max(1, int((rand.NormFloat64()*config.GlobalConfig.StdDev)+float64(config.GlobalConfig.D)))

	slog.Info("Client creating checkpoint onions", "num_onions", numCheckPointOnionsToSend)

	// create checkpoint onions
	for i := 0; i < numCheckPointOnionsToSend; i++ {
		if o, err = c.createCheckpointOnion(nodes, clients); err != nil {
			return nil, pl.WrapError(err, "failed to create checkpoint onion")
		}
		onions = append(onions, o)
	}

	return onions, nil
}

func (c *Client) formOnion(msg structs.Message, path []string) (onion structs.Onion, err error) {
	onion, err = structs.NewOnion(msg, len(path))
	for _, hop := range utils.Reverse(path)[1:] {
		if onion, err = onion.AddLayer(hop); err != nil {
			return onion, pl.WrapError(err, "failed to add layer")
		}
	}
	onion.From = c.Address
	go c.status.AddSent(msg, path)
	return onion, nil
}

// createCheckpointOnions creates checkpoint onions for the routing path
func (c *Client) createCheckpointOnion(nodes, clients []structs.PublicNodeApi) (onion structs.Onion, err error) {
	l := config.GlobalConfig.L
	layer := rand.Intn(l)
	receiver := utils.RandomElement(nodes)
	path := utils.Map(DetermineCheckpointRoutingPath(l, receiver, layer, nodes, clients), func(node structs.PublicNodeApi) string {
		return node.Address
	})
	dummyMsg := structs.NewMessage(c.Address, path[len(path)-1], "checkpoint onion")
	if onion, err = c.formOnion(dummyMsg, path); err != nil {
		return onion, pl.WrapError(err, "failed to form onion")
	} else {
		return onion, nil
	}
}

func (c *Client) startRun(start structs.StartRunAPI) error {

	slog.Info("Client starting run", "num clients", len(start.Clients), "num nodes", len(start.Nodes))

	if toSend, err := c.formOnions(start); err != nil {
		return pl.WrapError(err, "failed to form toSend")
	} else {
		numToSend := len(toSend)

		slog.Info("Client sending onions", "num_onions", numToSend)

		var wg sync.WaitGroup
		wg.Add(numToSend)
		for _, onion := range toSend {
			go func(onion structs.Onion) {
				defer wg.Done()
				if err = api_functions.SendOnion(onion); err != nil {
					slog.Error("failed to send onions", err)
				}
			}(onion)
		}

		wg.Wait()
		return nil
	}
}

func (c *Client) Receive(o structs.Onion) error {
	if o2, message, err := o.Peel(); err != nil {
		return pl.WrapError(err, "failed to peel onion")
	} else {
		slog.Info("Client received message", "from", message.From, "to", message.To, "msg", message.Msg, "o", o2)
		go c.status.AddReceived(message)
	}
	return nil
}

func (c *Client) GetStatus() string {
	return c.status.GetStatus()
}
