package node

import (
	"fmt"
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/api/structs"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/interfaces"
)

// Node represents a node in the onion routing network
type Node struct {
	ID          int
	isAdversary bool
	s           interfaces.System
}

// NewNode creates a new node
func NewNode(id int, isAdversary bool, s interfaces.System) (interfaces.Node, error) {
	n := &Node{
		ID:          id,
		isAdversary: isAdversary,
		s:           s,
	}
	s.RegisterParty(n)
	return n, nil
}

func (n *Node) GetID() int {
	return n.ID
}

func (n *Node) StartRun(scenario int) error {
	return nil
}

func (n *Node) Receive(o structs.Onion) error {
	if o.To != n.ID {
		return pl.NewError("node.Receive(): onion not meant for this node")
	}

	n.s.Receive(o.Layer, o.From, o.To)

	//slog.Debug("Received onion", "from", o.From, "layer", o.Layer)
	peeled, _, err := o.Peel()
	if err != nil {
		return pl.WrapError(err, "node.Receive(): failed to peel onion")
	}

	if n.isAdversary && o.From == 1 { //} || utils.ContainsElement(alwaysDropFrom, peeled.To)) {
		//slog.Info("Dropping onion", "from", o.From, "to", o.To)
		return nil
	}

	if err = n.s.Send(o.Layer, n.ID, peeled.To, peeled); err != nil {
		return pl.WrapError(err, "node.Receive(): "+fmt.Sprintf("failed to send onions (%d, %d, %d)", o.Layer, n.ID, peeled.To))
	}
	return nil
}
