package main

import (
	"encoding/json"
	"flag"
	"fmt"
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/cmd/adversary/adversary"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"golang.org/x/exp/slog"
	"math/rand"
	"os"
	"sort"
	"sync"
)

type Node struct {
	Id           int
	Round        int
	ReceivedFrom []*Node
	SentTo       []*Node
	IsCorrupted  bool
	Probability  float64
	mu           sync.RWMutex
}

func (n *Node) addSentTo(receiver *Node) {
	//if receiver.Id != 2 {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.SentTo = append(n.SentTo, receiver)
	//}
}

func (n *Node) addReceivedFrom(sender *Node) {
	//if sender.Id != 2 {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.ReceivedFrom = append(n.ReceivedFrom, sender)
	//}
}

type Rounds struct {
	rounds map[int]map[int]*Node
	p      adversary.P
	mu     sync.RWMutex
}

func (r *Rounds) add(n *Node) {
	r.mu.RLock()
	if r.rounds == nil {
		r.mu.RUnlock()
		r.mu.Lock()
		if r.rounds == nil {
			r.rounds = make(map[int]map[int]*Node)
		}
		r.mu.Unlock()
		r.mu.RLock()
	}
	if nodes, present := r.rounds[n.Round]; !present || nodes == nil {
		r.mu.RUnlock()
		r.mu.Lock()
		if nodes, present = r.rounds[n.Round]; !present || nodes == nil {
			r.rounds[n.Round] = make(map[int]*Node)
		}
		r.mu.Unlock()
		r.mu.RLock()
	}
	r.mu.RUnlock()
	r.mu.Lock()
	r.rounds[n.Round][n.Id] = n
	r.mu.Unlock()
}

