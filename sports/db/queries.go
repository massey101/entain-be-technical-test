package db

const (
	eventsList = "list"
)

func getEventQueries() map[string]string {
	return map[string]string{
		eventsList: `
			SELECT 
				id, 
				sport, 
				league, 
				home_side_name, 
				away_side_name,
				visible, 
				advertised_start_time 
			FROM events
		`,
	}
}
