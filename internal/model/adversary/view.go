package adversary

import (
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/config"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/api/structs"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"strings"
)

type View struct {
	Round        int
	Party        string
	ID           int
	NetworkId    int
	IsNode       bool
	ReceivedFrom []string
	SentTo       []string
}

type Views struct {
	Rounds    map[int][]View
	Parties   map[string][]View
	PartiesId map[int]map[int]View
}

func CollectViews(Nodes map[string]structs.NodeStatus, Clients map[string]structs.ClientStatus) *Views {

	views := Views{
		Rounds:    make(map[int][]View),
		Parties:   make(map[string][]View),
		PartiesId: make(map[int]map[int]View),
	}

	for i := 0; i <= config.GlobalConfig.L+1; i++ {
		views.Rounds[i] = make([]View, 0)
	}
	for _, status := range Clients {
		id := status.Client.ID
		address := status.Client.Address
		v := View{
			Round:        config.GlobalConfig.L + 1,
			Party:        address,
			ID:           id,
			NetworkId:    addressToId(address),
			IsNode:       false,
			ReceivedFrom: make([]string, 0),
			SentTo:       make([]string, 0),
		}
		for _, msg := range status.MessagesReceived {
			v.ReceivedFrom = append(v.ReceivedFrom, msg.Message.From)
		}
		views.Rounds[config.GlobalConfig.L+1] = append(views.Rounds[config.GlobalConfig.L+1], v)
		v = View{
			Round:        0,
			Party:        address,
			ID:           id,
			NetworkId:    addressToId(address),
			IsNode:       false,
			ReceivedFrom: make([]string, 0),
			SentTo:       make([]string, 0),
		}
		for _, msg := range status.MessagesSent {
			v.SentTo = append(v.SentTo, msg.RoutingPath[0].Address)
		}
		views.Rounds[0] = append(views.Rounds[0], v)
	}

	for _, status := range Nodes {
		id := status.Node.ID
		address := status.Node.Address
		rounds := make([]View, 0)
		for i := 1; i <= config.GlobalConfig.L; i++ {
			rounds = append(rounds, View{
				Round:        i,
				Party:        address,
				ID:           id,
				NetworkId:    addressToId(address),
				IsNode:       true,
				ReceivedFrom: make([]string, 0),
				SentTo:       make([]string, 0),
			})
		}
		for _, o := range status.Received {
			rounds[o.Layer].ReceivedFrom = append(rounds[o.Layer].ReceivedFrom, o.LastHop)
			rounds[o.Layer].SentTo = append(rounds[o.Layer].SentTo, o.NextHop)
		}
		for _, v := range rounds {
			views.Rounds[v.Round] = append(views.Rounds[v.Round], v)
		}
	}
	allValues := utils.GetValues(views.Rounds)
	allViews := utils.Flatten(allValues)

	for _, v := range allViews {
		if views.Parties[v.Party] == nil {
			views.Parties[v.Party] = make([]View, 0)
		}
		views.Parties[v.Party] = append(views.Parties[v.Party], v)
		if views.PartiesId[v.NetworkId] == nil {
			views.PartiesId[v.NetworkId] = make(map[int]View, 0)
		}
		views.PartiesId[v.NetworkId][v.Round] = v
	}
	return &views
}

func (v *Views) GetNumOnionsReceived(networkId int) []int {
	numOnionsReceived := make([]int, len(v.Rounds)+1)

	for _, view := range v.PartiesId[networkId] {
		numOnionsReceived[view.Round] = len(view.ReceivedFrom)
	}
	return numOnionsReceived

}

func (v *Views) WhoDidTheySendTo(networkId int, round int) []int {
	roundView := v.Rounds[round]
	for _, view := range roundView {
		if view.NetworkId == networkId {
			return utils.Map(view.SentTo, addressToId)
		}
	}
	return nil
}

func (v *Views) GetProbabilities() [][]float64 {
	probabilities := make([][]float64, len(v.Rounds))
	senderOfMessage := 2
	for i := range probabilities {
		probabilities[i] = make([]float64, len(config.GlobalConfig.Nodes)+len(config.GlobalConfig.Clients)+1)
	}

	probabilities[0][senderOfMessage] = 1.0

	for round := 0; round < len(v.Rounds)-1; round++ {
		for node := range probabilities[round][1:] {
			prOnionAtNodeThisRound := probabilities[round][node]
			if prOnionAtNodeThisRound > 0 {
				onionsSent := v.WhoDidTheySendTo(node, round)
				if onionsSent != nil {
					receivingNodes := utils.RemoveDuplicates(onionsSent)
					totalOnionsSent := float64(len(onionsSent))
					for _, nextHop := range receivingNodes {
						numberOfOnionsSentToNextHop := float64(utils.Count(onionsSent, nextHop))
						probabilityAtNextHop := numberOfOnionsSentToNextHop / totalOnionsSent
						probabilities[round+1][nextHop] = probabilities[round+1][nextHop] + (prOnionAtNodeThisRound * probabilityAtNextHop)
					}
				}
			}
		}
	}
	return probabilities
}

func addressToId(addr string) int {
	if node := utils.Find(config.GlobalConfig.Nodes, func(node config.Node) bool {
		return strings.Contains(node.Address, addr) || strings.Contains(addr, node.Address)
	}); node != nil {
		return node.ID + len(config.GlobalConfig.Clients)
	} else if client := utils.Find(config.GlobalConfig.Clients, func(client config.Client) bool {
		return strings.Contains(client.Address, addr) || strings.Contains(addr, client.Address)
	}); client != nil {
		return client.ID
	} else {
		pl.LogNewError("addressToId(): address not found %s", addr)
		return -1
	}
}
