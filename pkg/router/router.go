package router

import (
	"net/http"

	"ichenfu.com/rbd-api/pkg/handlers"

	"github.com/ceph/go-ceph/rados"
	"github.com/emicklei/go-restful"
	restfulspec "github.com/emicklei/go-restful-openapi"
)

func AddWebServices(conn *rados.Conn) error {
	rbdHandler := handlers.NewRbdHander(conn)
	restful.Add(rbdHandler.WebService())

	config := restfulspec.Config{
		WebServices: restful.RegisteredWebServices(), // you control what services are visible
		APIPath:     "/apidocs.json",
	}
	restful.Add(restfulspec.NewOpenAPIService(config))
	http.Handle("/apidocs/", http.StripPrefix("/apidocs/", http.FileServer(http.Dir("/tmp/swagger-ui/dist"))))

	// Optionally, you may need to enable CORS for the UI to work.
	cors := restful.CrossOriginResourceSharing{
		AllowedHeaders: []string{"Content-Type", "Accept"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		CookiesAllowed: false,
	}
	restful.Filter(cors.Filter)
	return nil
}
