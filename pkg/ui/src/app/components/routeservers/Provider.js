/**
 * RouteServers provider
 *
 * This provider fetches all route servers and makes them
 * available through a context
 */

import axios from 'axios';

import { useState
       , useEffect
       , useContext
       , createContext
       , useRef
       }
  from 'react';

import { useErrors }
  from 'app/components/errors/Provider';


const RouteServersContext = createContext([]);

export const useRouteServers = () => useContext(RouteServersContext);


/**
 * RouteServersProvider loads the route servers from the
 * backend and uses these as provider value.
 */
const RouteServersProvider = ({children}) => {
  const init          = useRef();
  const [handleError] = useErrors();
  const [rs, setRs]   = useState([]);
  
  // Load route servers from backend
  useEffect(() => {
    if (init.current) {
      return;
    }
    axios.get('/api/v1/routeservers')
      .then(
        ({data}) => setRs(data.routeservers),
        (error) => handleError(error)
      );
      init.current = true;
  }, [handleError]);

  return (
    <RouteServersContext.Provider value={rs}>
      {children}
    </RouteServersContext.Provider>
  );
}

export default RouteServersProvider;
