package store

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"time"

	"github.com/alice-lg/alice-lg/pkg/api"
	"github.com/alice-lg/alice-lg/pkg/config"
	"github.com/alice-lg/alice-lg/pkg/sources"
)

// RoutesStoreBackend interface
type RoutesStoreBackend interface {
	// SetRoutes updates the routes in the store after a refresh.
	SetRoutes(
		ctx context.Context,
		sourceID string,
		routes api.LookupRoutes,
	) error

	// CountRoutesAt returns the number of imported
	// and filtered routes for a given route server.
	// Example: (imported, filtered, error)
	CountRoutesAt(
		ctx context.Context,
		sourceID string,
	) (uint, uint, error)

	// FindByNeighbors retrieves the prefixes
	// announced by the neighbor at a given source
	FindByNeighbors(
		ctx context.Context,
		neighborIDs []string,
	) (api.LookupRoutes, error)

	// FindByPrefix
	FindByPrefix(
		ctx context.Context,
		prefix string,
	) (api.LookupRoutes, error)
}

// The RoutesStore holds a mapping of routes,
// status and cfgs and will be queried instead
// of a backend by the API
type RoutesStore struct {
	backend   RoutesStoreBackend
	sources   *SourcesStore
	neighbors *NeighborsStore
}

// NewRoutesStore makes a new store instance
// with a cfg.
func NewRoutesStore(
	neighbors *NeighborsStore,
	cfg *config.Config,
	backend RoutesStoreBackend,
) *RoutesStore {
	// Set refresh interval as duration, fall back to
	// five minutes if no interval is set.
	refreshInterval := time.Duration(
		cfg.Server.RoutesStoreRefreshInterval) * time.Minute
	if refreshInterval == 0 {
		refreshInterval = time.Duration(5) * time.Minute
	}
	refreshParallelism := cfg.Server.RoutesStoreRefreshParallelism
	if refreshParallelism <= 0 {
		refreshParallelism = 1
	}

	log.Println("Routes refresh interval set to:", refreshInterval)
	log.Println("Routes refresh parallelism:", refreshParallelism)

	// Store refresh information per store
	sources := NewSourcesStore(cfg, refreshInterval, refreshParallelism)
	store := &RoutesStore{
		backend:   backend,
		sources:   sources,
		neighbors: neighbors,
	}
	return store
}

// Start starts the routes store
func (s *RoutesStore) Start() {
	log.Println("Starting local routes store")

	// Periodically trigger updates
	for {
		s.update()
		time.Sleep(time.Second)
	}
}

// Update all routes from all sources, where the
// sources last refresh is longer ago than the configured
// refresh period. This is totally the same as the
// NeighborsStore.update and maybe these functions can be merged (TODO)
func (s *RoutesStore) update() {
	for _, id := range s.sources.GetSourceIDsForRefresh() {
		go s.safeUpdateSource(id)
	}
}

// safeUpdateSource will try to update a source but
// will recover from a panic if something goes wrong.
// In that case, the LastError and State will be updated.
// Again. The similarity to the NeighborsStore is really sus.
func (s *RoutesStore) safeUpdateSource(id string) {
	ctx := context.TODO()

	if !s.sources.ShouldRefresh(id) {
		return // Nothing to do here
	}

	if err := s.sources.LockSource(id); err != nil {
		log.Println("Cloud not start routes refresh:", err)
		return
	}

	// Apply jitter so, we do not hit everything at once.
	// TODO: Make configurable
	time.Sleep(time.Duration(rand.Intn(30)) * time.Second)

	src := s.sources.Get(id)
	srcName := s.sources.GetName(id)

	log.Println("[routes store] begin routes refresh of:", srcName)

	// Prepare for impact.
	defer func() {
		if err := recover(); err != nil {
			log.Println(
				"Recovering after failed routes refresh of",
				src.Name, "from:", err)
			s.sources.RefreshError(id, err)
		}
	}()

	if err := s.updateSource(ctx, src); err != nil {
		log.Println(
			"Refeshing routes of", src.Name, "failed:", err)
		s.sources.RefreshError(id, err)
	} else {
		status, err := s.sources.GetStatus(id)
		if err != nil {
			log.Println(err)
		} else {
			log.Println("Refreshed routes of", srcName, "in", status.LastRefreshDuration)
		}
	}
}

