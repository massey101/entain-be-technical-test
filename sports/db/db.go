package db

import (
	"math/rand"
	"time"

	"syreclabs.com/go/faker"
)

// Randomly selects a home and away team making sure the same team isn't
// playing itself.
func select_away_and_home(sides []string) (home string, away string) {
	home = sides[rand.Intn(len(sides))]
	away = sides[rand.Intn(len(sides))]
	for away == home {
		away = sides[rand.Intn(len(sides))]
	}

	return home, away
}

func (r *eventsRepo) seed() error {
	statement, err := r.db.Prepare(`CREATE TABLE IF NOT EXISTS events (id INTEGER PRIMARY KEY, sport TEXT, league INTEGER, home_side_name TEXT, away_side_name TEXT, visible INTEGER, advertised_start_time DATETIME)`)
	if err == nil {
		_, err = statement.Exec()
	}

	// Pre-generate teams and players so that the same side can appear in
	// different events to test filtering.
	var football_teams []string
	var tennis_players []string
	var hockey_teams []string

	for i := 0; i < 10; i++ {
		football_teams = append(football_teams, faker.Team().Name())
		tennis_players = append(tennis_players, faker.Name().Name())
		hockey_teams = append(hockey_teams, faker.Team().Name())
	}

	for i := 1; i <= 100; i++ {
		statement, err = r.db.Prepare(`INSERT OR IGNORE INTO events(id, sport, league, home_side_name, away_side_name, visible, advertised_start_time) VALUES (?,?,?,?,?,?,?)`)
		if err == nil {
			var sport string = faker.RandomChoice([]string{"football", "tennis", "hockey"})
			var league, home_side_name, away_side_name string
			rand.Seed(time.Now().UnixNano())

			switch sport {
			case "football":
				league = faker.Number().Between(0, 9)
				home_side_name, away_side_name = select_away_and_home(football_teams)
			case "tennis":
				league = faker.Number().Between(10, 20)
				home_side_name, away_side_name = select_away_and_home(tennis_players)
			case "hockey":
				league = faker.Number().Between(20, 30)
				home_side_name, away_side_name = select_away_and_home(hockey_teams)
			}

			_, err = statement.Exec(
				i,
				sport,
				league,
				home_side_name,
				away_side_name,
				faker.Number().Between(0, 1),
				faker.Time().Between(time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 0, 2)).Format(time.RFC3339),
			)
		}
	}

	return err
}
