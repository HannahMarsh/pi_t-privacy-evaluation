package client

import (
	"fmt"
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/api/structs"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/interfaces"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"golang.org/x/exp/slog"
	rng "math/rand"
	"sync"
	"time"
)

type Client struct {
	ID int
	s  interfaces.System
}

// NewNode creates a new client instance
func NewClient(id int, s interfaces.System) (interfaces.Node, error) {
	c := &Client{
		ID: id,
		s:  s,
	}
	//	slog.Info("Registering client", "id", c.ID)
	s.RegisterParty(c)
	return c, nil
}

func (c *Client) GetID() int {
	return c.ID
}

var rand = rng.New(rng.NewSource(time.Now().UnixNano()))

// DetermineRoutingPath determines a random routing path of a given length
func DetermineRoutingPath(l int, participants []int) []int {
	p := utils.Copy(participants)[:l]
	utils.Shuffle(p)
	return p
}

// DetermineCheckpointRoutingPath determines a routing path with a checkpoint
func DetermineCheckpointRoutingPath(l int, receiver int, layer int, nodes, clients []int) []int {
	removed := utils.Filter(nodes, func(node int) bool {
		return node != receiver
	})
	path := DetermineRoutingPath(l-1, removed)
	path2 := utils.InsertAtIndex(path, layer, receiver)

	//slog.Info("Client determining checkpoint routing path", "path", path2, "has_unique", hasUnique, "path3", path3)
	return append(path2, utils.RandomElement(clients))
}

// formOnions forms the onions for the messages to be sent
func (c *Client) formOnions(scenario int) (onions []structs.Onion, err error) {

	clients := c.s.GetClients()
	nodes := c.s.GetNodes()

	var sendToClient int
	var message string
	if scenario == 0 {
		sendToClient = (len(clients) - c.ID) + 1
	} else {
		sendToClient = ((c.ID + (len(clients) - 3)) % (len(clients))) + 1
	}
	if sendToClient == 0 {
		slog.Info("sendToClient is 0", "sendToClient", sendToClient, "len(clients)", len(clients))
	}
	if c.ID == 1 || c.ID == 2 {
		message = fmt.Sprintf("scenario %s from %d to %d", scenario, c.ID, sendToClient)
	} else {
		message = "dummy message"
	}
	if onions, err = c.processMessage(structs.NewMessage(c.ID, sendToClient, message), nodes, utils.RemoveElement(clients, c.ID)); err != nil {
		return nil, pl.WrapError(err, "failed to process message")
	} else {
		return onions, nil
	}
}

// processMessage processes a single message to form its onion
func (c *Client) processMessage(msg structs.Message, nodes, clients []int) (onions []structs.Onion, err error) {
	onions = make([]structs.Onion, 0)

	path := DetermineRoutingPath(c.s.GetParams().L, nodes)
	path = append(path, msg.To)
	o, err := c.formOnion(msg, path)
	onions = append(onions, o)

	numCheckPointOnionsToSend := utils.Max(1, int((rand.NormFloat64()*c.s.GetParams().StdDev)+float64(c.s.GetParams().D)))

	//slog.Info("Client creating checkpoint onions", "num_onions", numCheckPointOnionsToSend)

	// create checkpoint onions
	for i := 0; i < numCheckPointOnionsToSend; i++ {
		if o, err = c.createCheckpointOnion(nodes, clients); err != nil {
			return nil, pl.WrapError(err, "failed to create checkpoint onion")
		}
		onions = append(onions, o)
	}

	return onions, nil
}

func (c *Client) formOnion(msg structs.Message, path []int) (onion structs.Onion, err error) {
	onion, err = structs.NewOnion(msg, len(path))
	for _, hop := range utils.Reverse(path)[1:] {
		if onion, err = onion.AddLayer(hop); err != nil {
			return onion, pl.WrapError(err, "failed to add layer")
		}
	}
	onion.From = c.ID
	return onion, nil
}

// createCheckpointOnions creates checkpoint onions for the routing path
func (c *Client) createCheckpointOnion(nodes, clients []int) (onion structs.Onion, err error) {
	l := c.s.GetParams().L
	layer := rand.Intn(l)
	receiver := utils.RandomElement(nodes)
	path := DetermineCheckpointRoutingPath(l, receiver, layer, nodes, clients)
	dummyMsg := structs.NewMessage(c.ID, path[len(path)-1], "checkpoint onion")
	if onion, err = c.formOnion(dummyMsg, path); err != nil {
		return onion, pl.WrapError(err, "failed to form onion")
	} else {
		return onion, nil
	}
}

func (c *Client) StartRun(scenario int) error {

	if toSend, err := c.formOnions(scenario); err != nil {
		return pl.WrapError(err, "failed to form toSend")
	} else {
		//slog.Info("Client sending onions", "num_onions", numToSend)

		var wg sync.WaitGroup
		for _, onion := range toSend {
			wg.Add(1)
			go func(o structs.Onion) {
				defer wg.Done()
				if err = c.s.Send(0, c.ID, o.To, o); err != nil {
					slog.Error(fmt.Sprintf("failed to send onions (%d, %d, %d)", o.Layer, c.ID, o.To), err)
				}
			}(onion)
		}

		wg.Wait()
		return nil
	}
}

func (c *Client) Receive(o structs.Onion) error {
	if _, _, err := o.Peel(); err != nil {
		return pl.WrapError(err, "failed to peel onion")
	} else {
		//slog.Info("Client received message", "from", message.From, "to", message.To, "msg", message.Msg, "o", o2)
	}
	c.s.Receive(o.Layer, o.From, o.To)
	return nil
}
