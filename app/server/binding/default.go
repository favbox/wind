package binding

import "sync"

type defaultBinder struct {
	config             *BindConfig
	decoderCache       sync.Map
	pathDecoderCache   sync.Map
	queryDecoderCache  sync.Map
	headerDecoderCache sync.Map
	formDecoderCache   sync.Map
}
