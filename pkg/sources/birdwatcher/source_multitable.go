package birdwatcher

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/alice-lg/alice-lg/pkg/api"
	"github.com/alice-lg/alice-lg/pkg/decoders"
)

// MultiTableBirdwatcher implements a birdwatcher with
// a multitable bird as a datasource.
type MultiTableBirdwatcher struct {
	GenericBirdwatcher
}

func (src *MultiTableBirdwatcher) getMasterPipeName(table string) string {
	ptPrefix := src.config.PeerTablePrefix
	if strings.HasPrefix(table, ptPrefix) {
		return src.config.PipeProtocolPrefix + table[len(ptPrefix):]
	}
	return ""
}

func (src *MultiTableBirdwatcher) parseProtocolToTableTree(
	bird ClientResponse,
) map[string]interface{} {
	protocols := bird["protocols"].(map[string]interface{})

	response := make(map[string]interface{})

	for _, protocolData := range protocols {
		protocol := protocolData.(map[string]interface{})

		if protocol["bird_protocol"] == "BGP" {
			table := protocol["table"].(string)
			neighborAddress := protocol["neighbor_address"].(string)

			if _, ok := response[table]; !ok {
				response[table] = make(map[string]interface{})
			}

			if _, ok := response[table].(map[string]interface{})[neighborAddress]; !ok {
				response[table].(map[string]interface{})[neighborAddress] = make(map[string]interface{})
			}

			response[table].(map[string]interface{})[neighborAddress] = protocol
		}
	}

	return response
}

func (src *MultiTableBirdwatcher) fetchProtocols() (*api.Meta, map[string]interface{}, error) {
	// Query birdwatcher
	bird, err := src.client.GetJson("/protocols")
	if err != nil {
		return nil, nil, err
	}

	// Use api status from first request
	apiStatus, err := parseAPIStatus(bird, src.config)
	if err != nil {
		return nil, nil, err
	}

	if _, ok := bird["protocols"]; !ok {
		return nil, nil, fmt.Errorf("failed to fetch protocols")
	}

	return &apiStatus, bird, nil
}

func (src *MultiTableBirdwatcher) fetchReceivedRoutes(
	neighborID string,
) (*api.Meta, api.Routes, error) {
	// Query birdwatcher
	_, birdProtocols, err := src.fetchProtocols()
	if err != nil {
		return nil, nil, err
	}

	protocols := birdProtocols["protocols"].(map[string]interface{})

	if _, ok := protocols[neighborID]; !ok {
		return nil, nil, fmt.Errorf("Invalid Neighbor")
	}

	peer := protocols[neighborID].(map[string]interface{})["neighbor_address"].(string)

	// Query birdwatcher
	bird, err := src.client.GetJson("/routes/peer/" + peer)
	if err != nil {
		return nil, nil, err
	}

	// Use api status from first request
	apiStatus, err := parseAPIStatus(bird, src.config)
	if err != nil {
		return nil, nil, err
	}

	// Parse the routes
	received, err := parseRoutes(bird, src.config)
	if err != nil {
		log.Println("WARNING Could not retrieve received routes:", err)
		log.Println("Is the 'routes_peer' module active in birdwatcher?")
		return &apiStatus, nil, err
	}

	return &apiStatus, received, nil
}

func (src *MultiTableBirdwatcher) fetchFilteredRoutes(
	neighborID string,
) (*api.Meta, api.Routes, error) {
	// Query birdwatcher
	_, birdProtocols, err := src.fetchProtocols()
	if err != nil {
		return nil, nil, err
	}

	protocols := birdProtocols["protocols"].(map[string]interface{})

	if _, ok := protocols[neighborID]; !ok {
		return nil, nil, fmt.Errorf("Invalid Neighbor")
	}

	// Stage 1 filters
	birdFiltered, err := src.client.GetJson("/routes/filtered/" + neighborID)
	if err != nil {
		log.Println("WARNING Could not retrieve filtered routes:", err)
		log.Println("Is the 'routes_filtered' module active in birdwatcher?")
		return nil, nil, err
	}

	// Use api status from first request
	apiStatus, err := parseAPIStatus(birdFiltered, src.config)
	if err != nil {
		return nil, nil, err
	}

	// Parse the routes
	filtered := parseRoutesData(birdFiltered["routes"].([]interface{}), src.config)

	// Stage 2 filters
	table := protocols[neighborID].(map[string]interface{})["table"].(string)
	pipeName := src.getMasterPipeName(table)

	// If there is no pipe to master, there is nothing left to do
	if pipeName == "" {
		return &apiStatus, filtered, nil
	}

	// Query birdwatcher
	birdPipeFiltered, err := src.client.GetJson(
		"/routes/pipe/filtered/?table=" + table + "&pipe=" + pipeName)
	if err != nil {
		log.Println("WARNING Could not retrieve filtered routes:", err)
		log.Println("Is the 'pipe_filtered' module active in birdwatcher?")
		return &apiStatus, nil, err
	}

	// Parse the routes
	pipeFiltered := parseRoutesData(
		birdPipeFiltered["routes"].([]interface{}), src.config)

	// Sort routes for deterministic ordering
	filtered = append(filtered, pipeFiltered...)
	sort.Sort(filtered)

	return &apiStatus, filtered, nil
}

