package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/cmd/adversary/adversary"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"golang.org/x/exp/slog"
	"math/rand"
	"sort"
	"strings"
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
	n.SentTo = append(n.SentTo, receiver)
	//}
}

func (n *Node) addReceivedFrom(sender *Node) {
	//if sender.Id != 2 {
	n.ReceivedFrom = append(n.ReceivedFrom, sender)
	//}
}

type Rounds struct {
	rounds map[int]map[int]*Node
	p      adversary.P
}

func (r *Rounds) add(n *Node) {
	if r.rounds == nil {
		r.rounds = make(map[int]map[int]*Node)
	}
	if nodes, present := r.rounds[n.Round]; !present || nodes == nil {
		r.rounds[n.Round] = make(map[int]*Node)
	}
	r.rounds[n.Round][n.Id] = n
}

func (r *Rounds) get(round int, id int) *Node {
	if r.rounds == nil {
		r.rounds = make(map[int]map[int]*Node)
	}
	if nodes, present := r.rounds[round]; !present || nodes == nil {
		r.rounds[round] = make(map[int]*Node)
	}

	return r.rounds[round][id]
}

func (r *Rounds) getNodes(round int) []*Node {
	if r.rounds == nil {
		r.rounds = make(map[int]map[int]*Node)
	}
	if nodes, present := r.rounds[round]; !present || nodes == nil {
		r.rounds[round] = make(map[int]*Node)
	}
	return utils.GetValues(r.rounds[round])
}

func (r *Rounds) establishPath(path []int) {
	//r.get(0, path[0]).addSentTo(r.get(1, path[1]))

	if path[0] == 1 {
		return
	}

	// remove any corrupted node (since no mixing happens, we can think of onions as going straight through corrupted hops directly to the next destination)
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

		path := append(append([]int{clientId}, utils.Map(utils.NewIntArray(0, p.L), func(_ int) int {
			return utils.RandomElement(relayIds)
		})...), utils.RandomElement(clientIds))

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

		for i := 1; i <= p.L; i++ {
			if rand.Float64() <= pr {
				// create checkpoint onion
				path2 := append(append([]int{clientId}, utils.Map(utils.NewIntArray(0, p.L), func(_ int) int {
					return utils.RandomElement(relayIds)
				})...), utils.RandomElement(clientIds))

				path2[i] = path[i]

				system.establishPath(path2)
			}
		}
	}

	initial := make(map[int]float64)
	for _, clientId := range clientIds {
		initial[clientId] = 0.0
	}
	initial[clientIds[1]] = 1.0

	//// calculate probabilities
	//for clientId, pr := range initial {
	//	client := system.get(0, clientId)
	//	client.Probability = pr
	//
	//}

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
				nextNode.Probability += node.Probability / float64(len(node.SentTo))
			}
			// Distribute probability to all nodes in the next round
			nextNodes := system.getNodes(round + 1)
			for _, nextNode := range nextNodes {
				if !nextNode.IsCorrupted {
					nextNode.Probability += node.Probability / float64(len(nextNodes))
				}
			}
		}
	}

	//length0 := howManyPaths2(system.get(0, clientIds[1]), system.get(p.L+1, clientIds[len(clientIds)-2]), p.L+1)
	//length1 := howManyPaths2(system.get(0, clientIds[1]), system.get(p.L+1, clientIds[len(clientIds)-1]), p.L+1)
	//
	//system.get(p.L+1, clientIds[len(clientIds)-2]).Probability = float64(length0) / utils.Max(0.0000000000001, float64(length0+length1))
	//system.get(p.L+1, clientIds[len(clientIds)-1]).Probability = float64(length1) / utils.Max(0.0000000000001, float64(length0+length1))
	//
	//return system

	//// Propagate probabilities through the network
	//for round := 0; round <= system.p.L; round++ {
	//	nodes := system.getNodes(round)
	//	for _, node := range nodes {
	//		if node.Probability == 0 {
	//			continue // Skip nodes with zero probability
	//		}
	//		// Distribute probability to all nodes in the next round
	//		nextNodes := system.getNodes(round + 1)
	//		for _, nextNode := range nextNodes {
	//			if !nextNode.IsCorrupted {
	//				nextNode.Probability += node.Probability / float64(len(nextNodes))
	//			}
	//		}
	//	}
	//}
	//
	return system
}

