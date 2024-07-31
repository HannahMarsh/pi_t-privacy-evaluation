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
	Probabilities []float64 `json:"Probabilities"`
	ReceivedR     int       `json:"ReceivedR"`
	ReceivedR_1   int       `json:"ReceivedR_1"`
	ProbScen0     float64   `json:"ProbScen0"`
	ProbScen1     float64   `json:"ProbScen1"`
}

func GetReceivedR(v View) int {
	return v.ReceivedR
}
func GetReceivedR_1(v View) int {
	return v.ReceivedR_1
}

func GetProbScen0(v View) float64 {
	if v.ProbScen0+v.ProbScen1 == 0 {
		probScen0 := v.Probabilities[len(v.Probabilities)-2]
		probScen1 := v.Probabilities[len(v.Probabilities)-1]
		if probScen0+probScen1 == 0 {
			probScen0 = 0.5
			probScen1 = 0.5
		}
		v.ProbScen0 = probScen0 / (probScen1 + probScen0)
		v.ProbScen1 = probScen1 / (probScen1 + probScen0)
	}
	return v.ProbScen0
}

func GetProbScen1(v View) float64 {
	if v.ProbScen0+v.ProbScen1 == 0 {
		probScen0 := v.Probabilities[len(v.Probabilities)-2]
		probScen1 := v.Probabilities[len(v.Probabilities)-1]
		if probScen0+probScen1 == 0 {
			probScen0 = 0.5
			probScen1 = 0.5
		}
		v.ProbScen0 = probScen0 / (probScen1 + probScen0)
		v.ProbScen1 = probScen1 / (probScen1 + probScen0)
	}
	return v.ProbScen1
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