func (src *MultiTableBirdwatcher) fetchNotExportedRoutes(
	neighborID string,
) (*api.Meta, api.Routes, error) {
	// Query birdwatcher
	_, birdProtocols, err := src.fetchProtocols()
	if err != nil {
		return nil, nil, err
	}

	protocols := birdProtocols["protocols"].(map[string]interface{})

	if _, ok := protocols[neighborID]; !ok {
		return nil, nil, fmt.Errorf("invalid Neighbor")
	}

	table := protocols[neighborID].(map[string]interface{})["table"].(string)
	pipeName := src.getMasterPipeName(table)

	// Query birdwatcher
	bird, err := src.client.GetJson("/routes/noexport/" + pipeName)

	// Use api status from first request
	apiStatus, err := parseAPIStatus(bird, src.config)
	if err != nil {
		return nil, nil, err
	}

	notExported, err := parseRoutes(bird, src.config)
	if err != nil {
		log.Println("WARNING Could not retrieve routes not exported:", err)
		log.Println("Is the 'routes_noexport' module active in birdwatcher?")
	}

	return &apiStatus, notExported, nil
}

// RoutesRequired is a specialized request to fetch:
//
// - RoutesExported and
// - RoutesFiltered
//
// from Birdwatcher. As the not exported routes can be very many
// these are optional and can be loaded on demand using the
// RoutesNotExported() API.
//
// A route deduplication is applied.
func (src *MultiTableBirdwatcher) fetchRequiredRoutes(
	neighborID string,
) (*api.RoutesResponse, error) {
	// Allow only one concurrent request for this neighbor
	// to our backend server.
	src.routesFetchMutex.Lock(neighborID)
	defer src.routesFetchMutex.Unlock(neighborID)

	// Check if we have a cache hit
	response := src.routesRequiredCache.Get(neighborID)
	if response != nil {
		return response, nil
	}

	// First: get routes received
	apiStatus, receivedRoutes, err := src.fetchReceivedRoutes(neighborID)
	if err != nil {
		return nil, err
	}

	// Second: get routes filtered
	_, filteredRoutes, err := src.fetchFilteredRoutes(neighborID)
	if err != nil {
		return nil, err
	}

	// Perform route deduplication
	importedRoutes := api.Routes{}
	if len(receivedRoutes) > 0 {
		peer := receivedRoutes[0].Gateway
		learntFrom := decoders.String(receivedRoutes[0].Details["learnt_from"], peer)

		filteredRoutes = src.filterRoutesByPeerOrLearntFrom(filteredRoutes, peer, learntFrom)
		importedRoutes = src.filterRoutesByDuplicates(receivedRoutes, filteredRoutes)
	}

	response = &api.RoutesResponse{
		Response: api.Response{
			Meta: *apiStatus,
		},
		Imported: importedRoutes,
		Filtered: filteredRoutes,
	}

	// Cache result
	src.routesRequiredCache.Set(neighborID, response)

	return response, nil
}

