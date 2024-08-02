package adversary

import (
	"fmt"
)

type P struct {
	C          int
	R          int
	X          float64
	ServerLoad float64
	L          int
	str        string
}

func (p *P) Hash() string {
	if p.str == "" {
		p.str = fmt.Sprintf("%d-%d-%d-%d-%d", p.C, p.R, int(p.X*float64(p.R)), int(p.ServerLoad), p.L)
	}
	return p.str
}

func (p *P) Equals(p2 *P) bool {
	return p.Hash() == p2.Hash()
}

type V struct {
	P      P
	Pr0    []float64
	Pr1    []float64
	Ratios []float64
}
