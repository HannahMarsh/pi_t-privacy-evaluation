package display

import (
	"fmt"
	pl "github.com/HannahMarsh/PrettyLogger"
	data2 "github.com/HannahMarsh/pi_t-privacy-evaluation/internal/data"
	"github.com/HannahMarsh/pi_t-privacy-evaluation/pkg/utils"
	"image/color"
	"io/ioutil"
	"os"
)

var firstColor = color.RGBA{R: 217, G: 156, B: 201, A: 255}
var secondColor = color.RGBA{R: 173, G: 202, B: 237, A: 255}
var overlap = color.RGBA{R: 143, G: 106, B: 176, A: 255}

type Images struct {
	Ratios       string `json:"ratios_img"`
	EpsilonDelta string `json:"epsilon_delta_img"`
	RatiosPlot   string `json:"ratios_plot_img"`
}

func PlotView(v data2.Result, numBuckets int) (Images, error) {

	// Read the contents of the directory
	contents, err := ioutil.ReadDir("static/plots")
	if err != nil {
		return Images{}, pl.WrapError(err, "failed to read directory")
	}

	// Remove each item in the directory
	for _, item := range contents {
		itemPath := "static/plots/" + item.Name()
		if err = os.RemoveAll(itemPath); err != nil {
			return Images{}, pl.WrapError(err, "failed to remove item")
		}
	}

	ratiosPDF := computeHistogram(v.Ratios, numBuckets)

	prConfidence, err := createFloatCDFPlot("ratio", ratiosPDF, "Ratio of Pr[0] Over Pr[1] "+fmt.Sprintf("(mean=%f)", utils.Mean(v.Ratios)), "Ratio", "Frequency (# of trials)")
	if err != nil {
		return Images{}, pl.WrapError(err, "failed to create CDF plot")
	}

	epDelta, err := createEpsilonDeltaPlot(v.Ratios)
	if err != nil {
		return Images{}, pl.WrapError(err, "failed to create CDF plot")
	}

	ratiosPlot, err := createRatiosPlot(v.Pr0, v.Pr1)
	if err != nil {
		return Images{}, pl.WrapError(err, "failed to create CDF plot")
	}

	return Images{
		Ratios:       prConfidence,
		EpsilonDelta: epDelta,
		RatiosPlot:   ratiosPlot,
	}, nil
}