// Neighbors get neighbors from protocols.
// TODO: this. needs. refactoring.
func (src *MultiTableBirdwatcher) Neighbors() (*api.NeighborsResponse, error) {
	// Check if we hit the cache
	response := src.neighborsCache.Get()
	if response != nil {
		return response, nil
	}

	// Query birdwatcher
	apiStatus, birdProtocols, err := src.fetchProtocols()
	if err != nil {
		return nil, err
	}

	// Parse the neighbors
	neighbors, err := parseNeighbors(
		src.filterProtocolsBgp(birdProtocols), src.config)
	if err != nil {
		return nil, err
	}

	pipes := src.filterProtocolsPipe(
		birdProtocols)["protocols"].(map[string]interface{})
	tree := src.parseProtocolToTableTree(birdProtocols)

	// Now determine the session count for each neighbor and check if the pipe
	// did filter anything
	filtered := make(map[string]int)
	for table := range tree {
		allRoutesImported := int64(0)
		pipeRoutesImported := int64(0)

		// Sum up all routes from all peers for a table
		for _, protocol := range tree[table].(map[string]interface{}) {
			// Skip peers that are not up (start/down)
			if !isProtocolUp(protocol.(map[string]interface{})["state"].(string)) {
				continue
			}
			allRoutesImported += int64(protocol.(map[string]interface{})["routes"].(map[string]interface{})["imported"].(float64))

			pipeName := src.getMasterPipeName(table)

			if _, ok := pipes[pipeName]; ok {
				if _, ok := pipes[pipeName].(map[string]interface{})["routes"].(map[string]interface{})["imported"]; ok {
					pipeRoutesImported = int64(pipes[pipeName].(map[string]interface{})["routes"].(map[string]interface{})["imported"].(float64))
				} else {
					continue
				}
			} else {
				continue
			}
		}

		// If no routes were imported, there is nothing left to filter
		if allRoutesImported == 0 {
			continue
		}

		// If the pipe did not filter anything, there is nothing left to do
		if pipeRoutesImported == allRoutesImported {
			continue
		}

		if len(tree[table].(map[string]interface{})) == 1 {
			// Single router
			for _, protocol := range tree[table].(map[string]interface{}) {
				filtered[protocol.(map[string]interface{})["protocol"].(string)] = int(allRoutesImported - pipeRoutesImported)
			}
		} else {
			// Multiple routers
			if pipeRoutesImported == 0 {
				// 0 is a special condition, which means that the pipe did filter ALL routes of
				// all peers. Therefore we already know the amount of filtered routes and don't have
				// to query birdwatcher again.
				for _, protocol := range tree[table].(map[string]interface{}) {
					// Skip peers that are not up (start/down)
					if !isProtocolUp(protocol.(map[string]interface{})["state"].(string)) {
						continue
					}
					filtered[protocol.(map[string]interface{})["protocol"].(string)] = int(protocol.(map[string]interface{})["routes"].(map[string]interface{})["imported"].(float64))
				}
			} else {
				// Otherwise the pipe did import at least some routes which means that
				// we have to query birdwatcher to get the count for each peer.
				for neighborAddress, protocol := range tree[table].(map[string]interface{}) {
					table := protocol.(map[string]interface{})["table"].(string)
					pipe := src.getMasterPipeName(table)

					count, err := src.client.GetJson("/routes/pipe/filtered/count?table=" + table + "&pipe=" + pipe + "&address=" + neighborAddress)
					if err != nil {
						log.Println("WARNING Could not retrieve filtered routes count:", err)
						log.Println("Is the 'pipe_filtered_count' module active in birdwatcher?")
						return nil, err
					}

					if _, ok := count["routes"]; ok {
						filtered[protocol.(map[string]interface{})["protocol"].(string)] = int(count["routes"].(float64))
					}
				}
			}
		}
	}

	// Update the results with the information about filtered routes from the pipe
	for _, neighbor := range neighbors {
		if pipeRoutesFiltered, ok := filtered[neighbor.ID]; ok {
			neighbor.RoutesAccepted -= pipeRoutesFiltered
			neighbor.RoutesFiltered += pipeRoutesFiltered
		}
	}

	response = &api.NeighborsResponse{
		Response: api.Response{
			Meta: *apiStatus,
		},
		Neighbors: neighbors,
	}

	// Cache result
	src.neighborsCache.Set(response)

	return response, nil // dereference for now
}

