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
	NetworkId    int
	IsNode       bool
	ReceivedFrom []int
	SentTo       []int
}

type Views struct {
	Rounds map[int]map[int]*View
}

func (v *Views) addView(view *View) {
	if _, present := v.Rounds[view.Round]; !present {
		v.Rounds[view.Round] = make(map[int]*View)
	}
	v.Rounds[view.Round][view.NetworkId] = view
}

func (v *Views) getView(round int, networkId int) *View {
	if _, present := v.Rounds[round]; !present {
		return nil
	}
	m := v.Rounds[round]
	if _, present := m[networkId]; !present {
		return nil
	}
	return m[networkId]
	//return v.Rounds[round][networkId]
}

func (v *Views) getViewsForRound(round int) []*View {
	if _, present := v.Rounds[round]; !present {
		return nil
	}
	return utils.Filter(utils.GetValues(v.Rounds[round]), func(view *View) bool {
		return view != nil
	})
}

func (v *Views) getViewsForParty(networkId int) []*View {
	return utils.MapToPointerArray(v.Rounds, func(_ int, views map[int]*View) *View {
		if view, present := views[networkId]; present {
			return view
		}
		return nil
	})
}

func CollectViews(Nodes map[int]*structs.NodeStatus, Clients map[int]*structs.ClientStatus) *Views {

	views := &Views{
		Rounds: make(map[int]map[int]*View),
	}

	for round := 0; round <= config.GlobalConfig.L+1; round++ {
		views.Rounds[round] = make(map[int]*View)
	}

	for _, status := range Clients {
		networkId := addressToId(status.Client.Address)
		views.addView(&View{
			Round:        0,
			NetworkId:    networkId,
			IsNode:       false,
			ReceivedFrom: make([]int, 0),
			SentTo: utils.Map(status.MessagesSent, func(msg structs.Sent) int {
				return addressToId(utils.GetFirst(msg.RoutingPath).Address)
			}),
		})
		views.addView(&View{
			Round:     config.GlobalConfig.L + 1,
			NetworkId: networkId,
			IsNode:    false,
			ReceivedFrom: utils.Map(status.MessagesReceived, func(msg structs.Received) int {
				return addressToId(msg.ReceivedFromNode.Address)
			}),
			SentTo: make([]int, 0),
		})
	}

	for _, status := range Nodes {
		networkId := addressToId(status.Node.Address)
		for round := range views.Rounds {
			onionsReceived := utils.Filter(status.Received, func(o structs.OnionStatus) bool {
				return o.Layer == round
			})
			received := utils.Map(onionsReceived, func(o structs.OnionStatus) int {
				return addressToId(o.LastHop)
			})
			sent := utils.Map(onionsReceived, func(o structs.OnionStatus) int {
				return addressToId(o.NextHop)
			})
			if len(received) > 0 || len(sent) > 0 {
				views.addView(&View{
					Round:        round + 1,
					NetworkId:    networkId,
					IsNode:       true,
					ReceivedFrom: received,
					SentTo:       sent,
				})
			}
		}
	}
	return views
}

func (v *Views) GetNumOnionsReceived(networkId int, round int) int {
	return len(v.getView(round, networkId).ReceivedFrom)
}

func (v *Views) WhoDidTheySendTo(networkId int, round int) []int {
	if view := v.getView(round, networkId); view != nil {
		return view.SentTo
	}
	return nil
}

func (v *Views) GetProbabilities(senderOfMessage int) [][]float64 {
	probabilities := make([][]float64, len(v.Rounds))
	for i := range probabilities {
		probabilities[i] = make([]float64, len(config.GlobalConfig.Nodes)+len(config.GlobalConfig.Clients)+1)
	}

	probabilities[0][senderOfMessage] = 1.0

	for round := 0; round < len(v.Rounds)-1; round++ {
		for node := 1; node < len(probabilities[round]); node++ {
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
