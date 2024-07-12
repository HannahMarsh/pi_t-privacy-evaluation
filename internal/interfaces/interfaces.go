package interfaces

import "github.com/HannahMarsh/pi_t-privacy-evaluation/internal/api/structs"

type Node interface {
	Receive(structs.Onion) error
	StartRun(scenario int) error
	GetID() int
}

type System interface {
	Send(layer, from, to int, o structs.Onion) error
	Receive(layer, from, to int)
	GetParams() Params
	RegisterParty(n Node)
	StartRun() error
	GetNodes() []int
	GetClients() []int
	GetNumOnionsReceived(id int) int
	GetProbabilities(senderOfMessage int) []float64
}

type Params struct {
	N          int `json:"N"`
	R          int `json:"R"`
	L          int `json:"L"`
	D          float64
	ServerLoad int     `json:"ServerLoad"`
	X          float64 `json:"X"`
	Scenario   int     `json:"Scenario"`
}