// propagateProbabilityFromNode propagates probabilities from a specific node, bypassing corrupted nodes
func propagateProbabilityFromNode(node *Node) {
	//var wg sync.WaitGroup

	for _, nextNode := range utils.RemoveDuplicates(node.SentTo) {

		count := utils.Count(node.SentTo, nextNode)

		//node.mu.RLock()
		if node.Probability > 0 && len(node.SentTo) > 0 {
			//node.mu.RUnlock()
			//wg.Add(1)
			// Calculate probability for non-corrupted next nodes
			//go func(nextNode, node *Node) {
			//defer wg.Done()
			//nextNode.mu.Lock()
			nextNode.Probability += node.Probability * (float64(count) / float64(len(node.SentTo)))
			//nextNode.mu.Unlock()
			//node.mu.RUnlock()
			propagateProbabilityFromNode(nextNode)
			//}(nextNode, node)
		}
	}
	//wg.Wait()
}

func howManyPaths2(node1, node2 *Node, length int) float64 {

	var cache map[*Node]float64

	var howManyPaths func(node1, node2 *Node, length int) float64

	howManyPaths = func(node1, node2 *Node, length int) float64 {
		if cache == nil {
			cache = make(map[*Node]float64)
		}
		if n, present := cache[node1]; present {
			return n
		}

		// Only return 1 if node1 is already node2, otherwise return 0
		if node1 == node2 {
			return 1
		}
		if length == 0 {
			return 0
		}

		paths := float64(0)
		for _, nextNode := range node1.SentTo {
			if nextNode == node2 {
				paths += 0.00000000000000000001
			} else {
				paths += howManyPaths(nextNode, node2, length-1)
			}
		}
		cache[node1] = paths
		return paths
	}

	return howManyPaths(node1, node2, length)
}

func (r *Rounds) getProb0() float64 {
	return r.get(r.p.L+1, r.p.C-1).Probability
}

func (r *Rounds) getProb1() float64 {
	return r.get(r.p.L+1, r.p.C-2).Probability
}

func (r *Rounds) getRatio() float64 {
	if r.getProb1() == 0 {
		return 1.0
	}
	return r.getProb0() / r.getProb1()
}

func (r *Rounds) displayProbs() {
	getName := func(id int) string {
		name := fmt.Sprintf("N%d", id-r.p.C)
		if id < r.p.C {
			name = fmt.Sprintf("C%d", id)
		}
		return name
	}

	// Display probabilities for demonstration
	for round := 0; round <= r.p.L+1; round++ {
		nodes := r.getNodes(round)
		utils.Sort(nodes, func(a, b *Node) bool {
			return a.Id < b.Id
		})
		fmt.Printf("Round %d:\n", round)
		for _, node := range nodes {
			name := getName(node.Id)
			corruptedStr := "(Corrupted)"
			if !node.IsCorrupted {
				corruptedStr = "           "
			}
			sentTo := utils.Map(node.SentTo, func(n *Node) string {
				return getName(n.Id)
			})
			sort.Strings(sentTo)
			fmt.Printf("%s %s - Probability: %.4f\t sentTo: [%s]\n", name, corruptedStr, node.Probability, strings.Join(sentTo, ", "))
		}
		fmt.Println()
	}
}

//func run(p adversary.P, numRUns int) *adversary.V {
//	P0 := make([]float64, numRUns)
//	P1 := make([]float64, numRUns)
//	ratios := make([]float64, numRUns)
//
//	for i := 0; i < numRUns; i++ {
//		system := createGraph(p)
//		P0[i] = system.getProb0()
//		P1[i] = system.getProb1()
//		ratios[i] = system.getRatio()
//	}
//	return &adversary.V{
//		P:      p,
//		Pr0:    P0,
//		Pr1:    P1,
//		Ratios: ratios,
//	}
//
//}

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

	C := flag.Int("C", 250, "Number of clients")
	R := flag.Int("R", 50, "Number of relays")
	X := flag.Float64("X", 0.9, "Fraction of corrupted relays")
	serverLoad := flag.Float64("serverLoad", 60.0, "Server load, i.e. the expected number of onions processed per node per node")
	L := flag.Int("L", 75, "Number of rounds")
	numRuns := flag.Int("numRuns", 1, "Number of runs")

	flag.Parse()

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
