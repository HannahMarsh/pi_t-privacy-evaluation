package node

import (
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/api/structs"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/interfaces"
	"golang.org/x/exp/slog"
)

// Node represents a node in the onion routing network
type Node struct {
	ID          int
	isCorrupted bool
	s           interfaces.System
}

// NewNode creates a new node
func NewNode(id int, isAdversary bool, s interfaces.System) (interfaces.Node, error) {
	n := &Node{
		ID:          id,
		isCorrupted: isAdversary,
		s:           s,
	}
	s.RegisterParty(n)
	return n, nil
}

func (n *Node) GetID() int {
	return n.ID
}

func (n *Node) StartRun(scenario int) {

}

func (n *Node) Receive(o structs.Onion) {
	if o.To != n.ID {
		pl.LogNewError("node.Receive(): onion not meant for this node")
		return
	}

	n.s.Receive(o.Layer, o.From, o.To)

	//slog.Debug("Received onion", "from", o.From, "layer", o.Layer)
	peeled, _, err := o.Peel()
	if err != nil {
		slog.Error("node.Receive(): failed to peel onion", err)
		return
	}

	n.s.Send(o.Layer, n.ID, peeled.To, peeled)
}
