package switchclient_test

import (
	"testing"

	"github.com/t0mer/SwitchDeck/internal/switchclient"
	"github.com/t0mer/SwitchDeck/internal/switchclient/tplink"
)

func TestClientInterface(t *testing.T) {
	var _ switchclient.Client = tplink.New(true)
}
