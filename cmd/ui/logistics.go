package main

import (
	"fmt"
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/optimize"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"math"
	"time"
)

// logisticFunction defines the logistic function with parameters mu and s.
func logisticFunction(x, mu, s float64) float64 {
	return 1.0 / (1.0 + math.Exp(-(x-mu)/s))
}

// residual calculates the residuals between the observed and predicted values.
func residual(p []float64, observedX, observedY []float64) float64 {
	mu, s := p[0], p[1]
	sum := 0.0
	for i := range observedX {
		predictedY := logisticFunction(observedX[i], mu, s)
		diff := observedY[i] - predictedY
		sum += diff * diff
	}
	return sum
}

// generateLogisticPoints generates points for the logistic function based on fitted parameters.
func generateLogisticPoints(mu, s float64, minX, maxX float64, numPoints int) plotter.XYs {
	pts := make(plotter.XYs, numPoints)
	step := (maxX - minX) / float64(numPoints-1)
	for i := range pts {
		x := minX + step*float64(i)
		pts[i].X = x
		pts[i].Y = logisticFunction(x, mu, s)
	}
	return pts
}

func guess(observedX, observedY []float64, xAxis, yAxis, title, lineLabel, file string) (string, error) {

	newName := fmt.Sprintf("/plots/%s_%d.png", file, time.Now().UnixNano()/int64(time.Millisecond))

	// Create a new plot
	p := plot.New()
	p.Title.Text = title
	p.X.Label.Text = xAxis
	p.Y.Label.Text = yAxis

	pts := make(plotter.XYs, len(observedX))
	for i := range pts {
		pts[i].X = observedX[i]
		pts[i].Y = observedY[i]
	}

	utils.Sort(pts, func(i, j plotter.XY) bool {
		return i.X < j.X
	})

	observedX = utils.Map(pts, func(xy plotter.XY) float64 {
		return xy.X
	})

	observedY = utils.Map(pts, func(xy plotter.XY) float64 {
		return xy.Y
	})

	//points, err := plotter.NewScatter(pts)
	//if err != nil {
	//	return "", err
	//}
	//points.Shape = draw.CircleGlyph{}
	//points.Color = overlap
	//points.Radius = vg.Points(3)
	//
	//p.Add(points)
	//p.Legend.Add(lineLabel, points)

	// Initial guesses for mu and s
	initial := []float64{floats.Sum(observedX) / float64(len(observedX)), 1.0}

	// Define problem for optimization
	problem := optimize.Problem{
		Func: func(p []float64) float64 {
			return residual(p, observedX, observedY)
		},
	}

	// Settings for optimization
	settings := optimize.Settings{
		GradientThreshold: 1e-6,
		FuncEvaluations:   100,
		MajorIterations:   100,
	}

	// Perform the optimization
	result, err := optimize.Minimize(problem, initial, &settings, nil)
	if err != nil {
		return "", pl.WrapError(err, "Failed to optimize: %v", err)
	}

	// Extract the optimized parameters
	mu, s := result.X[0], result.X[1]
	fmt.Printf("Estimated parameters: mu = %f, s = %f\n", mu, s)

	// Generate and plot the logistic curve
	logisticPts := generateLogisticPoints(mu, s, floats.Min(observedX), floats.Max(observedX), 100)
	line, err := plotter.NewLine(logisticPts)
	if err != nil {
		return "", pl.WrapError(err, "Failed to create line plot: %v", err)
	}
	line.LineStyle.Width = vg.Points(5) // Thicker line
	line.LineStyle.Color = firstColor
	p.Add(line)

	p.Legend.Add(fmt.Sprintf("1 / 1 + e^(-(ϵ+%f)/%f", mu, s), line)

	points, err := plotter.NewScatter(pts)
	if err != nil {
		return "", err
	}
	points.Shape = draw.CircleGlyph{}
	points.Color = overlap
	points.Radius = vg.Points(3)

	p.Add(points)
	p.Legend.Add(lineLabel, points)

	// Save the plot to a PNG file
	if err := p.Save(8*vg.Inch, 6*vg.Inch, "static"+newName); err != nil {
		return "", pl.WrapError(err, "failed to save plot")
	}
	return newName, nil
}
