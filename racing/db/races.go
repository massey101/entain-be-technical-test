package db

import (
	"database/sql"
	"github.com/golang/protobuf/ptypes"
	_ "github.com/mattn/go-sqlite3"
	"strings"
	"sync"
	"time"

	"git.neds.sh/matty/entain/racing/proto/racing"
)

// RacesRepo provides repository access to races.
type RacesRepo interface {
	// Init will initialise our races repository.
	Init() error

	// List will return a list of races.
	List(filter *racing.ListRacesRequestFilter, orderBy *string) ([]*racing.Race, error)
}

type racesRepo struct {
	db   *sql.DB
	init sync.Once
}

// NewRacesRepo creates a new races repository.
func NewRacesRepo(db *sql.DB) RacesRepo {
	return &racesRepo{db: db}
}

// Init prepares the race repository dummy data.
func (r *racesRepo) Init() error {
	var err error

	r.init.Do(func() {
		// For test/example purposes, we seed the DB with some dummy races.
		err = r.seed()
	})

	return err
}

func (r *racesRepo) List(filter *racing.ListRacesRequestFilter, orderBy *string) ([]*racing.Race, error) {
	var (
		err   error
		query string
		args  []interface{}
	)

	query = getRaceQueries()[racesList]

	query, args = r.applyFilter(query, filter)
	query = r.applyOrdering(query, orderBy)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}

	return r.scanRaces(rows)
}

func (r *racesRepo) applyFilter(query string, filter *racing.ListRacesRequestFilter) (string, []interface{}) {
	var (
		clauses []string
		args    []interface{}
	)

	if filter == nil {
		return query, args
	}

	if len(filter.MeetingIds) > 0 {
		clauses = append(clauses, "meeting_id IN ("+strings.Repeat("?,", len(filter.MeetingIds)-1)+"?)")

		for _, meetingID := range filter.MeetingIds {
			args = append(args, meetingID)
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
		"name":                  "name",
		"number":                "number",
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
func (r *racesRepo) applyOrdering(query string, orderBy *string) string {
	if orderBy == nil {
		return query
	}

	orderBySql := convertOrderByToSql(*orderBy)

	return query + orderBySql
}

func getRaceStatus(advertisedStart time.Time) string {
	if advertisedStart.Before(time.Now()) {
		return "CLOSED"
	}
	return "OPEN"
}

func (m *racesRepo) scanRaces(
	rows *sql.Rows,
) ([]*racing.Race, error) {
	var races []*racing.Race

	for rows.Next() {
		var race racing.Race
		var advertisedStart time.Time

		if err := rows.Scan(&race.Id, &race.MeetingId, &race.Name, &race.Number, &race.Visible, &advertisedStart); err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}

			return nil, err
		}

		ts, err := ptypes.TimestampProto(advertisedStart)
		if err != nil {
			return nil, err
		}

		race.AdvertisedStartTime = ts

		race.Status = getRaceStatus(advertisedStart)

		races = append(races, &race)
	}

	return races, nil
}
