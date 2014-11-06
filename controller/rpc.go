package main

import (
	"net/http"
	"time"

	ct "github.com/flynn/flynn/controller/types"
	"github.com/flynn/flynn/pkg/rpcplus"
	rpc "github.com/flynn/flynn/pkg/rpcplus/comborpc"
)

func rpcHandler(repo *FormationRepo) http.Handler {
	rpcplus.RegisterName("Controller", &ControllerRPC{formations: repo})
	return rpc.New(rpcplus.DefaultServer)
}

type ControllerRPC struct {
	formations *FormationRepo
}

func (s *ControllerRPC) StreamFormations(since time.Time, stream rpcplus.Stream) error {
	ch := make(chan *ct.ExpandedFormation)
	done := make(chan struct{})
	go func() {
	outer:
		for {
			select {
			case f := <-ch:
				select {
				case stream.Send <- f:
				case <-stream.Error:
					break outer
				}
			case <-stream.Error:
				break outer
			}
		}
		close(done)
	}()

	if err := s.formations.Subscribe(ch, since); err != nil {
		return err
	}
	defer func() {
		go func() {
			// drain to prevent deadlock while removing the listener
			for range ch {
			}
		}()
		s.formations.Unsubscribe(ch)
		close(ch)
	}()

	<-done
	return nil
}
