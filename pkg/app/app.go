package app

import (
	"html/template"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"

	"github.com/jbeda/kuard/pkg/debugprobe"
	"github.com/jbeda/kuard/pkg/dnsapi"
	"github.com/jbeda/kuard/pkg/env"
	"github.com/jbeda/kuard/pkg/htmlutils"
	"github.com/jbeda/kuard/pkg/keygen"
	"github.com/jbeda/kuard/pkg/sitedata"
	"github.com/jbeda/kuard/pkg/version"
	"github.com/julienschmidt/httprouter"
)

func loggingMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}

type pageContext struct {
	Hostname     string       `json:"hostname"`
	Addrs        []string     `json:"addrs"`
	Version      string       `json:"version"`
	VersionColor template.CSS `json:"versionColor"`
	RequestDump  string       `json:"requestDump"`
	RequestProto string       `json:"requestProto"`
	RequestAddr  string       `json:"requestAddr"`
}

type App struct {
	c  Config
	tg *htmlutils.TemplateGroup

	kg *keygen.Workload
	r  *httprouter.Router
}

func (k *App) getPageContext(r *http.Request) *pageContext {
	c := &pageContext{}
	c.Hostname, _ = os.Hostname()

	addrs, _ := net.InterfaceAddrs()
	c.Addrs = []string{}
	for _, addr := range addrs {
		// check the address type and if it is not a loopback
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				c.Addrs = append(c.Addrs, ipnet.IP.String())
			}
		}
	}

	c.Version = version.VERSION
	c.VersionColor = template.CSS(htmlutils.ColorFromString(version.VERSION))
	reqDump, _ := httputil.DumpRequest(r, false)
	c.RequestDump = strings.TrimSpace(string(reqDump))
	c.RequestProto = r.Proto
	c.RequestAddr = r.RemoteAddr

	return c
}

func (k *App) rootHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	k.tg.Render(w, "index.html", k.getPageContext(r))
}

func (k *App) Run() {
	log.Printf("Serving on %v", k.c.ServeAddr)
	log.Fatal(http.ListenAndServe(k.c.ServeAddr, loggingMiddleware(k.r)))
}

func NewApp() *App {
	k := &App{
		tg: &htmlutils.TemplateGroup{},
		r:  httprouter.New(),
	}

	router := k.r

	// Add the root handler
	router.GET("/", k.rootHandler)

	// Add the static files
	sitedata.AddRoutes(router, "/built")
	sitedata.AddRoutes(router, "/static")

	router.Handler("GET", "/fs/*filepath", http.StripPrefix("/fs", http.FileServer(http.Dir("/"))))

	debugprobe.New("/healthy").AddRoutes(router)
	debugprobe.New("/ready").AddRoutes(router)
	env.New("/env").AddRoutes(router)
	dnsapi.New("/dns").AddRoutes(router)

	k.kg = keygen.New("/keygen")
	k.kg.AddRoutes(router)

	return k
}