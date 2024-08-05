package data

import (
	"fmt"
)

type Parameters struct {
	C          int
	R          int
	X          float64
	ServerLoad float64
	L          int
	str        string
}

type Result struct {
	P      Parameters
	Pr0    []float64
	Pr1    []float64
	Ratios []float64
}

func (p *Parameters) Hash() string {
	if p.str == "" {
		p.str = fmt.Sprintf("%d-%d-%d-%d-%d", p.C, p.R, int(p.X*float64(p.R)), int(p.ServerLoad), p.L)
	}
	return p.str
}

func (p *Parameters) Equals(p2 *Parameters) bool {
	return p.Hash() == p2.Hash()
}
