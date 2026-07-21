package clickup

import "context"

// Member è un membro di un workspace.
type Member struct {
	ID       int
	Username string
}

// Team è un workspace ClickUp (nell'API "team").
type Team struct {
	ID      string
	Name    string
	Members []Member
}

// Teams ritorna i workspace accessibili col token, con i loro membri.
func (c *Client) Teams(ctx context.Context) ([]Team, error) {
	var resp struct {
		Teams []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Members []struct {
				User struct {
					ID       int    `json:"id"`
					Username string `json:"username"`
				} `json:"user"`
			} `json:"members"`
		} `json:"teams"`
	}
	if err := c.get(ctx, "/team", nil, &resp); err != nil {
		return nil, err
	}
	teams := make([]Team, 0, len(resp.Teams))
	for _, t := range resp.Teams {
		team := Team{ID: t.ID, Name: t.Name}
		for _, m := range t.Members {
			team.Members = append(team.Members, Member{ID: m.User.ID, Username: m.User.Username})
		}
		teams = append(teams, team)
	}
	return teams, nil
}
