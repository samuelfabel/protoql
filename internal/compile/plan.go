package compile

// BindingKind classifies how a requested field is obtained.
type BindingKind string

const (
	KindDirect    BindingKind = "direct"
	KindPath      BindingKind = "path"
	KindList      BindingKind = "list"
	KindDerived   BindingKind = "derived"
	KindAggregate BindingKind = "aggregate"
)

// FieldBinding is one slot of the projection, in document order.
type FieldBinding struct {
	Index       int            `json:"index"`
	Name        string         `json:"name"`
	Kind        BindingKind    `json:"kind"`
	TypeName    string         `json:"typeName,omitempty"`
	Nullable    bool           `json:"nullable,omitempty"`
	Path        []string       `json:"path,omitempty"`
	Expr        string         `json:"expr,omitempty"`
	Aggregate   string         `json:"aggregate,omitempty"`
	AggregateOf string         `json:"aggregateOf,omitempty"`
	Children    []FieldBinding `json:"children,omitempty"`
}

// ProjectionPlan is the compiler output: root GraphQL field + bindings.
type ProjectionPlan struct {
	RootField string         `json:"rootField"`
	Bindings  []FieldBinding `json:"bindings"`
}
