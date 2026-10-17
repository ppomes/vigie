// Package cachetapi serves a Cachet 2.x compatible API under /cachet/api/v1,
// so tools written for Cachet (monitoring scripts, the Python cachet-client)
// work with Vigie by changing their base URL.
//
// Reads query the database directly and render Cachet's JSON. Writes are
// translated into Vigie requests and handed to the native handlers, so the
// business rules (component statuses, maintenances, notifications) live in
// one place.
package cachetapi
