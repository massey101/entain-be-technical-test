package db

import (
	"database/sql"
	"fmt"
	"github.com/golang/protobuf/ptypes"
	_ "github.com/mattn/go-sqlite3"
	"strings"
	"sync"
	"time"

	"git.neds.sh/jmassey/entain/sports/proto/sports"
)

// EventsRepo provides repository access to events.
type EventsRepo interface {
	// Init will initialise our events repository.
	Init() error

	// List will return a list of events.
	List(filter *sports.ListEventsRequestFilter, orderBy *string) ([]*sports.Event, error)
	// Get will return an event by ID.
	Get(id int64) (*sports.Event, error)
}

type eventsRepo struct {
	db   *sql.DB
	init sync.Once
}

// NewEventsRepo creates a new events repository.
func NewEventsRepo(db *sql.DB) EventsRepo {
	return &eventsRepo{db: db}
}

// Init prepares the event repository dummy data.
func (r *eventsRepo) Init() error {
	var err error

	r.init.Do(func() {
		// For test/example purposes, we seed the DB with some dummy events.
		err = r.seed()
	})

	return err
}

func (r *eventsRepo) List(filter *sports.ListEventsRequestFilter, orderBy *string) ([]*sports.Event, error) {
	var (
		err   error
		query string
		args  []interface{}
	)

	query = getEventQueries()[eventsList]

	query, args = r.applyFilter(query, filter)
	query = r.applyOrdering(query, orderBy)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}

	return r.scanEvents(rows)
}

func (r *eventsRepo) Get(id int64) (*sports.Event, error) {
	// Repurpose listing functionality with an additional filter for
	// consistancy.
	filter := sports.ListEventsRequestFilter{Ids: []int64{id}}
	events, err := r.List(&filter, nil)
	if err != nil {
		return nil, err
	}
	if len(events) != 1 {
		// From the uber style guide fmt.Errorf is appropriate
		return nil, fmt.Errorf("no event with id: %v", id)
	}
	return events[0], nil
}

func (r *eventsRepo) applyFilter(query string, filter *sports.ListEventsRequestFilter) (string, []interface{}) {
	var (
		clauses []string
		args    []interface{}
	)

	if filter == nil {
		return query, args
	}

	if len(filter.Sports) > 0 {
		clauses = append(clauses, "sport IN ("+strings.Repeat("?,", len(filter.Sports)-1)+"?)")

		for _, sport := range filter.Sports {
			args = append(args, sport)
		}
	}

	if len(filter.Leagues) > 0 {
		clauses = append(clauses, "league IN ("+strings.Repeat("?,", len(filter.Leagues)-1)+"?)")

		for _, league := range filter.Leagues {
			args = append(args, league)
		}
	}

	if len(filter.Sides) > 0 {
		clauses = append(clauses, "(home_side_name IN ("+strings.Repeat("?,", len(filter.Sides)-1)+"?) OR away_side_name IN ("+strings.Repeat("?,", len(filter.Sides)-1)+"?))")

		for _, side := range filter.Sides {
			args = append(args, side)
		}
		for _, side := range filter.Sides {
			args = append(args, side)
		}
	}

	if len(filter.Ids) > 0 {
		clauses = append(clauses, "id IN ("+strings.Repeat("?,", len(filter.Ids)-1)+"?)")

		for _, ID := range filter.Ids {
			args = append(args, ID)
		}
	}

	if filter.Visible != nil {
		clauses = append(clauses, "visible = ?")
		args = append(args, *filter.Visible)
	}

	if len(clauses) != 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	return query, args
}

func convertOrderByFieldToSql(orderByField string) (orderByFieldSql string, ok bool) {
	// Important to verify against allowed field names to protect from SQL
	// injection.
	sortableFields := map[string]string{
		"home_side_name":        "home_side_name",
		"away_side_name":        "away_side_name",
		"league":                "league",
		"sport":                 "sport",
		"advertised_start_time": "advertised_start_time",
	}

	orderByFieldSplit := strings.Fields(orderByField)
	if len(orderByFieldSplit) > 2 {
		return "", false
	}
	field := orderByFieldSplit[0]
	databaseField, ok := sortableFields[field]
	if !ok {
		return "", false
	}
	orderByFieldSql += databaseField
	if len(orderByFieldSplit) == 2 {
		desc := strings.ToLower(orderByFieldSplit[1])
		if strings.Contains(desc, "desc") {
			orderByFieldSql += " DESC"
		}
	}
	return orderByFieldSql, true
}

func convertOrderByToSql(orderBy string) (orderBySql string) {
	var sqls []string
	orderBySql = ""

	for _, orderByField := range strings.Split(orderBy, ",") {
		orderByField = strings.TrimSpace(orderByField)
		if orderByFieldSql, ok := convertOrderByFieldToSql(orderByField); ok {
			sqls = append(sqls, orderByFieldSql)
		}
	}
	if len(sqls) != 0 {
		orderBySql = " ORDER BY " + strings.Join(sqls, ", ")
	}
	return orderBySql
}

// If specified this will apply the ordering specified in the request to the
// SQL SELECT query. The format of the order_by in the query is a comma
// seperated list of fields with "desc" as a suffix to change the ordering.
// e.g. "advertised_start_time, name desc"
// https://cloud.google.com/apis/design/design_patterns#sorting_order
func (r *eventsRepo) applyOrdering(query string, orderBy *string) string {
	if orderBy == nil {
		return query
	}

	orderBySql := convertOrderByToSql(*orderBy)

	return query + orderBySql
}

func getEventStatus(advertisedStart time.Time) string {
	if advertisedStart.Before(time.Now()) {
		return "CLOSED"
	}
	return "OPEN"
}

func (m *eventsRepo) scanEvents(
	rows *sql.Rows,
) ([]*sports.Event, error) {
	var events []*sports.Event

	for rows.Next() {
		var event sports.Event
		var advertisedStart time.Time

		if err := rows.Scan(&event.Id, &event.Sport, &event.League, &event.HomeSideName, &event.AwaySideName, &event.Visible, &advertisedStart); err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}

			return nil, err
		}

		event.Name = event.HomeSideName + " vs " + event.AwaySideName

		ts, err := ptypes.TimestampProto(advertisedStart)
		if err != nil {
			return nil, err
		}

		event.AdvertisedStartTime = ts

		event.Status = getEventStatus(advertisedStart)

		events = append(events, &event)
	}

	return events, nil
}
