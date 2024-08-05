package display

import (
	"fmt"
	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
	"image/color"
	"log"
	"math"
	"time"
)

type pair struct {
	key      float64
	value0   float64
	interval float64
}
type range_ struct {
	min, max float64
	count0   int
	count1   int
	interval int
	width    float64
}

func newRange(min, max float64, interval int, width float64) *range_ {
	return &range_{min: min, max: max, interval: interval, width: width}
}

func (r range_) contains(value float64) bool {
	return r.min <= value && value <= r.max
}

func (r *range_) add0() {
	r.count0++
}

func (r *range_) add1() {
	r.count1++
}

func computeHistogram(data []float64, numBuckets int) []pair {
	// Compute frequencies
	//freq := make(map[float64]int)
	numBuckets = utils.Max(numBuckets, 5)
	xMin := utils.MinOver(data)
	xMax := utils.MaxOver(data)
	interval := (xMax - xMin) / float64(numBuckets)
	freq := make([]*range_, numBuckets)
	for i := range freq {
		freq[i] = newRange(xMin+(float64(i)*interval), xMin+(float64(i+1)*interval), i, interval)
	}
	for _, value := range data {
		if r := utils.FindPointer(freq, func(r *range_) bool {
			return r.contains(value)
		}); r != nil {
			r.add0()
		}
	}

	pdf := make([]pair, 0)

	for _, r := range freq {
		pdf = append(pdf, pair{
			key:      r.min,
			value0:   float64(r.count0),
			interval: r.width,
		})
	}

	return pdf
}

func createFloatCDFPlot(file string, probabilities []pair, title, xLabel, yLabel string) (string, error) {
	newName := fmt.Sprintf("/plots/%s_%d.png", file, time.Now().UnixNano()/int64(time.Millisecond))
	// Create a new plot
	p := plot.New()
	p.Title.Text = title
	p.Y.Label.Text = yLabel
	p.X.Label.Text = xLabel

	keys := utils.Map(probabilities, func(p pair) float64 {
		return p.key
	})
	utils.SortOrdered(keys)

	xLabels := make([]string, len(keys))
	values0 := make([]float64, len(keys))

	totalArea0 := 0.0
	yMax := 0.0

	for i, label := range keys {
		xLabels[i] = fmt.Sprintf("%.2f", label)
		values := utils.Find(probabilities, func(p pair) bool {
			return p.key == label
		})
		values0[i] = values.value0

		area0 := values0[i] * values.interval

		totalArea0 += area0

		yMax = utils.Max(yMax, values0[i])
	}

	// Calculate bar width based on the number of points
	plotWidth := 8 * vg.Inch
	barWidth := plotWidth / vg.Length(int(float64(len(probabilities))*float64(1.2)))

	// Create a bar chart
	//w := vg.Points(20) // Width of the bars

	bars0, err := plotter.NewBarChart(plotter.Values(values0), barWidth)
	if err != nil {
		return "", pl.WrapError(err, "failed to create bar chart")
	}
	bars0.LineStyle.Width = vg.Length(0)  // No line around bars
	bars0.Color = color.Color(firstColor) // Set the color of the bars

	p.Add(bars0)

	// Create a legend
	p.Legend.Add(fmt.Sprintf("(total area = %.6f)", totalArea0), bars0)
	p.Legend.Top = true // Position the legend at the top

	// Add text annotation
	notes, _ := plotter.NewLabels(plotter.XYLabels{
		XYs:    []plotter.XY{{X: 1, Y: yMax * 1.1}}, // Position of the note
		Labels: []string{""},
	})
	p.Add(notes)

	p.NominalX(xLabels...) // Set relay IDs as labels on the X-axis

	// Save the plot to a PNG file
	if err := p.Save(8*vg.Inch, 4*vg.Inch, "static"+newName); err != nil {
		log.Panic(err)
	}
	return newName, nil
}

func createEpsilonDeltaPlot(ratios []float64) (string, error) {

	//fmt.Printf("\nR=\\left[%s\\right]\n", strings.Join(utils.Map(ratios, func(ratio float64) string {
	//	return fmt.Sprintf("%.7f", ratio)
	//}), ","))

	epsilonValues := utils.Map(ratios, func(ratio float64) float64 {
		return math.Log(ratio)
	})

	deltaValues := utils.Map(epsilonValues, func(epsilon float64) float64 {
		bound := math.Pow(math.E, epsilon)
		return utils.Mean(utils.Map(ratios, func(ratio float64) float64 {
			if ratio >= bound {
				return 1.0
			}
			return 0.0
		}))
	})

	return guess(epsilonValues, deltaValues, "Epsilon", "Delta", "Values of ϵ and δ for which (ϵ,δ)-DP is Satisfied", "Epsilon-Delta", "epsilon_delta")
}

func createRatiosPlot(prob0, prob1 []float64) (string, error) {

	mean0 := utils.Mean(prob0)
	mean1 := utils.Mean(prob1)

	return createDotPlot(prob1, prob0, "Probability of Being in Scenario 0 "+fmt.Sprintf("(mean=%f", mean0), "Probability of Being in Scenario 1 "+fmt.Sprintf("(mean=%f", mean1), "Observed Data Pairs", "A single trial: (pr[0], Pr[1])", "epsilon_delta")

}

func createDotPlot(x []float64, y []float64, xAxis, yAxis, title, lineLabel, file string) (string, error) {
	newName := fmt.Sprintf("/plots/%s_%d.png", file, time.Now().UnixNano()/int64(time.Millisecond))

	// Create a new plot
	p := plot.New()
	p.Title.Text = title
	p.X.Label.Text = xAxis
	p.Y.Label.Text = yAxis

	pts := make(plotter.XYs, len(x))
	for i := range pts {
		pts[i].X = x[i]
		pts[i].Y = y[i]
	}

	utils.Sort(pts, func(i, j plotter.XY) bool {
		return i.X < j.X
	})

	err := plotutil.AddScatters(p, lineLabel, pts)
	if err != nil {
		return "", pl.WrapError(err, "failed to add line points")
	}

	if err := p.Save(8*vg.Inch, 6*vg.Inch, "static"+newName); err != nil {
		return "", pl.WrapError(err, "failed to save plot")
	}

	return newName, nil
}