// Routes gets filtered and exported route
// from the birdwatcher backend.
func (src *MultiTableBirdwatcher) Routes(
	neighbourID string,
) (*api.RoutesResponse, error) {
	response := &api.RoutesResponse{}
	// Fetch required routes first (received and filtered)
	// However: Store in separate cache for faster access
	required, err := src.fetchRequiredRoutes(neighbourID)
	if err != nil {
		return nil, err
	}

	// Optional: NoExport
	_, notExported, err := src.fetchNotExportedRoutes(neighbourID)
	if err != nil {
		return nil, err
	}

	response.Meta = required.Meta
	response.Imported = required.Imported
	response.Filtered = required.Filtered
	response.NotExported = notExported

	return response, nil
}

// RoutesReceived returns all received routes
func (src *MultiTableBirdwatcher) RoutesReceived(
	neighborID string,
) (*api.RoutesResponse, error) {
	response := &api.RoutesResponse{}

	// Check if we have a cache hit
	cachedRoutes := src.routesRequiredCache.Get(neighborID)
	if cachedRoutes != nil {
		response.Meta = cachedRoutes.Meta
		response.Imported = cachedRoutes.Imported
		return response, nil
	}

	// Fetch required routes first (received and filtered)
	routes, err := src.fetchRequiredRoutes(neighborID)
	if err != nil {
		return nil, err
	}

	response.Meta = routes.Meta
	response.Imported = routes.Imported

	return response, nil
}

// RoutesFiltered gets all filtered routes from the backend
func (src *MultiTableBirdwatcher) RoutesFiltered(
	neighborID string,
) (*api.RoutesResponse, error) {
	response := &api.RoutesResponse{}

	// Check if we have a cache hit
	cachedRoutes := src.routesRequiredCache.Get(neighborID)
	if cachedRoutes != nil {
		response.Meta = cachedRoutes.Meta
		response.Filtered = cachedRoutes.Filtered
		return response, nil
	}

	// Fetch required routes first (received and filtered)
	routes, err := src.fetchRequiredRoutes(neighborID)
	if err != nil {
		return nil, err
	}

	response.Meta = routes.Meta
	response.Filtered = routes.Filtered

	return response, nil
}

// RoutesNotExported gets all not exported routes
func (src *MultiTableBirdwatcher) RoutesNotExported(
	neighborID string,
) (*api.RoutesResponse, error) {
	// Check if we have a cache hit
	response := src.routesNotExportedCache.Get(neighborID)
	if response != nil {
		return response, nil
	}

	// Fetch not exported routes
	apiStatus, routes, err := src.fetchNotExportedRoutes(neighborID)
	if err != nil {
		return nil, err
	}

	response = &api.RoutesResponse{
		Response: api.Response{
			Meta: *apiStatus,
		},
		NotExported: routes,
	}

	// Cache result
	src.routesNotExportedCache.Set(neighborID, response)

	return response, nil
}

// AllRoutes retrieves a routes dump from the server
func (src *MultiTableBirdwatcher) AllRoutes() (*api.RoutesResponse, error) {
	// Query birdwatcher
	_, birdProtocols, err := src.fetchProtocols()
	if err != nil {
		return nil, err
	}
	mainTable := src.GenericBirdwatcher.config.MainTable

	// Fetch received routes first
	birdImported, err := src.client.GetJson("/routes/table/" + mainTable)
	if err != nil {
		return nil, err
	}

	// Use api status from first request
	apiStatus, err := parseAPIStatus(birdImported, src.config)
	if err != nil {
		return nil, err
	}

	response := &api.RoutesResponse{
		Response: api.Response{
			Meta: apiStatus,
		},
	}

	// Parse the routes
	imported := parseRoutesData(birdImported["routes"].([]interface{}), src.config)
	// Sort routes for deterministic ordering
	sort.Sort(imported)
	response.Imported = imported

	// Iterate over all the protocols and fetch the filtered routes for everyone
	protocolsBgp := src.filterProtocolsBgp(birdProtocols)
	for protocolID, protocolsData := range protocolsBgp["protocols"].(map[string]interface{}) {
		peer := protocolsData.(map[string]interface{})["neighbor_address"].(string)
		learntFrom := decoders.String(protocolsData.(map[string]interface{})["learnt_from"], peer)

		// Fetch filtered routes
		_, filtered, err := src.fetchFilteredRoutes(protocolID)
		if err != nil {
			continue
		}

		// Perform route deduplication
		filtered = src.filterRoutesByPeerOrLearntFrom(filtered, peer, learntFrom)
		response.Filtered = append(response.Filtered, filtered...)
	}

	return response, nil
}
