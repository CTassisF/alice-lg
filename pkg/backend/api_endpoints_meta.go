package backend

import (
	"net/http"

	"github.com/julienschmidt/httprouter"

	"github.com/alice-lg/alice-lg/pkg/api"
)

// Handle Status Endpoint, this is intended for
// monitoring and service health checks
func apiStatusShow(_req *http.Request, _params httprouter.Params) (api.Response, error) {
	status, err := NewAppStatus()
	return status, err
}

// Handle status
func apiStatus(_req *http.Request, params httprouter.Params) (api.Response, error) {
	rsId, err := validateSourceID(params.ByName("id"))
	if err != nil {
		return nil, err
	}

	source := AliceConfig.SourceInstanceById(rsId)
	if source == nil {
		return nil, SOURCE_NOT_FOUND_ERROR
	}

	result, err := source.Status()
	if err != nil {
		apiLogSourceError("status", rsId, err)
	}

	return result, err
}

// Handle Config Endpoint
func apiConfigShow(_req *http.Request, _params httprouter.Params) (api.Response, error) {
	result := api.ConfigResponse{
		Asn:            AliceConfig.Server.Asn,
		BgpCommunities: AliceConfig.Ui.BgpCommunities,
		RejectReasons:  AliceConfig.Ui.RoutesRejections.Reasons,
		Noexport: api.Noexport{
			LoadOnDemand: AliceConfig.Ui.RoutesNoexports.LoadOnDemand,
		},
		NoexportReasons: AliceConfig.Ui.RoutesNoexports.Reasons,
		RejectCandidates: api.RejectCandidates{
			Communities: AliceConfig.Ui.RoutesRejectCandidates.Communities,
		},
		Rpki:                   api.Rpki(AliceConfig.Ui.Rpki),
		RoutesColumns:          AliceConfig.Ui.RoutesColumns,
		RoutesColumnsOrder:     AliceConfig.Ui.RoutesColumnsOrder,
		NeighboursColumns:      AliceConfig.Ui.NeighboursColumns,
		NeighboursColumnsOrder: AliceConfig.Ui.NeighboursColumnsOrder,
		LookupColumns:          AliceConfig.Ui.LookupColumns,
		LookupColumnsOrder:     AliceConfig.Ui.LookupColumnsOrder,
		PrefixLookupEnabled:    AliceConfig.Server.EnablePrefixLookup,
	}
	return result, nil
}
