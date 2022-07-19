/*
 * Communities helper
 */

/* 
 * Communities are represented as a nested object:
 *     {
 *         1234: {
 *            23: "community-leaf",
 *            42: { 
 *              1: "large-community-leaf"
 *            }
 *     }
 */

/*
 * Resolve a community description from the above described
 * tree structure.
 */
export const resolveCommunity = (base, community) => {
  let lookup = base;
  for (const part of community) {
    if (typeof(lookup) !== "object") {
      return null;
    }
    let res = lookup[part];
    if (!res) {
      // Try the wildcard
      if (lookup["*"]) {
        res = lookup["*"]
      } else {
        return null; // We did everything we could
      }
    }
    lookup = res;
  }
  return lookup;
}

/*
 * Resolve all communities
 */
export const resolveCommunities = (base, communities) => {
  let results = [];
  for (const c of communities) {
    const description = resolveCommunity(base, c);
    if (description != null) {
      results.push([c, description]);
    }
  }
  return results;
}


/*
 * Reject candidate helpers:
 * 
 *  - check if prefix is a reject candidate
 *  - make css classes
 */

export const isRejectCandidate = (rejectCommunities, route) => {
  // Check if any reject candidate community is set
  const largeCommunities = route.bgp.large_communities;
  const resolved = resolveCommunities(
    rejectCommunities, largeCommunities
  );

  return (resolved.length > 0);
}

/*
 * Expand variables in string:
 *    "Test AS$0 rejects $2"
 * will expand with [23, 42, 123] to
 *    "Test AS23 rejects 123"
 */
export const expandVars = (str, vars) => {
  if (!str) {
    return str; // We don't have to do anything.
  }
  let res = str;
  vars.map((v, i) => {
    res = res.replace(`$${i}`, v); 
    return 0;
  });
  return res;
}

export const makeReadableCommunity = (communities, community) => {
    const label = resolveCommunity(communities, community);
    return expandVars(label, community);
}

export const communityRepr = (community) => {
  return community.join(":");
}

