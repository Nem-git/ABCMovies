package oas

// This file adds the missing ErrorStatusCode marker methods for image
// response interfaces that ogen does not generate.

func (*ErrorStatusCode) getMoviePosterRes()       {}
func (*ErrorStatusCode) getMovieBackdropRes()     {}
func (*ErrorStatusCode) getSeriesPosterRes()      {}
func (*ErrorStatusCode) getSeriesBackdropRes()    {}
func (*ErrorStatusCode) getSeasonPosterRes()      {}
func (*ErrorStatusCode) getSeasonBackdropRes()    {}
func (*ErrorStatusCode) getEpisodeThumbnailRes()  {}
