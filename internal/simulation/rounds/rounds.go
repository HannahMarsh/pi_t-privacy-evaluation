package rounds

import (
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/data"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/simulation/rounds/node"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"math/rand"
	"sync"
)

type Rounds struct {
	rounds map[int]map[int]*node.Node
	P      data.Parameters
	mu     sync.RWMutex
}

// helper function to sample from a binomial distribution
func sampleBinomial(expectedValue int) int {
	value := 0
	for i := 0; i < expectedValue*2; i++ {
		if rand.Float64() <= 0.5 {
			value++
		}
	}
	return value
}

func SetUpSystem(clientIds, relayIds []int, p data.Parameters) *Rounds {
	var system = &Rounds{
		P: p,
	}

	system.addClients(clientIds)

	system.addRelays(relayIds)

	system.EstablishPaths(clientIds, relayIds)

	return system

}

func (r *Rounds) generatePath(sender, receiver int, relayIds []int) {
	path := make([]int, r.P.L+2)

	path[0] = sender
	path[r.P.L+1] = receiver

	for i := 1; i <= r.P.L; i++ {
		path[i] = utils.RandomElement(relayIds)
	}

	r.EstablishPath(path)
}

func (r *Rounds) EstablishPaths(clientIds, relayIds []int) {
	messageDestinations := make(map[int]int)

	for i, receiver := range utils.GetShuffledCopy(clientIds[:len(clientIds)-2]) {
		messageDestinations[clientIds[i+2]] = receiver
	}

	messageDestinations[clientIds[0]] = clientIds[len(clientIds)-1]
	messageDestinations[clientIds[1]] = clientIds[len(clientIds)-2]

	expectedToSend := int(((float64(r.P.R) * r.P.ServerLoad) / float64(r.P.C)) - 1.0)

	for sender, receiver := range messageDestinations {
		r.generatePath(sender, receiver, relayIds)

		// create checkpoint onion
		numToSend := sampleBinomial(expectedToSend)

		receivers := make([]int, numToSend)
		for i := 0; i < numToSend; i++ {
			receivers[i] = utils.RandomElement(clientIds)
		}

		for _, checkPointReceiver := range receivers {
			r.generatePath(sender, checkPointReceiver, relayIds)
		}
	}
}

func (r *Rounds) addRelays(relayIds []int) {
	numCorrupted := int(r.P.X * float64(r.P.R))
	corrupted := utils.RandomSubset(relayIds, numCorrupted)
	isCorrupted := make(map[int]bool)
	for _, c := range relayIds {
		isCorrupted[c] = false
	}
	for _, c := range corrupted {
		isCorrupted[c] = true
	}

	for round := 1; round <= r.P.L; round++ {
		for _, relayId := range relayIds {
			r.Add(&node.Node{
				Id:           relayId,
				Round:        round,
				ReceivedFrom: make([]*node.Node, 0),
				SentTo:       make([]*node.Node, 0),
				IsCorrupted:  isCorrupted[relayId],
				Probability:  0.0,
			})
		}
	}
}

func (r *Rounds) addClients(clientIds []int) {
	for _, clientId := range clientIds {
		clientSender := &node.Node{
			Id:           clientId,
			Round:        0,
			ReceivedFrom: make([]*node.Node, 0),
			SentTo:       make([]*node.Node, 0),
			IsCorrupted:  false,
			Probability:  0.0,
		}

		clientReceiver := &node.Node{
			Id:           clientId,
			Round:        r.P.L + 1,
			ReceivedFrom: make([]*node.Node, 0),
			SentTo:       make([]*node.Node, 0),
			IsCorrupted:  false,
			Probability:  0.0,
		}

		r.Add(clientSender)
		r.Add(clientReceiver)
	}
}

func (r *Rounds) Add(n *node.Node) {
	r.mu.RLock()
	if r.rounds == nil {
		r.mu.RUnlock()
		r.mu.Lock()
		if r.rounds == nil {
			r.rounds = make(map[int]map[int]*node.Node)
		}
		r.mu.Unlock()
		r.mu.RLock()
	}
	if nodes, present := r.rounds[n.Round]; !present || nodes == nil {
		r.mu.RUnlock()
		r.mu.Lock()
		if nodes, present = r.rounds[n.Round]; !present || nodes == nil {
			r.rounds[n.Round] = make(map[int]*node.Node)
		}
		r.mu.Unlock()
		r.mu.RLock()
	}
	r.mu.RUnlock()
	r.mu.Lock()
	r.rounds[n.Round][n.Id] = n
	r.mu.Unlock()
}

