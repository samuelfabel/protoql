package catalog

import _ "embed"

// SDL is the GraphQL schema of the catalog fixture.
//
//go:embed catalog.graphql
var SDL string
