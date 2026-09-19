package memstore

import (
	"testing"

	"github.com/msaliheroglu/tekses/services/control-api/internal/store"
	"github.com/msaliheroglu/tekses/services/control-api/internal/store/storetest"
)

func TestConformance(t *testing.T) {
	storetest.Run(t, func(*testing.T) store.Store { return New() })
}
