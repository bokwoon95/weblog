package blog

import "github.com/bokwoon95/weblog/pagemanager"

type Server struct {
	*pagemanager.Server
	namespace string
}

func New(namespace string) func(*pagemanager.Server) pagemanager.Plugin {
	return func(server *pagemanager.Server) pagemanager.Plugin {
		return &Server{
			Server: server,
		}
	}
}

func (srv *Server) AddRoutes() error {
	// ensureTables()
	return nil
}
