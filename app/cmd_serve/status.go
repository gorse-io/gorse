package cmd_serve

type Status struct {
}

// CurrentRatings gets the number of ratings at current.
func (stat *Status) CurrentRatings() int {
	return 0
}

// LastRatings gets the number of ratings at the time of last update.
func (stat *Status) LastRatings() int {
	return 0
}
