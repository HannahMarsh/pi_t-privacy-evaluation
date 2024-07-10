package adversary

import (
	"github.com/HannahMarsh/pi_t-privacy-evaluation/config"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/internal/api/structs"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
)

type View struct {
	Round        int
	Party        string
	ID           int
	IsNode       bool
	ReceivedFrom []string
	SentTo       []string
}

type Views struct {
	Rounds  map[int][]View
	Parties map[string][]View
}

func CollectViews(Nodes map[string]structs.NodeStatus, Clients map[string]structs.ClientStatus) *Views {

	views := Views{
		Rounds:  make(map[int][]View),
		Parties: make(map[string][]View),
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
			IsNode:       false,
			ReceivedFrom: make([]string, 0),
			SentTo:       make([]string, 0),
		}
		for _, msg := range status.MessagesSent {
			v.SentTo = append(v.SentTo, msg.Message.To)
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

	for _, v := range utils.Flatten(utils.GetValues(views.Rounds)) {
		if views.Parties[v.Party] == nil {
			views.Parties[v.Party] = make([]View, 0)
		}
		views.Parties[v.Party] = append(views.Parties[v.Party], v)
	}
	return &views
}
