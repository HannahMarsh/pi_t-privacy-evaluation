package node

import "sync"

type Node struct {
	Id           int
	Round        int
	ReceivedFrom []*Node
	SentTo       []*Node
	IsCorrupted  bool
	Probability  float64
	mu           sync.RWMutex
}

func (n *Node) AddSentTo(receiver *Node) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.SentTo = append(n.SentTo, receiver)
}

func (n *Node) AddReceivedFrom(sender *Node) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.ReceivedFrom = append(n.ReceivedFrom, sender)
}