func (r *Rounds) get(round int, id int) *Node {
	r.mu.RLock()
	if r.rounds == nil {
		r.mu.RUnlock()
		r.mu.Lock()
		if r.rounds == nil {
			r.rounds = make(map[int]map[int]*Node)
		}
		r.mu.Unlock()
		r.mu.RLock()
	}
	if nodes, present := r.rounds[round]; !present || nodes == nil {
		r.mu.RUnlock()
		r.mu.Lock()
		if nodes, present = r.rounds[round]; !present || nodes == nil {
			r.rounds[round] = make(map[int]*Node)
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

func (r *Rounds) getNodes(round int) []*Node {
	r.mu.RLock()
	if r.rounds == nil {
		r.mu.RUnlock()
		r.mu.Lock()
		if r.rounds == nil {
			r.rounds = make(map[int]map[int]*Node)
		}
		r.mu.Unlock()
		r.mu.RLock()
	}
	if nodes, present := r.rounds[round]; !present || nodes == nil {
		r.mu.RUnlock()
		r.mu.Lock()
		if nodes, present = r.rounds[round]; !present || nodes == nil {
			r.rounds[round] = make(map[int]*Node)
		}
		r.mu.Unlock()
		r.mu.RLock()
	}
	defer r.mu.RLock()
	return utils.GetValues(r.rounds[round])
}

func (r *Rounds) establishPath(path []int) {
	//r.get(0, path[0]).addSentTo(r.get(1, path[1]))

	if path[0] == 1 {
		return
	}

	nodes := utils.Map(utils.NewIntArray(0, len(path)), func(l int) *Node {
		return r.get(l, path[l])
	})

	for _, hop := range nodes {
		if !hop.IsCorrupted {
			l := hop.Round

			if l-1 > 0 {
				last := r.get(l-1, path[l-1])
				// find last non-corrupted node
				for l_ := l - 1; last.IsCorrupted; l_-- {
					last = r.get(l_, path[l_])
				}
				hop.addReceivedFrom(last)
			}
			if l < len(path)-1 {
				hop.addSentTo(r.get(l+1, path[l+1]))

				next := r.get(l+1, path[l+1])
				// find next non-corrupted node
				for l_ := l + 1; next.IsCorrupted; l_++ {
					next = r.get(l_, path[l_])
				}
				hop.addSentTo(next)
			}
		}

	}
}

func createGraph(p adversary.P) *Rounds {
	clientIds := utils.NewIntArray(1, p.C+1)
	relayIds := utils.NewIntArray(p.C+1, p.C+1+p.R)

	var system *Rounds = &Rounds{
		p: p,
	}

	for _, clientId := range clientIds {
		clientSender := &Node{
			Id:           clientId,
			Round:        0,
			ReceivedFrom: make([]*Node, 0),
			SentTo:       make([]*Node, 0),
			IsCorrupted:  false,
			Probability:  0.0,
		}

		clientReceiver := &Node{
			Id:           clientId,
			Round:        p.L + 1,
			ReceivedFrom: make([]*Node, 0),
			SentTo:       make([]*Node, 0),
			IsCorrupted:  false,
			Probability:  0.0,
		}

		system.add(clientSender)
		system.add(clientReceiver)
	}

	numCorrupted := int(p.X * float64(p.R))
	corrupted := utils.RandomSubset(relayIds, numCorrupted)
	isCorrupted := make(map[int]bool)
	for _, c := range relayIds {
		isCorrupted[c] = false
	}
	for _, c := range corrupted {
		isCorrupted[c] = true
	}

	for round := 1; round <= p.L; round++ {
		for _, relayId := range relayIds {
			system.add(&Node{
				Id:           relayId,
				Round:        round,
				ReceivedFrom: make([]*Node, 0),
				SentTo:       make([]*Node, 0),
				IsCorrupted:  isCorrupted[relayId],
				Probability:  0.0,
			})
		}
	}

	for _, clientId := range clientIds {

		path := make([]int, p.L+2)

		path[0] = clientId
		for i := 1; i <= p.L; i++ {
			path[i] = utils.RandomElement(relayIds)
		}
		path[p.L+1] = utils.RandomElement(clientIds)

		if clientId == clientIds[0] {
			path[len(path)-1] = clientIds[len(clientIds)-1]
		}
		if clientId == clientIds[1] {
			path[len(path)-1] = clientIds[len(clientIds)-2]
		}

		if clientId == clientIds[len(clientIds)-1] {
			path[len(path)-1] = clientIds[0]
		}
		if clientId == clientIds[len(clientIds)-2] {
			path[len(path)-1] = clientIds[1]
		}

		system.establishPath(path)

		expectedToSend := ((float64(p.R) * p.ServerLoad) / float64(p.C)) - 1.0
		pr := expectedToSend / float64(p.L)

		var wg sync.WaitGroup
		for i := 1; i <= p.L; i++ {
			if rand.Float64() <= pr {
				wg.Add(1)
				go func(i2 int) {
					defer wg.Done()
					// create checkpoint onion
					path2 := make([]int, p.L+2)

					path2[0] = clientId
					for j := 1; j <= p.L; j++ {
						path2[j] = utils.RandomElement(relayIds)
					}
					path2[p.L+1] = utils.RandomElement(clientIds)

					path2[i2] = path[i2]

					if system.get(len(path2)-1, path2[len(path2)-1]) == nil {
						ids := utils.RemoveDuplicates(utils.Map(system.getNodes(len(path2)-1), func(nn *Node) int { return nn.Id }))
						sort.Ints(ids)
						lookingFor := path2[len(path2)-1]
						if !utils.ContainsElement(clientIds, lookingFor) {
							pl.LogNewError("Client not found")
						}
						pl.LogNewError("Node not found")
					}

					system.establishPath(path2)
				}(i)
			}
		}
		wg.Wait()

	}

	//slog.Info("here")

	initial := make(map[int]float64)
	for _, clientId := range clientIds {
		initial[clientId] = 0.0
	}
	initial[clientIds[1]] = 1.0

	// Calculate probabilities using actual node paths
	for clientId, pr := range initial {
		client := system.get(0, clientId)
		client.Probability = pr
		//propagateProbabilityFromNode(client)
	}

	for round := 0; round <= system.p.L; round++ {
		nodes := system.getNodes(round)
		for _, node := range nodes {
			if node.Probability == 0 {
				continue // Skip nodes with zero probability
			}

			for _, nextNode := range node.SentTo {

				if nextNode.Probability+node.Probability/float64(len(node.SentTo)) > 1.0 {
					pl.LogNewError("Probability %f exceeds 1.0", nextNode.Probability+node.Probability/float64(len(node.SentTo)))
				}
				nextNode.Probability += node.Probability / float64(len(node.SentTo))
			}
			//// Distribute probability to all nodes in the next round
			//nextNodes := system.getNodes(round + 1)
			//for _, nextNode := range nextNodes {
			//	if !nextNode.IsCorrupted {
			//		nextNode.Probability += node.Probability / float64(len(nextNodes))
			//	}
			//}
		}
	}
	return system
}

func (r *Rounds) getProb0() float64 {
	return r.get(r.p.L+1, r.p.C-1).Probability
}

func (r *Rounds) getProb1() float64 {
	return r.get(r.p.L+1, r.p.C).Probability
}

func (r *Rounds) getRatio() float64 {
	if r.getProb1() == 0 {
		return 1.0
	}
	return r.getProb0() / r.getProb1()
}

func run(p adversary.P, numRuns int) *adversary.V {
	P0 := make([]float64, numRuns)
	P1 := make([]float64, numRuns)
	ratios := make([]float64, numRuns)

	// Create a WaitGroup to synchronize goroutines
	var wg sync.WaitGroup

	// Use a channel to collect results from goroutines
	resultCh := make(chan struct {
		index int
		prob0 float64
		prob1 float64
		ratio float64
	}, numRuns)

	for i := 0; i < numRuns; i++ {
		wg.Add(1)

		// Start a goroutine for each simulation run
		go func(index int) {
			defer wg.Done()

			// Seed random number generator for this goroutine
			rand.Seed(int64(index))

			system := createGraph(p)
			prob0 := system.getProb0()
			prob1 := system.getProb1()
			ratio := system.getRatio()

			// Send the results back via the channel
			resultCh <- struct {
				index int
				prob0 float64
				prob1 float64
				ratio float64
			}{
				index: index,
				prob0: prob0,
				prob1: prob1,
				ratio: ratio,
			}
		}(i)
	}

	// Wait for all goroutines to finish
	wg.Wait()
	close(resultCh)

	// Collect results from the channel
	for result := range resultCh {
		P0[result.index] = result.prob0
		P1[result.index] = result.prob1
		ratios[result.index] = result.ratio
	}

	return &adversary.V{
		P:      p,
		Pr0:    P0,
		Pr1:    P1,
		Ratios: ratios,
	}
}

func main() {

	C := flag.Int("C", 2000, "Number of clients")
	R := flag.Int("R", 1000, "Number of relays")
	X := flag.Float64("X", 0.0, "Fraction of corrupted relays")
	serverLoad := flag.Float64("serverLoad", 150.0, "Server load, i.e. the expected number of onions processed per node per node")
	L := flag.Int("L", 100, "Number of rounds")
	numRuns := flag.Int("numRuns", 1, "Number of runs")

	flag.Parse()

	if int(*serverLoad) > int(float64(*L)*float64(*C)/float64(*R)) {
		pl.LogNewError(fmt.Sprintf("Server load %d too high. Reduce server load to %d.\n", int(*serverLoad), int(float64(*L)*float64(*C)/float64(*R))))
		os.Exit(1)
		return
	}

	// Debug: Print parsed values
	//fmt.Printf("Parsed flags: C=%d, R=%d, serverLoad=%.1f, X=%.1f, L=%d, numRuns=%d\n", *C, *R, *serverLoad, *X, *L, *numRuns)

	//slog.Info("", "numRuns", *numRuns)

	p := adversary.P{
		C:          *C,
		R:          *R,
		X:          *X,
		ServerLoad: *serverLoad,
		L:          *L,
	}
	v := run(p, *numRuns)

	str, err := json.Marshal(v)
	if err != nil {
		slog.Error("Couldnt marshall V.", err)
	} else {
		fmt.Println(string(str))
	}
}
