package http

import (
	"net/http"
	"sort"
	"time"

	"github.com/julienschmidt/httprouter"

	"github.com/alice-lg/alice-lg/pkg/api"
	"github.com/alice-lg/alice-lg/pkg/decoders"
)

// Handle global lookup
func (s *Server) apiLookupPrefixGlobal(
	req *http.Request,
	params httprouter.Params,
) (response, error) {
	// TODO: This function is way too long
	ctx := req.Context()

	// Get prefix to query
	q, err := validateQueryString(req, "q")
	if err != nil {
		return nil, err
	}

	q, err = validatePrefixQuery(q)
	if err != nil {
		return nil, err
	}

	// Check what we want to query
	//  Prefix -> fetch prefix
	//       _ -> fetch neighbors and routes
	lookupPrefix := decoders.MaybePrefix(q)

	// Measure response time
	t0 := time.Now()

	// Get additional filter criteria
	filtersApplied, err := api.FiltersFromQuery(req.URL.Query())
	if err != nil {
		return nil, err
	}

	// Perform query
	var routes api.LookupRoutes
	if lookupPrefix {
		routes, err = s.routesStore.LookupPrefix(ctx, q)
		if err != nil {
			return nil, err
		}

	} else {
		neighbors, err := s.neighborsStore.LookupNeighbors(ctx, q)
		if err != nil {
			return nil, err
		}
		routes, err = s.routesStore.LookupPrefixForNeighbors(ctx, neighbors)
		if err != nil {
			return nil, err
		}
	}

	// Split routes
	// TODO: Refactor at neighbors store
	totalResults := len(routes)
	imported := make(api.LookupRoutes, 0, totalResults)
	filtered := make(api.LookupRoutes, 0, totalResults)

	// Now, as we have allocated even more space process routes by, splitting,
	// filtering and updating the available filters...
	filtersAvailable := api.NewSearchFilters()
	for _, r := range routes {

		if !filtersApplied.MatchRoute(r) {
			continue // Exclude route from results set
		}

		switch r.State {
		case api.RouteStateFiltered:
			filtered = append(filtered, r)
		case api.RouteStateImported:
			imported = append(imported, r)
		}

		filtersAvailable.UpdateFromLookupRoute(r)
	}

	// Remove applied filters from available
	filtersApplied.MergeProperties(filtersAvailable)
	filtersAvailable = filtersAvailable.Sub(filtersApplied)

	// Homogenize results
	sort.Sort(imported)
	sort.Sort(filtered)

	// Paginate results
	pageImported := apiQueryMustInt(req, "page_imported", 0)
	pageSizeImported := s.cfg.UI.Pagination.RoutesAcceptedPageSize
	routesImported, paginationImported := apiPaginateLookupRoutes(
		imported, pageImported, pageSizeImported,
	)

	pageFiltered := apiQueryMustInt(req, "page_filtered", 0)
	pageSizeFiltered := s.cfg.UI.Pagination.RoutesFilteredPageSize
	routesFiltered, paginationFiltered := apiPaginateLookupRoutes(
		filtered, pageFiltered, pageSizeFiltered,
	)

	// Calculate query duration
	queryDuration := time.Since(t0)

	cachedAt, err := s.routesStore.CachedAt(ctx)
	if err != nil {
		return nil, err
	}

	ttl, err := s.routesStore.CacheTTL(ctx)
	if err != nil {
		return nil, err
	}

	// Make response
	response := api.PaginatedRoutesLookupResponse{
		Response: api.Response{
			Meta: &api.Meta{
				CacheStatus: api.CacheStatus{
					CachedAt: cachedAt,
				},
				ResultFromCache: true, // Well.
				TTL:             ttl,
			},
		},
		TimedResponse: api.TimedResponse{
			RequestDuration: DurationMs(queryDuration),
		},
		Imported: &api.RoutesLookup{
			Routes:     routesImported,
			Pagination: paginationImported,
		},
		Filtered: &api.RoutesLookup{
			Routes:     routesFiltered,
			Pagination: paginationFiltered,
		},
		FilteredResponse: api.FilteredResponse{
			FiltersAvailable: filtersAvailable,
			FiltersApplied:   filtersApplied,
		},
	}

	return response, nil
}

func (s *Server) apiLookupNeighborsGlobal(
	req *http.Request,
	params httprouter.Params,
) (response, error) {
	// Query neighbors store
	filter := api.NeighborFilterFromQuery(req.URL.Query())
	neighbors := s.neighborsStore.FilterNeighbors(filter)

	sort.Sort(neighbors)

	// Make response
	response := &api.NeighborsResponse{
		Response: api.Response{
			Meta: &api.Meta{
				CacheStatus: api.CacheStatus{
					CachedAt: s.neighborsStore.CachedAt(),
				},
				ResultFromCache: true, // You would not have guessed.
				TTL:             s.neighborsStore.CacheTTL(),
			},
		},
		Neighbors: neighbors,
	}
	return response, nil
}
