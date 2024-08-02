package main

import (
	"fmt"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"math/rand"
	"sort"
	"strings"
)

type P struct {
	C int
	R int
	X float64
	x float64
	L int
}

type Node struct {
	Id           int
	Round        int
	ReceivedFrom []*Node
	SentTo       []*Node
	IsCorrupted  bool
	Probability  float64
}

func (n *Node) addSentTo(receiver *Node) {
	n.SentTo = append(n.SentTo, receiver)
}

func (n *Node) addReceivedFrom(sender *Node) {
	n.ReceivedFrom = append(n.ReceivedFrom, sender)
}

type Rounds struct {
	rounds map[int]map[int]*Node
	p      P
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
	r.get(0, path[0]).addSentTo(r.get(1, path[1]))

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

func createGraph(p P) *Rounds {
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

		system.establishPath(path)

		expectedToSend := ((float64(p.R) * p.x) / float64(p.C)) - 1.0
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
	initial[clientIds[0]] = 1.0

	// calculate probabilities
	for clientId, pr := range initial {
		client := system.get(0, clientId)
		client.Probability = pr

	}

	// How to calculate probabilitities? do we do breadth first? linear programming?

	//	recurse := func(node *Node) {
	//
	//		if node.Probability > 0 && len(node.SentTo) > 0 {
	//			receivers := utils.RemoveDuplicates(node.SentTo)
	//			totalNumSent := len(receivers)
	//			for _, receiver := range receivers {
	//				numSent := utils.Count(node.SentTo, receiver)
	//				newPr := pr * (float64(numSent) / float64(totalNumSent))
	//				receiver.Probability += newPr
	//			}
	//		}
	//	}
	//
	//	recurse(client)
	//}
	//
	//
	//
	//layer2s := utils.RemoveDuplicates()utils.FlatMap(system.getNodes(0), func(n *Node) []*Node {
	//	return n.SentTo
	//})
	//
	//for round := 0; round <= p.L; round++ {
	//	for _, node := range system.getNodes(round) {
	//		if node.Probability > 0.0 {
	//			node.SentTo
	//		}
	//	}
	//}

	// Propagate probabilities through the network
	for round := 0; round <= system.p.L; round++ {
		nodes := system.getNodes(round)
		for _, node := range nodes {
			if node.Probability == 0 {
				continue // Skip nodes with zero probability
			}

			if node.IsCorrupted && round < system.p.L {
				// Corrupted node: pass probability directly to the next round's corresponding node
				nextNode := system.get(round+1, node.Id)
				nextNode.Probability += node.Probability
			} else {
				// Distribute probability to all nodes in the next round
				nextNodes := system.getNodes(round + 1)
				for _, nextNode := range nextNodes {
					if !nextNode.IsCorrupted {
						nextNode.Probability += node.Probability / float64(len(nextNodes))
					}
				}
			}
		}
	}

	getName := func(id int) string {
		name := fmt.Sprintf("N%d", id-system.p.C)
		if id < system.p.C {
			name = fmt.Sprintf("C%d", id)
		}
		return name
	}

	// Display probabilities for demonstration
	for round := 0; round <= system.p.L+1; round++ {
		nodes := system.getNodes(round)
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

	return system
}

func main() {

	p := P{
		C: 10,
		R: 10,
		X: 0.5,
		x: 4.0,
		L: 4,
	}
	createGraph(p)
}
