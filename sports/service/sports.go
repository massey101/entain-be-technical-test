package service

import (
	"git.neds.sh/jmassey/entain/sports/db"
	"git.neds.sh/jmassey/entain/sports/proto/sports"
	"golang.org/x/net/context"
)

type Sports interface {
	// ListEvents will return a collection of events.
	ListEvents(ctx context.Context, in *sports.ListEventsRequest) (*sports.ListEventsResponse, error)
	GetEvent(ctx context.Context, in *sports.GetEventRequest) (*sports.Event, error)
}

// sportsService implements the Sports interface.
type sportsService struct {
	eventsRepo db.EventsRepo
}

// NewSportsService instantiates and returns a new sportsService.
func NewSportsService(eventsRepo db.EventsRepo) Sports {
	return &sportsService{eventsRepo}
}

func (s *sportsService) ListEvents(ctx context.Context, in *sports.ListEventsRequest) (*sports.ListEventsResponse, error) {
	events, err := s.eventsRepo.List(in.Filter, in.OrderBy)
	if err != nil {
		return nil, err
	}

	return &sports.ListEventsResponse{Events: events}, nil
}

func (s *sportsService) GetEvent(ctx context.Context, in *sports.GetEventRequest) (*sports.Event, error) {
	event, err := s.eventsRepo.Get(in.Id)
	if err != nil {
		return nil, err
	}

	return event, nil
}
