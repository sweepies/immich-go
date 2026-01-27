package filters

import (
	"github.com/sweepies/immich-go/internal/assets"
)

/*
Applies filters to a group of assets.
*/

type Filter func(g *assets.Group) *assets.Group

func ApplyFilters(g *assets.Group, filters ...Filter) *assets.Group {
	if g.Grouping != assets.GroupByNone {
		for _, f := range filters {
			g = f(g)
		}
	}
	return g
}
