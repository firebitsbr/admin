package admin

import (
	"errors"
	"launchpad.net/mgo"
	"net/http"
	"strings"
)

//Admin is an http.Handler for serving up the admin pages
type Admin struct {
	Auth     AuthFunc          //If not nil, the AuthFunc is called on every request to determine if the request should be handled or not.
	Session  *mgo.Session      //Session is the mongo session with the databases and collections to be handled.
	Renderer Renderer          //If nil, a default renderer is used to render the admin pages.
	Routes   map[string]string //Routes lets you change the url paths for the admin. If nil, uses DefaultRoutes.

	//created on demand
	server      *http.ServeMux
	types       map[string]collectionInfo
	index_cache map[string][]string
}

//DefaultRoutes is the mapping of actions to url paths by default.
var DefaultRoutes = map[string]string{
	"index":  "/",
	"list":   "/list/",
	"update": "/update/",
	"create": "/create/",
	"detail": "/detail/",
}

//useful type because these get made so often
type d map[string]interface{}

//AuthFunc is a function used to determine if the request is authorized
type AuthFunc func(*http.Request) bool

//adminHandler is a type representing a handler function on an *Admin
type adminHandler func(*Admin, http.ResponseWriter, *http.Request)

//routes defines the mapping of type to function for the admin
var routes = map[string]adminHandler{
	"index":  (*Admin).index,
	"list":   (*Admin).list,
	"update": (*Admin).update,
	"create": (*Admin).create,
	"detail": (*Admin).detail,
}

//generateMux creates the internal http.ServeMux to dispatch reqeusts to the
//appropriate handler.
func (a *Admin) generateMux() {
	if a.server != nil {
		return
	}
	if a.Routes == nil {
		a.Routes = DefaultRoutes
	}

	a.server = http.NewServeMux()
	for key, path := range a.Routes {
		r, fn := path, routes[key]
		a.server.Handle(r, http.StripPrefix(r, a.bind(fn)))
	}
}

//generateIndexCache generates the values needed for IndexContext and stores
//them for efficient lookup.
func (a *Admin) generateIndexCache() {
	if a.index_cache != nil {
		return
	}

	a.index_cache = make(map[string][]string)
	for key := range a.types {
		pieces := strings.Split(key, ".")
		if _, ex := a.index_cache[pieces[0]]; ex {
			a.index_cache[pieces[0]] = append(a.index_cache[pieces[0]], pieces[1])
		} else {
			a.index_cache[pieces[0]] = []string{pieces[1]}
		}
	}
}

//bind turns an adminHandler into an http.HandlerFunc by closing on the admin
//value on the adminHandler.
func (a *Admin) bind(fn adminHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		fn(a, w, req)
	}
}

//Returns the mgo.Collection for the specified collection
func (a *Admin) collFor(dbcoll string) mgo.Collection {
	pieces := strings.Split(dbcoll, ".")
	return a.Session.DB(pieces[0]).C(pieces[1])
}

//ServeHTTP lets *Admin conform to the http.Handler interface for use in web servers
func (a *Admin) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if a.Renderer == nil {
		a.Renderer = defaultRenderer{}
	}

	if a.Auth != nil && !a.Auth(req) {
		a.Renderer.Unauthorized(w, req)
		return
	}

	//ensure a valid database
	if a.Session == nil {
		a.Renderer.InternalError(w, req, errors.New("Mongo session not configured"))
		return
	}

	//pass it off to our internal muxer
	a.generateMux()
	a.server.ServeHTTP(w, req)
}
