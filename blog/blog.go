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

func (srv *Blog) AddRoutes() error {
	// ensureTables()
	return nil
}
