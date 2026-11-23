package boundary

type T struct{ n int }

// This file is not strict, so every pointer type written here is nullable.

var (
	NS  []*T
	NA  [2]*T
	NM  map[string]*T
	NC  chan *T
	NPP **T
	NB  Box[*T]
)

type Box[E any] struct{ v E }

// Emb embeds a nullable pointer.
type Emb struct{ *T }

var NE Emb

// Function and method types of this file take and return nullable pointers.

var (
	NGet func() *T
	NPut func(*T)
)

type Loose struct{}

func (Loose) Get() *T { return nil }
