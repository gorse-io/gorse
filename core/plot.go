package core

import (
	"github.com/skratchdot/open-golang/open"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"path/filepath"
)

func (trainSet *TrainSet) PlotHistogram() {
	// Add the number of ratings for each user
	v := make(plotter.Values, trainSet.UserCount)
	s := make(map[int]interface{})
	for i := range v {
		num := len(trainSet.UserRatings()[i])
		v[i] = float64(num)
		s[num] = true
	}
	// Make a plot and set its title.
	p, err := plot.New()
	if err != nil {
		panic(err)
	}
	p.Title.Text = "Histogram"
	// Create a histogram of our values drawn from the standard normal.
	h, err := plotter.NewHist(v, len(s))
	if err != nil {
		panic(err)
	}
	p.Add(h)
	// Save the plot to a PNG file.
	filePath := filepath.Join(tempDir, "hist.png")
	if err := p.Save(4*vg.Inch, 4*vg.Inch, filePath); err != nil {
		panic(err)
	}
	open.Start(filePath)
}
