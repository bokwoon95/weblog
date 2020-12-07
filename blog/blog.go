package blog

import "github.com/bokwoon95/weblog/pagemanager"

type Blog struct {
	*pagemanager.PageManager
	namespace string
}

func New(namespace string) func(*pagemanager.PageManager) pagemanager.Plugin {
	return func(server *pagemanager.PageManager) pagemanager.Plugin {
		return &Blog{
			PageManager: server,
		}
	}
}

// dumps a form out for pagemanager to show to the user; must be completed before the rest of the routes can be set up.
func (srv *Blog) Config() error {
	return nil
}

func (srv *Blog) AddRoutes() error {
	// ensureTables()
	return nil
}
