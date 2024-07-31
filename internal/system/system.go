package system

import (
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/api/structs"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/interfaces"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"sync"
)

type NoMixing struct {
	NodeId int
	Onions map[int][]*Transaction
	mu     sync.RWMutex
}

func NewNoMixing(nodeId int) *NoMixing {
	return &NoMixing{
		NodeId: nodeId,
		Onions: make(map[int][]*Transaction),
	}
}

func (n *NoMixing) AddTransaction(layer int, t *Transaction) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		n.mu.Lock()
		defer n.mu.Unlock()
		if _, exists := n.Onions[layer]; !exists {
			n.Onions[layer] = make([]*Transaction, 0)
		}
		n.Onions[layer] = append(n.Onions[layer], t)
	}()
	wg.Wait()
}

type Transaction struct {
	Processor int
	LastHop   int
	NextHop   int
}

type System struct {
	allParties     map[int]interfaces.Node
	nodes          []int
	clients        []int
	p              interfaces.Params
	mu             sync.RWMutex
	sent           [][][]int // sent[i][j][k] = number of onions sent during round i from node j to node k
	received       [][][]int // received[i][j][k]number of onions received during round i from node j to node k
	corruptedViews map[int]*NoMixing
}

func NewSystem(p interfaces.Params) interfaces.System {
	sent := make([][][]int, p.L+2)
	received := make([][][]int, p.L+2)
	for i := range sent {
		sent[i] = make([][]int, p.R+p.N+1)
		received[i] = make([][]int, p.R+p.N+1)
		for j := range sent[i] {
			sent[i][j] = make([]int, p.R+p.N+1)
			received[i][j] = make([]int, p.R+p.N+1)
		}
	}
	return &System{
		allParties: make(map[int]interfaces.Node),
		p:          p,
		nodes:      utils.NewIntArray(p.R+1, p.R+p.N+1),
		clients:    utils.NewIntArray(1, p.R+1),
		sent:       sent,
		received:   received,
	}
}

func (s *System) Receive(layer, from, to int) {
	var wg sync.WaitGroup
	wg.Add(1) // make the lock reentrant
	go func() {
		defer wg.Done()
		s.mu.Lock()
		defer s.mu.Unlock()
		s.received[layer][from][to]++
	}()
	wg.Wait()
}

func (s *System) GetClients() []int {
	// Implementation of the GetClients method
	return s.clients
}

func (s *System) GetNodes() []int {
	// Implementation of the GetNodes method
	return s.nodes
}

func (s *System) RegisterParty(n interfaces.Node) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.allParties[n.GetID()] = n
}

func (s *System) Send(layer, from, to int, o structs.Onion) error {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.mu.Lock()
		defer s.mu.Unlock()
		s.sent[layer][from][to] = s.sent[layer][from][to] + 1
	}()
	wg.Wait()
	node := s.getNode(to)
	if node == nil {
		return pl.NewError("node with id %d not found", to)
	}
	return node.Receive(o)
}

func (s *System) GetParams() interfaces.Params {
	return s.p
}

func (s *System) StartRun() error {
	s.mu.Lock()
	for i := range s.sent {
		for j := range s.sent[i] {
			for k := range s.sent[i][j] {
				s.sent[i][j][k] = 0
				s.received[i][j][k] = 0
			}
		}
	}
	s.mu.Unlock()

	if len(s.GetClients()) != s.p.R {
		return pl.NewError("number of clients does not match R")
	}
	if len(s.GetNodes()) != s.p.N {
		return pl.NewError("number of nodes does not match N")
	}
	var err error
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, node := range s.allParties {
		wg.Add(1)
		go func(n interfaces.Node) {
			defer wg.Done()
			if err2 := n.StartRun(s.p.Scenario); err2 != nil {
				pl.LogNewError("failed to start run", err2)
				mu.Lock()
				defer mu.Unlock()
				err = err2
			}
		}(node)
	}
	wg.Wait()
	return err
	//slog.Info("All nodes have started their runs")
}

func (s *System) getNode(id int) interfaces.Node {
	if node, exists := s.allParties[id]; exists {
		return node
	}
	return nil
}

func (s *System) GetNumOnionsReceived(id int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for j := range s.received[s.p.L+1] {
		for k := range s.received[s.p.L+1][j] {
			if k == id {
				count += s.received[s.p.L+1][j][k]
			}
		}
	}
	return count
}

func (s *System) GetProbabilities(senderOfMessage int) []float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	probabilities := make([][]float64, s.GetParams().L+2)
	for i := range probabilities {
		probabilities[i] = make([]float64, s.p.R+s.p.N+1)
	}
	probabilities[0][senderOfMessage] = 1.0

	for round := 0; round <= s.GetParams().L; round++ {
		for from := range s.sent[round] {
			if prOnionAtNodeThisRound := probabilities[round][from]; prOnionAtNodeThisRound > 0 {
				totalOnionsSent := float64(utils.Sum(s.sent[round][from]))
				if totalOnionsSent > 0 {
					for to := range s.sent[round][from] {
						if s.sent[round][from][to] > 0 {
							probabilityAtNextHop := float64(s.sent[round][from][to]) / totalOnionsSent
							probabilities[round+1][to] = probabilities[round+1][to] + (prOnionAtNodeThisRound * probabilityAtNextHop)
						}
					}
				}
			}
		}
	}
	return utils.GetLast(probabilities)
}

func (s *System) GetReverseProbabilities(receiverOfMessage int) []float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	probabilities := make([][]float64, s.GetParams().L+2)
	for i := range probabilities {
		probabilities[i] = make([]float64, s.p.R+s.p.N+1)
	}
	probabilities[s.GetParams().L+1][receiverOfMessage] = 1.0

	for round := s.GetParams().L + 1; round > 0; round-- {
		for from := range s.received[round] {
			if prOnionAtNodeThisRound := probabilities[round][from]; prOnionAtNodeThisRound > 0 {
				totalOnionsSent := float64(utils.Sum(s.received[round][from]))
				if totalOnionsSent > 0 {
					for to := range s.sent[round][from] {
						if s.received[round][from][to] > 0 {
							probabilityAtNextHop := float64(s.received[round][from][to]) / totalOnionsSent
							probabilities[round-1][to] = probabilities[round-1][to] + (prOnionAtNodeThisRound * probabilityAtNextHop)
						}
					}
				}
			}
		}
	}
	return utils.GetLast(probabilities)
}
