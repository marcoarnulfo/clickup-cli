package clickup

import "context"

// Space is a ClickUp space within a workspace.
type Space struct {
	ID   string
	Name string
}

// List is a ClickUp list (the leaf that holds tasks/time).
type List struct {
	ID   string
	Name string
}

// Folder is a ClickUp folder within a space; its lists are returned inline.
type Folder struct {
	ID    string
	Name  string
	Lists []List
}

// Spaces returns the spaces of a workspace (GET /team/{id}/space).
func (c *Client) Spaces(ctx context.Context, teamID string) ([]Space, error) {
	var resp struct {
		Spaces []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"spaces"`
	}
	if err := c.get(ctx, "/team/"+teamID+"/space", nil, &resp); err != nil {
		return nil, err
	}
	out := make([]Space, 0, len(resp.Spaces))
	for _, s := range resp.Spaces {
		out = append(out, Space{ID: s.ID, Name: s.Name})
	}
	return out, nil
}

// SpaceContents returns a space's folders (with their lists inline) and its
// folderless lists (GET /space/{id}/folder + GET /space/{id}/list).
func (c *Client) SpaceContents(ctx context.Context, spaceID string) ([]Folder, []List, error) {
	var fresp struct {
		Folders []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Lists []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"lists"`
		} `json:"folders"`
	}
	if err := c.get(ctx, "/space/"+spaceID+"/folder", nil, &fresp); err != nil {
		return nil, nil, err
	}
	folders := make([]Folder, 0, len(fresp.Folders))
	for _, f := range fresp.Folders {
		lists := make([]List, 0, len(f.Lists))
		for _, l := range f.Lists {
			lists = append(lists, List{ID: l.ID, Name: l.Name})
		}
		folders = append(folders, Folder{ID: f.ID, Name: f.Name, Lists: lists})
	}

	var lresp struct {
		Lists []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"lists"`
	}
	if err := c.get(ctx, "/space/"+spaceID+"/list", nil, &lresp); err != nil {
		return nil, nil, err
	}
	folderless := make([]List, 0, len(lresp.Lists))
	for _, l := range lresp.Lists {
		folderless = append(folderless, List{ID: l.ID, Name: l.Name})
	}
	return folders, folderless, nil
}
