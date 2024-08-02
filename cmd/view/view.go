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

func GetRatio(v View) float64 {
	if v.ProbR == 0.0 {
		v.ProbR = 0.000001
	}

	return v.ProbR_1 / v.ProbR

}

func GetProbR(v View) float64 {
	return v.ProbR
}

func GetProbR_1(v View) float64 {
	return v.ProbR_1
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
