package stream

import (
	acgv1 "github.com/p-/ai-credential-gateway/gen/acg/v1"
)

// Server implements the gRPC RequestStreamService.
type Server struct {
	acgv1.UnimplementedRequestStreamServiceServer
	hub *Hub
}

func NewServer(hub *Hub) *Server {
	return &Server{hub: hub}
}

func (s *Server) StreamRequests(filter *acgv1.StreamFilter, stream acgv1.RequestStreamService_StreamRequestsServer) error {
	sub := s.hub.Subscribe(filter)
	defer s.hub.Unsubscribe(sub)

	for {
		select {
		case ev, ok := <-sub.Events():
			if !ok {
				return nil
			}
			if err := stream.Send(ev); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}
