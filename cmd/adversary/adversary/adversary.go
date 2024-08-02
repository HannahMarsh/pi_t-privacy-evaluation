package adversary

type P struct {
	C          int
	R          int
	X          float64
	ServerLoad float64
	L          int
}

type V struct {
	P      P
	Pr0    []float64
	Pr1    []float64
	Ratios []float64
}