// Update all routes
func (s *RoutesStore) updateSource(
	ctx context.Context,
	src *config.SourceConfig,
) error {

	log.Println("[routes store] start retrive routes dump from RS", src.Name)

	rs := src.GetInstance()
	res, err := rs.AllRoutes()
	if err != nil {
		return err
	}

	log.Println("[routes store] finished fetching routes dump from RS", src.Name)

	log.Println("[routes store] awaiting neighbor store HAS DATA for", src.Name)
	if err := s.awaitNeighborStore(ctx, src.ID); err != nil {
		return err
	}
	log.Println("[routes store] neighbor store HAS DATA for", src.Name)

	neighbors, err := s.neighbors.GetNeighborsMapAt(ctx, src.ID)
	if err != nil {
		return err
	}

	log.Println("[routes store] retrieved neighbors for:", src.Name)

	log.Println("[routes store] preparing routes for import of", src.Name)
	// Prepare imported routes for lookup
	imported := s.routesToLookupRoutes(ctx, "imported", src, neighbors, res.Imported)
	filtered := s.routesToLookupRoutes(ctx, "filtered", src, neighbors, res.Filtered)
	lookupRoutes := append(imported, filtered...)

	log.Println("[routes store] importing", len(lookupRoutes), "into store from", src.Name)
	if err = s.backend.SetRoutes(ctx, src.ID, lookupRoutes); err != nil {
		return err
	}

	log.Println("[routes store] import success.")
	return s.sources.RefreshSuccess(src.ID)
}

// awaitNeighborStore polls the neighbor store state
// for the sourceID until the context is not longer valid.
func (s *RoutesStore) awaitNeighborStore(
	ctx context.Context,
	srcID string,
) error {
	for {
		err := ctx.Err()
		if err != nil {
			return err
		}
		if s.neighbors.IsInitialized(srcID) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *RoutesStore) routesToLookupRoutes(
	ctx context.Context,
	state string,
	src *config.SourceConfig,
	neighbors map[string]*api.Neighbor,
	routes api.Routes,
) api.LookupRoutes {
	lookupRoutes := make(api.LookupRoutes, 0, len(routes))
	for _, route := range routes {
		neighbor, ok := neighbors[route.NeighborID]
		if !ok {
			log.Println("prepare route, neighbor not found:", route.NeighborID)
			continue
		}
		lr := &api.LookupRoute{
			Route:    route,
			State:    state,
			Neighbor: neighbor,
			RouteServer: &api.RouteServer{
				ID:   src.ID,
				Name: src.Name,
			},
		}
		lr.Route.Details = nil
		lookupRoutes = append(lookupRoutes, lr)
	}
	return lookupRoutes
}

// Stats calculates some store insights
func (s *RoutesStore) Stats() *api.RoutesStoreStats {
	ctx := context.TODO()

	totalImported := uint(0)
	totalFiltered := uint(0)

	rsStats := []api.RouteServerRoutesStats{}

	for _, sourceID := range s.sources.GetSourceIDs() {
		status, err := s.sources.GetStatus(sourceID)
		if err != nil {
			log.Println("error while getting source status:", err)
			continue
		}

		src := s.sources.Get(sourceID)

		nImported, nFiltered, err := s.backend.CountRoutesAt(ctx, sourceID)
		if err != nil {
			if !errors.Is(err, sources.ErrSourceNotFound) {
				log.Println("error during routes count:", err)
			}
		}

		totalImported += nImported
		totalFiltered += nFiltered

		serverStats := api.RouteServerRoutesStats{
			Name: src.Name,
			Routes: api.RoutesStats{
				Imported: nImported,
				Filtered: nFiltered,
			},
			State:     status.State.String(),
			UpdatedAt: status.LastRefresh,
		}
		rsStats = append(rsStats, serverStats)
	}

	// Make stats
	storeStats := &api.RoutesStoreStats{
		TotalRoutes: api.RoutesStats{
			Imported: totalImported,
			Filtered: totalFiltered,
		},
		RouteServers: rsStats,
	}
	return storeStats
}

// CachedAt returns the time of the oldest partial
// refresh of the dataset.
func (s *RoutesStore) CachedAt(
	ctx context.Context,
) time.Time {
	return s.sources.CachedAt(ctx)
}

// CacheTTL returns the TTL time
func (s *RoutesStore) CacheTTL(
	ctx context.Context,
) time.Time {
	return s.sources.NextRefresh(ctx)
}

// LookupPrefix performs a lookup over all route servers
func (s *RoutesStore) LookupPrefix(
	ctx context.Context,
	prefix string,
) (api.LookupRoutes, error) {
	return s.backend.FindByPrefix(ctx, prefix)
}

// LookupPrefixForNeighbors returns all routes for
// a set of neighbors.
func (s *RoutesStore) LookupPrefixForNeighbors(
	ctx context.Context,
	neighbors api.NeighborsLookupResults,
) (api.LookupRoutes, error) {
	neighborIDs := []string{}
	for _, rs := range neighbors {
		for _, neighbor := range rs {
			neighborIDs = append(neighborIDs, neighbor.ID)
		}
	}
	return s.backend.FindByNeighbors(ctx, neighborIDs)
}
