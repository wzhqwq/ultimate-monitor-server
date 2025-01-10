package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

var devServer = "http://localhost:5173"

func proxy(c *gin.Context) {
	remote, err := url.Parse(devServer)
	if err != nil {
		panic(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)
	proxy.Director = func(req *http.Request) {
		req.Header = c.Request.Header
		req.Host = remote.Host
		req.URL.Scheme = remote.Scheme
		req.URL.Host = remote.Host
		req.URL.Path = c.Param("proxyPath")
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}

var args struct {
	Port        int    `arg:"-p,--port,help:port to listen on" default:"8080"`
	StaticPath  string `arg:"-s,--static-path,help:directory to serve static files from" default:"../ultimate-monitor/dist"`
	Dev         bool   `arg:"-d,--dev-mode,help:run in development mode" default:"false"`
	DevPort     int    `arg:"--dev-port,help:port to run the development server on" default:"5173"`
	ProjectPath string `arg:"-b,--project-path,help:project path" default:"."`
}

func main() {
	arg.MustParse(&args)

	apiEngine := gin.New()
	InitApi(apiEngine)
	InitProject(args.ProjectPath)

	r := gin.New()
	r.Use(cors.New(cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		AllowAllOrigins:  true,
	}))

	var wildcardHandler gin.HandlerFunc
	if args.Dev {
		wildcardHandler = proxy
	} else {
		fileServer := http.FileServer(http.Dir(args.StaticPath))
		wildcardHandler = func(c *gin.Context) {
			fileServer.ServeHTTP(c.Writer, c.Request)
		}
	}

	r.Any("/*proxyPath", func(c *gin.Context) {
		path := c.Param("proxyPath")
		if strings.HasPrefix(path, "/api") {
			apiEngine.HandleContext(c)
		} else {
			wildcardHandler(c)
		}
	})

	r.Run(":" + strconv.Itoa(args.Port))
}
