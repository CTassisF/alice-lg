
import axios from 'axios';

import { createContext
       , useContext
       , useState
       , useEffect
       , useMemo
       }
  from 'react';

import { useErrors }
  from 'app/context/errors';
import { ApiStatusProvider }
  from 'app/context/api-status';

import { isUpState } 
  from 'app/components/neighbors/state';

const initialState = {
  neighbors: [],
  api: {},
  isLoading: true,
};

// Contexts
const NeighborsContext = createContext();
const NeighborContext  = createContext();

export const useNeighbors = () => useContext(NeighborsContext);
export const useNeighbor  = () => useContext(NeighborContext);


/**
 * useLocalRelatedPeers returns all neighbors on an rs
 * sharing the same ASN and are in state 'up'
 */
export const useLocalRelatedPeers = () => {
  const { neighbors } = useNeighbors();
  const neighbor  = useNeighbor();
  return useMemo(() => {
    if (!neighbor) {
      return [];
    }
    return neighbors.filter((n) =>
      (n.asn === neighbor.asn && isUpState(n.state)));
  }, [neighbors, neighbor])
}

/**
 * NeighborsProvider loads the neighbors for a selected
 * route server identified by id
 */
export const NeighborsProvider = ({children, routeServerId}) => {
  const [handleError]     = useErrors();
  const [state, setState] = useState(initialState);

  useEffect(() => {
    setState((s) => ({...s, isLoading: true}));
    // Load RouteServer's neighbors
    axios.get(`/api/v1/routeservers/${routeServerId}/neighbors`).then(
      ({data}) => {
        setState({
          isLoading: false,
          neighbors: data.neighbors,
          api: data.api,
        });
      },
      (error) => {
        handleError(error);
        setState((s) => ({...s, isLoading: false}));
      }
    );
  }, [routeServerId, handleError]);

  return (
    <NeighborsContext.Provider value={state}>
      <ApiStatusProvider api={state.api}>
        {children}
      </ApiStatusProvider>
    </NeighborsContext.Provider>
  );
}

/**
 * NeighborProvider provides a single neighbor context
 */
export const NeighborProvider = ({neighborId, children}) => {
  const { neighbors } = useNeighbors();
  const neighbor = useMemo(
    () => neighbors.find((n) => n.id === neighborId),
    [neighbors, neighborId]);
  return (
    <NeighborContext.Provider value={neighbor}>
      {children}
    </NeighborContext.Provider>
  );
};