func (r *Rounds) Get(round int, id int) *node.Node {
	r.mu.RLock()
	if r.rounds == nil {
		r.mu.RUnlock()
		r.mu.Lock()
		if r.rounds == nil {
			r.rounds = make(map[int]map[int]*node.Node)
		}
		r.mu.Unlock()
		r.mu.RLock()
	}
	if nodes, present := r.rounds[round]; !present || nodes == nil {
		r.mu.RUnlock()
		r.mu.Lock()
		if nodes, present = r.rounds[round]; !present || nodes == nil {
			r.rounds[round] = make(map[int]*node.Node)
		}
		r.mu.Unlock()
		r.mu.RLock()
	}
	defer r.mu.RLock()
	if n, present := r.rounds[round][id]; !present {
		return nil
	} else {
		return n
	}
}

func (r *Rounds) GetNodes(round int) []*node.Node {
	r.mu.RLock()
	if r.rounds == nil {
		r.mu.RUnlock()
		r.mu.Lock()
		if r.rounds == nil {
			r.rounds = make(map[int]map[int]*node.Node)
		}
		r.mu.Unlock()
		r.mu.RLock()
	}
	if nodes, present := r.rounds[round]; !present || nodes == nil {
		r.mu.RUnlock()
		r.mu.Lock()
		if nodes, present = r.rounds[round]; !present || nodes == nil {
			r.rounds[round] = make(map[int]*node.Node)
		}
		r.mu.Unlock()
		r.mu.RLock()
	}
	defer r.mu.RLock()
	return utils.GetValues(r.rounds[round])
}

func (r *Rounds) EstablishPath(path []int) {

	if path[0] == 1 {
		return
	}

	nodes := utils.Map(utils.NewIntArray(0, len(path)), func(l int) *node.Node {
		return r.Get(l, path[l])
	})

	for _, hop := range nodes {
		if !hop.IsCorrupted {
			l := hop.Round

			if l-1 > 0 {
				last := r.Get(l-1, path[l-1])
				// find last non-corrupted relay
				for l_ := l - 1; last.IsCorrupted; l_-- {
					last = r.Get(l_, path[l_])
				}
				hop.AddReceivedFrom(last)
			}
			if l < len(path)-1 {
				hop.AddSentTo(r.Get(l+1, path[l+1]))

				next := r.Get(l+1, path[l+1])
				// find next non-corrupted relay
				for l_ := l + 1; next.IsCorrupted; l_++ {
					next = r.Get(l_, path[l_])
				}
				hop.AddSentTo(next)
			}
		}

	}
}

func (r *Rounds) GetProb0() float64 {
	return r.Get(r.P.L+1, r.P.C-1).Probability
}

func (r *Rounds) GetProb1() float64 {
	return r.Get(r.P.L+1, r.P.C).Probability
}

func (r *Rounds) GetRatio() float64 {
	if r.GetProb1() == 0 {
		if r.GetProb0() == 0 {
			return 1.0
		}
		return 1000.0
	}
	return r.GetProb0() / r.GetProb1()
}

func (r *Rounds) CalculateProbabilities(initial map[int]float64) {
	// Calculate probabilities using actual relay paths
	for clientId, pr := range initial {
		client := r.Get(0, clientId)
		client.Probability = pr
	}

	for round := 0; round <= r.P.L; round++ {
		nodes := r.GetNodes(round)
		for _, n := range nodes {
			if n.Probability == 0 {
				continue // Skip nodes with zero probability
			}
			for _, nextNode := range n.SentTo {
				if nextNode.Probability+n.Probability/float64(len(n.SentTo)) > 1.000000001 {
					pl.LogNewError("Probability %f exceeds 1.0", nextNode.Probability+n.Probability/float64(len(n.SentTo)))
				}
				nextNode.Probability += n.Probability / float64(len(n.SentTo))
			}
		}
	}
}
