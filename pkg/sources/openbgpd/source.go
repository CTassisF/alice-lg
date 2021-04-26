package openbgpd

import (
	"context"
	"net/http"
	"time"

	"github.com/alice-lg/alice-lg/pkg/api"
	"github.com/alice-lg/alice-lg/pkg/decoders"
)

const (
	// SourceVersion is currently fixed at 1.0
	SourceVersion = "1.0"
)

// Source implements the OpenBGPD source for Alice.
// It is intendet to consume structured bgpctl output
// queried over HTTP using a `openbgpd-state-server`.
type Source struct {
	// API is the http host and api prefix. For
	// example http://rs1.mgmt.ixp.example.net:29111/api
	API string
}

// ExpireCaches expires all cached data
func (src *Source) ExpireCaches() int {
	return 0 // Nothing to expire yet
}

// Status returns an API status response. In our case
// this is pretty much only that the service is available.
func (src *Source) Status() (*api.StatusResponse, error) {
	apiStatus := api.ApiStatus{
		Version:         SourceVersion,
		ResultFromCache: false,
		Ttl:             time.Now().UTC(),
	}

	// Make API request and read response
	req, err := StatusRequest(context.Background(), src)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := decoders.ReadJSONResponse(res)
	if err != nil {
		return nil, err
	}

	status := decodeAPIStatus(body)

	response := &api.StatusResponse{
		Api:    apiStatus,
		Status: status,
	}
	return response, nil
}

// Neighbours retrievs a full list of all neighbors
func (src *Source) Neighbours() (*api.NeighboursResponse, error) {
	// Retrieve neighbours
	apiStatus := api.ApiStatus{
		Version:         SourceVersion,
		ResultFromCache: false,
		Ttl:             time.Now().UTC(),
	}

	// Make API request and read response
	req, err := ShowNeighborsRequest(context.Background(), src)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := decoders.ReadJSONResponse(res)
	if err != nil {
		return nil, err
	}

	nb, err := decodeNeighbors(body)
	if err != nil {
		return nil, err
	}

	response := &api.NeighboursResponse{
		Api:        apiStatus,
		Neighbours: nb,
	}
	return response, nil
}

// NeighboursStatus retrives the status summary
// for all neightbors
func (src *Source) NeighboursStatus() (*api.NeighboursStatusResponse, error) {
	// Retrieve neighbours
	apiStatus := api.ApiStatus{
		Version:         SourceVersion,
		ResultFromCache: false,
		Ttl:             time.Now().UTC(),
	}

	// Make API request and read response
	req, err := ShowNeighborsSummaryRequest(context.Background(), src)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Read and decode response
	body, err := decoders.ReadJSONResponse(res)
	if err != nil {
		return nil, err
	}

	nb, err := decodeNeighborsStatus(body)
	if err != nil {
		return nil, err
	}

	response := &api.NeighboursStatusResponse{
		Api:        apiStatus,
		Neighbours: nb,
	}
	return response, nil
}

// Routes retrieves the routes for a specific neighbor
// identified by ID.
func (src *Source) Routes(neighborID string) (*api.RoutesResponse, error) {
	// Retrieve routes for the specific neighbor
	apiStatus := api.ApiStatus{
		Version:         SourceVersion,
		ResultFromCache: false,
		Ttl:             time.Now().UTC(),
	}

	// Query RIB for routes received
	req, err := ShowNeighborRIBInRequest(context.Background(), src, neighborID)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Read and decode response
	body, err := decoders.ReadJSONResponse(res)
	if err != nil {
		return nil, err
	}

	recv, err := decodeRoutes(body)
	if err != nil {
		return nil, err
	}

	response := &api.RoutesResponse{
		Api:      apiStatus,
		Imported: recv,
	}
	return response, nil
}

// RoutesReceived returns the routes exported by the neighbor.
func (src *Source) RoutesReceived(neighborID string) (*api.RoutesResponse, error) {
	// Retrieve routes for the specific neighbor
	apiStatus := api.ApiStatus{
		Version:         SourceVersion,
		ResultFromCache: false,
		Ttl:             time.Now().UTC(),
	}

	// Query RIB for routes received
	req, err := ShowNeighborRIBInRequest(context.Background(), src, neighborID)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Read and decode response
	body, err := decoders.ReadJSONResponse(res)
	if err != nil {
		return nil, err
	}

	recv, err := decodeRoutes(body)
	if err != nil {
		return nil, err
	}

	response := &api.RoutesResponse{
		Api:         apiStatus,
		Imported:    recv,
		NotExported: api.Routes{},
	}
	return response, nil
}

// RoutesFiltered retrieves the routes filtered / not valid
func (src *Source) RoutesFiltered(neighborID string) (*api.RoutesResponse, error) {
	apiStatus := api.ApiStatus{
		Version:         SourceVersion,
		ResultFromCache: false,
		Ttl:             time.Now().UTC(),
	}
	response := &api.RoutesResponse{
		Api: apiStatus,
	}
	return response, nil
}

// RoutesNotExported retrievs the routes not exported
// from the rs for a neighbor.
func (src *Source) RoutesNotExported(neighborID string) (*api.RoutesResponse, error) {
	apiStatus := api.ApiStatus{
		Version:         SourceVersion,
		ResultFromCache: false,
		Ttl:             time.Now().UTC(),
	}
	response := &api.RoutesResponse{
		Api: apiStatus,
	}
	return response, nil
}

// AllRoutes retrievs the entire RIB from the source
func (src *Source) AllRoutes() (*api.RoutesResponse, error) {
	apiStatus := api.ApiStatus{
		Version:         SourceVersion,
		ResultFromCache: false,
		Ttl:             time.Now().UTC(),
	}

	req, err := ShowRIBRequest(context.Background(), src)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Read and decode response
	body, err := decoders.ReadJSONResponse(res)
	if err != nil {
		return nil, err
	}

	recv, err := decodeRoutes(body)
	if err != nil {
		return nil, err
	}

	response := &api.RoutesResponse{
		Api:         apiStatus,
		Imported:    recv,
		NotExported: api.Routes{},
	}
	return response, nil
}
