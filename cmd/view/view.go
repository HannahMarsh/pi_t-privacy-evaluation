package view

import "github.com/HannahMarsh/pi_t-privacy-evaluation/internal/interfaces"

type AllData struct {
	Data []Data `json:"Data"`
}

type Data struct {
	Params interfaces.Params `json:"Params"`
	Views  []View            `json:"Views"`
}

type View struct {
	ProbR   float64 `json:"ProbR"`
	ProbR_1 float64 `json:"ProbR_1"`
}

//	func GetReceivedR(v View) int {
//		return v.ReceivedR
//	}
//
//	func GetReceivedR_1(v View) int {
//		return v.ReceivedR_1
//	}
func GetProbScen0(v View) float64 {

	probScen0 := v.ProbR_1
	probScen1 := v.ProbR
	if probScen0+probScen1 == 0 {
		probScen0 = 0.5
		probScen1 = 0.5
	}
	return probScen0 / (probScen1 + probScen0)

}

func GetProbScen1(v View) float64 {
	probScen0 := v.ProbR_1
	probScen1 := v.ProbR
	if probScen0+probScen1 == 0 {
		probScen0 = 0.5
		probScen1 = 0.5
	}
	return probScen1 / (probScen1 + probScen0)
}

type ExpectedValues struct {
	N          []int     `json:"N"`
	R          []int     `json:"R"`
	ServerLoad []int     `json:"ServerLoad"`
	L          []int     `json:"L"`
	Scenario   []int     `json:"Scenario"`
	NumRuns    []int     `json:"NumRuns"`
	NumBuckets []int     `json:"NumBuckets"`
	X          []float64 `json:"X"`
}
