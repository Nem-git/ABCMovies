package search

import (
	"sort"
	"strings"

	"github.com/adrg/strutil"
	"github.com/adrg/strutil/metrics"
	"github.com/nem-git/abcmovies/internal/oas"
)

type Result struct {
	Tag  string
	Item oas.SearchResultItem
}

func ScoreAndSort(query string, items []Result) []Result {
	if query == "" || len(items) == 0 {
		return items
	}

	lq := strings.ToLower(query)
	sim := metrics.NewJaroWinkler()

	for i := range items {
		name := ResourceName(items[i].Item)
		ln := strings.ToLower(name)

		switch {
		case lq == ln:
			items[i].Item.Score = 1.0
		case strings.HasPrefix(ln, lq):
			items[i].Item.Score = 0.9
		case strings.Contains(ln, lq):
			items[i].Item.Score = 0.7
		default:
			items[i].Item.Score = float32(strutil.Similarity(lq, ln, sim))
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Item.Score > items[j].Item.Score
	})

	return items
}

func FilterByType(items []Result, types []string) []Result {
	if len(types) == 0 {
		return items
	}
	allowed := make(map[string]bool, len(types))
	for _, t := range types {
		allowed[t] = true
	}
	filtered := make([]Result, 0, len(items))
	for _, item := range items {
		switch {
		case item.Item.Resource.IsMovie() && allowed["movie"]:
			filtered = append(filtered, item)
		case item.Item.Resource.IsSeries() && allowed["series"]:
			filtered = append(filtered, item)
		case item.Item.Resource.IsService() && allowed["service"]:
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func ResourceName(item oas.SearchResultItem) string {
	switch {
	case item.Resource.IsMovie():
		return item.Resource.Movie.Name
	case item.Resource.IsSeries():
		return item.Resource.Series.Name
	case item.Resource.IsService():
		return item.Resource.Service.Name
	}
	return ""
}
